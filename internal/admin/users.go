package admin

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/notification"
	"retrotide/internal/repository"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
)

const (
	maxEmailLength  = 254
	verificationTTL = 24 * time.Hour

	verifyEmailPage    = "verify-email"
	forgotPasswordPage = "forgot-password"
)

func (s *service) ListUsers(ctx context.Context, listing spec.UserListing) (*dto.AdminUserListResponse, error) {
	limit, offset := clampPage(listing.Limit, listing.Offset, defaultUserPageSize)
	page := spec.UserListing{Search: strings.TrimSpace(listing.Search), Limit: limit, Offset: offset}

	rows, err := s.users.ListWithRoles(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	total, err := s.users.Count(ctx, page.Search)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	listed := make([]dto.AdminUserResponse, 0, len(rows))
	for _, row := range rows {
		listed = append(listed, dto.NewAdminUserResponse(row))
	}

	return &dto.AdminUserListResponse{Users: listed, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *service) GetUser(ctx context.Context, userID uuid.UUID) (*dto.AdminUserDetailResponse, error) {
	found, err := s.lookUp(ctx, userID)
	if err != nil {
		return nil, err
	}

	current, err := s.authz.GetRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	return new(dto.NewAdminUserDetailResponse(model.UserWithRole{User: *found, Role: current})), nil
}

func (s *service) SetUserRole(ctx context.Context, change spec.StaffRoleChange) error {
	if !change.Role.IsSiteStaff() {
		return ErrUnknownRole
	}

	current, err := s.authorise(ctx, change.ActorID, change.UserID)
	if err != nil {
		return err
	}

	if !current.MayChangeTo(hierarchy.Holder{Site: change.Role}) {
		return ErrRoleOutranksActor
	}

	err = s.roles.SetRole(ctx, spec.RoleAssignment{UserID: change.UserID, Role: change.Role})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil && !errors.Is(err, repository.ErrStaleStaffCache) {
		return fmt.Errorf("give user %s the %s role: %w", change.UserID, change.Role, err)
	}

	details := map[string]string{"role": string(change.Role)}
	if current.Target.Site != "" {
		details["previous_role"] = string(current.Target.Site)
	}

	return s.recordStaffChange(ctx, err, userEntry(change.StaffAction, audit.ActionSetRole, change.UserID, details))
}

func (s *service) RemoveUserRole(ctx context.Context, change spec.StaffRoleChange) error {
	if !change.Role.IsSiteStaff() {
		return ErrUnknownRole
	}

	if _, err := s.authorise(ctx, change.ActorID, change.UserID); err != nil {
		return err
	}

	err := s.roles.RemoveRole(ctx, spec.RoleAssignment{UserID: change.UserID, Role: change.Role})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrRoleNotHeld
	}
	if err != nil && !errors.Is(err, repository.ErrStaleStaffCache) {
		return fmt.Errorf("remove the %s role from user %s: %w", change.Role, change.UserID, err)
	}

	return s.recordStaffChange(ctx, err, userEntry(change.StaffAction, audit.ActionRemoveRole, change.UserID, map[string]string{"role": string(change.Role)}))
}

func (s *service) BanUser(ctx context.Context, restriction spec.StaffUserRestriction) error {
	reason := strings.TrimSpace(restriction.Reason)
	if reason == "" {
		return ErrReasonRequired
	}

	if _, err := s.authorise(ctx, restriction.ActorID, restriction.UserID); err != nil {
		return err
	}

	err := s.users.BanAccount(ctx, spec.UserBan{UserID: restriction.UserID, BannedBy: restriction.ActorID, Reason: reason})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("ban user %s: %w", restriction.UserID, err)
	}

	s.sessions.Disconnect(restriction.UserID)

	return s.record(ctx, userEntry(restriction.StaffAction, audit.ActionBanUser, restriction.UserID, map[string]string{"reason": reason}))
}

func (s *service) UnbanUser(ctx context.Context, action spec.StaffUserAction) error {
	if _, err := s.authorise(ctx, action.ActorID, action.UserID); err != nil {
		return err
	}

	err := s.users.Unban(ctx, action.UserID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("unban user %s: %w", action.UserID, err)
	}

	return s.record(ctx, userEntry(action.StaffAction, audit.ActionUnbanUser, action.UserID, nil))
}

