package invite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/notification"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
)

func (s *service) Lookup(ctx context.Context, presented string) (*dto.InviteDetailsResponse, error) {
	details, err := s.liveInvite(ctx, presented)
	if err != nil {
		return nil, err
	}

	return new(dto.NewInviteDetailsResponse(*details)), nil
}

func (s *service) Accept(ctx context.Context, join spec.InviteJoin) (*dto.JoinedOrganisationResponse, error) {
	details, err := s.liveInvite(ctx, join.Token)
	if err != nil {
		return nil, err
	}

	account, err := s.users.GetByID(ctx, join.UserID)
	if err != nil {
		return nil, fmt.Errorf("look up user %s: %w", join.UserID, err)
	}

	if err := requireAddress(*details, *account); err != nil {
		return nil, err
	}

	if err := s.requireNotJoined(ctx, details.OrgID, account.ID); err != nil {
		return nil, err
	}

	quota, err := s.memberQuota(ctx, details.OrgID)
	if err != nil {
		return nil, err
	}

	accepted, err := s.orgs.AcceptInvite(ctx, spec.InviteAcceptance{TokenHash: details.TokenHash, UserID: account.ID, Email: account.Email, MaxMembers: quota.allowed})
	if accepted == nil {
		return nil, quota.refuseJoin(err)
	}

	logger.Ctx(ctx).Info().Str("user_id", account.ID.String()).Str("org_id", details.OrgID.String()).Str("invite_id", details.ID.String()).Msg("joined an organisation through an invite")

	response := new(dto.NewJoinedOrganisationResponse(*details, *accepted))

	return response, errors.Join(staleCache(err), s.recordJoin(ctx, *accepted, join.IP))
}

func (s *service) Register(ctx context.Context, registration spec.InviteRegistration) (*dto.InviteRegistrationResponse, error) {
	passwordSignIn, err := s.settings.GetBool(ctx, config.SettingAllowPasswordSignIn)
	if err != nil {
		return nil, fmt.Errorf("read whether password sign-in is allowed: %w", err)
	}

	if !passwordSignIn {
		return nil, ErrPasswordSignInDisabled
	}

	details, err := s.liveInvite(ctx, registration.Token)
	if err != nil {
		return nil, err
	}

	address, err := registrationAddress(*details, registration.Email)
	if err != nil {
		return nil, err
	}

	if user.ClampDisplayName(registration.DisplayName) == "" {
		return nil, user.ErrDisplayNameRequired
	}

	inUse, err := s.users.EmailInUse(ctx, spec.UserEmailFilter{Email: address})
	if err != nil {
		return nil, fmt.Errorf("check whether the email address has an account: %w", err)
	}

	if inUse {
		return nil, ErrEmailTaken
	}

	quota, err := s.memberQuota(ctx, details.OrgID)
	if err != nil {
		return nil, err
	}

	account, err := s.accounts.NewAccountSpec(ctx, spec.NewAccount{Email: address, Password: registration.Password, DisplayName: registration.DisplayName})
	if err != nil {
		return nil, err
	}

	account.EmailVerified = details.BoundToEmail()

	pending, verification, err := s.draftRegistration(ctx, account)
	if err != nil {
		return nil, err
	}

	created, err := s.users.RegisterAccount(ctx, pending)
	if errors.Is(err, dao.ErrConflict) {
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, err
	}

	accepted, err := s.orgs.AcceptInvite(ctx, spec.InviteAcceptance{TokenHash: details.TokenHash, UserID: created.ID, Email: created.Email, MaxMembers: quota.allowed})
	if accepted == nil {
		return nil, s.withdrawAccount(ctx, created.ID, quota.refuseJoin(err))
	}

	logger.Ctx(ctx).Info().Str("user_id", created.ID.String()).Str("org_id", details.OrgID.String()).Str("invite_id", details.ID.String()).Bool("email_verified", created.EmailVerified).Msg("account registered through an invite")

	response := &dto.InviteRegistrationResponse{
		Email:                created.Email,
		VerificationRequired: !created.EmailVerified,
		Joined:               dto.NewJoinedOrganisationResponse(*details, *accepted),
	}

	aftermath := errors.Join(staleCache(err), s.recordJoin(ctx, *accepted, registration.IP))

	if created.EmailVerified {
		return response, aftermath
	}

	if err := s.email.Send(ctx, verification); err != nil {
		return response, errors.Join(fmt.Errorf("%w to user %s: %w", ErrVerificationNotSent, created.ID, err), aftermath)
	}

	return response, aftermath
}

func (s *service) requireNotJoined(ctx context.Context, orgID uuid.UUID, userID uuid.UUID) error {
	_, err := s.orgs.GetMember(ctx, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check whether user %s is a member of organisation %s: %w", userID, orgID, err)
	}

	return ErrAlreadyMember
}

func (s *service) draftRegistration(ctx context.Context, account spec.NewUser) (spec.NewRegistration, email.Message, error) {
	pending := spec.NewRegistration{User: account}
	if account.EmailVerified {
		return pending, email.Message{}, nil
	}

	if err := s.requireEmail(ctx); err != nil {
		return spec.NewRegistration{}, email.Message{}, err
	}

	current, err := s.readSite(ctx)
	if err != nil {
		return spec.NewRegistration{}, email.Message{}, err
	}

	drawn, err := token.New()
	if err != nil {
		return spec.NewRegistration{}, email.Message{}, err
	}

	verification, err := notification.VerificationEmail(notification.VerificationEmailData{To: account.Email, SiteName: current.name, LinkURL: current.tokenLink(verifyEmailPage, drawn)})
	if err != nil {
		return spec.NewRegistration{}, email.Message{}, err
	}

	pending.VerificationHash = token.Hash(drawn)
	pending.VerificationExpiresAt = time.Now().Add(verificationTTL)

	return pending, verification, nil
}

func (s *service) withdrawAccount(ctx context.Context, userID uuid.UUID, refusal error) error {
	if err := s.users.Delete(ctx, userID); err != nil {
		return fmt.Errorf("%w: %w", ErrAccountLeftBehind, errors.Join(refusal, fmt.Errorf("delete user %s: %w", userID, err)))
	}

	return refusal
}

func requireAddress(details model.InviteDetails, account model.User) error {
	if details.Email == nil {
		return nil
	}

	if !strings.EqualFold(*details.Email, account.Email) {
		return ErrEmailMismatch
	}

	if !account.EmailVerified {
		return ErrEmailUnverified
	}

	return nil
}

func registrationAddress(details model.InviteDetails, requested string) (string, error) {
	if details.Email == nil {
		return normaliseEmail(requested)
	}

	supplied := strings.TrimSpace(requested)
	if supplied != "" && !strings.EqualFold(supplied, *details.Email) {
		return "", ErrEmailMismatch
	}

	return *details.Email, nil
}
