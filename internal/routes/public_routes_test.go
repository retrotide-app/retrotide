package routes

import (
	"net/http"
	"testing"
	"testing/fstest"

	"retrotide/internal/config"
	"retrotide/internal/controllers"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/health"
	"retrotide/internal/og"
	"retrotide/internal/sitemap"

	"github.com/stretchr/testify/assert"
)

const (
	testShell = "<html>shell</html>"
)

func newPublicHarness(t *testing.T) *testutil.Harness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	ctrl := &controllers.Service{
		HealthService: health.NewMockService(t),
		Guards:        h.Guards,
		StaticFS: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte(testShell)},
		},
		OGResolver:     og.NewResolver(og.Deps{Settings: h.SettingsService, DeploymentMode: config.DeploymentHosted}),
		SitemapService: sitemap.NewService(sitemap.Deps{Settings: h.SettingsService, DeploymentMode: config.DeploymentHosted}),
	}
	PublicRoutes(ctrl, h.Engine)

	return h
}

func TestPublicRoutes_MountsThePageRoutesAtTheRoot(t *testing.T) {
	// given
	h := newPublicHarness(t)

	// when
	status, body := h.NewRequest(http.MethodGet, "/livez").Do()

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"status":"ok"}`, string(body))
}

func TestPublicRoutes_ServesTheShellThroughNoRoute(t *testing.T) {
	// given
	h := newPublicHarness(t)

	// when
	rec := h.NewRequest(http.MethodGet, "/anything").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, testShell, rec.Body.String())
}

func TestPublicRoutes_ServesTheCrawlerFilesBeforeTheSPAHandlerSeesThem(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
	}{
		{path: "/robots.txt", contentType: "text/plain; charset=utf-8"},
		{path: "/sitemap.xml", contentType: "application/xml; charset=utf-8"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given neither file among the embedded files, where the spa handler would answer 404
			h := newPublicHarness(t)

			// when
			rec := h.NewRequest(http.MethodGet, tc.path).Record()

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tc.contentType, rec.Header().Get("Content-Type"))
		})
	}
}

func TestPublicRoutes_AnswersAnUnknownApiPathWithJSON404(t *testing.T) {
	// given
	h := newPublicHarness(t)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/nope").Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
}

func TestPublicRoutes_AnswersAWrongMethodWithJSON405(t *testing.T) {
	// given
	h := newPublicHarness(t)

	// when
	rec := h.NewRequest(http.MethodPost, "/livez").Record()

	// then
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.JSONEq(t, `{"error":"method not allowed"}`, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Allow"), http.MethodGet)
}
