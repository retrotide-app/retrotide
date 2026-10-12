package identity

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/auth"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testOrigin         = "https://retrotide.test"
	testChallengeToken = "3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c"
	testTrustedDevice  = "4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d"

	otherProvider model.IdentityProvider = "other"
)

func signInFlow() *model.OIDCFlow {
	return &model.OIDCFlow{
		TokenHash:    token.Hash(testFlowToken),
		Provider:     testProvider,
		Purpose:      model.OIDCFlowSignIn,
		State:        testState,
		Nonce:        testNonce,
		CodeVerifier: testVerifier,
		CreatedAt:    time.Now().Add(-time.Minute),
		ExpiresAt:    time.Now().Add(9 * time.Minute),
	}
}

func linkFlow(userID uuid.UUID) *model.OIDCFlow {
	flow := signInFlow()
	flow.Purpose = model.OIDCFlowLink
	flow.UserID = &userID
	flow.SessionTokenHash = new(token.Hash(testSessionToken))

	return flow
}

func reauthenticationFlow(userID uuid.UUID) *model.OIDCFlow {
	flow := linkFlow(userID)
	flow.Purpose = model.OIDCFlowReauthenticate
	flow.ThenLink = new(otherProvider)

	return flow
}

func providerCallback() spec.IdentityCallback {
	return spec.IdentityCallback{Provider: testProvider, FlowToken: testFlowToken, State: testState, Code: testCode, IP: testIP}
}

func providerIdentity(address string, authoritative bool) *model.ProviderIdentity {
	return &model.ProviderIdentity{Provider: testProvider, Subject: testSubject, Email: address, EmailVerified: true, EmailAuthoritative: authoritative, Name: testName}
}

func refusedAt(page string, code string) string {
	return testOrigin + page + "?error=" + code + "&provider=" + string(testProvider)
}

func consumes(m *testMocks, flow *model.OIDCFlow) *model.OIDCFlow {
	key := spec.OIDCFlowKey{TokenHash: token.Hash(testFlowToken), Provider: testProvider}
	m.flows.EXPECT().Consume(mock.Anything, key).Return(flow, nil)

	return flow
}

func returns(m *testMocks, flow *model.OIDCFlow, found *model.ProviderIdentity) {
	consumes(m, flow)
	m.provider.EXPECT().Complete(mock.Anything, *flow, testCode).Return(found, nil)
}

func arrivesUnlinked(m *testMocks, flow *model.OIDCFlow, found *model.ProviderIdentity) {
	returns(m, flow, found)
	m.identities.EXPECT().GetBySubject(mock.Anything, spec.IdentitySubject{Provider: testProvider, Subject: found.Subject}).Return(nil, fmt.Errorf("get identity by subject: %w", dao.ErrNotFound))
}

func arrivesUnknown(m *testMocks, flow *model.OIDCFlow, found *model.ProviderIdentity) {
	arrivesUnlinked(m, flow, found)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, found.Email).Return(nil, fmt.Errorf("get user by email: %w", dao.ErrNotFound))
}

func arrivesMatching(m *testMocks, flow *model.OIDCFlow, found *model.ProviderIdentity, account *model.User) {
	arrivesUnlinked(m, flow, found)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, found.Email).Return(account, nil)
}

func registers(m *testMocks, created *model.User) *spec.IdentityRegistration {
	captured := &spec.IdentityRegistration{}

	m.identities.EXPECT().RegisterWithIdentity(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.IdentityRegistration, _ ...*sql.Tx) (*model.User, error) {
		*captured = s

		return created, nil
	})

	return captured
}

func links(m *testMocks, linked *model.UserIdentity) *spec.IdentityLink {
	captured := &spec.IdentityLink{}

	m.identities.EXPECT().LinkToAccount(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.IdentityLink, _ ...*sql.Tx) (*model.UserIdentity, error) {
		*captured = s

		return linked, nil
	})

	return captured
}

func admitsOwner(m *testMocks, owner uuid.UUID) {
	m.authz.EXPECT().IsBanned(mock.Anything, owner).Return(false, nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, owner).Return(&model.User{ID: owner, Email: testAddress, EmailVerified: true}, nil)
}

func TestFinish_WithoutAFlowCookieEndsBeforeTheStoreOrTheProvider(t *testing.T) {
	// given
	svc, _ := newSiteService(t)

	callback := providerCallback()
	callback.FlowToken = ""

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then no flow is consumed and the provider is never asked, which the strict mocks would report
	require.NoError(t, err)
	assert.Equal(t, refusedAt(LoginPage, CodeExpired), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_AFlowThatIsNotThereEndsBeforeTheProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider model.IdentityProvider
	}{
		{name: "already used", provider: testProvider},
		{name: "expired", provider: testProvider},
		{name: "started for another provider", provider: otherProvider},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the single conditional consume, keyed on the route's provider, matches no row
			svc, m := newSiteService(t)
			key := spec.OIDCFlowKey{TokenHash: token.Hash(testFlowToken), Provider: tc.provider}
			m.flows.EXPECT().Consume(mock.Anything, key).Return(nil, fmt.Errorf("consume flow: %w", dao.ErrNotFound))

			callback := providerCallback()
			callback.Provider = tc.provider

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then the refusal names the provider whose callback it was
			require.NoError(t, err)
			assert.Equal(t, testOrigin+LoginPage+"?error="+CodeExpired+"&provider="+string(tc.provider), outcome.Location)
		})
	}
}