func (s *service) LockUser(ctx context.Context, restriction spec.StaffUserRestriction) error {
	reason := strings.TrimSpace(restriction.Reason)
	if reason == "" {
		return ErrReasonRequired
	}

	if _, err := s.authorise(ctx, restriction.ActorID, restriction.UserID); err != nil {
		return err
	}

	err := s.users.Lock(ctx, spec.UserLock{UserID: restriction.UserID, LockedBy: restriction.ActorID, Reason: reason})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("lock user %s: %w", restriction.UserID, err)
	}

	return s.record(ctx, userEntry(restriction.StaffAction, audit.ActionLockUser, restriction.UserID, map[string]string{"reason": reason}))
}

func (s *service) UnlockUser(ctx context.Context, action spec.StaffUserAction) error {
	if _, err := s.authorise(ctx, action.ActorID, action.UserID); err != nil {
		return err
	}

	err := s.users.Unlock(ctx, action.UserID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("unlock user %s: %w", action.UserID, err)
	}

	return s.record(ctx, userEntry(action.StaffAction, audit.ActionUnlockUser, action.UserID, nil))
}

func (s *service) ForceLogout(ctx context.Context, action spec.StaffUserAction) error {
	if _, err := s.authorise(ctx, action.ActorID, action.UserID); err != nil {
		return err
	}

	if _, err := s.lookUp(ctx, action.UserID); err != nil {
		return err
	}

	if err := s.sessions.DeleteAllForUser(ctx, action.UserID); err != nil {
		return fmt.Errorf("sign user %s out everywhere: %w", action.UserID, err)
	}

	return s.record(ctx, userEntry(action.StaffAction, audit.ActionForceLogout, action.UserID, nil))
}

func (s *service) DeleteUser(ctx context.Context, action spec.StaffUserAction) error {
	if _, err := s.authorise(ctx, action.ActorID, action.UserID); err != nil {
		return err
	}

	target, err := s.lookUp(ctx, action.UserID)
	if err != nil {
		return err
	}

	err = s.users.DeleteAccount(ctx, action.UserID)
	if errors.Is(err, repository.ErrOwnsSharedOrganisation) {
		return fmt.Errorf("%w: %w", ErrSoleOwner, err)
	}
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("delete user %s: %w", action.UserID, err)
	}

	s.sessions.Disconnect(action.UserID)

	return s.record(ctx, staffEntry(action.StaffAction, audit.ActionDeleteUser, audit.TargetUser, action.UserID.String(), map[string]string{"email": target.Email}))
}

func (s *service) ResetUserPassword(ctx context.Context, action spec.StaffUserAction) (*dto.AdminPasswordResetResponse, error) {
	if _, err := s.authorise(ctx, action.ActorID, action.UserID); err != nil {
		return nil, err
	}

	minLength, err := s.settings.GetInt(ctx, config.SettingMinPasswordLength)
	if err != nil {
		return nil, fmt.Errorf("read the minimum password length: %w", err)
	}

	password := rand.Text()
	for len(password) < minLength {
		password += rand.Text()
	}

	passwordHash, err := s.accounts.HashPassword(ctx, password)
	if err != nil {
		return nil, fmt.Errorf("hash a new password for user %s: %w", action.UserID, err)
	}

	err = s.users.ChangePassword(ctx, spec.PasswordChange{UserID: action.UserID, PasswordHash: passwordHash})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reset the password of user %s: %w", action.UserID, err)
	}

	s.sessions.Disconnect(action.UserID)

	return &dto.AdminPasswordResetResponse{Password: password}, s.record(ctx, userEntry(action.StaffAction, audit.ActionResetPassword, action.UserID, nil))
}

