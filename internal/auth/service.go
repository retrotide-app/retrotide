package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/logger"
	"retrotide/internal/model/spec"
	"retrotide/internal/notification"
	"retrotide/internal/org"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/token"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"
)

type (
	SessionStarter interface {
		StartSession(ctx context.Context, pending spec.TwoFactorPending) (*SessionStart, error)
	}

	Service interface {
		SessionStarter
		CompleteTwoFactor(ctx context.Context, verification spec.TwoFactorVerification) (*TwoFactorSignIn, error)
		RegistrationOpen(ctx context.Context) (bool, error)
		Register(ctx context.Context, registration spec.AccountRegistration) (*dto.RegistrationResponse, error)
		GetInvitation(ctx context.Context, token string) (*dto.InvitationResponse, error)
		AcceptInvitation(ctx context.Context, acceptance spec.InvitationAcceptance) (*dto.InvitationAcceptedResponse, error)
		Login(ctx context.Context, signIn spec.PasswordSignIn) (*LoginResult, error)
		Logout(ctx context.Context, token string) error
		VerifyEmail(ctx context.Context, token string) error
		ResendVerification(ctx context.Context, address string) error
		ForgotPassword(ctx context.Context, address string) error
		ResetPassword(ctx context.Context, completion spec.PasswordResetCompletion) error
		ChangePassword(ctx context.Context, update spec.PasswordUpdate) error
		SetEmail(ctx context.Context, change spec.EmailChange) error
		SignOutOtherDevices(ctx context.Context, keep spec.SessionDeletionExcept) error
		DeleteAccount(ctx context.Context, confirmation spec.UserPasswordConfirmation) error
	}

	Deps struct {
		Users         user.Service
		Authz         authz.Service
		Sessions      *session.Manager
		Settings      settings.Service
		Email         email.Service
		UserRepo      repository.UserRepository
		Verifications repository.EmailVerificationRepository
		Resets        repository.PasswordResetRepository
		Invitations   repository.InvitationRepository
		AuditLog      repository.AuditLogRepository
		Organisations org.Service
		TwoFactor     twofactor.Service
	}

	LoginResult struct {
		User           *dto.UserResponse
		SessionToken   string
		ChallengeToken string
	}

	SessionStart struct {
		SessionToken   string
		ChallengeToken string
	}

	TwoFactorSignIn struct {
		User               dto.UserResponse
		SessionToken       string
		Remember           bool
		TrustedDeviceToken string
		Location           string
	}

	service struct {
		users         user.Service
		authz         authz.Service
		sessions      *session.Manager
		settings      settings.Service
		email         email.Service
		userRepo      repository.UserRepository
		verifications repository.EmailVerificationRepository
		resets        repository.PasswordResetRepository
		invitations   repository.InvitationRepository
		auditLog      repository.AuditLogRepository
		organisations org.Service
		twoFactor     twofactor.Service
	}

	site struct {
		name    string
		baseURL string
	}

	admission struct {
		refusal      error
		firstAccount bool
	}
)

const (
	maxEmailLength  = 254
	verificationTTL = 24 * time.Hour
	resetTTL        = time.Hour

	verifyEmailPage    = "verify-email"
	resetPasswordPage  = "reset-password"
	forgotPasswordPage = "forgot-password"
)

func NewService(deps Deps) Service {
	return &service{
		users:         deps.Users,
		authz:         deps.Authz,
		sessions:      deps.Sessions,
		settings:      deps.Settings,
		email:         deps.Email,
		userRepo:      deps.UserRepo,
		verifications: deps.Verifications,
		resets:        deps.Resets,
		invitations:   deps.Invitations,
		auditLog:      deps.AuditLog,
		organisations: deps.Organisations,
		twoFactor:     deps.TwoFactor,
	}
}

