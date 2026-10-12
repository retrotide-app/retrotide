package microsoft

import (
	"net/url"
	"strings"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/identity"
	"retrotide/internal/model"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNew_ReachesNoOneUntilFirstUse(t *testing.T) {
	// given settings that fail the test if the adapter reads them
	settingsSvc := settings.NewMockService(t)

	// when
	adapter := New(settingsSvc)

	// then
	assert.Equal(t, microsoftAuthority, adapter.authority)
	assert.Equal(t, clientTimeout, adapter.client.Timeout)
	assert.Empty(t, adapter.discovered)
}

func TestAdapter_NamesItselfForTheCore(t *testing.T) {
	// given
	adapter := New(settings.NewMockService(t))

	// when
	id, name := adapter.ID(), adapter.Name()

	// then
	assert.Equal(t, model.ProviderMicrosoft, id)
	assert.Equal(t, "Microsoft", name)
}

func TestEnabled_NeedsBothMicrosoftCredentials(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
		secret   *string
		want     bool
	}{
		{name: "both set", clientID: testClientID, secret: new("a-secret"), want: true},
		{name: "no client id", clientID: "", want: false},
		{name: "a blank client id", clientID: "   ", want: false},
		{name: "no client secret", clientID: testClientID, secret: new(""), want: false},
		{name: "a blank client secret", clientID: testClientID, secret: new("  "), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			settingsSvc := settings.NewMockService(t)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingMicrosoftClientID).Return(tc.clientID, nil)
			if tc.secret != nil {
				settingsSvc.EXPECT().Get(mock.Anything, config.SettingMicrosoftClientSecret).Return(*tc.secret, nil)
			}

			adapter := New(settingsSvc)

			// when
			enabled, err := adapter.Enabled(t.Context())

			// then microsoft is never reached to answer
			require.NoError(t, err)
			assert.Equal(t, tc.want, enabled)
		})
	}
}

func TestEnabled_ReportsASettingThatCannotBeRead(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingMicrosoftClientID).Return("", errSettingsDown)
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
		{name: "both set", values: map[config.SiteSettingKey]string{config.SettingMicrosoftClientID.Key: testClientID, config.SettingMicrosoftClientSecret.Key: testClientSecret}, want: true},
		{name: "no client id", values: map[config.SiteSettingKey]string{config.SettingMicrosoftClientSecret.Key: testClientSecret}, want: false},
		{name: "a blank client secret", values: map[config.SiteSettingKey]string{config.SettingMicrosoftClientID.Key: testClientID, config.SettingMicrosoftClientSecret.Key: "  "}, want: false},
		{name: "only google configured", values: map[config.SiteSettingKey]string{config.SettingGoogleClientID.Key: "client", config.SettingGoogleClientSecret.Key: "secret"}, want: false},
		{name: "nothing", values: map[config.SiteSettingKey]string{}, want: false},
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
	fake := newFakeMicrosoft(t)
	adapter := newTestAdapter(t, fake, configuredSettings())

	// when
	authURL, err := adapter.Begin(t.Context(), testAuthorisation())

	// then the work-account endpoint is used and the core's state, nonce and verifier are the ones microsoft sees
	require.NoError(t, err)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	query := parsed.Query()
	parsed.RawQuery = ""

	assert.Equal(t, fake.server.URL+"/organizations/oauth2/v2.0/authorize", parsed.String())
	assert.Equal(t, "code", query.Get("response_type"))
	assert.Equal(t, testClientID, query.Get("client_id"))
	assert.Equal(t, testRedirectURL, query.Get("redirect_uri"))
	assert.Equal(t, "openid email profile", query.Get("scope"))
	assert.Equal(t, "select_account", query.Get("prompt"))
	assert.Equal(t, testState, query.Get("state"))
	assert.Equal(t, testNonce, query.Get("nonce"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, s256(testVerifier), query.Get("code_challenge"))
	assert.False(t, query.Has("client_secret"))
	assert.False(t, query.Has("code_verifier"))
}

