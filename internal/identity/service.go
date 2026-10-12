package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/auth"
	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/token"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"

	"github.com/google/uuid"
)

type (
	Service interface {
		Enabled(ctx context.Context, provider model.IdentityProvider) (bool, error)
		EnabledProviders(ctx context.Context) ([]dto.IdentityProviderResponse, error)
		Start(ctx context.Context, start spec.IdentityFlowStart) (*Started, error)
		Finish(ctx context.Context, callback spec.IdentityCallback) (*Outcome, error)
		ListIdentities(ctx context.Context, userID uuid.UUID) (*dto.IdentityListResponse, error)
		Unlink(ctx context.Context, unlinking spec.IdentityUnlinking) error
	}

	Started struct {
		AuthURL   string
		FlowToken string
	}

	Outcome struct {
		Location       string
		SessionToken   string
		FlowToken      string
		ChallengeToken string
	}

	Deps struct {
		Providers  []Provider
		Users      user.Service
		Authz      authz.Service
		Sessions   *session.Manager
		Settings   settings.Service
		Email      email.Service
		UserRepo   repository.UserRepository
		Identities repository.UserIdentityRepository
		Flows      repository.OIDCFlowRepository
		AuditLog   repository.AuditLogRepository
		TwoFactor  twofactor.Gate
		SignIns    auth.SessionStarter
	}

	service struct {
		providers  []Provider
		byID       map[model.IdentityProvider]Provider
		users      user.Service
		authz      authz.Service
		sessions   *session.Manager
		settings   settings.Service
		email      email.Service
		userRepo   repository.UserRepository
		identities repository.UserIdentityRepository
		flows      repository.OIDCFlowRepository
		auditLog   repository.AuditLogRepository
		twoFactor  twofactor.Gate
		signIns    auth.SessionStarter
	}
)

const (
	FlowCookieName = "rt_oidc"
	FlowTTL        = 10 * time.Minute
)

func NewService(deps Deps) (Service, error) {
	byID := make(map[model.IdentityProvider]Provider, len(deps.Providers))
	for _, provider := range deps.Providers {
		if _, taken := byID[provider.ID()]; taken {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateProvider, provider.ID())
		}

		byID[provider.ID()] = provider
	}

	return &service{
		providers:  deps.Providers,
		byID:       byID,
		users:      deps.Users,
		authz:      deps.Authz,
		sessions:   deps.Sessions,
		settings:   deps.Settings,
		email:      deps.Email,
		userRepo:   deps.UserRepo,
		identities: deps.Identities,
		flows:      deps.Flows,
		auditLog:   deps.AuditLog,
		twoFactor:  deps.TwoFactor,
		signIns:    deps.SignIns,
	}, nil
}

func (s *service) Enabled(ctx context.Context, id model.IdentityProvider) (bool, error) {
	provider, known := s.byID[id]
	if !known {
		return false, nil
	}

	allowed, err := s.providersAllowed(ctx)
	if err != nil {
		return false, err
	}

	if !allowed {
		return false, nil
	}

	enabled, err := provider.Enabled(ctx)
	if err != nil {
		return false, fmt.Errorf("check whether %s sign-in is on: %w", id, err)
	}

	return enabled, nil
}

func (s *service) EnabledProviders(ctx context.Context) ([]dto.IdentityProviderResponse, error) {
	listed := make([]dto.IdentityProviderResponse, 0, len(s.providers))

	allowed, err := s.providersAllowed(ctx)
	if err != nil {
		return nil, err
	}

	if !allowed {
		return listed, nil
	}

	for _, provider := range s.providers {
		enabled, err := provider.Enabled(ctx)
		if err != nil {
			return nil, fmt.Errorf("check whether %s sign-in is on: %w", provider.ID(), err)
		}

		if enabled {
			listed = append(listed, dto.IdentityProviderResponse{ID: string(provider.ID()), Name: provider.Name()})
		}
	}

	return listed, nil
}

func (s *service) Start(ctx context.Context, start spec.IdentityFlowStart) (*Started, error) {
	if _, known := s.byID[start.Provider]; !known {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, start.Provider)
	}

	pending := spec.NewOIDCFlow{Provider: start.Provider, Purpose: start.Purpose, ReturnTo: CleanReturnTo(start.ReturnTo)}

	switch start.Purpose {
	case model.OIDCFlowSignIn:
	case model.OIDCFlowLink:
		reauthenticateWith, err := s.confirmLinker(ctx, start)
		if err != nil {
			return nil, err
		}

		pending.UserID = start.UserID
		pending.SessionTokenHash = token.Hash(start.SessionToken)

		if reauthenticateWith != "" {
			pending.Provider = reauthenticateWith
			pending.Purpose = model.OIDCFlowReauthenticate
			pending.ThenLink = start.Provider
		}

	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownPurpose, start.Purpose)
	}

	return s.begin(ctx, pending)
}

