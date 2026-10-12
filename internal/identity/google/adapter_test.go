package google

import (
	"net/url"
	"testing"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/identity"
	"retrotide/internal/model"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestNew_ReachesNoOneUntilFirstUse(t *testing.T) {
	// given settings that fail the test if the adapter reads them
	settingsSvc := settings.NewMockService(t)

	// when
	adapter := New(settingsSvc)

	// then
	assert.Equal(t, googleIssuer, adapter.issuer)
	assert.Equal(t, clientTimeout, adapter.client.Timeout)
	assert.Nil(t, adapter.discovered)
}

func TestAdapter_NamesItselfForTheCore(t *testing.T) {
	// given
	adapter := New(settings.NewMockService(t))

	// when
	id, name := adapter.ID(), adapter.Name()

	// then the id is the route segment and the stored provider, and the name is what users read
	assert.Equal(t, model.ProviderGoogle, id)
	assert.Equal(t, "Google", name)
}

func TestEnabled_NeedsBothGoogleSettings(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
		secret   *string
		want     bool
	}{
		{name: "both set", clientID: "client.apps.googleusercontent.com", secret: new("a-secret"), want: true},
		{name: "no client id", clientID: "", want: false},
		{name: "a blank client id", clientID: "   ", want: false},
		{name: "no client secret", clientID: "client.apps.googleusercontent.com", secret: new(""), want: false},
		{name: "a blank client secret", clientID: "client.apps.googleusercontent.com", secret: new("  "), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			settingsSvc := settings.NewMockService(t)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingGoogleClientID).Return(tc.clientID, nil)
			if tc.secret != nil {
				settingsSvc.EXPECT().Get(mock.Anything, config.SettingGoogleClientSecret).Return(*tc.secret, nil)
			}

			adapter := New(settingsSvc)

			// when
			enabled, err := adapter.Enabled(t.Context())

			// then google is never reached to answer
			require.NoError(t, err)
			assert.Equal(t, tc.want, enabled)
		})
	}
}

func TestEnabled_ReportsASettingThatCannotBeRead(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingGoogleClientID).Return("", errSettingsDown)
	adapter := New(settingsSvc)

	// when
	enabled, err := adapter.Enabled(t.Context())

	// then a failed read is never an off switch
	require.ErrorIs(t, err, errSettingsDown)
	assert.False(t, enabled)
}