func (s *service) RegistrationOpen(ctx context.Context) (bool, error) {
	refusal, err := s.registrationRefusal(ctx)
	if err != nil {
		return false, err
	}

	if refusal == nil {
		return true, nil
	}

	established, err := s.userRepo.AnyExists(ctx)
	if err != nil {
		return false, fmt.Errorf("check whether any account exists: %w", err)
	}

	return !established, nil
}

func (s *service) Register(ctx context.Context, registration spec.AccountRegistration) (*dto.RegistrationResponse, error) {
	if err := s.requirePasswordSignIn(ctx); err != nil {
		return nil, err
	}

	admitted, err := s.admit(ctx)
	if err != nil {
		return nil, err
	}

	address, err := normaliseEmail(registration.Email)
	if err != nil {
		return nil, err
	}

	if err := s.ensureEmailFree(ctx, spec.UserEmailFilter{Email: address}); err != nil {
		return nil, err
	}

	account, err := s.users.NewAccountSpec(ctx, spec.NewAccount{Email: address, Password: registration.Password, DisplayName: registration.DisplayName})
	if err != nil {
		return nil, err
	}

	verificationToken, err := token.New()
	if err != nil {
		return nil, err
	}

	pending := spec.NewRegistration{
		User:                  account,
		FirstAccount:          admitted.firstAccount,
		VerificationHash:      token.Hash(verificationToken),
		VerificationExpiresAt: time.Now().Add(verificationTTL),
	}

	created, err := s.userRepo.RegisterAccount(ctx, pending)
	if errors.Is(err, repository.ErrNotFirstAccount) {
		if admitted.refusal != nil {
			return nil, admitted.refusal
		}

		pending.FirstAccount = false

		created, err = s.userRepo.RegisterAccount(ctx, pending)
	}
	if errors.Is(err, dao.ErrConflict) {
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, err
	}

	logger.Ctx(ctx).Info().Str("user_id", created.ID.String()).Bool("first_account", pending.FirstAccount).Msg("account registered")

	response := &dto.RegistrationResponse{Email: created.Email, VerificationRequired: !created.EmailVerified}
	if created.EmailVerified {
		return response, nil
	}

	if err := s.sendVerification(ctx, created.Email, verificationToken, false); err != nil {
		return response, fmt.Errorf("%w: %w", ErrEmailNotSent, err)
	}

	return response, nil
}

func (s *service) Login(ctx context.Context, signIn spec.PasswordSignIn) (*LoginResult, error) {
	if err := s.requirePasswordSignIn(ctx); err != nil {
		return nil, err
	}

	account, err := s.users.ValidateCredentials(ctx, spec.UserCredentials{Email: strings.TrimSpace(signIn.Email), Password: signIn.Password})
	if err != nil {
		return nil, err
	}

	banned, err := s.authz.IsBanned(ctx, account.ID)
	if err != nil {
		return nil, fmt.Errorf("check whether user %s is banned: %w", account.ID, err)
	}

	if banned {
		return nil, ErrUserBanned
	}

	if err := s.requireVerified(ctx, account); err != nil {
		return nil, err
	}

	pending := spec.TwoFactorPending{UserID: account.ID, Remember: signIn.Remember, TrustedDeviceToken: signIn.TrustedDeviceToken}

	started, err := s.StartSession(ctx, pending)
	if err != nil {
		return nil, err
	}

	return &LoginResult{User: account, SessionToken: started.SessionToken, ChallengeToken: started.ChallengeToken}, nil
}

func (s *service) StartSession(ctx context.Context, pending spec.TwoFactorPending) (*SessionStart, error) {
	challengeToken, err := s.twoFactor.Challenge(ctx, pending)
	if err != nil {
		return nil, fmt.Errorf("check the second factor of user %s: %w", pending.UserID, err)
	}

	if challengeToken != "" {
		return &SessionStart{ChallengeToken: challengeToken}, nil
	}

	sessionToken, err := s.sessions.Create(ctx, spec.SessionGrant{UserID: pending.UserID, Remember: pending.Remember})
	if err != nil {
		return nil, fmt.Errorf("create a session for user %s: %w", pending.UserID, err)
	}

	return &SessionStart{SessionToken: sessionToken}, nil
}