func (s *service) begin(ctx context.Context, pending spec.NewOIDCFlow) (*Started, error) {
	provider, known := s.byID[pending.Provider]
	if !known {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, pending.Provider)
	}

	drawn, err := drawFlowSecrets()
	if err != nil {
		return nil, err
	}

	authURL, err := provider.Begin(ctx, drawn.authorisation)
	if err != nil {
		return nil, fmt.Errorf("begin the %s sign-in: %w", pending.Provider, err)
	}

	pending.TokenHash = token.Hash(drawn.cookieToken)
	pending.State = drawn.authorisation.State
	pending.Nonce = drawn.authorisation.Nonce
	pending.CodeVerifier = drawn.authorisation.Verifier
	pending.ExpiresAt = time.Now().Add(FlowTTL)

	if err := s.flows.Create(ctx, pending); err != nil {
		return nil, fmt.Errorf("store the %s %s flow: %w", pending.Provider, pending.Purpose, err)
	}

	return &Started{AuthURL: authURL, FlowToken: drawn.cookieToken}, nil
}

func (s *service) ListIdentities(ctx context.Context, userID uuid.UUID) (*dto.IdentityListResponse, error) {
	identities, err := s.identities.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the identities of user %s: %w", userID, err)
	}

	response := &dto.IdentityListResponse{Identities: make([]dto.IdentityResponse, 0, len(identities))}
	for _, linked := range identities {
		response.Identities = append(response.Identities, dto.NewIdentityResponse(linked, s.providerName(linked.Provider)))
	}

	return response, nil
}

func (s *service) Unlink(ctx context.Context, unlinking spec.IdentityUnlinking) error {
	key := spec.UserIdentityKey{UserID: unlinking.UserID, Provider: unlinking.Provider}

	removed, err := s.identities.Unlink(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrIdentityNotFound, err)
	}
	if errors.Is(err, dao.ErrLastSignInMethod) {
		return fmt.Errorf("%w: %w", ErrLastSignInMethod, err)
	}
	if err != nil {
		return err
	}

	logger.Ctx(ctx).Info().Str("user_id", unlinking.UserID.String()).Str("provider", string(unlinking.Provider)).Msg("identity unlinked")

	return s.announceUnlink(ctx, *removed, unlinking.IP)
}

func (s *service) confirmLinker(ctx context.Context, start spec.IdentityFlowStart) (model.IdentityProvider, error) {
	if start.UserID == uuid.Nil || start.SessionToken == "" {
		return "", ErrSignInRequired
	}

	account, err := s.users.GetByID(ctx, start.UserID)
	if err != nil {
		return "", err
	}

	locked, err := s.authz.IsLocked(ctx, start.UserID)
	if err != nil {
		return "", fmt.Errorf("check whether user %s is locked: %w", start.UserID, err)
	}

	if locked {
		return "", ErrAccountLocked
	}

	if err := s.requireVerified(ctx, start.UserID, account.EmailVerified, ErrAccountUnverified); err != nil {
		return "", err
	}

	if account.HasPassword {
		return "", s.users.CheckPassword(ctx, spec.UserPasswordConfirmation{UserID: start.UserID, Password: start.CurrentPassword})
	}

	if err := s.confirmReauthentication(ctx, start); err != nil {
		return "", err
	}

	return start.ReauthenticateWith, nil
}

func (s *service) confirmReauthentication(ctx context.Context, start spec.IdentityFlowStart) error {
	with := start.ReauthenticateWith
	if with == "" || with == start.Provider {
		return ErrReauthenticationRequired
	}

	enabled, err := s.Enabled(ctx, with)
	if err != nil {
		return err
	}

	if !enabled {
		return fmt.Errorf("%w: %s sign-in is off", ErrReauthenticationRequired, with)
	}

	held, err := s.identities.ListForUser(ctx, start.UserID)
	if err != nil {
		return fmt.Errorf("list the identities of user %s: %w", start.UserID, err)
	}

	for _, linked := range held {
		if linked.Provider == with {
			return nil
		}
	}

	return fmt.Errorf("%w: the account holds no %s identity", ErrReauthenticationRequired, with)
}

func (s *service) providersAllowed(ctx context.Context) (bool, error) {
	mode, err := settings.ReadSiteMode(ctx, s.settings)
	if err != nil {
		return false, fmt.Errorf("check whether the site allows sign-in providers: %w", err)
	}

	return mode == config.SiteModePublic, nil
}

func (s *service) providerName(id model.IdentityProvider) string {
	provider, known := s.byID[id]
	if !known {
		return string(id)
	}

	return provider.Name()
}
