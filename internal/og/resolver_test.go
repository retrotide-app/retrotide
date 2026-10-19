package og

import (
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testBaseURL = "https://retrotide.test"
)

func newTestResolver(t *testing.T, mode config.DeploymentMode, baseURL string) Resolver {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(baseURL, nil)

	return NewResolver(Deps{Settings: settingsSvc, DeploymentMode: mode})
}

func TestResolve_GivesEveryMarketingAndLegalPageItsOwnCopy(t *testing.T) {
	tests := []struct {
		path  string
		title string
	}{
		{path: "/", title: "RetroTide: sprint retrospectives your team will use"},
		{path: "/compare/easyretro", title: "RetroTide and EasyRetro compared | RetroTide"},
		{path: "/self-host", title: "Self-host RetroTide | RetroTide"},
		{path: "/templates", title: "Retrospective templates | RetroTide"},
		{path: "/legal/privacy", title: "Privacy notice | RetroTide"},
		{path: "/legal/terms", title: "Terms of service | RetroTide"},
		{path: "/legal/acceptable-use", title: "Acceptable use policy | RetroTide"},
		{path: "/legal/cookies", title: "Cookie statement | RetroTide"},
		{path: "/legal/sub-processors", title: "Sub-processors | RetroTide"},
		{path: "/legal/security", title: "Security | RetroTide"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentHosted, testBaseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), tc.path)

			// then the page is indexable under its own canonical url
			require.NoError(t, err)
			assert.Equal(t, tc.title, meta.Title)
			assert.NotEmpty(t, meta.Description)
			assert.Empty(t, meta.Robots)
			assert.Equal(t, testBaseURL+tc.path, meta.Canonical)
			assert.Equal(t, testBaseURL, meta.BaseURL)
		})
	}
}

func TestResolve_MatchesMarketingPathsCaseInsensitively(t *testing.T) {
	tests := []struct {
		path      string
		canonical string
	}{
		{path: "/SELF-HOST", canonical: testBaseURL + "/self-host"},
		{path: "/Legal/Privacy/", canonical: testBaseURL + "/legal/privacy"},
		{path: "//compare//EasyRetro", canonical: testBaseURL + "/compare/easyretro"},
		{path: "/Templates/", canonical: testBaseURL + "/templates"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentHosted, testBaseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), tc.path)

			// then every spelling the router accepts names the one canonical page
			require.NoError(t, err)
			assert.NotEmpty(t, meta.Title)
			assert.Equal(t, tc.canonical, meta.Canonical)
		})
	}
}

func TestResolve_GivesATemplatePageItsCatalogueMeta(t *testing.T) {
	tests := []string{"/templates/sailboat", "/Templates/sailboat", "/templates/sailboat/"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentHosted, testBaseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), path)

			// then
			require.NoError(t, err)
			assert.Equal(t, Meta{
				BaseURL:     testBaseURL,
				Canonical:   testBaseURL + "/templates/sailboat",
				Title:       "Sailboat retrospective template | RetroTide",
				Description: "Picture the team as a boat: what drives it, what drags it and what lies ahead. Columns: Wind, Anchors, Rocks and Island.",
			}, meta)
		})
	}
}

func TestResolve_KeepsTheDefaultMetaForATemplateWithNoPublicPage(t *testing.T) {
	tests := []string{"/templates/nope", "/templates/blank", "/templates/Sailboat", "/templates/sailboat/columns"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentHosted, testBaseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), path)

			// then an unknown key is no error, only a page the resolver has no copy for
			require.NoError(t, err)
			assert.Equal(t, Meta{BaseURL: testBaseURL, Canonical: testBaseURL + path}, meta)
		})
	}
}