func (s *service) CompleteTwoFactor(ctx context.Context, verification spec.TwoFactorVerification) (*TwoFactorSignIn, error) {
	passed, err := s.twoFactor.Verify(ctx, verification)
	if err != nil {
		return nil, err
	}

	banned, err := s.authz.IsBanned(ctx, passed.UserID)
	if err != nil {
		return nil, fmt.Errorf("check whether user %s is banned: %w", passed.UserID, err)
	}

	if banned {
		return nil, ErrUserBanned
	}

	account, err := s.userRepo.GetByID(ctx, passed.UserID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", twofactor.ErrChallengeExpired, err)
	}
	if err != nil {
		return nil, fmt.Errorf("look up user %s after the second factor: %w", passed.UserID, err)
	}

	sessionToken, err := s.sessions.Create(ctx, spec.SessionGrant{UserID: account.ID, Remember: passed.Remember})
	if err != nil {
		return nil, fmt.Errorf("create a session for user %s: %w", account.ID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", account.ID.String()).Msg("two-factor sign-in completed")

	signedIn := TwoFactorSignIn{
		User:               dto.NewUserResponse(account),
		SessionToken:       sessionToken,
		Remember:           passed.Remember,
		TrustedDeviceToken: passed.TrustedDeviceToken,
		Location:           passed.ReturnTo,
	}

	return &signedIn, nil
}

func (s *service) Logout(ctx context.Context, sessionToken string) error {
	if err := s.sessions.Delete(ctx, sessionToken); err != nil {
		return fmt.Errorf("delete the session: %w", err)
	}

	return nil
}

func (s *service) VerifyEmail(ctx context.Context, presented string) error {
	if presented == "" {
		return ErrInvalidVerificationToken
	}

	consumed, err := s.userRepo.ConfirmEmailVerification(ctx, token.Hash(presented))
	if errors.Is(err, dao.ErrNotFound) {
		return ErrInvalidVerificationToken
	}
	if errors.Is(err, dao.ErrConflict) {
		return ErrEmailTaken
	}
	if err != nil {
		return err
	}

	logger.Ctx(ctx).Info().Str("user_id", consumed.UserID.String()).Bool("changed_email", consumed.ChangesEmail()).Msg("email address verified")

	return nil
}

func (s *service) ResendVerification(ctx context.Context, address string) error {
	if err := s.requireEmail(ctx); err != nil {
		return err
	}

	normalised, err := normaliseEmail(address)
	if err != nil {
		return err
	}

	account, err := s.userRepo.GetByEmail(ctx, normalised)
	if errors.Is(err, dao.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("look up the account by email: %w", err)
	}

	if account.EmailVerified {
		return nil
	}

	drawn, err := token.New()
	if err != nil {
		return err
	}

	if err := s.verifications.Issue(ctx, spec.NewEmailVerification{TokenHash: token.Hash(drawn), UserID: account.ID, ExpiresAt: time.Now().Add(verificationTTL)}); err != nil {
		return err
	}

	if err := s.sendVerification(ctx, account.Email, drawn, false); err != nil {
		return fmt.Errorf("%w to user %s: %w", ErrVerificationNotSent, account.ID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", account.ID.String()).Msg("verification email sent again")

	return nil
}

func (s *service) ForgotPassword(ctx context.Context, address string) error {
	if err := s.requirePasswordSignIn(ctx); err != nil {
		return err
	}

	if err := s.requireEmail(ctx); err != nil {
		return err
	}

	normalised, err := normaliseEmail(address)
	if err != nil {
		return err
	}

	account, err := s.userRepo.GetByEmail(ctx, normalised)
	if errors.Is(err, dao.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("look up the account by email: %w", err)
	}

	drawn, err := token.New()
	if err != nil {
		return err
	}

	if err := s.resets.Issue(ctx, spec.NewPasswordReset{TokenHash: token.Hash(drawn), UserID: account.ID, ExpiresAt: time.Now().Add(resetTTL)}); err != nil {
		return err
	}

	current, err := s.readSite(ctx)
	if err != nil {
		return err
	}

	message, err := notification.PasswordResetEmail(notification.PasswordResetEmailData{To: account.Email, SiteName: current.name, LinkURL: current.tokenLink(resetPasswordPage, drawn)})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, message); err != nil {
		return fmt.Errorf("send the password reset email to user %s: %w", account.ID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", account.ID.String()).Msg("password reset email sent")

	return nil
}

func (s *service) ResetPassword(ctx context.Context, completion spec.PasswordResetCompletion) error {
	if err := s.requirePasswordSignIn(ctx); err != nil {
		return err
	}

	if completion.Token == "" {
		return ErrInvalidResetToken
	}

	passwordHash, err := s.users.HashPassword(ctx, completion.NewPassword)
	if err != nil {
		return err
	}

	userID, err := s.userRepo.ResetPassword(ctx, spec.PasswordReset{TokenHash: token.Hash(completion.Token), PasswordHash: passwordHash})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return err
	}

	s.sessions.Disconnect(userID)

	logger.Ctx(ctx).Info().Str("user_id", userID.String()).Msg("password reset, every session revoked")

	return nil
}

func (s *service) ChangePassword(ctx context.Context, update spec.PasswordUpdate) error {
	if err := s.users.CheckPassword(ctx, spec.UserPasswordConfirmation{UserID: update.UserID, Password: update.CurrentPassword}); err != nil {
		return err
	}

	passwordHash, err := s.users.HashPassword(ctx, update.NewPassword)
	if err != nil {
		return err
	}

	err = s.userRepo.ChangePassword(ctx, spec.PasswordChange{UserID: update.UserID, PasswordHash: passwordHash, KeepToken: update.KeepToken})
	if errors.Is(err, dao.ErrNotFound) {
		return user.ErrUserNotFound
	}
	if err != nil {
		return err
	}

	s.sessions.Disconnect(update.UserID)

	logger.Ctx(ctx).Info().Str("user_id", update.UserID.String()).Msg("password changed, other sessions revoked")

	return nil
}

func (s *service) SetEmail(ctx context.Context, change spec.EmailChange) error {
	address, err := normaliseEmail(change.Email)
	if err != nil {
		return err
	}

	if err := s.requireEmail(ctx); err != nil {
		return err
	}

	if err := s.users.CheckPassword(ctx, spec.UserPasswordConfirmation{UserID: change.UserID, Password: change.Password}); err != nil {
		return err
	}

	account, err := s.users.GetByID(ctx, change.UserID)
	if err != nil {
		return err
	}

	if strings.EqualFold(account.Email, address) {
		return ErrEmailUnchanged
	}

	if err := s.ensureEmailFree(ctx, spec.UserEmailFilter{Email: address, ExcludeUserID: change.UserID}); err != nil {
		return err
	}

	drawn, err := token.New()
	if err != nil {
		return err
	}

	if err := s.verifications.Issue(ctx, spec.NewEmailVerification{TokenHash: token.Hash(drawn), UserID: change.UserID, PendingEmail: new(address), ExpiresAt: time.Now().Add(verificationTTL)}); err != nil {
		return err
	}

	current, err := s.readSite(ctx)
	if err != nil {
		return err
	}

	notice, err := notification.EmailChangedEmail(notification.EmailChangedEmailData{To: account.Email, SiteName: current.name, NewEmail: address, LinkURL: current.pageLink(forgotPasswordPage)})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, notice); err != nil {
		return fmt.Errorf("notify the previous address of user %s: %w", change.UserID, err)
	}

	return s.sendVerification(ctx, address, drawn, true)
}

func (s *service) SignOutOtherDevices(ctx context.Context, keep spec.SessionDeletionExcept) error {
	if err := s.sessions.DeleteAllForUserExcept(ctx, keep); err != nil {
		return fmt.Errorf("sign out the other devices of user %s: %w", keep.UserID, err)
	}

	if err := s.verifications.DeletePendingEmailChangesForUser(ctx, keep.UserID); err != nil {
		return fmt.Errorf("cancel the pending email changes of user %s: %w", keep.UserID, err)
	}

	return nil
}

func (s *service) DeleteAccount(ctx context.Context, confirmation spec.UserPasswordConfirmation) error {
	if err := s.users.CheckPassword(ctx, confirmation); err != nil {
		return err
	}

	if err := s.organisations.DeleteAccount(ctx, confirmation.UserID); err != nil {
		return err
	}

	s.sessions.Disconnect(confirmation.UserID)

	logger.Ctx(ctx).Info().Str("user_id", confirmation.UserID.String()).Msg("account deleted")

	return nil
}

func (s *service) admit(ctx context.Context) (admission, error) {
	refusal, err := s.registrationRefusal(ctx)
	if err != nil {
		return admission{}, err
	}

	established, err := s.userRepo.AnyExists(ctx)
	if err != nil {
		return admission{}, fmt.Errorf("check whether any account exists: %w", err)
	}

	if refusal != nil && established {
		return admission{}, refusal
	}

	return admission{refusal: refusal, firstAccount: !established}, nil
}

func (s *service) registrationRefusal(ctx context.Context) (refusal error, err error) {
	mode, err := settings.ReadSiteMode(ctx, s.settings)
	if err != nil {
		return nil, err
	}

	if mode != config.SiteModePublic {
		return ErrRegistrationPrivate, nil
	}

	enabled, err := s.email.Enabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("check the email provider: %w", err)
	}

	if !enabled {
		return ErrRegistrationClosed, nil
	}

	return nil, nil
}

func (s *service) ensureEmailFree(ctx context.Context, filter spec.UserEmailFilter) error {
	inUse, err := s.userRepo.EmailInUse(ctx, filter)
	if err != nil {
		return fmt.Errorf("check whether the email address is in use: %w", err)
	}

	if inUse {
		return ErrEmailTaken
	}

	return nil
}

func (s *service) requireVerified(ctx context.Context, account *dto.UserResponse) error {
	if account.EmailVerified {
		return nil
	}

	exempt, err := s.authz.BypassesRestrictions(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("check whether user %s bypasses email verification: %w", account.ID, err)
	}

	if !exempt {
		return ErrEmailUnverified
	}

	return nil
}

func (s *service) requirePasswordSignIn(ctx context.Context) error {
	allowed, err := s.settings.GetBool(ctx, config.SettingAllowPasswordSignIn)
	if err != nil {
		return fmt.Errorf("check whether password sign-in is on: %w", err)
	}

	if !allowed {
		return ErrPasswordSignInDisabled
	}

	return nil
}

func (s *service) requireEmail(ctx context.Context) error {
	enabled, err := s.email.Enabled(ctx)
	if err != nil {
		return fmt.Errorf("check the email provider: %w", err)
	}

	if !enabled {
		return ErrEmailUnavailable
	}

	return nil
}

func (s *service) sendVerification(ctx context.Context, to string, token string, changesEmail bool) error {
	current, err := s.readSite(ctx)
	if err != nil {
		return err
	}

	message, err := notification.VerificationEmail(notification.VerificationEmailData{To: to, SiteName: current.name, LinkURL: current.tokenLink(verifyEmailPage, token), ChangesEmail: changesEmail})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, message); err != nil {
		return fmt.Errorf("send the verification email: %w", err)
	}

	return nil
}

func (s *service) readSite(ctx context.Context) (site, error) {
	baseURL, err := s.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return site{}, fmt.Errorf("read the base url: %w", err)
	}

	name, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return site{}, fmt.Errorf("read the site name: %w", err)
	}

	return site{name: name, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s site) pageLink(page string) string {
	return s.baseURL + "/" + page
}

func (s site) tokenLink(page string, token string) string {
	return s.baseURL + "/" + page + "#" + token
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