func TestBegin_PicksTheAuthorityFromTheSettings(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		personal string
		want     string
	}{
		{name: "work accounts from any organisation", tenantID: "", personal: "false", want: "organizations"},
		{name: "work and personal accounts", tenantID: "", personal: "true", want: "common"},
		{name: "one tenant", tenantID: testTenantID, personal: "false", want: testTenantID},
		{name: "one tenant ignores the personal switch", tenantID: testTenantID, personal: "true", want: testTenantID},
		{name: "one tenant written in capitals", tenantID: "72F988BF-86F1-41AF-91AB-2D7CD011DB47", personal: "false", want: testTenantID},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeMicrosoft(t)
			values := configuredSettings()
			values[config.SettingMicrosoftTenantID.Key] = tc.tenantID
			values[config.SettingMicrosoftPersonalAccounts.Key] = tc.personal
			adapter := newTestAdapter(t, fake, values)

			// when
			authURL, err := adapter.Begin(t.Context(), testAuthorisation())

			// then
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, fake.discoveredSegments())

			parsed, err := url.Parse(authURL)
			require.NoError(t, err)
			assert.Equal(t, "/"+tc.want+"/oauth2/v2.0/authorize", parsed.Path)
		})
	}
}

func TestComplete_SignsInAWorkAccount(t *testing.T) {
	// given a work account from an organisation, through the organizations endpoint
	fake := newFakeMicrosoft(t)
	adapter := newTestAdapter(t, fake, configuredSettings())
	flow, code := startFlow(t, fake, adapter)

	// when
	got, err := adapter.Complete(t.Context(), flow, code)

	// then the subject is the tenant and object id, never the pairwise sub
	require.NoError(t, err)
	assert.Equal(t, &model.ProviderIdentity{
		Provider:           model.ProviderMicrosoft,
		Subject:            testTenantID + ":" + testObjectID,
		Email:              testEmail,
		EmailVerified:      true,
		EmailAuthoritative: true,
		Name:               testName,
	}, got)
}

func TestComplete_LowersTheCaseOfTheSubject(t *testing.T) {
	// given
	fake := newFakeMicrosoft(t)
	adapter := newTestAdapter(t, fake, configuredSettings())
	flow, code := startFlow(t, fake, adapter)
	fake.issue(tokenPlan{claims: func(claims map[string]any) {
		claims["oid"] = "3D2E4F60-7A8B-4C9D-8E0F-1A2B3C4D5E6F"
	}})

	// when
	got, err := adapter.Complete(t.Context(), flow, code)

	// then
	require.NoError(t, err)
	assert.Equal(t, testTenantID+":"+testObjectID, got.Subject)
}

func TestComplete_AdmitsOnlyTheConfiguredTenant(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		tid      string
		allowed  bool
	}{
		{name: "the configured tenant", tenantID: testTenantID, tid: testTenantID, allowed: true},
		{name: "the configured tenant written in capitals", tenantID: "72F988BF-86F1-41AF-91AB-2D7CD011DB47", tid: testTenantID, allowed: true},
		{name: "another tenant", tenantID: testTenantID, tid: testOtherTenantID, allowed: false},
		{name: "a personal account", tenantID: testTenantID, tid: personalTenantID, allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeMicrosoft(t)
			values := configuredSettings()
			values[config.SettingMicrosoftTenantID.Key] = tc.tenantID
			values[config.SettingMicrosoftPersonalAccounts.Key] = "true"
			adapter := newTestAdapter(t, fake, values)
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tokenPlan{claims: func(claims map[string]any) {
				claims["tid"] = tc.tid
				claims["iss"] = fake.issuerFor(tc.tid)
			}})

			// when
			got, err := adapter.Complete(t.Context(), flow, code)

			// then
			if tc.allowed {
				require.NoError(t, err)
				assert.Equal(t, tc.tid+":"+testObjectID, got.Subject)

				return
			}

			require.ErrorIs(t, err, identity.ErrAccountNotAllowed)
			assert.Nil(t, got)
		})
	}
}