func TestResolve_GivesSharedPathsTheGenericMetaAndNoIndex(t *testing.T) {
	tests := []struct {
		path      string
		canonical string
	}{
		{path: "/b/k3v9x2QpLmNoRsTu", canonical: "/b/k3v9x2QpLmNoRsTu"},
		{path: "/B/k3v9x2QpLmNoRsTu", canonical: "/b/k3v9x2QpLmNoRsTu"},
		{path: "/b", canonical: "/b"},
		{path: "/BOARDS/8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b/summary", canonical: "/boards/8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b/summary"},
		{path: "/o/Acme/teams", canonical: "/o/Acme/teams"},
		{path: "/invite", canonical: "/invite"},
		{path: "/Invite/", canonical: "/invite"},
	}

	for _, mode := range []config.DeploymentMode{config.DeploymentHosted, config.DeploymentSelfHosted} {
		for _, tc := range tests {
			t.Run(string(mode)+" "+tc.path, func(t *testing.T) {
				// given
				resolver := newTestResolver(t, mode, testBaseURL)

				// when
				meta, err := resolver.Resolve(t.Context(), tc.path)

				// then a link unfurled by a third party never names the board, and its slug keeps its case
				require.NoError(t, err)
				assert.Equal(t, Meta{
					BaseURL:     testBaseURL,
					Canonical:   testBaseURL + tc.canonical,
					Title:       "Join a retrospective on RetroTide",
					Description: "Someone shared a RetroTide retrospective with you. Open the link to join it.",
					Robots:      "noindex, nofollow",
				}, meta)
			})
		}
	}
}

func TestResolve_KeepsTheDefaultMetaForEveryOtherPath(t *testing.T) {
	tests := []string{"/bo", "/boardsx", "/dashboard", "/login", "/Settings/Profile"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentHosted, testBaseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), path)

			// then
			require.NoError(t, err)
			assert.Equal(t, Meta{BaseURL: testBaseURL, Canonical: testBaseURL + path}, meta)
		})
	}
}

func TestResolve_ServesTheDefaultMetaForTheHiddenPricingPage(t *testing.T) {
	// given
	resolver := newTestResolver(t, config.DeploymentHosted, testBaseURL)

	// when
	meta, err := resolver.Resolve(t.Context(), "/pricing")

	// then pricing is not described while it is hidden
	require.NoError(t, err)
	assert.Equal(t, Meta{BaseURL: testBaseURL, Canonical: testBaseURL + "/pricing"}, meta)
}

func TestResolve_ServesTheDefaultMetaForMarketingPathsWhenSelfHosted(t *testing.T) {
	tests := []string{"/", "/pricing", "/compare/easyretro", "/templates", "/templates/sailboat", "/legal/privacy", "/legal/security"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentSelfHosted, testBaseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), path)

			// then a self-hosted instance offers no marketing page to describe
			require.NoError(t, err)
			assert.Equal(t, Meta{BaseURL: testBaseURL, Canonical: testBaseURL + path}, meta)
		})
	}
}

func TestResolve_BuildsTheCanonicalFromTheBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		path      string
		wantBase  string
		canonical string
	}{
		{name: "a trailing slash", baseURL: "https://retrotide.test/", path: "/pricing", wantBase: "https://retrotide.test", canonical: "https://retrotide.test/pricing"},
		{name: "a base path", baseURL: "https://example.test/retro", path: "/pricing", wantBase: "https://example.test/retro", canonical: "https://example.test/retro/pricing"},
		{name: "the front door", baseURL: "http://localhost:4323", path: "/", wantBase: "http://localhost:4323", canonical: "http://localhost:4323/"},
		{name: "an empty path", baseURL: "https://retrotide.test", path: "", wantBase: "https://retrotide.test", canonical: "https://retrotide.test/"},
		{name: "characters a url must escape", baseURL: "https://retrotide.test", path: `/a b"<x>`, wantBase: "https://retrotide.test", canonical: "https://retrotide.test/a%20b%22%3Cx%3E"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentHosted, tc.baseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), tc.path)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.wantBase, meta.BaseURL)
			assert.Equal(t, tc.canonical, meta.Canonical)
		})
	}
}

func TestResolve_ReportsABaseURLThatCannotBeRead(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)
	resolver := NewResolver(Deps{Settings: settingsSvc, DeploymentMode: config.DeploymentHosted})

	// when
	_, err := resolver.Resolve(t.Context(), "/pricing")

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
}

func TestListOf_JoinsTitlesAsProse(t *testing.T) {
	tests := []struct {
		items []string
		want  string
	}{
		{items: []string{"Notes"}, want: "Notes"},
		{items: []string{"Good", "Bad"}, want: "Good and Bad"},
		{items: []string{"Start", "Stop", "Continue"}, want: "Start, Stop and Continue"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			// given
			items := tc.items

			// when
			got := listOf(items)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