func TestConfiguredWith_NeedsBothCredentialsInTheValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[config.SiteSettingKey]string
		want   bool
	}{
		{name: "both set", values: map[config.SiteSettingKey]string{config.SettingGoogleClientID.Key: testClientID, config.SettingGoogleClientSecret.Key: testClientSecret}, want: true},
		{name: "no client id", values: map[config.SiteSettingKey]string{config.SettingGoogleClientSecret.Key: testClientSecret}, want: false},
		{name: "a blank client id", values: map[config.SiteSettingKey]string{config.SettingGoogleClientID.Key: "  ", config.SettingGoogleClientSecret.Key: testClientSecret}, want: false},
		{name: "a blank client secret", values: map[config.SiteSettingKey]string{config.SettingGoogleClientID.Key: testClientID, config.SettingGoogleClientSecret.Key: ""}, want: false},
		{name: "only microsoft configured", values: map[config.SiteSettingKey]string{config.SettingMicrosoftClientID.Key: "client", config.SettingMicrosoftClientSecret.Key: "secret"}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given settings that fail the test if the adapter reads them
			adapter := New(settings.NewMockService(t))

			// when
			got := adapter.ConfiguredWith(tc.values)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBegin_BuildsTheAuthorisationURL(t *testing.T) {
	// given
	fake := newFakeGoogle(t)
	adapter := newTestAdapter(t, fake, configuredSettings())

	// when
	authURL, err := adapter.Begin(t.Context(), testAuthorisation())

	// then the state, the nonce and the verifier the core drew are the ones google sees
	require.NoError(t, err)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	query := parsed.Query()
	parsed.RawQuery = ""

	assert.Equal(t, fake.server.URL+"/auth", parsed.String())
	assert.Equal(t, "code", query.Get("response_type"))
	assert.Equal(t, testClientID, query.Get("client_id"))
	assert.Equal(t, testRedirectURL, query.Get("redirect_uri"))
	assert.Equal(t, "openid email profile", query.Get("scope"))
	assert.Equal(t, "select_account", query.Get("prompt"))
	assert.Equal(t, testState, query.Get("state"))
	assert.Equal(t, testNonce, query.Get("nonce"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, s256(testVerifier), query.Get("code_challenge"))
	assert.False(t, query.Has("access_type"), "google sign-in never asks for offline access")
	assert.False(t, query.Has("client_secret"))
	assert.False(t, query.Has("code_verifier"))
}

func TestComplete_ReturnsTheVerifiedIdentity(t *testing.T) {
	// given a flow google sends back with a valid id token
	fake := newFakeGoogle(t)
	adapter := newTestAdapter(t, fake, configuredSettings())
	flow, code := startFlow(t, fake, adapter)

	// when
	got, err := adapter.Complete(t.Context(), flow, code)

	// then
	require.NoError(t, err)
	assert.Equal(t, &model.ProviderIdentity{Provider: model.ProviderGoogle, Subject: testSubject, Email: testEmail, EmailAliases: []string{"ada@googlemail.com"}, EmailVerified: true, EmailAuthoritative: true, Name: testName}, got)
}

func TestComplete_RefusesAFlowThatDoesNotVerify(t *testing.T) {
	keys, err := testKeys()
	require.NoError(t, err)

	tests := []struct {
		name string
		plan tokenPlan
		flow func(flow *model.OIDCFlow)
		code string
		is   error
		says string
	}{
		{
			name: "a wrong audience",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["aud"] = "someone-else.apps.googleusercontent.com"
			}},
			says: "expected audience",
		},
		{
			name: "a wrong issuer",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["iss"] = "https://accounts.example.com"
			}},
			says: "issued by a different provider",
		},
		{
			name: "an expired token",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["iat"] = time.Now().Add(-2 * time.Hour).Unix()
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			}},
			says: "token is expired",
		},
		{
			name: "a token signed by an unknown key",
			plan: tokenPlan{signer: keys.stranger, keyID: "unknown-key"},
			says: "failed to verify signature",
		},
		{
			name: "a forged signature under google's key id",
			plan: tokenPlan{signer: keys.stranger},
			says: "failed to verify signature",
		},
		{
			name: "a wrong nonce",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["nonce"] = "nonce-of-another-flow-entirely-0123456789abcdef"
			}},
			is: identity.ErrNonceMismatch,
		},
		{
			name: "a missing nonce",
			plan: tokenPlan{claims: func(claims map[string]any) {
				delete(claims, "nonce")
			}},
			is: identity.ErrNonceMismatch,
		},
		{
			name: "a flow whose stored nonce is empty",
			plan: tokenPlan{claims: func(claims map[string]any) {
				delete(claims, "nonce")
			}},
			flow: func(flow *model.OIDCFlow) {
				flow.Nonce = ""
			},
			is: identity.ErrNonceMismatch,
		},
		{
			name: "a wrong pkce verifier",
			flow: func(flow *model.OIDCFlow) {
				flow.CodeVerifier = oauth2.GenerateVerifier()
			},
			says: "invalid_grant",
		},
		{
			name: "a token endpoint that answers invalid_grant",
			code: "a-code-google-never-issued",
			says: "invalid_grant",
		},
		{
			name: "a token answer without an id token",
			plan: tokenPlan{omitIDToken: true},
			is:   identity.ErrNoIDToken,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeGoogle(t)
			adapter := newTestAdapter(t, fake, configuredSettings())
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tc.plan)

			if tc.flow != nil {
				tc.flow(&flow)
			}

			if tc.code != "" {
				code = tc.code
			}

			// when
			got, err := adapter.Complete(t.Context(), flow, code)

			// then
			require.Error(t, err)
			assert.Nil(t, got)

			if tc.is != nil {
				assert.ErrorIs(t, err, tc.is)
			}

			if tc.says != "" {
				assert.ErrorContains(t, err, tc.says)
			}
		})
	}
}

