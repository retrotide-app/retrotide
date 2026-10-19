package twofactor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
)

type (
	Service interface {
		Gate
		Status(ctx context.Context, userID uuid.UUID) (*dto.TwoFactorStatusResponse, error)
		BeginEnrolment(ctx context.Context, start spec.TwoFactorEnrolmentStart) (*dto.TwoFactorEnrolmentResponse, error)
		ConfirmEnrolment(ctx context.Context, confirmation spec.TwoFactorConfirmation) (*dto.TwoFactorRecoveryCodesResponse, error)
		Disable(ctx context.Context, entry spec.TwoFactorCodeEntry) error
		ChallengeOwner(ctx context.Context, challengeToken string) (uuid.UUID, error)
		Verify(ctx context.Context, verification spec.TwoFactorVerification) (*Passed, error)
	}

	Passed struct {
		UserID             uuid.UUID
		ReturnTo           string
		Remember           bool
		TrustedDeviceToken string
	}

	SessionDisconnector interface {
		Disconnect(userID uuid.UUID)
	}

	Deps struct {
		TwoFactor repository.TwoFactorRepository
		Users     user.Service
		Settings  settings.Service
		AuditLog  repository.AuditLogRepository
		Sessions  SessionDisconnector
	}

	service struct {
		twoFactor repository.TwoFactorRepository
		users     user.Service
		settings  settings.Service
		auditLog  repository.AuditLogRepository
		sessions  SessionDisconnector
		now       func() time.Time
	}
)

func NewService(deps Deps) Service {
	return &service{
		twoFactor: deps.TwoFactor,
		users:     deps.Users,
		settings:  deps.Settings,
		auditLog:  deps.AuditLog,
		sessions:  deps.Sessions,
		now:       time.Now,
	}
}

func (s *service) Enabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	current, err := s.twoFactor.GetTOTP(ctx, userID)
	if errors.Is(err, dao.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check whether user %s has two-factor authentication on: %w", userID, err)
	}

	return current.Enabled(), nil
}

func (s *service) Challenge(ctx context.Context, pending spec.TwoFactorPending) (string, error) {
	enabled, err := s.Enabled(ctx, pending.UserID)
	if err != nil {
		return "", err
	}

	if !enabled {
		return "", nil
	}

	trusted, err := s.deviceTrusted(ctx, pending)
	if err != nil {
		return "", err
	}

	if trusted {
		logger.Ctx(ctx).Info().Str("user_id", pending.UserID.String()).Msg("two-factor skipped on a trusted device")

		return "", nil
	}

	drawn, err := token.New()
	if err != nil {
		return "", fmt.Errorf("draw a two-factor challenge token: %w", err)
	}

	challenge := spec.NewTwoFactorChallenge{
		TokenHash: token.Hash(drawn),
		UserID:    pending.UserID,
		ReturnTo:  pending.ReturnTo,
		Remember:  pending.Remember,
		ExpiresAt: s.now().Add(ChallengeTTL),
	}

	if err := s.twoFactor.CreateChallenge(ctx, challenge); err != nil {
		return "", fmt.Errorf("store the two-factor challenge for user %s: %w", pending.UserID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", pending.UserID.String()).Msg("two-factor challenge issued")

	return drawn, nil
}

func (s *service) Status(ctx context.Context, userID uuid.UUID) (*dto.TwoFactorStatusResponse, error) {
	enabled, err := s.Enabled(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &dto.TwoFactorStatusResponse{Enabled: enabled}, nil
}

func (s *service) BeginEnrolment(ctx context.Context, start spec.TwoFactorEnrolmentStart) (*dto.TwoFactorEnrolmentResponse, error) {
	account, err := s.users.GetByID(ctx, start.UserID)
	if err != nil {
		return nil, err
	}

	if !account.HasPassword {
		return nil, ErrPasswordRequired
	}

	if err := s.users.CheckPassword(ctx, spec.UserPasswordConfirmation{UserID: start.UserID, Password: start.CurrentPassword}); err != nil {
		return nil, err
	}

	secret, err := GenerateSecret()
	if err != nil {
		return nil, err
	}

	_, err = s.twoFactor.BeginTOTP(ctx, spec.TwoFactorEnrolment{UserID: start.UserID, Secret: secret})
	if errors.Is(err, dao.ErrConflict) {
		return nil, fmt.Errorf("%w: %w", ErrAlreadyEnabled, err)
	}
	if err != nil {
		return nil, fmt.Errorf("begin two-factor enrolment of user %s: %w", start.UserID, err)
	}

	issuer, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return nil, fmt.Errorf("read the site name: %w", err)
	}

	logger.Ctx(ctx).Info().Str("user_id", start.UserID.String()).Msg("two-factor enrolment started")

	return &dto.TwoFactorEnrolmentResponse{Secret: secret, URI: URI(issuer, account.Email, secret)}, nil
}

func (s *service) ConfirmEnrolment(ctx context.Context, confirmation spec.TwoFactorConfirmation) (*dto.TwoFactorRecoveryCodesResponse, error) {
	userID := confirmation.UserID

	current, err := s.twoFactor.GetTOTP(ctx, userID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrNotEnrolling, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the two-factor enrolment of user %s: %w", userID, err)
	}

	if current.Enabled() {
		return nil, ErrNotEnrolling
	}

	step, matched := Match(current.Secret, confirmation.Code, s.now())
	if !matched {
		return nil, ErrInvalidCode
	}

	codes := GenerateRecoveryCodes()

	hashes := make([]string, 0, len(codes))
	for _, code := range codes {
		hashes = append(hashes, RecoveryCodeHash(code))
	}

	_, err = s.twoFactor.Activate(ctx, spec.TwoFactorActivation{UserID: userID, Step: step, CodeHashes: hashes, KeepToken: confirmation.KeepToken})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCode, err)
	}
	if err != nil {
		return nil, fmt.Errorf("confirm the two-factor enrolment of user %s: %w", userID, err)
	}

	s.sessions.Disconnect(userID)

	logger.Ctx(ctx).Info().Str("user_id", userID.String()).Msg("two-factor authentication turned on, other sessions revoked")

	response := dto.TwoFactorRecoveryCodesResponse{RecoveryCodes: codes}

	return &response, s.record(ctx, confirmation.TwoFactorCodeEntry, audit.ActionTwoFactorEnabled)
}

