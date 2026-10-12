package controllers

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"retrotide/internal/config"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/sitemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newSitemapHarness(t *testing.T, mode config.DeploymentMode) *testutil.Harness {
	t.Helper()

	h := testutil.NewHarness(t)

	s := &Service{
		Guards:         h.Guards,
		StaticFS:       fstest.MapFS{},
		SitemapService: sitemap.NewService(sitemap.Deps{Settings: h.SettingsService, DeploymentMode: mode}),
	}
	RegisterRoutes(h.Engine, s.getAllSitemapRoutes())
	h.Engine.NoRoute(s.SPA)

	return h
}

func TestSitemapController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine, s.getAllSitemapRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /robots.txt",
		"HEAD /robots.txt",
		"GET /sitemap.xml",
		"HEAD /sitemap.xml",
	}, registeredRoutes(h.Engine))
}

func TestSitemapController_ServesRobotsFromAPageRouteRatherThanTheEmbeddedFiles(t *testing.T) {
	// given no robots.txt among the embedded files, so the spa handler would answer 404
	h := newSitemapHarness(t, config.DeploymentHosted)
	h.UseBaseURL(testutil.TestBaseURL)

	// when
	rec := h.NewRequest(http.MethodGet, "/robots.txt").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "User-agent: *\nDisallow: /api/\n\nSitemap: https://retrotide.test/sitemap.xml\n", rec.Body.String())
}

func TestSitemapController_ServesRobotsThatDisallowEverythingWhenSelfHosted(t *testing.T) {
	// given
	h := newSitemapHarness(t, config.DeploymentSelfHosted)

	// when
	rec := h.NewRequest(http.MethodGet, "/robots.txt").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "User-agent: *\nDisallow: /\n", rec.Body.String())
}

func TestSitemapController_ServesTheSitemap(t *testing.T) {
	// given
	h := newSitemapHarness(t, config.DeploymentHosted)
	h.UseBaseURL(testutil.TestBaseURL)

	// when
	rec := h.NewRequest(http.MethodGet, "/sitemap.xml").Record()

	// then
	body := rec.Body.String()
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/xml; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.True(t, strings.HasPrefix(body, `<?xml version="1.0" encoding="UTF-8"?>`))
	assert.Contains(t, body, "<loc>https://retrotide.test/pricing</loc>")
	assert.Contains(t, body, "<loc>https://retrotide.test/templates/sailboat</loc>")
	assert.NotContains(t, body, "/templates/blank")
}

func TestSitemapController_ServesAnEmptySitemapWhenSelfHosted(t *testing.T) {
	// given
	h := newSitemapHarness(t, config.DeploymentSelfHosted)

	// when
	rec := h.NewRequest(http.MethodGet, "/sitemap.xml").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "<url>")
}

func TestSitemapController_AnswersAnUnreadableBaseURLWith500(t *testing.T) {
	tests := []struct {
		path string
		body string
	}{
		{path: "/robots.txt", body: `{"error":"failed to build robots.txt"}`},
		{path: "/sitemap.xml", body: `{"error":"failed to build the sitemap"}`},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given
			h := newSitemapHarness(t, config.DeploymentHosted)
			h.SettingsService.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errServiceDown)

			// when
			rec := h.NewRequest(http.MethodGet, tc.path).Record()

			// then
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}