func TestComplete_AdmitsPersonalAccountsOnlyWhileTheSwitchIsOn(t *testing.T) {
	tests := []struct {
		name     string
		personal string
		allowed  bool
	}{
		{name: "the switch on", personal: "true", allowed: true},
		{name: "the switch off", personal: "false", allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a personal microsoft account
			fake := newFakeMicrosoft(t)
			values := configuredSettings()
			values[config.SettingMicrosoftPersonalAccounts.Key] = tc.personal
			adapter := newTestAdapter(t, fake, values)
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tokenPlan{claims: func(claims map[string]any) {
				claims["tid"] = personalTenantID
				claims["iss"] = fake.issuerFor(personalTenantID)
				claims["email"] = "ada@outlook.example"
				delete(claims, "xms_edov")
			}})

			// when
			got, err := adapter.Complete(t.Context(), flow, code)

			// then
			if tc.allowed {
				require.NoError(t, err)
				assert.Equal(t, personalTenantID+":"+testObjectID, got.Subject)
				assert.True(t, got.EmailVerified)
				assert.False(t, got.EmailAuthoritative)

				return
			}

			require.ErrorIs(t, err, identity.ErrAccountNotAllowed)
			assert.ErrorContains(t, err, "personal Microsoft accounts")
			assert.Nil(t, got)
		})
	}
}

func TestComplete_RefusesAFlowThatDoesNotVerify(t *testing.T) {
	keys, err := testKeys()
	require.NoError(t, err)

	tests := []struct {
		name string
		plan tokenPlan
		flow func(flow *model.OIDCFlow)
		is   error
		says string
	}{
		{
			name: "a wrong audience",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["aud"] = "another-application"
			}},
			says: "expected audience",
		},
		{
			name: "a token signed by an unknown key",
			plan: tokenPlan{signer: keys.stranger, keyID: "unknown-key"},
			says: "failed to verify signature",
		},
		{
			name: "a forged signature under entra's key id",
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
			name: "a token answer without an id token",
			plan: tokenPlan{omitIDToken: true},
			is:   identity.ErrNoIDToken,
		},
		{
			name: "an issuer that is not the token's own directory",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["iss"] = "https://login.microsoftonline.com/" + testTenantID + "/v2.0"
			}},
			is: identity.ErrAccountNotAllowed,
		},
		{
			name: "an issuer naming another directory",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["iss"] = strings.Replace(claims["iss"].(string), testTenantID, testOtherTenantID, 1)
			}},
			is: identity.ErrAccountNotAllowed,
		},
		{
			name: "a missing tid",
			plan: tokenPlan{claims: func(claims map[string]any) {
				delete(claims, "tid")
			}},
			is: identity.ErrAccountNotAllowed,
		},
		{
			name: "a tid that is not a guid",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["tid"] = "contoso.example"
			}},
			is: identity.ErrAccountNotAllowed,
		},
		{
			name: "a missing oid",
			plan: tokenPlan{claims: func(claims map[string]any) {
				delete(claims, "oid")
			}},
			is: identity.ErrAccountNotAllowed,
		},
		{
			name: "an unreadable xms_edov",
			plan: tokenPlan{claims: func(claims map[string]any) {
				claims["xms_edov"] = "perhaps"
			}},
			is: errUnreadableFlag,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeMicrosoft(t)
			adapter := newTestAdapter(t, fake, configuredSettings())
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tc.plan)

			if tc.flow != nil {
				tc.flow(&flow)
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

func TestComplete_DecidesWhetherMicrosoftIsAuthoritativeForTheAddress(t *testing.T) {
	tests := []struct {
		name  string
		edov  any
		omit  bool
		email string
		want  bool
	}{
		{name: "xms_edov true", edov: true, want: true},
		{name: "xms_edov as the string true", edov: "true", want: true},
		{name: "xms_edov as the string 1", edov: "1", want: true},
		{name: "xms_edov false", edov: false, want: false},
		{name: "xms_edov as the string false", edov: "false", want: false},
		{name: "no xms_edov claim", omit: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeMicrosoft(t)
			adapter := newTestAdapter(t, fake, configuredSettings())
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tokenPlan{claims: func(claims map[string]any) {
				claims["xms_edov"] = tc.edov

				if tc.omit {
					delete(claims, "xms_edov")
				}
			}})

			// when
			got, err := adapter.Complete(t.Context(), flow, code)

			// then
			require.NoError(t, err)
			assert.Equal(t, testEmail, got.Email)
			assert.True(t, got.EmailVerified)
			assert.Equal(t, tc.want, got.EmailAuthoritative)
		})
	}
}

