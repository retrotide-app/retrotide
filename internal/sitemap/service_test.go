package sitemap

import (
	"encoding/xml"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testBaseURL = "https://retrotide.test"

	hostedSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://retrotide.test/</loc>
  </url>
  <url>
    <loc>https://retrotide.test/pricing</loc>
  </url>
  <url>
    <loc>https://retrotide.test/compare/easyretro</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates</loc>
  </url>
  <url>
    <loc>https://retrotide.test/legal/privacy</loc>
  </url>
  <url>
    <loc>https://retrotide.test/legal/terms</loc>
  </url>
  <url>
    <loc>https://retrotide.test/legal/acceptable-use</loc>
  </url>
  <url>
    <loc>https://retrotide.test/legal/cookies</loc>
  </url>
  <url>
    <loc>https://retrotide.test/legal/sub-processors</loc>
  </url>
  <url>
    <loc>https://retrotide.test/legal/security</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/start-stop-continue</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/mad-sad-glad</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/went-well</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/four-ls</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/sailboat</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/starfish</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/kalm</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/daki</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/rose-thorn-bud</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/wrap</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/lean-coffee</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/three-little-pigs</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/hot-air-balloon</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/speed-car</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/mountain-climb</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/desert-island</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/weather-report</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/traffic-lights</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/plus-delta</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/keep-drop-add</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/good-bad-ugly</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/fun-done-learn</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/highs-lows</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/timeline</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/learning-matrix</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/what-so-what-now-what</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/lessons-learned</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/post-mortem</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/incident-review</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/pre-mortem</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/swot</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/pros-cons</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/six-thinking-hats</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/kudos</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/daily-standup</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/kanban</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/quarter-in-review</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/expectations-reality</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/brainstorm</loc>
  </url>
  <url>
    <loc>https://retrotide.test/templates/hopes-and-fears</loc>
  </url>
</urlset>
`

	selfHostedSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>
`
)

func newTestService(t *testing.T, mode config.DeploymentMode, baseURL string) Service {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	if baseURL != "" {
		settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(baseURL, nil)
	}

	return NewService(Deps{Settings: settingsSvc, DeploymentMode: mode})
}

func TestSitemap_ListsTheMarketingLegalAndPublicTemplatePagesAsAbsoluteURLs(t *testing.T) {
	// given
	svc := newTestService(t, config.DeploymentHosted, testBaseURL)

	// when
	body, err := svc.Sitemap(t.Context())

	// then boards never appear and blank has no public page
	require.NoError(t, err)
	assert.Equal(t, hostedSitemap, string(body))
	assert.NotContains(t, string(body), "/templates/blank")
	assert.NotContains(t, string(body), "/b/")
	assert.NotContains(t, string(body), "/boards/")
}

func TestSitemap_TrimsTrailingSlashesFromTheBaseURL(t *testing.T) {
	// given
	svc := newTestService(t, config.DeploymentHosted, testBaseURL+"//")

	// when
	body, err := svc.Sitemap(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, hostedSitemap, string(body))
}

func TestSitemap_EscapesTheBaseURLForXML(t *testing.T) {
	// given
	svc := newTestService(t, config.DeploymentHosted, "https://retrotide.test/a&b")

	// when
	body, err := svc.Sitemap(t.Context())

	// then the document still parses and carries the url as written
	require.NoError(t, err)
	assert.Contains(t, string(body), "<loc>https://retrotide.test/a&amp;b/pricing</loc>")

	var parsed urlSet
	require.NoError(t, xml.Unmarshal(body, &parsed))
	assert.Equal(t, "https://retrotide.test/a&b/", parsed.URLs[0].Location)
}

func TestSitemap_IsAnEmptyURLSetWhenSelfHosted(t *testing.T) {
	// given a self-hosted instance, whose base url is never read, which the strict settings mock would report
	svc := newTestService(t, config.DeploymentSelfHosted, "")

	// when
	body, err := svc.Sitemap(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, selfHostedSitemap, string(body))
}

func TestSitemap_ReportsAnUnreadableBaseURL(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)
	svc := NewService(Deps{Settings: settingsSvc, DeploymentMode: config.DeploymentHosted})

	// when
	body, err := svc.Sitemap(t.Context())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, body)
}

func TestRobots_DisallowsTheAPIAndNamesTheSitemap(t *testing.T) {
	// given
	svc := newTestService(t, config.DeploymentHosted, testBaseURL+"/")

	// when
	body, err := svc.Robots(t.Context())

	// then the board, share, organisation and invite paths stay crawlable, so a crawler sees their noindex
	require.NoError(t, err)
	assert.Equal(t, "User-agent: *\nDisallow: /api/\n\nSitemap: https://retrotide.test/sitemap.xml\n", string(body))
}

func TestRobots_DisallowsEverythingWhenSelfHosted(t *testing.T) {
	// given
	svc := newTestService(t, config.DeploymentSelfHosted, "")

	// when
	body, err := svc.Robots(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, "User-agent: *\nDisallow: /\n", string(body))
}

func TestRobots_ReportsAnUnreadableBaseURL(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)
	svc := NewService(Deps{Settings: settingsSvc, DeploymentMode: config.DeploymentHosted})

	// when
	body, err := svc.Robots(t.Context())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, body)
}