func (s *service) SetUserEmail(ctx context.Context, change spec.StaffEmailChange) error {
	address, err := normaliseEmail(change.Email)
	if err != nil {
		return err
	}

	if _, err := s.authorise(ctx, change.ActorID, change.UserID); err != nil {
		return err
	}

	enabled, err := s.email.Enabled(ctx)
	if err != nil {
		return fmt.Errorf("check the email provider: %w", err)
	}

	if !enabled {
		return ErrEmailUnavailable
	}

	account, err := s.lookUp(ctx, change.UserID)
	if err != nil {
		return err
	}

	if strings.EqualFold(account.Email, address) {
		return ErrEmailUnchanged
	}

	inUse, err := s.users.EmailInUse(ctx, spec.UserEmailFilter{Email: address, ExcludeUserID: change.UserID})
	if err != nil {
		return fmt.Errorf("check whether the email address is in use: %w", err)
	}

	if inUse {
		return ErrEmailTaken
	}

	drawn, err := token.New()
	if err != nil {
		return err
	}

	pending := spec.NewEmailVerification{TokenHash: token.Hash(drawn), UserID: change.UserID, PendingEmail: new(address), ExpiresAt: time.Now().Add(verificationTTL)}
	if err := s.verifications.Issue(ctx, pending); err != nil {
		return fmt.Errorf("hold the new email address of user %s: %w", change.UserID, err)
	}

	details := map[string]string{"previous_email": account.Email, "pending_email": address}
	auditErr := s.record(ctx, userEntry(change.StaffAction, audit.ActionSetUserEmail, change.UserID, details))

	if err := s.sendEmailChange(ctx, account.Email, address, drawn); err != nil {
		sendErr := fmt.Errorf("send the email change messages for user %s: %w", change.UserID, err)

		return fmt.Errorf("%w: %w", ErrEmailNotSent, errors.Join(sendErr, auditErr))
	}

	return auditErr
}

func (s *service) VerifyUserEmail(ctx context.Context, action spec.StaffUserAction) error {
	if _, err := s.authorise(ctx, action.ActorID, action.UserID); err != nil {
		return err
	}

	err := s.users.MarkEmailVerified(ctx, action.UserID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("mark the email of user %s verified: %w", action.UserID, err)
	}

	return s.record(ctx, userEntry(action.StaffAction, audit.ActionVerifyUserEmail, action.UserID, nil))
}

func (s *service) UnverifyUserEmail(ctx context.Context, action spec.StaffUserAction) error {
	if _, err := s.authorise(ctx, action.ActorID, action.UserID); err != nil {
		return err
	}

	err := s.users.UnverifyAccount(ctx, action.UserID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("mark the email of user %s unverified: %w", action.UserID, err)
	}

	s.sessions.Disconnect(action.UserID)

	return s.record(ctx, userEntry(action.StaffAction, audit.ActionUnverifyUserEmail, action.UserID, nil))
}

func (s *service) SetUserDisplayName(ctx context.Context, change spec.StaffDisplayNameChange) error {
	displayName := user.ClampDisplayName(change.DisplayName)
	if displayName == "" {
		return ErrDisplayNameRequired
	}

	if _, err := s.authorise(ctx, change.ActorID, change.UserID); err != nil {
		return err
	}

	account, err := s.lookUp(ctx, change.UserID)
	if err != nil {
		return err
	}

	err = s.users.SetDisplayName(ctx, spec.UserDisplayNameUpdate{UserID: change.UserID, DisplayName: displayName})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("set the display name of user %s: %w", change.UserID, err)
	}

	details := map[string]string{"previous_display_name": account.DisplayName, "display_name": displayName}

	return s.record(ctx, userEntry(change.StaffAction, audit.ActionSetDisplayName, change.UserID, details))
}

func (s *service) sendEmailChange(ctx context.Context, previous string, address string, token string) error {
	current, err := s.readSite(ctx)
	if err != nil {
		return err
	}

	notice, err := notification.EmailChangedEmail(notification.EmailChangedEmailData{To: previous, SiteName: current.name, NewEmail: address, LinkURL: current.pageLink(forgotPasswordPage)})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, notice); err != nil {
		return fmt.Errorf("notify the previous address: %w", err)
	}

	verification, err := notification.VerificationEmail(notification.VerificationEmailData{To: address, SiteName: current.name, LinkURL: current.tokenLink(verifyEmailPage, token), ChangesEmail: true})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, verification); err != nil {
		return fmt.Errorf("send the verification email to the new address: %w", err)
	}

	return nil
}

func normaliseEmail(raw string) (string, error) {
	address := strings.TrimSpace(raw)
	if address == "" || len(address) > maxEmailLength {
		return "", ErrInvalidEmail
	}

	parsed, err := mail.ParseAddress(address)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidEmail, err)
	}

	if parsed.Address != address {
		return "", ErrInvalidEmail
	}

	return address, nil
}