func TestComplete_FallsBackToThePreferredUsername(t *testing.T) {
	tests := []struct {
		name              string
		preferredUsername string
		wantEmail         string
	}{
		{name: "an address", preferredUsername: "ada@fabrikam.example", wantEmail: "ada@fabrikam.example"},
		{name: "a phone number", preferredUsername: "+44 20 7946 0000", wantEmail: ""},
		{name: "nothing", preferredUsername: "", wantEmail: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given an id token without the optional email claim, carrying xms_edov anyway
			fake := newFakeMicrosoft(t)
			adapter := newTestAdapter(t, fake, configuredSettings())
			flow, code := startFlow(t, fake, adapter)
			fake.issue(tokenPlan{claims: func(claims map[string]any) {
				delete(claims, "email")
				claims["preferred_username"] = tc.preferredUsername
			}})

			// when
			got, err := adapter.Complete(t.Context(), flow, code)

			// then the fallback is verified when present, but never authoritative
			require.NoError(t, err)
			assert.Equal(t, tc.wantEmail, got.Email)
			assert.Equal(t, tc.wantEmail != "", got.EmailVerified)
			assert.False(t, got.EmailAuthoritative)
		})
	}
}

func TestBegin_IsOffWhileEitherCredentialIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		blank []config.SiteSettingKey
	}{
		{name: "no client id", blank: []config.SiteSettingKey{config.SettingMicrosoftClientID.Key}},
		{name: "no client secret", blank: []config.SiteSettingKey{config.SettingMicrosoftClientSecret.Key}},
		{name: "neither", blank: []config.SiteSettingKey{config.SettingMicrosoftClientID.Key, config.SettingMicrosoftClientSecret.Key}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake := newFakeMicrosoft(t)
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
			assert.Zero(t, fake.discoveries.Load(), "a switched-off adapter never reaches microsoft")
		})
	}
}

func TestBegin_AnUnreadableSettingIsAnErrorRatherThanOff(t *testing.T) {
	// given
	fake := newFakeMicrosoft(t)
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingMicrosoftClientID).Return("", errSettingsDown).Once()
	adapter := &Adapter{settings: settingsSvc, authority: fake.server.URL, client: fake.server.Client()}

	// when
	_, err := adapter.Begin(t.Context(), testAuthorisation())

	// then
	require.ErrorIs(t, err, errSettingsDown)
	assert.NotErrorIs(t, err, identity.ErrProviderNotConfigured)
	assert.Zero(t, fake.discoveries.Load())
}

func TestBegin_DiscoversMicrosoftOnFirstUseAndOnlyOnce(t *testing.T) {
	// given an adapter that has not reached microsoft yet
	fake := newFakeMicrosoft(t)
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
	// given entra's discovery document cannot be fetched
	fake := newFakeMicrosoft(t)
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

func TestOnSettingChanged_DropsTheDiscoveredProviderForTheMicrosoftSettings(t *testing.T) {
	tests := []struct {
		name string
		key  config.SiteSettingKey
		want int32
	}{
		{name: "the client id", key: config.SettingMicrosoftClientID.Key, want: 2},
		{name: "the client secret", key: config.SettingMicrosoftClientSecret.Key, want: 2},
		{name: "the tenant id", key: config.SettingMicrosoftTenantID.Key, want: 2},
		{name: "the personal accounts switch", key: config.SettingMicrosoftPersonalAccounts.Key, want: 2},
		{name: "a google setting", key: config.SettingGoogleClientID.Key, want: 1},
		{name: "another setting", key: config.SettingSiteName.Key, want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given an adapter that has discovered microsoft
			fake := newFakeMicrosoft(t)
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

func TestBegin_CachesEachAuthorityOnItsOwn(t *testing.T) {
	// given an adapter that has discovered the organizations endpoint
	fake := newFakeMicrosoft(t)
	values := configuredSettings()
	adapter := newTestAdapter(t, fake, values)
	_, err := adapter.Begin(t.Context(), testAuthorisation())
	require.NoError(t, err)

	// when the tenant is pinned and the change is announced
	values[config.SettingMicrosoftTenantID.Key] = testTenantID
	adapter.OnSettingChanged(config.SettingMicrosoftTenantID.Key, testTenantID)
	_, firstErr := adapter.Begin(t.Context(), testAuthorisation())
	_, secondErr := adapter.Begin(t.Context(), testAuthorisation())

	// then the tenant is discovered once and reused
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.Equal(t, []string{"organizations", testTenantID}, fake.discoveredSegments())
}
