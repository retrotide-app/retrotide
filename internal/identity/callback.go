package identity

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"

	"github.com/google/uuid"
)

type (
	landing struct {
		path           string
		defaultPage    string
		sessionToken   string
		challengeToken string
	}
)

const (
	LoginPage           = "/login"
	accountPage         = "/account"
	existingAccountPage = "/"
	newAccountPage      = "/onboarding"

	errorParam    = "error"
	providerParam = "provider"

	accessDenied = "access_denied"
)

func (s *service) Finish(ctx context.Context, callback spec.IdentityCallback) (*Outcome, error) {
	flow, err := s.claimFlow(ctx, callback)
	if err != nil {
		return s.refuse(ctx, callback.Provider, LoginPage, err)
	}

	refusalPage := LoginPage
	if flow.Purpose == model.OIDCFlowLink || flow.Purpose == model.OIDCFlowReauthenticate {
		refusalPage = accountPage
	}

	found, err := s.verify(ctx, *flow, callback)
	if err != nil {
		return s.refuse(ctx, flow.Provider, refusalPage, err)
	}

	if flow.Purpose == model.OIDCFlowReauthenticate {
		handedOn, err := s.finishReauthentication(ctx, *flow, *found, callback)
		if err != nil {
			return s.refuse(ctx, flow.Provider, refusalPage, err)
		}

		return handedOn, nil
	}

	var arrived *landing
	if flow.Purpose == model.OIDCFlowLink {
		arrived, err = s.finishLink(ctx, *flow, *found, callback)
	} else {
		arrived, err = s.finishSignIn(ctx, *flow, *found, callback)
	}
	if arrived == nil {
		return s.refuse(ctx, flow.Provider, refusalPage, err)
	}

	location, locateErr := s.location(ctx, arrived.path, arrived.defaultPage)

	return &Outcome{Location: location, SessionToken: arrived.sessionToken, ChallengeToken: arrived.challengeToken}, errors.Join(err, locateErr)
}

func (s *service) verify(ctx context.Context, flow model.OIDCFlow, callback spec.IdentityCallback) (*model.ProviderIdentity, error) {
	if callback.ProviderError == accessDenied {
		return nil, errCancelled
	}

	if callback.ProviderError != "" {
		return nil, fmt.Errorf("the %s sign-in answered with the error %q", flow.Provider, callback.ProviderError)
	}

	if callback.Code == "" {
		return nil, fmt.Errorf("the %s callback carried no code", flow.Provider)
	}

	provider, known := s.byID[flow.Provider]
	if !known {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, flow.Provider)
	}

	found, err := provider.Complete(ctx, flow, callback.Code)
	if err != nil {
		return nil, fmt.Errorf("complete the %s sign-in: %w", flow.Provider, err)
	}

	if !found.EmailVerified {
		return nil, errEmailUnverified
	}

	return found, nil
}

func (s *service) finishSignIn(ctx context.Context, flow model.OIDCFlow, found model.ProviderIdentity, callback spec.IdentityCallback) (*landing, error) {
	linked, err := s.identities.GetBySubject(ctx, spec.IdentitySubject{Provider: flow.Provider, Subject: found.Subject})
	if err == nil {
		return s.signInLinked(ctx, flow, *linked, callback)
	}
	if !errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("look up the %s identity: %w", flow.Provider, err)
	}

	account, err := s.accountByAddress(ctx, found)
	if errors.Is(err, dao.ErrNotFound) {
		return s.signUp(ctx, flow, found, callback)
	}
	if err != nil {
		return nil, err
	}

	return s.linkAutomatically(ctx, flow, found, *account, callback)
}

func (s *service) accountByAddress(ctx context.Context, found model.ProviderIdentity) (*model.User, error) {
	addresses := append([]string{found.Email}, found.EmailAliases...)

	for _, address := range addresses {
		account, err := s.userRepo.GetByEmail(ctx, address)
		if err == nil {
			return account, nil
		}

		if !errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("look up the account by email: %w", err)
		}
	}

	return nil, fmt.Errorf("look up the account by email: %w", dao.ErrNotFound)
}