func TestComplete_ReportsWhetherGoogleVerifiedTheAddress(t *testing.T) {
	tests := []struct {
		name   string
		claims func(claims map[string]any)
	}{
		{name: "email_verified false", claims: func(claims map[string]any) {
			claims["email_verified"] = false
		}},
		{name: "no email_verified claim", claims: func(claims map[string]any) {
			delete(claims, "email_verified")
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeGoogle(t)
			adapter := newTestAdapter(t, fake, configuredSettings())
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tokenPlan{claims: tc.claims})

			// when
			got, err := adapter.Complete(t.Context(), flow, code)

			// then the core refuses it on every path, so the adapter only reports it
			require.NoError(t, err)
			assert.False(t, got.EmailVerified)
			assert.Equal(t, testEmail, got.Email)
		})
	}
}

func TestComplete_DecidesWhetherGoogleIsAuthoritativeForTheAddress(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		hd      string
		want    bool
		aliases []string
	}{
		{name: "a gmail address", email: "ada@gmail.com", want: true, aliases: []string{"ada@googlemail.com"}},
		{name: "a gmail address in capitals", email: "Ada@GMail.com", want: true, aliases: []string{"Ada@googlemail.com"}},
		{name: "a googlemail address", email: "ada@googlemail.com", want: true, aliases: []string{"ada@gmail.com"}},
		{name: "a googlemail address in capitals", email: "Ada@GoogleMail.com", want: true, aliases: []string{"Ada@gmail.com"}},
		{name: "a workspace address carrying hd", email: "ada@example.org", hd: "example.org", want: true},
		{name: "a verified address on another domain without hd", email: "ada@example.org", want: false},
		{name: "a domain that only ends in gmail.com", email: "ada@notgmail.com", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeGoogle(t)
			adapter := newTestAdapter(t, fake, configuredSettings())
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tokenPlan{claims: func(claims map[string]any) {
				claims["email"] = tc.email

				if tc.hd != "" {
					claims["hd"] = tc.hd
				}
			}})

			// when
			got, err := adapter.Complete(t.Context(), flow, code)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.email, got.Email)
			assert.True(t, got.EmailVerified)
			assert.Equal(t, tc.want, got.EmailAuthoritative)
			assert.Equal(t, tc.aliases, got.EmailAliases)
		})
	}
}

func TestBegin_IsOffWhileEitherCredentialIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		blank []config.SiteSettingKey
	}{
		{name: "no client id", blank: []config.SiteSettingKey{config.SettingGoogleClientID.Key}},
		{name: "no client secret", blank: []config.SiteSettingKey{config.SettingGoogleClientSecret.Key}},
		{name: "neither", blank: []config.SiteSettingKey{config.SettingGoogleClientID.Key, config.SettingGoogleClientSecret.Key}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeGoogle(t)
			values := configuredSettings()
			for _, key := range tc.blank {
				values[key] = ""
			}

			adapter := newTestAdapter(t, fake, values)

			// when
			_, beginErr := adapter.Begin(t.Context(), testAuthorisation())
			_, completeErr := adapter.Complete(t.Context(), model.OIDCFlow{Nonce: testNonce, CodeVerifier: testVerifier}, "code")

			// then
			assert.ErrorIs(t, beginErr, identity.ErrProviderNotConfigured)
			assert.ErrorIs(t, completeErr, identity.ErrProviderNotConfigured)
			assert.Zero(t, fake.discoveries.Load(), "a switched-off adapter never reaches google")
		})
	}
}

