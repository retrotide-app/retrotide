package dao_test

import (
	"strings"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/token"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	otherProvider model.IdentityProvider = "microsoft"
)

func signInFlow() spec.NewOIDCFlow {
	return spec.NewOIDCFlow{
		TokenHash:    uuid.NewString(),
		Provider:     model.ProviderGoogle,
		Purpose:      model.OIDCFlowSignIn,
		State:        strings.Repeat("s", 43),
		Nonce:        strings.Repeat("n", 43),
		CodeVerifier: strings.Repeat("v", 64),
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
}

func linkFlow(userID uuid.UUID, sessionToken string) spec.NewOIDCFlow {
	flow := signInFlow()
	flow.Purpose = model.OIDCFlowLink
	flow.UserID = userID
	flow.SessionTokenHash = token.Hash(sessionToken)

	return flow
}

func TestOIDCFlowDAO_ALinkFlowComesBackWhole(t *testing.T) {
	// given a link flow that carries every optional field
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	session := daotest.CreateSession(t, repos, user.ID)
	flow := linkFlow(user.ID, session)
	flow.ReturnTo = "/account?tab=sign-in"
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), flow))

	// when
	got, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: flow.TokenHash, Provider: model.ProviderGoogle})

	// then
	require.NoError(t, err)
	assert.Equal(t, flow.TokenHash, got.TokenHash)
	assert.Equal(t, model.ProviderGoogle, got.Provider)
	assert.Equal(t, model.OIDCFlowLink, got.Purpose)
	assert.Equal(t, flow.State, got.State)
	assert.Equal(t, flow.Nonce, got.Nonce)
	assert.Equal(t, flow.CodeVerifier, got.CodeVerifier)
	require.NotNil(t, got.UserID)
	assert.Equal(t, user.ID, *got.UserID)
	require.NotNil(t, got.SessionTokenHash)
	assert.Equal(t, token.Hash(session), *got.SessionTokenHash)
	require.NotNil(t, got.ReturnTo)
	assert.Equal(t, "/account?tab=sign-in", *got.ReturnTo)
	assert.WithinDuration(t, flow.ExpiresAt, got.ExpiresAt, time.Millisecond)
	assert.WithinDuration(t, time.Now(), got.CreatedAt, time.Minute)
}

func TestOIDCFlowDAO_ASignInFlowCarriesNoUserSessionOrReturnPath(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	flow := signInFlow()
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), flow))

	// when
	got, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: flow.TokenHash, Provider: model.ProviderGoogle})

	// then the empty fields were stored as NULL, not as empty strings
	require.NoError(t, err)
	assert.Equal(t, model.OIDCFlowSignIn, got.Purpose)
	assert.Nil(t, got.UserID)
	assert.Nil(t, got.SessionTokenHash)
	assert.Nil(t, got.ReturnTo)
}

func TestOIDCFlowDAO_AFlowIsConsumedOnce(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	flow := signInFlow()
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), flow))
	key := spec.OIDCFlowKey{TokenHash: flow.TokenHash, Provider: model.ProviderGoogle}
	_, err := repos.OIDCFlow.Consume(t.Context(), key)
	require.NoError(t, err)

	// when
	_, err = repos.OIDCFlow.Consume(t.Context(), key)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestOIDCFlowDAO_AnExpiredFlowIsNotConsumed(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	flow := signInFlow()
	flow.ExpiresAt = time.Now().Add(-time.Second)
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), flow))

	// when
	_, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: flow.TokenHash, Provider: model.ProviderGoogle})

	// then it is refused, and left for the sweep
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, 1, countRows(t, repos, "oidc_flows"))
}

func TestOIDCFlowDAO_AFlowIsNeverConsumedAtAnotherProvidersCallback(t *testing.T) {
	// given a flow started for Google
	repos := daotest.NewRepos(t)
	flow := signInFlow()
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), flow))

	// when another provider's callback presents its cookie
	_, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: flow.TokenHash, Provider: otherProvider})

	// then it matches nothing, and the flow is still Google's to finish
	require.ErrorIs(t, err, dao.ErrNotFound)
	got, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: flow.TokenHash, Provider: model.ProviderGoogle})
	require.NoError(t, err)
	assert.Equal(t, flow.State, got.State)
}

func TestOIDCFlowDAO_AnUnknownCookieIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), signInFlow()))

	// when
	_, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: "never-issued", Provider: model.ProviderGoogle})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, 1, countRows(t, repos, "oidc_flows"))
}

func TestOIDCFlowDAO_CleanExpired_RemovesOnlyExpiredFlows(t *testing.T) {
	// given one live flow and two expired ones
	repos := daotest.NewRepos(t)
	live := signInFlow()
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), live))
	for range 2 {
		expired := signInFlow()
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		require.NoError(t, repos.OIDCFlow.Create(t.Context(), expired))
	}

	// when
	removed, err := repos.OIDCFlow.CleanExpired(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)
	_, err = repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: live.TokenHash, Provider: model.ProviderGoogle})
	require.NoError(t, err)
}