func (s *service) signInLinked(ctx context.Context, flow model.OIDCFlow, linked model.UserIdentity, callback spec.IdentityCallback) (*landing, error) {
	banned, err := s.authz.IsBanned(ctx, linked.UserID)
	if err != nil {
		return nil, fmt.Errorf("check whether user %s is banned: %w", linked.UserID, err)
	}

	if banned {
		return nil, errAccountBanned
	}

	account, err := s.userRepo.GetByID(ctx, linked.UserID)
	if err != nil {
		return nil, fmt.Errorf("look up user %s: %w", linked.UserID, err)
	}

	if err := s.requireVerified(ctx, linked.UserID, account.EmailVerified, errAccountUnverified); err != nil {
		return nil, err
	}

	arrived, err := s.replaceSession(ctx, linked.UserID, callback, arrive(flow, existingAccountPage, ""))
	if err != nil {
		return nil, err
	}

	logger.Ctx(ctx).Info().Str("user_id", linked.UserID.String()).Str("provider", string(flow.Provider)).Bool("two_factor", arrived.challengeToken != "").Msg("signed in with a linked identity")

	return arrived, nil
}

func (s *service) signUp(ctx context.Context, flow model.OIDCFlow, found model.ProviderIdentity, callback spec.IdentityCallback) (*landing, error) {
	if !vouchesForEmail(found) {
		return nil, errEmailNotAuthoritative
	}

	established, err := s.userRepo.AnyExists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check whether any account exists: %w", err)
	}

	registration := spec.IdentityRegistration{
		User:         spec.NewUser{Email: found.Email, DisplayName: user.ClampDisplayName(found.Name), EmailVerified: true},
		FirstAccount: !established,
		Provider:     flow.Provider,
		Subject:      found.Subject,
	}

	created, err := s.identities.RegisterWithIdentity(ctx, registration)
	if errors.Is(err, repository.ErrNotFirstAccount) {
		registration.FirstAccount = false

		created, err = s.identities.RegisterWithIdentity(ctx, registration)
	}
	if errors.Is(err, repository.ErrIdentityInUse) {
		return nil, err
	}
	if errors.Is(err, dao.ErrConflict) {
		return nil, fmt.Errorf("%w: %w", errLinkRequired, err)
	}
	if err != nil {
		return nil, err
	}

	logger.Ctx(ctx).Info().Str("user_id", created.ID.String()).Str("provider", string(flow.Provider)).Bool("first_account", registration.FirstAccount).Msg("account registered with an identity")

	return s.replaceSession(ctx, created.ID, callback, arrive(flow, newAccountPage, ""))
}

func (s *service) linkAutomatically(ctx context.Context, flow model.OIDCFlow, found model.ProviderIdentity, account model.User, callback spec.IdentityCallback) (*landing, error) {
	if !vouchesForEmail(found) || !account.EmailVerified {
		return nil, errLinkRequired
	}

	secondFactor, err := s.twoFactor.Enabled(ctx, account.ID)
	if err != nil {
		return nil, fmt.Errorf("check the second factor of user %s: %w", account.ID, err)
	}

	if secondFactor {
		return nil, fmt.Errorf("%w: the account has two-factor authentication on", errLinkRequired)
	}

	link := spec.IdentityLink{
		Identity:     spec.NewUserIdentity{UserID: account.ID, Provider: flow.Provider, Subject: found.Subject, Email: found.Email},
		Automatic:    true,
		MatchedEmail: account.Email,
	}

	linked, err := s.identities.LinkToAccount(ctx, link)
	if err != nil {
		return nil, err
	}

	s.sessions.Disconnect(account.ID)

	aftermath := s.announceLink(ctx, *linked, methodAutomatic, callback.IP)

	arrived, err := s.replaceSession(ctx, account.ID, callback, arrive(flow, existingAccountPage, ""))
	if err != nil {
		return nil, errors.Join(err, aftermath)
	}

	return arrived, aftermath
}