func (s *service) Disable(ctx context.Context, entry spec.TwoFactorCodeEntry) error {
	current, err := s.twoFactor.GetTOTP(ctx, entry.UserID)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrNotEnabled, err)
	}
	if err != nil {
		return fmt.Errorf("read the two-factor secret of user %s: %w", entry.UserID, err)
	}

	if !current.Enabled() {
		return ErrNotEnabled
	}

	accepted, err := s.codeAccepted(ctx, current, entry.Code)
	if err != nil {
		return err
	}

	if !accepted {
		return ErrInvalidCode
	}

	err = s.twoFactor.DeleteTOTP(ctx, entry.UserID)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrNotEnabled, err)
	}
	if err != nil {
		return fmt.Errorf("turn off two-factor authentication for user %s: %w", entry.UserID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", entry.UserID.String()).Msg("two-factor authentication turned off")

	return s.record(ctx, entry, audit.ActionTwoFactorDisabled)
}

func (s *service) ChallengeOwner(ctx context.Context, challengeToken string) (uuid.UUID, error) {
	challenge, err := s.twoFactor.GetLiveChallenge(ctx, token.Hash(challengeToken))
	if errors.Is(err, dao.ErrNotFound) {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrChallengeExpired, err)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("read the two-factor challenge: %w", err)
	}

	return challenge.UserID, nil
}