func TestBegin_AnUnreadableSettingIsAnErrorRatherThanOff(t *testing.T) {
	// given
	fake := newFakeGoogle(t)
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingGoogleClientID).Return("", errSettingsDown).Once()
	adapter := &Adapter{settings: settingsSvc, issuer: fake.server.URL, client: fake.server.Client()}

	// when
	_, err := adapter.Begin(t.Context(), testAuthorisation())

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.NotErrorIs(t, err, identity.ErrProviderNotConfigured)
	assert.Zero(t, fake.discoveries.Load())
}

func TestBegin_DiscoversGoogleOnFirstUseAndOnlyOnce(t *testing.T) {
	// given an adapter that has not reached google yet
	fake := newFakeGoogle(t)
	adapter := newTestAdapter(t, fake, configuredSettings())
	require.Zero(t, fake.discoveries.Load())

	// when
	flow, code := startFlow(t, fake, adapter)
	_, completeErr := adapter.Complete(t.Context(), flow, code)
	_, beginErr := adapter.Begin(t.Context(), testAuthorisation())

	// then
	require.NoError(t, completeErr)
	require.NoError(t, beginErr)
	assert.Equal(t, int32(1), fake.discoveries.Load())
}

func TestBegin_ReportsAFailedDiscoveryAndTriesAgainNextTime(t *testing.T) {
	// given google's discovery document cannot be fetched
	fake := newFakeGoogle(t)
	fake.discoveryDown.Store(true)
	adapter := newTestAdapter(t, fake, configuredSettings())

	// when
	_, beginErr := adapter.Begin(t.Context(), testAuthorisation())
	_, completeErr := adapter.Complete(t.Context(), model.OIDCFlow{Nonce: testNonce, CodeVerifier: testVerifier}, "code")

	fake.discoveryDown.Store(false)
	authURL, recoveredErr := adapter.Begin(t.Context(), testAuthorisation())

	// then
	require.ErrorIs(t, beginErr, identity.ErrProviderUnavailable)
	assert.ErrorContains(t, beginErr, "discovery is down")
	require.ErrorIs(t, completeErr, identity.ErrProviderUnavailable)
	require.NoError(t, recoveredErr)
	assert.NotEmpty(t, authURL)
	assert.Equal(t, int32(3), fake.discoveries.Load())
}

func TestOnSettingChanged_DropsTheDiscoveredProviderForTheGoogleCredentials(t *testing.T) {
	tests := []struct {
		name string
		key  config.SiteSettingKey
		want int32
	}{
		{name: "the client id", key: config.SettingGoogleClientID.Key, want: 2},
		{name: "the client secret", key: config.SettingGoogleClientSecret.Key, want: 2},
		{name: "another setting", key: config.SettingSiteName.Key, want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given an adapter that has discovered google
			fake := newFakeGoogle(t)
			adapter := newTestAdapter(t, fake, configuredSettings())
			_, err := adapter.Begin(t.Context(), testAuthorisation())
			require.NoError(t, err)

			// when
			adapter.OnSettingChanged(tc.key, "changed")
			_, err = adapter.Begin(t.Context(), testAuthorisation())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, fake.discoveries.Load())
		})
	}
}

func TestComplete_FailsAFlowStartedBeforeTheClientWasReplaced(t *testing.T) {
	// given a flow started under one client, then the client replaced in the settings
	fake := newFakeGoogle(t)
	values := configuredSettings()
	adapter := newTestAdapter(t, fake, values)
	flow, code := startFlow(t, fake, adapter)

	values[config.SettingGoogleClientID.Key] = "replacement.apps.googleusercontent.com"
	adapter.OnSettingChanged(config.SettingGoogleClientID.Key, "replacement.apps.googleusercontent.com")

	// when
	got, err := adapter.Complete(t.Context(), flow, code)

	// then
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid_client")
	assert.Nil(t, got)
	assert.Equal(t, int32(2), fake.discoveries.Load())
}