func (s *service) finishReauthentication(ctx context.Context, flow model.OIDCFlow, found model.ProviderIdentity, callback spec.IdentityCallback) (*Outcome, error) {
	userID, err := s.boundUser(ctx, flow, callback.SessionToken)
	if err != nil {
		return nil, err
	}

	confirmed, err := s.identities.GetBySubject(ctx, spec.IdentitySubject{Provider: flow.Provider, Subject: found.Subject})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", errReauthenticationFailed, err)
	}
	if err != nil {
		return nil, fmt.Errorf("look up the %s identity: %w", flow.Provider, err)
	}

	if confirmed.UserID != userID {
		return nil, errReauthenticationFailed
	}

	if flow.ThenLink == nil {
		return nil, fmt.Errorf("%w: the flow names no provider to link", errReauthenticationFailed)
	}

	target := *flow.ThenLink

	enabled, err := s.Enabled(ctx, target)
	if err != nil {
		return nil, err
	}

	if !enabled {
		return nil, fmt.Errorf("%w: %s sign-in is off", errReauthenticationFailed, target)
	}

	returnTo := ""
	if flow.ReturnTo != nil {
		returnTo = *flow.ReturnTo
	}

	started, err := s.begin(ctx, spec.NewOIDCFlow{Provider: target, Purpose: model.OIDCFlowLink, UserID: userID, SessionTokenHash: *flow.SessionTokenHash, ReturnTo: returnTo})
	if err != nil {
		return nil, err
	}

	logger.Ctx(ctx).Info().Str("user_id", userID.String()).Str("provider", string(flow.Provider)).Str("then_link", string(target)).Msg("reauthenticated to link a sign-in method")

	return &Outcome{Location: started.AuthURL, FlowToken: started.FlowToken}, nil
}

func (s *service) finishLink(ctx context.Context, flow model.OIDCFlow, found model.ProviderIdentity, callback spec.IdentityCallback) (*landing, error) {
	userID, err := s.boundUser(ctx, flow, callback.SessionToken)
	if err != nil {
		return nil, err
	}

	link := spec.IdentityLink{
		Identity: spec.NewUserIdentity{UserID: userID, Provider: flow.Provider, Subject: found.Subject, Email: found.Email},
	}

	linked, err := s.identities.LinkToAccount(ctx, link)
	if err != nil {
		return nil, err
	}

	aftermath := s.announceLink(ctx, *linked, methodAccountSettings, callback.IP)

	return arrive(flow, existingAccountPage, ""), aftermath
}

func (s *service) requireVerified(ctx context.Context, userID uuid.UUID, verified bool, refusal error) error {
	if verified {
		return nil
	}

	exempt, err := s.authz.BypassesRestrictions(ctx, userID)
	if err != nil {
		return fmt.Errorf("check whether user %s bypasses email verification: %w", userID, err)
	}

	if !exempt {
		return refusal
	}

	return nil
}

func (s *service) replaceSession(ctx context.Context, userID uuid.UUID, callback spec.IdentityCallback, arrived *landing) (*landing, error) {
	if err := s.sessions.Delete(ctx, callback.SessionToken); err != nil {
		return nil, fmt.Errorf("delete the session the browser presented: %w", err)
	}

	pending := spec.TwoFactorPending{UserID: userID, ReturnTo: arrived.path, Remember: true, TrustedDeviceToken: callback.TrustedDeviceToken}

	started, err := s.signIns.StartSession(ctx, pending)
	if err != nil {
		return nil, err
	}

	if started.ChallengeToken != "" {
		return &landing{path: twofactor.PagePath, defaultPage: twofactor.PagePath, challengeToken: started.ChallengeToken}, nil
	}

	arrived.sessionToken = started.SessionToken

	return arrived, nil
}

func (s *service) refuse(ctx context.Context, provider model.IdentityProvider, page string, reason error) (*Outcome, error) {
	code, cause := CodeFailed, reason
	for _, known := range refusals {
		if errors.Is(reason, known.reason) {
			code, cause = known.code, nil

			break
		}
	}

	logger.Ctx(ctx).Info().Str("provider", string(provider)).Str("code", code).Msg("identity sign-in refused")

	target := RefusalPath(page, code, provider)
	location, err := s.location(ctx, target, target)

	return &Outcome{Location: location}, errors.Join(cause, err)
}

func (s *service) location(ctx context.Context, path string, bare string) (string, error) {
	baseURL, err := s.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return bare, fmt.Errorf("read the base url for the redirect: %w", err)
	}

	return strings.TrimRight(baseURL, "/") + path, nil
}

func RefusalPath(page string, code string, provider model.IdentityProvider) string {
	query := url.Values{errorParam: {code}, providerParam: {string(provider)}}

	return page + "?" + query.Encode()
}

func vouchesForEmail(found model.ProviderIdentity) bool {
	return found.EmailVerified && found.EmailAuthoritative
}

func arrive(flow model.OIDCFlow, defaultPage string, sessionToken string) *landing {
	path := defaultPage
	if flow.ReturnTo != nil && *flow.ReturnTo != "" {
		path = *flow.ReturnTo
	}

	return &landing{path: path, defaultPage: defaultPage, sessionToken: sessionToken}
}