func TestFinish_AnotherBrowsersStateEndsTheFlowBeforeTheProvider(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{name: "a different state", state: "YW5vdGhlci1icm93c2Vycy1zdGF0ZS10aGF0LWlzLWxvbmctZW5vdWdo"},
		{name: "a state one character short", state: testState[:len(testState)-1]},
		{name: "no state", state: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given this is the login CSRF case: a callback built for someone else's flow
			svc, m := newSiteService(t)
			consumes(m, signInFlow())

			callback := providerCallback()
			callback.State = tc.state

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then the code is never exchanged
			require.NoError(t, err)
			assert.Equal(t, refusedAt(LoginPage, CodeExpired), outcome.Location)
		})
	}
}

func TestFinish_AFlowThatCannotBeReadIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	m.flows.EXPECT().Consume(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then the cause is reported and the browser still gets a redirect
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
}

func TestFinish_ACancelledSignInSaysSo(t *testing.T) {
	tests := []struct {
		name string
		flow *model.OIDCFlow
		page string
	}{
		{name: "a sign-in", flow: signInFlow(), page: LoginPage},
		{name: "a link", flow: linkFlow(uuid.New()), page: accountPage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			consumes(m, tc.flow)

			callback := providerCallback()
			callback.Code = ""
			callback.ProviderError = "access_denied"

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then
			require.NoError(t, err)
			assert.Equal(t, refusedAt(tc.page, CodeCancelled), outcome.Location)
		})
	}
}

func TestFinish_AnyOtherProviderErrorIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	consumes(m, signInFlow())

	callback := providerCallback()
	callback.Code = ""
	callback.ProviderError = "server_error"

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"server_error"`)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
}

func TestFinish_ACallbackWithoutACodeIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	consumes(m, signInFlow())

	callback := providerCallback()
	callback.Code = ""

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.Error(t, err)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
}

func TestFinish_AProviderThatCannotCompleteRedirectsWithTheCause(t *testing.T) {
	tests := []struct {
		name  string
		flow  *model.OIDCFlow
		page  string
		cause error
	}{
		{name: "discovery failed", flow: signInFlow(), page: LoginPage, cause: fmt.Errorf("%w: dial tcp: i/o timeout", ErrProviderUnavailable)},
		{name: "a settings change dropped the provider", flow: signInFlow(), page: LoginPage, cause: ErrProviderNotConfigured},
		{name: "the nonce does not match", flow: signInFlow(), page: LoginPage, cause: ErrNonceMismatch},
		{name: "no id token", flow: signInFlow(), page: LoginPage, cause: ErrNoIDToken},
		{name: "the token endpoint refused the code", flow: signInFlow(), page: LoginPage, cause: fmt.Errorf("exchange code: %w", errDatabaseDown)},
		{name: "discovery failed during a link", flow: linkFlow(uuid.New()), page: accountPage, cause: ErrProviderUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			consumes(m, tc.flow)
			m.provider.EXPECT().Complete(mock.Anything, *tc.flow, testCode).Return(nil, tc.cause)

			// when
			outcome, err := svc.Finish(t.Context(), providerCallback())

			// then nothing is looked up, and the cause is recorded rather than answered as JSON
			require.ErrorIs(t, err, tc.cause)
			assert.Equal(t, refusedAt(tc.page, CodeFailed), outcome.Location)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_AnAddressTheProviderHasNotVerifiedIsRefusedBeforeAnyLookup(t *testing.T) {
	tests := []struct {
		name          string
		flow          *model.OIDCFlow
		page          string
		authoritative bool
	}{
		{name: "a sign-in", flow: signInFlow(), page: LoginPage},
		{name: "a sign-in the provider is otherwise authoritative for", flow: signInFlow(), page: LoginPage, authoritative: true},
		{name: "a link", flow: linkFlow(uuid.New()), page: accountPage, authoritative: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			found := providerIdentity(testAddress, tc.authoritative)
			found.EmailVerified = false
			returns(m, tc.flow, found)

			callback := providerCallback()
			callback.SessionToken = testSessionToken

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then no identity, account or session is looked up, and nothing is written
			require.NoError(t, err)
			assert.Equal(t, refusedAt(tc.page, CodeEmailUnverified), outcome.Location)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_ALinkedIdentitySignsInItsOwnUserWhateverItsAddressIsNow(t *testing.T) {
	// given the provider's address changed, and now belongs to another account
	svc, m := newSiteService(t)
	flow := signInFlow()
	found := providerIdentity(testOtherAddress, false)
	returns(m, flow, found)

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, spec.IdentitySubject{Provider: testProvider, Subject: testSubject}).Return(storedIdentity(owner, testAddress), nil)
	admitsOwner(m, owner)
	issued := issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then the account is never looked up by email, which the strict mock would report
	require.NoError(t, err)
	assert.Equal(t, testOrigin+"/", outcome.Location)
	assert.Equal(t, issued.Token, outcome.SessionToken)
	assert.Equal(t, owner, issued.UserID)
}

func TestFinish_ALinkedIdentityReturnsToThePageTheSPASet(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	flow := signInFlow()
	flow.ReturnTo = new("/boards/42?view=list")
	returns(m, flow, providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	admitsOwner(m, owner)
	issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.NoError(t, err)
	assert.Equal(t, testOrigin+"/boards/42?view=list", outcome.Location)
}

func TestFinish_ASignInDeletesTheSessionTheBrowserPresentedBeforeCreatingItsOwn(t *testing.T) {
	// given the browser is still signed in, perhaps as someone else
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	admitsOwner(m, owner)
	deletesPresentedSession(m, testOtherSession)
	issued := issuesSession(m)

	callback := providerCallback()
	callback.SessionToken = testOtherSession

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{eventSessionDeleted, eventSessionCreated}, m.events)
	assert.Equal(t, issued.Token, outcome.SessionToken)
	assert.NotEqual(t, testOtherSession, outcome.SessionToken)
}

func TestFinish_ABannedUserIsRefused(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	m.authz.EXPECT().IsBanned(mock.Anything, owner).Return(true, nil)

	callback := providerCallback()
	callback.SessionToken = testOtherSession

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then no session is created, and the one the browser holds is left alone
	require.NoError(t, err)
	assert.Equal(t, refusedAt(LoginPage, CodeAccountBanned), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_ABanThatCannotBeCheckedFailsClosed(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	m.authz.EXPECT().IsBanned(mock.Anything, owner).Return(true, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_ALinkedIdentityOfAnUnverifiedAccountIsRefused(t *testing.T) {
	// given staff marked the account's address unverified after the identity was linked
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	m.authz.EXPECT().IsBanned(mock.Anything, owner).Return(false, nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, owner).Return(&model.User{ID: owner, Email: testAddress}, nil)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, owner).Return(false, nil)

	callback := providerCallback()
	callback.SessionToken = testOtherSession

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then no session is created, and the one the browser holds is left alone, which the strict session mock would report
	require.NoError(t, err)
	assert.Equal(t, refusedAt(LoginPage, CodeAccountUnverified), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_ALinkedIdentityOfAnUnverifiedSuperAdminSignsIn(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	m.authz.EXPECT().IsBanned(mock.Anything, owner).Return(false, nil)
	m.userRepo.EXPECT().GetByID(mock.Anything, owner).Return(&model.User{ID: owner, Email: testAddress}, nil)
	m.authz.EXPECT().BypassesRestrictions(mock.Anything, owner).Return(true, nil)
	issued := issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.NoError(t, err)
	assert.Equal(t, testOrigin+"/", outcome.Location)
	assert.Equal(t, issued.Token, outcome.SessionToken)
	assert.Equal(t, owner, issued.UserID)
}

func TestFinish_AVerificationThatCannotBeCheckedFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		expect func(m *testMocks, owner uuid.UUID)
	}{
		{
			name: "the account",
			expect: func(m *testMocks, owner uuid.UUID) {
				m.userRepo.EXPECT().GetByID(mock.Anything, owner).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the exemption",
			expect: func(m *testMocks, owner uuid.UUID) {
				m.userRepo.EXPECT().GetByID(mock.Anything, owner).Return(&model.User{ID: owner, Email: testAddress}, nil)
				m.authz.EXPECT().BypassesRestrictions(mock.Anything, owner).Return(false, errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			returns(m, signInFlow(), providerIdentity(testAddress, true))

			owner := uuid.New()
			m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
			m.authz.EXPECT().IsBanned(mock.Anything, owner).Return(false, nil)
			tc.expect(m, owner)

			// when
			outcome, err := svc.Finish(t.Context(), providerCallback())

			// then
			require.ErrorIs(t, err, errDatabaseDown)
			assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_AnIdentityLookupThatFailsIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then a failed lookup never reads as an unknown identity that may sign up
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
}

func TestFinish_AnAccountLookupThatFailsIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	found := providerIdentity(testAddress, true)
	arrivesUnlinked(m, signInFlow(), found)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testAddress).Return(nil, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
}

func TestFinish_AnUnknownAddressTheProviderVouchesForSignsUp(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{name: "an address on the provider's own domain", address: testAddress},
		{name: "an address on a domain the provider hosts", address: testWorkAddress},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the adapter has already decided authority, so the service sees only the verdict
			svc, m := newSiteService(t)
			found := providerIdentity(tc.address, true)
			arrivesUnknown(m, signInFlow(), found)
			m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)

			created := &model.User{ID: uuid.New(), Email: tc.address, DisplayName: testName, EmailVerified: true}
			registration := registers(m, created)
			issued := issuesSession(m)

			// when
			outcome, err := svc.Finish(t.Context(), providerCallback())

			// then the account is verified, has no password, and needs no verification email
			require.NoError(t, err)
			assert.Equal(t, spec.NewUser{Email: tc.address, PasswordHash: nil, DisplayName: testName, EmailVerified: true}, registration.User)
			assert.False(t, registration.FirstAccount)
			assert.Equal(t, testProvider, registration.Provider)
			assert.Equal(t, testSubject, registration.Subject)

			assert.Equal(t, testOrigin+"/onboarding", outcome.Location)
			assert.Equal(t, issued.Token, outcome.SessionToken)
			assert.Equal(t, created.ID, issued.UserID)
		})
	}
}

func TestFinish_ASignUpTakesTheNameClaimAsItsStartingDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		claim string
		want  string
	}{
		{name: "no name", claim: "", want: ""},
		{name: "markup", claim: "<b>Alice</b> Example", want: "Alice Example"},
		{name: "longer than the limit", claim: strings.Repeat("a", 50), want: strings.Repeat("a", user.MaxDisplayNameRunes)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			found := providerIdentity(testAddress, true)
			found.Name = tc.claim
			arrivesUnknown(m, signInFlow(), found)
			m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)
			registration := registers(m, &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true})
			issuesSession(m)

			// when
			_, err := svc.Finish(t.Context(), providerCallback())

			// then an empty name is left for the SPA to ask for
			require.NoError(t, err)
			assert.Equal(t, tc.want, registration.User.DisplayName)
		})
	}
}

func TestFinish_ASignUpReturnsToThePageTheSPASet(t *testing.T) {
	// given a user started from the organisation invite page
	svc, m := newSiteService(t)
	flow := signInFlow()
	flow.ReturnTo = new("/invite")
	found := providerIdentity(testAddress, true)
	arrivesUnknown(m, flow, found)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)
	registers(m, &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true})
	issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then they never land on workspace creation
	require.NoError(t, err)
	assert.Equal(t, testOrigin+"/invite", outcome.Location)
}

func TestFinish_ASignUpReplacesTheSessionTheBrowserPresented(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	found := providerIdentity(testAddress, true)
	arrivesUnknown(m, signInFlow(), found)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)
	registers(m, &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true})
	deletesPresentedSession(m, testOtherSession)
	issuesSession(m)

	callback := providerCallback()
	callback.SessionToken = testOtherSession

	// when
	_, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{eventSessionDeleted, eventSessionCreated}, m.events)
}

func TestFinish_APendingChangeOfAddressMatchesNothing(t *testing.T) {
	// given another account is only waiting to move to this address, which is not yet its users.email
	svc, m := newSiteService(t)
	found := providerIdentity(testAddress, true)
	arrivesUnknown(m, signInFlow(), found)
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)
	registration := registers(m, &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true})
	issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then it signs up as an unknown address would, and links nothing to the waiting account
	require.NoError(t, err)
	assert.Equal(t, testAddress, registration.User.Email)
	assert.Equal(t, testOrigin+"/onboarding", outcome.Location)
}

func TestFinish_AnUnknownAddressTheProviderIsNotAuthoritativeForNeverSignsUp(t *testing.T) {
	// given a consumer account at the provider made on a work address
	svc, m := newSiteService(t)
	arrivesUnknown(m, signInFlow(), providerIdentity(testOtherAddress, false))

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then the registration rules are never read and nothing is written
	require.NoError(t, err)
	assert.Equal(t, refusedAt(LoginPage, CodeEmailNotAuthoritative), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_FailsWhenItCannotTellWhetherAnyAccountExists(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	arrivesUnknown(m, signInFlow(), providerIdentity(testAddress, true))
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then a failed check never reads as an empty site, and nothing is written
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_TheFirstAccountSignsUpAsTheVerifiedSuperAdmin(t *testing.T) {
	// given no account exists yet
	svc, m := newSiteService(t)
	arrivesUnknown(m, signInFlow(), providerIdentity(testAddress, true))
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, nil)
	registration := registers(m, &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true})
	issued := issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then the repository claims super admin under the same lock as password registration, verified from the start
	require.NoError(t, err)
	assert.True(t, registration.FirstAccount)
	assert.True(t, registration.User.EmailVerified)
	assert.Equal(t, testOrigin+"/onboarding", outcome.Location)
	assert.Equal(t, issued.Token, outcome.SessionToken)
}

func TestFinish_AnUnknownAddressSignsUpWithoutAnEmailProvider(t *testing.T) {
	// given a site with an account, and an email provider that is never asked, since the provider vouches for the address
	svc, m := newSiteService(t)
	arrivesUnknown(m, signInFlow(), providerIdentity(testAddress, true))
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)
	registration := registers(m, &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true})
	issued := issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.NoError(t, err)
	assert.False(t, registration.FirstAccount)
	assert.True(t, registration.User.EmailVerified)
	assert.Equal(t, testOrigin+"/onboarding", outcome.Location)
	assert.Equal(t, issued.Token, outcome.SessionToken)
}

func TestFinish_TheFirstAccountSignsUpWithoutAnEmailProvider(t *testing.T) {
	// given no account exists yet, and the email provider is never asked
	svc, m := newSiteService(t)
	arrivesUnknown(m, signInFlow(), providerIdentity(testAddress, true))
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, nil)
	registration := registers(m, &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true})
	issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.NoError(t, err)
	assert.True(t, registration.FirstAccount)
	assert.Equal(t, testOrigin+"/onboarding", outcome.Location)
}

func TestFinish_LosingTheFirstAccountRaceRegistersAnOrdinaryAccount(t *testing.T) {
	// given another registration claimed the empty site between the check and the transaction, and no email provider is asked
	svc, m := newSiteService(t)
	arrivesUnknown(m, signInFlow(), providerIdentity(testAddress, true))
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(false, nil)
	issuesSession(m)

	created := &model.User{ID: uuid.New(), Email: testAddress, EmailVerified: true}

	var attempts []spec.IdentityRegistration
	m.identities.EXPECT().RegisterWithIdentity(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.IdentityRegistration, _ ...*sql.Tx) (*model.User, error) {
		attempts = append(attempts, s)

		if s.FirstAccount {
			return nil, fmt.Errorf("register: %w", repository.ErrNotFirstAccount)
		}

		return created, nil
	})

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then the second attempt is an ordinary account
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	assert.True(t, attempts[0].FirstAccount)
	assert.False(t, attempts[1].FirstAccount)
	assert.Equal(t, attempts[0].User, attempts[1].User)
	assert.Equal(t, testOrigin+"/onboarding", outcome.Location)
}

func TestFinish_ASignUpTheRepositoryRefuses(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		want  string
	}{
		{name: "the same identity signed up meanwhile", cause: fmt.Errorf("register: %w: %w", repository.ErrIdentityInUse, dao.ErrConflict), want: CodeIdentityInUse},
		{name: "another account took the address meanwhile", cause: fmt.Errorf("register: %w", dao.ErrConflict), want: CodeLinkRequired},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			arrivesUnknown(m, signInFlow(), providerIdentity(testAddress, true))
			m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)
			m.identities.EXPECT().RegisterWithIdentity(mock.Anything, mock.Anything).Return(nil, tc.cause)

			// when
			outcome, err := svc.Finish(t.Context(), providerCallback())

			// then
			require.NoError(t, err)
			assert.Equal(t, refusedAt(LoginPage, tc.want), outcome.Location)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_ASignUpThatCannotBeWrittenIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	arrivesUnknown(m, signInFlow(), providerIdentity(testAddress, true))
	m.userRepo.EXPECT().AnyExists(mock.Anything).Return(true, nil)
	m.identities.EXPECT().RegisterWithIdentity(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
}

func TestFinish_AnAccountStoredUnderAnAliasOfTheAddressLinksAutomatically(t *testing.T) {
	// given someone registered the alias of the address the provider reports, and the browser holds another session
	svc, m := newSiteService(t)
	alias := "ada@alias.example"
	account := newAccount(alias, true)
	found := providerIdentity(testAddress, true)
	found.EmailAliases = []string{alias}

	arrivesUnlinked(m, signInFlow(), found)
	m.userRepo.EXPECT().GetByEmail(mock.Anything, testAddress).Return(nil, fmt.Errorf("get user by email: %w", dao.ErrNotFound))
	m.userRepo.EXPECT().GetByEmail(mock.Anything, alias).Return(account, nil)
	m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(false, nil)

	link := links(m, storedIdentity(account.ID, testAddress))
	disconnects(m, account.ID)
	audits(m)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	mails(m)
	deletesPresentedSession(m, testOtherSession)
	issued := issuesSession(m)

	callback := providerCallback()
	callback.SessionToken = testOtherSession

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then the identity keeps the provider's address and the link is checked against the account's own
	require.NoError(t, err)
	assert.Equal(t, spec.IdentityLink{
		Identity:     spec.NewUserIdentity{UserID: account.ID, Provider: testProvider, Subject: testSubject, Email: testAddress},
		Automatic:    true,
		MatchedEmail: alias,
	}, *link)
	assert.Equal(t, account.ID, issued.UserID)
	assert.Equal(t, issued.Token, outcome.SessionToken)
}

func TestFinish_AVerifiedAccountLinksAutomaticallyWhenTheProviderVouchesForTheAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{name: "an address on the provider's own domain", address: testAddress},
		{name: "an address on a domain the provider hosts", address: testWorkAddress},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given someone registered this address with a password, and the browser holds another session
			svc, m := newSiteService(t)
			account := newAccount(tc.address, true)
			found := providerIdentity(tc.address, true)
			arrivesMatching(m, signInFlow(), found, account)
			m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(false, nil)

			link := links(m, storedIdentity(account.ID, tc.address))
			disconnects(m, account.ID)
			audits(m)
			m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
			mails(m)
			deletesPresentedSession(m, testOtherSession)
			issued := issuesSession(m)

			callback := providerCallback()
			callback.SessionToken = testOtherSession

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then
			require.NoError(t, err)
			assert.Equal(t, spec.IdentityLink{
				Identity:     spec.NewUserIdentity{UserID: account.ID, Provider: testProvider, Subject: testSubject, Email: tc.address},
				Automatic:    true,
				MatchedEmail: tc.address,
			}, *link)

			assert.Equal(t, []string{eventDisconnected, eventSessionDeleted, eventSessionCreated}, m.events, "the account's other sockets drop before the new session exists")
			assert.Equal(t, account.ID, issued.UserID)
			assert.Equal(t, issued.Token, outcome.SessionToken)
			assert.Equal(t, testOrigin+"/", outcome.Location)

			require.Len(t, m.audited, 1)
			assert.Equal(t, audit.NewEntry{
				ActorID:    account.ID,
				ActorKind:  audit.ActorUser,
				Action:     audit.ActionIdentityLinked,
				TargetType: audit.TargetIdentity,
				TargetID:   "example",
				SubjectID:  account.ID,
				Details:    map[string]string{"provider": "example", "provider_name": "Example ID", "method": "automatic"},
				IP:         testIP,
			}, m.audited[0])

			require.Len(t, m.sent, 1)
			assert.Equal(t, tc.address, m.sent[0].To)
			assert.Equal(t, "Example ID sign-in was linked to your RetroTide account", m.sent[0].Subject)
			assert.Contains(t, m.sent[0].HTML, "linked automatically")
			assert.Contains(t, m.sent[0].HTML, `href="https://retrotide.test/account#sign-in-methods"`)
			assert.Contains(t, m.sent[0].HTML, "If you did not set a password here, reset it now.")
			assert.Contains(t, m.sent[0].HTML, testCreatedOn)
			assert.Contains(t, m.sent[0].HTML, `href="https://retrotide.test/forgot-password"`)
		})
	}
}

func TestFinish_NoAutomaticLinkWithoutBothSidesVerified(t *testing.T) {
	tests := []struct {
		name          string
		verified      bool
		authoritative bool
	}{
		{name: "a verified account, but the provider is not authoritative for the address", verified: true, authoritative: false},
		{name: "an unverified account, although the provider is authoritative", verified: false, authoritative: true},
		{name: "an unverified account, and the provider is not authoritative", verified: false, authoritative: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			account := newAccount(testOtherAddress, tc.verified)
			arrivesMatching(m, signInFlow(), providerIdentity(testOtherAddress, tc.authoritative), account)

			callback := providerCallback()
			callback.SessionToken = testOtherSession

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then nothing is linked, signed out, audited, sent or created
			require.NoError(t, err)
			assert.Equal(t, refusedAt(LoginPage, CodeLinkRequired), outcome.Location)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_AMatchingAccountTheLinkRefuses(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
		want     string
	}{
		{name: "a banned account", sentinel: repository.ErrAccountBanned, want: CodeAccountBanned},
		{name: "an account that holds a different identity from the provider", sentinel: repository.ErrAlreadyLinked, want: CodeAlreadyLinked},
		{name: "an account whose email or verification changed before the insert", sentinel: repository.ErrAccountChanged, want: CodeLinkRequired},
		{name: "an identity linked to another account meanwhile", sentinel: repository.ErrIdentityInUse, want: CodeIdentityInUse},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the rule allows the link, and the locked re-check in the repository refuses it
			svc, m := newSiteService(t)
			account := newAccount(testAddress, true)
			arrivesMatching(m, signInFlow(), providerIdentity(testAddress, true), account)
			m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(false, nil)
			m.identities.EXPECT().LinkToAccount(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("link an identity to user %s: %w", account.ID, tc.sentinel))

			// when
			outcome, err := svc.Finish(t.Context(), providerCallback())

			// then nothing else happens: no sockets drop, no session, audit entry or notice
			require.NoError(t, err)
			assert.Equal(t, refusedAt(LoginPage, tc.want), outcome.Location)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_AMatchingAccountDeletedMeanwhileIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testAddress, true)
	arrivesMatching(m, signInFlow(), providerIdentity(testAddress, true), account)
	m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(false, nil)
	m.identities.EXPECT().LinkToAccount(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("link: %w", dao.ErrNotFound))

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
}

func TestFinish_AnAutomaticLinkThatCannotBeAuditedStillSignsIn(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testAddress, true)
	arrivesMatching(m, signInFlow(), providerIdentity(testAddress, true), account)
	m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(false, nil)
	links(m, storedIdentity(account.ID, testAddress))
	disconnects(m, account.ID)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabaseDown)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	mails(m)
	issued := issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then the link committed, so the user is signed in and the cause is recorded
	require.ErrorIs(t, err, ErrNotAudited)
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, issued.Token, outcome.SessionToken)
	assert.Equal(t, testOrigin+"/", outcome.Location)
	require.Len(t, m.sent, 1)
}

func TestFinish_AnAutomaticLinkWhoseNoticeFailsStillSignsIn(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testAddress, true)
	arrivesMatching(m, signInFlow(), providerIdentity(testAddress, true), account)
	m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(false, nil)
	links(m, storedIdentity(account.ID, testAddress))
	disconnects(m, account.ID)
	audits(m)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errSMTPRefused)
	issued := issuesSession(m)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.ErrorIs(t, err, ErrNoticeNotSent)
	require.ErrorIs(t, err, errSMTPRefused)
	assert.NotErrorIs(t, err, ErrNotAudited)
	assert.Equal(t, issued.Token, outcome.SessionToken)
	require.Len(t, m.audited, 1)
}

func TestFinish_ASessionThatCannotBeCreatedIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	admitsOwner(m, owner)
	m.signIns.EXPECT().StartSession(mock.Anything, spec.TwoFactorPending{UserID: owner, ReturnTo: existingAccountPage, Remember: true}).Return(nil, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_ALinkFromAccountSettingsLinksAndKeepsItsSession(t *testing.T) {
	// given the provider's address is neither the account's nor one the provider is authoritative for
	svc, m := newSiteService(t)
	account := newAccount(testWorkAddress, true)
	flow := linkFlow(account.ID)
	flow.ReturnTo = new("/account")
	returns(m, flow, providerIdentity(testOtherAddress, false))

	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: account.ID}, nil)
	link := links(m, storedIdentity(account.ID, testOtherAddress))
	audits(m)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	mails(m)

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then the account was proved by its password at the start, so the identity is linked as it is
	require.NoError(t, err)
	assert.Equal(t, spec.IdentityLink{Identity: spec.NewUserIdentity{UserID: account.ID, Provider: testProvider, Subject: testSubject, Email: testOtherAddress}}, *link)
	assert.Equal(t, testOrigin+"/account", outcome.Location)
	assert.Empty(t, outcome.SessionToken, "the session the link is bound to stays, and no other is created or deleted")

	require.Len(t, m.audited, 1)
	assert.Equal(t, map[string]string{"provider": "example", "provider_name": "Example ID", "method": "account_settings"}, m.audited[0].Details)
	assert.Equal(t, audit.ActionIdentityLinked, m.audited[0].Action)

	require.Len(t, m.sent, 1)
	assert.Equal(t, testWorkAddress, m.sent[0].To)
	assert.Contains(t, m.sent[0].HTML, "from your account settings")
	assert.Contains(t, m.sent[0].HTML, testOtherAddress)
	assert.Contains(t, m.sent[0].HTML, "If you did not set a password here, reset it now.")
}

func TestFinish_ALinkWithoutAReturnToLandsOnTheDefaultPage(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testWorkAddress, true)
	flow := linkFlow(account.ID)
	returns(m, flow, providerIdentity(testAddress, true))

	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: account.ID}, nil)
	links(m, storedIdentity(account.ID, testAddress))
	audits(m)
	m.userRepo.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
	mails(m)

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, testOrigin+"/", outcome.Location)
}

func TestFinish_ALinkNeedsTheVerySessionThatStartedIt(t *testing.T) {
	tests := []struct {
		name    string
		session string
	}{
		{name: "the browser is signed out", session: ""},
		{name: "the browser is signed in as another user", session: testOtherSession},
		{name: "the browser is on another session of the same user", session: strings.Repeat("5", 64)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			returns(m, linkFlow(uuid.New()), providerIdentity(testAddress, true))

			callback := providerCallback()
			callback.SessionToken = tc.session

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then the session is compared by its hash, and nothing is validated, linked or sent
			require.NoError(t, err)
			assert.Equal(t, refusedAt(accountPage, CodeExpired), outcome.Location)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_ALinkWhoseSessionEndedMeanwhileIsRefused(t *testing.T) {
	// given the user signed out elsewhere, or reset the password, after starting the link
	svc, m := newSiteService(t)
	returns(m, linkFlow(uuid.New()), providerIdentity(testAddress, true))
	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, refusedAt(accountPage, CodeExpired), outcome.Location)
}

func TestFinish_ALinkWhoseSessionBelongsToAnotherUserIsRefused(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, linkFlow(uuid.New()), providerIdentity(testAddress, true))
	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: uuid.New()}, nil)

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, refusedAt(accountPage, CodeExpired), outcome.Location)
}

func TestFinish_ALinkWhoseSessionCannotBeCheckedIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, linkFlow(uuid.New()), providerIdentity(testAddress, true))
	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(nil, errDatabaseDown)

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(accountPage, CodeFailed), outcome.Location)
}

func TestFinish_ALinkTheRepositoryRefuses(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
		want     string
	}{
		{name: "the identity is linked to another account", sentinel: repository.ErrIdentityInUse, want: CodeIdentityInUse},
		{name: "the account already holds an identity from the provider", sentinel: repository.ErrAlreadyLinked, want: CodeAlreadyLinked},
		{name: "the account was banned meanwhile", sentinel: repository.ErrAccountBanned, want: CodeAccountBanned},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newSiteService(t)
			userID := uuid.New()
			returns(m, linkFlow(userID), providerIdentity(testAddress, true))
			m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: userID}, nil)
			m.identities.EXPECT().LinkToAccount(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("link an identity to user %s: %w", userID, tc.sentinel))

			callback := providerCallback()
			callback.SessionToken = testSessionToken

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then the identity is never moved, and nothing is audited or sent
			require.NoError(t, err)
			assert.Equal(t, refusedAt(accountPage, tc.want), outcome.Location)
		})
	}
}

func TestFinish_FallsBackToThePathWhenTheBaseURLCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)

	callback := providerCallback()
	callback.FlowToken = ""

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then the browser is still redirected, and the cause is reported
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, LoginPage+"?error="+CodeExpired+"&provider="+string(testProvider), outcome.Location)
}

func TestFinish_ASignInLandsOnItsDefaultPageWhenTheBaseURLCannotBeRead(t *testing.T) {
	tests := []struct {
		name     string
		returnTo string
	}{
		{name: "a page on this origin", returnTo: "/boards/42?view=list"},
		{name: "a stored value that leaves the origin without the base url", returnTo: "///evil.example%7C"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)

			flow := signInFlow()
			flow.ReturnTo = new(tc.returnTo)
			returns(m, flow, providerIdentity(testAddress, true))

			owner := uuid.New()
			m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
			admitsOwner(m, owner)
			issued := issuesSession(m)

			// when
			outcome, err := svc.Finish(t.Context(), providerCallback())

			// then the stored value is never sent on bare, and the cause is reported
			require.ErrorIs(t, err, errDatabaseDown)
			assert.Equal(t, existingAccountPage, outcome.Location)
			assert.Equal(t, issued.Token, outcome.SessionToken)
		})
	}
}

func TestFinish_ALinkedIdentityWithTwoFactorGetsAChallengeInsteadOfASession(t *testing.T) {
	tests := []struct {
		name     string
		returnTo *string
		want     string
	}{
		{name: "a page the SPA set", returnTo: new("/boards/42?view=list"), want: "/boards/42?view=list"},
		{name: "no page set", returnTo: nil, want: existingAccountPage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the browser still holds another session
			svc, m := newSiteService(t)
			flow := signInFlow()
			flow.ReturnTo = tc.returnTo
			returns(m, flow, providerIdentity(testAddress, true))

			owner := uuid.New()
			m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
			admitsOwner(m, owner)
			deletesPresentedSession(m, testOtherSession)
			pending := spec.TwoFactorPending{UserID: owner, ReturnTo: tc.want, Remember: true, TrustedDeviceToken: testTrustedDevice}
			m.signIns.EXPECT().StartSession(mock.Anything, pending).Return(&auth.SessionStart{ChallengeToken: testChallengeToken}, nil)

			callback := providerCallback()
			callback.SessionToken = testOtherSession
			callback.TrustedDeviceToken = testTrustedDevice

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then the presented session is gone, no session is created, and the challenge remembers where to land and is told of the trusted device
			require.NoError(t, err)
			assert.Equal(t, testOrigin+"/two-factor", outcome.Location)
			assert.Equal(t, testChallengeToken, outcome.ChallengeToken)
			assert.Empty(t, outcome.SessionToken)
			assert.Empty(t, outcome.FlowToken)
			assert.Equal(t, []string{eventSessionDeleted}, m.events)
		})
	}
}

func TestFinish_ASecondFactorThatCannotBeCheckedIsAFailure(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	returns(m, signInFlow(), providerIdentity(testAddress, true))

	owner := uuid.New()
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(owner, testAddress), nil)
	admitsOwner(m, owner)
	m.signIns.EXPECT().StartSession(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then no session is created, which the strict session mock would report
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
	assert.Empty(t, outcome.ChallengeToken)
}

func TestFinish_NoAutomaticLinkToAnAccountWithTwoFactor(t *testing.T) {
	// given a verified account the provider vouches for, which has two-factor authentication on
	svc, m := newSiteService(t)
	account := newAccount(testAddress, true)
	arrivesMatching(m, signInFlow(), providerIdentity(testAddress, true), account)
	m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(true, nil)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then nothing is linked, signed out, audited, sent or created, which the strict mocks would report
	require.NoError(t, err)
	assert.Equal(t, refusedAt(LoginPage, CodeLinkRequired), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
	assert.Empty(t, outcome.ChallengeToken)
}

func TestFinish_AnAutomaticLinkFailsClosedWhenTheSecondFactorCannotBeChecked(t *testing.T) {
	// given
	svc, m := newSiteService(t)
	account := newAccount(testAddress, true)
	arrivesMatching(m, signInFlow(), providerIdentity(testAddress, true), account)
	m.twoFactor.EXPECT().Enabled(mock.Anything, account.ID).Return(false, errDatabaseDown)

	// when
	outcome, err := svc.Finish(t.Context(), providerCallback())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, refusedAt(LoginPage, CodeFailed), outcome.Location)
	assert.Empty(t, outcome.SessionToken)
}

func TestFinish_AReauthenticationHandsOnToTheProviderBeingLinked(t *testing.T) {
	// given the account confirmed itself with the provider it already holds
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newTestService(t, other)
	userID := uuid.New()
	flow := reauthenticationFlow(userID)
	flow.ReturnTo = new("/account")
	returns(m, flow, providerIdentity(testAddress, true))

	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: userID}, nil)
	m.identities.EXPECT().GetBySubject(mock.Anything, spec.IdentitySubject{Provider: testProvider, Subject: testSubject}).Return(storedIdentity(userID, testAddress), nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	other.EXPECT().Enabled(mock.Anything).Return(true, nil)

	drawn := &Authorisation{}
	other.EXPECT().Begin(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, authorisation Authorisation) (string, error) {
		*drawn = authorisation

		return testOtherAuthURL, nil
	})
	stored := captureFlow(m)

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then a link flow bound to the same user and session starts at the other provider, and nothing is linked yet
	require.NoError(t, err)
	assert.Equal(t, testOtherAuthURL, outcome.Location)
	assert.Regexp(t, `^[0-9a-f]{64}$`, outcome.FlowToken)
	assert.Empty(t, outcome.SessionToken)
	assert.Empty(t, outcome.ChallengeToken)

	assert.Equal(t, token.Hash(outcome.FlowToken), stored.TokenHash)
	assert.Equal(t, otherProvider, stored.Provider)
	assert.Equal(t, model.OIDCFlowLink, stored.Purpose)
	assert.Equal(t, userID, stored.UserID)
	assert.Equal(t, token.Hash(testSessionToken), stored.SessionTokenHash)
	assert.Equal(t, "/account", stored.ReturnTo)
	assert.Empty(t, stored.ThenLink)
	assert.Equal(t, drawn.State, stored.State)
	assert.Equal(t, drawn.Nonce, stored.Nonce)
	assert.Equal(t, drawn.Verifier, stored.CodeVerifier)
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), stored.ExpiresAt, time.Minute)
}

func TestFinish_AReauthenticationAsAnIdentityOfAnotherAccountIsRefused(t *testing.T) {
	tests := []struct {
		name      string
		confirmed *model.UserIdentity
		err       error
	}{
		{name: "the identity belongs to another user", confirmed: storedIdentity(uuid.New(), testOtherAddress)},
		{name: "the identity belongs to nobody", err: fmt.Errorf("get identity by subject: %w", dao.ErrNotFound)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			other := newProvider(t, otherProvider, "Other ID")
			svc, m := newSiteService(t, other)
			userID := uuid.New()
			returns(m, reauthenticationFlow(userID), providerIdentity(testOtherAddress, true))
			m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: userID}, nil)
			m.identities.EXPECT().GetBySubject(mock.Anything, spec.IdentitySubject{Provider: testProvider, Subject: testSubject}).Return(tc.confirmed, tc.err)

			callback := providerCallback()
			callback.SessionToken = testSessionToken

			// when
			outcome, err := svc.Finish(t.Context(), callback)

			// then the other provider never begins and no flow is written, which the strict mocks would report
			require.NoError(t, err)
			assert.Equal(t, refusedAt(accountPage, CodeReauthenticationFailed), outcome.Location)
			assert.Empty(t, outcome.FlowToken)
			assert.Empty(t, outcome.SessionToken)
		})
	}
}

func TestFinish_AReauthenticationWhoseTargetWasSwitchedOffIsRefused(t *testing.T) {
	// given
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newSiteService(t, other)
	userID := uuid.New()
	returns(m, reauthenticationFlow(userID), providerIdentity(testAddress, true))
	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: userID}, nil)
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(userID, testAddress), nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	other.EXPECT().Enabled(mock.Anything).Return(false, nil)

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, refusedAt(accountPage, CodeReauthenticationFailed), outcome.Location)
	assert.Empty(t, outcome.FlowToken)
}

func TestFinish_AReauthenticationNeedsTheVerySessionThatStartedIt(t *testing.T) {
	// given
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newSiteService(t, other)
	returns(m, reauthenticationFlow(uuid.New()), providerIdentity(testAddress, true))

	callback := providerCallback()
	callback.SessionToken = testOtherSession

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, refusedAt(accountPage, CodeExpired), outcome.Location)
	assert.Empty(t, outcome.FlowToken)
}

func TestFinish_ACancelledReauthenticationReturnsToTheAccountPage(t *testing.T) {
	// given
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newSiteService(t, other)
	consumes(m, reauthenticationFlow(uuid.New()))

	callback := providerCallback()
	callback.Code = ""
	callback.ProviderError = "access_denied"

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then
	require.NoError(t, err)
	assert.Equal(t, refusedAt(accountPage, CodeCancelled), outcome.Location)
}

func TestFinish_AReauthenticationWhoseLinkCannotBeginIsAFailure(t *testing.T) {
	// given
	other := newProvider(t, otherProvider, "Other ID")
	svc, m := newSiteService(t, other)
	userID := uuid.New()
	returns(m, reauthenticationFlow(userID), providerIdentity(testAddress, true))
	m.sessionRepo.EXPECT().Get(mock.Anything, testSessionToken).Return(&model.Session{UserID: userID}, nil)
	m.identities.EXPECT().GetBySubject(mock.Anything, mock.Anything).Return(storedIdentity(userID, testAddress), nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	other.EXPECT().Enabled(mock.Anything).Return(true, nil)
	cause := fmt.Errorf("%w: dial tcp: i/o timeout", ErrProviderUnavailable)
	other.EXPECT().Begin(mock.Anything, mock.Anything).Return("", cause)

	callback := providerCallback()
	callback.SessionToken = testSessionToken

	// when
	outcome, err := svc.Finish(t.Context(), callback)

	// then no flow is written, which the strict flow mock would report
	require.ErrorIs(t, err, ErrProviderUnavailable)
	assert.Equal(t, refusedAt(accountPage, CodeFailed), outcome.Location)
	assert.Empty(t, outcome.FlowToken)
}