func TestOIDCFlowDAO_CleanExpired_NothingToRemove(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), signInFlow()))

	// when
	removed, err := repos.OIDCFlow.CleanExpired(t.Context())

	// then
	require.NoError(t, err)
	assert.Zero(t, removed)
}

func TestOIDCFlowDAO_DeletingASessionRemovesTheLinkFlowsItStarted(t *testing.T) {
	// given a link flow started from one session, and a sign-in flow that belongs to no session
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	session := daotest.CreateSession(t, repos, user.ID)
	kept := daotest.CreateSession(t, repos, user.ID)
	link := linkFlow(user.ID, session)
	otherLink := linkFlow(user.ID, kept)
	signIn := signInFlow()
	for _, flow := range []spec.NewOIDCFlow{link, otherLink, signIn} {
		require.NoError(t, repos.OIDCFlow.Create(t.Context(), flow))
	}

	// when the starting session signs out
	require.NoError(t, repos.Session.Delete(t.Context(), session))

	// then its link can no longer be finished, and the other flows are untouched
	_, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: link.TokenHash, Provider: model.ProviderGoogle})
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, err = repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: otherLink.TokenHash, Provider: model.ProviderGoogle})
	require.NoError(t, err)
	_, err = repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: signIn.TokenHash, Provider: model.ProviderGoogle})
	require.NoError(t, err)
}

func TestOIDCFlowDAO_AReauthenticateFlowComesBackWithTheProviderItHandsOnTo(t *testing.T) {
	// given a reauthenticate flow at Microsoft that hands on to a Google link
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	session := daotest.CreateSession(t, repos, user.ID)
	flow := linkFlow(user.ID, session)
	flow.Provider = model.ProviderMicrosoft
	flow.Purpose = model.OIDCFlowReauthenticate
	flow.ThenLink = model.ProviderGoogle
	flow.ReturnTo = "/account"
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), flow))

	// when
	got, err := repos.OIDCFlow.Consume(t.Context(), spec.OIDCFlowKey{TokenHash: flow.TokenHash, Provider: model.ProviderMicrosoft})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.ProviderMicrosoft, got.Provider)
	assert.Equal(t, model.OIDCFlowReauthenticate, got.Purpose)
	require.NotNil(t, got.ThenLink)
	assert.Equal(t, model.ProviderGoogle, *got.ThenLink)
	require.NotNil(t, got.UserID)
	assert.Equal(t, user.ID, *got.UserID)
	require.NotNil(t, got.SessionTokenHash)
	assert.Equal(t, token.Hash(session), *got.SessionTokenHash)
}

func TestOIDCFlowDAO_OnlyAReauthenticateFlowNamesAProviderToHandOnTo(t *testing.T) {
	tests := []struct {
		name     string
		purpose  model.OIDCFlowPurpose
		thenLink model.IdentityProvider
	}{
		{name: "a reauthenticate flow that hands on to its own provider", purpose: model.OIDCFlowReauthenticate, thenLink: model.ProviderGoogle},
		{name: "a reauthenticate flow that hands on to nothing", purpose: model.OIDCFlowReauthenticate, thenLink: ""},
		{name: "a reauthenticate flow that hands on to an unknown provider", purpose: model.OIDCFlowReauthenticate, thenLink: "github"},
		{name: "a link flow that hands on", purpose: model.OIDCFlowLink, thenLink: model.ProviderMicrosoft},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)
			user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
			flow := linkFlow(user.ID, daotest.CreateSession(t, repos, user.ID))
			flow.Purpose = tc.purpose
			flow.ThenLink = tc.thenLink

			// when
			err := repos.OIDCFlow.Create(t.Context(), flow)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), "oidc_flows_then_link_check")
			assert.Zero(t, countRows(t, repos, "oidc_flows"))
		})
	}
}

func TestOIDCFlowDAO_ASignInFlowNeverNamesAProviderToHandOnTo(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	flow := signInFlow()
	flow.ThenLink = model.ProviderMicrosoft

	// when
	err := repos.OIDCFlow.Create(t.Context(), flow)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc_flows_then_link_check")
}

func TestOIDCFlowDAO_ALinkFlowNeedsALiveSession(t *testing.T) {
	// given a session token that was never stored
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())

	// when
	err := repos.OIDCFlow.Create(t.Context(), linkFlow(user.ID, "signed-out-token"))

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc_flows_session_token_hash_fkey")
	assert.Zero(t, countRows(t, repos, "oidc_flows"))
}