func (s *service) Verify(ctx context.Context, verification spec.TwoFactorVerification) (*Passed, error) {
	tokenHash := token.Hash(verification.ChallengeToken)

	challenge, err := s.twoFactor.AttemptChallenge(ctx, spec.TwoFactorChallengeAttempt{TokenHash: tokenHash, MaxAttempts: MaxChallengeAttempts})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrChallengeExpired, err)
	}
	if err != nil {
		return nil, fmt.Errorf("claim a two-factor attempt: %w", err)
	}

	current, err := s.enabledSecret(ctx, challenge.UserID)
	if err != nil {
		return nil, s.abandon(ctx, tokenHash, err)
	}

	accepted, err := s.codeAccepted(ctx, current, verification.Code)
	if err != nil {
		return nil, err
	}

	if !accepted {
		return nil, s.refuseCode(ctx, *challenge)
	}

	_, err = s.twoFactor.ConsumeChallenge(ctx, tokenHash)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrChallengeExpired, err)
	}
	if err != nil {
		return nil, fmt.Errorf("spend the two-factor challenge of user %s: %w", challenge.UserID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", challenge.UserID.String()).Msg("two-factor code accepted")

	passed := Passed{UserID: challenge.UserID, Remember: challenge.Remember}
	if challenge.ReturnTo != nil {
		passed.ReturnTo = *challenge.ReturnTo
	}

	if !verification.TrustDevice {
		return &passed, nil
	}

	passed.TrustedDeviceToken, err = s.trustDevice(ctx, challenge.UserID)
	if err != nil {
		return nil, err
	}

	return &passed, nil
}

func (s *service) deviceTrusted(ctx context.Context, pending spec.TwoFactorPending) (bool, error) {
	if pending.TrustedDeviceToken == "" {
		return false, nil
	}

	check := spec.TwoFactorTrustedDeviceCheck{TokenHash: token.Hash(pending.TrustedDeviceToken), UserID: pending.UserID}

	trusted, err := s.twoFactor.TrustedDeviceLive(ctx, check)
	if err != nil {
		return false, fmt.Errorf("check whether user %s trusts this device: %w", pending.UserID, err)
	}

	return trusted, nil
}

func (s *service) trustDevice(ctx context.Context, userID uuid.UUID) (string, error) {
	drawn, err := token.New()
	if err != nil {
		return "", fmt.Errorf("draw a trusted device token: %w", err)
	}

	device := spec.NewTwoFactorTrustedDevice{TokenHash: token.Hash(drawn), UserID: userID, ExpiresAt: s.now().Add(TrustedDeviceTTL)}

	err = s.twoFactor.CreateTrustedDevice(ctx, device)
	if errors.Is(err, dao.ErrNotFound) {
		return "", fmt.Errorf("%w: %w", ErrChallengeExpired, err)
	}
	if err != nil {
		return "", fmt.Errorf("trust a device for user %s: %w", userID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", userID.String()).Msg("device trusted for two-factor")

	return drawn, nil
}

func (s *service) enabledSecret(ctx context.Context, userID uuid.UUID) (*model.UserTOTP, error) {
	current, err := s.twoFactor.GetTOTP(ctx, userID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrChallengeExpired, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the two-factor secret of user %s: %w", userID, err)
	}

	if !current.Enabled() {
		return nil, ErrChallengeExpired
	}

	return current, nil
}

func (s *service) abandon(ctx context.Context, tokenHash string, cause error) error {
	if !errors.Is(cause, ErrChallengeExpired) {
		return cause
	}

	if _, err := s.twoFactor.ConsumeChallenge(ctx, tokenHash); err != nil && !errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("spend the abandoned two-factor challenge: %w", err)
	}

	return cause
}

func (s *service) codeAccepted(ctx context.Context, current *model.UserTOTP, presented string) (bool, error) {
	if !wellFormedCode(strings.ReplaceAll(presented, " ", "")) {
		return s.recoveryCodeAccepted(ctx, current.UserID, presented)
	}

	step, matched := Match(current.Secret, presented, s.now())
	if !matched {
		return false, nil
	}

	err := s.twoFactor.UseTOTPStep(ctx, spec.TwoFactorStepUse{UserID: current.UserID, Step: step})
	if errors.Is(err, dao.ErrConflict) {
		logger.Ctx(ctx).Info().Str("user_id", current.UserID.String()).Msg("two-factor code replayed")

		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("use the two-factor code of user %s: %w", current.UserID, err)
	}

	return true, nil
}

func (s *service) recoveryCodeAccepted(ctx context.Context, userID uuid.UUID, presented string) (bool, error) {
	err := s.twoFactor.UseRecoveryCode(ctx, spec.TwoFactorRecoveryCodeUse{UserID: userID, CodeHash: RecoveryCodeHash(presented)})
	if errors.Is(err, dao.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("use a two-factor recovery code of user %s: %w", userID, err)
	}

	logger.Ctx(ctx).Info().Str("user_id", userID.String()).Msg("two-factor recovery code used")

	return true, nil
}

func (s *service) refuseCode(ctx context.Context, challenge model.TwoFactorChallenge) error {
	logger.Ctx(ctx).Info().Str("user_id", challenge.UserID.String()).Int("attempts", challenge.Attempts).Msg("two-factor code refused")

	if challenge.Attempts < MaxChallengeAttempts {
		return ErrInvalidCode
	}

	if _, err := s.twoFactor.ConsumeChallenge(ctx, challenge.TokenHash); err != nil && !errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("spend the used-up two-factor challenge of user %s: %w", challenge.UserID, err)
	}

	return ErrInvalidCode
}

func (s *service) record(ctx context.Context, entry spec.TwoFactorCodeEntry, action audit.Action) error {
	logged := audit.NewEntry{
		ActorID:    entry.UserID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetUser,
		TargetID:   entry.UserID.String(),
		SubjectID:  entry.UserID,
		Details:    map[string]string{},
		IP:         entry.IP,
	}

	if err := s.auditLog.Create(ctx, logged); err != nil {
		return fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return nil
}
