package controllers

import (
	"mime"
	"net/http"
	"testing"
	"testing/fstest"

	"retrotide/internal/config"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/og"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testShell        = "<html>shell</html>"
	testAppScript    = "console.log(1)"
	testFavicon      = `<svg xmlns="http://www.w3.org/2000/svg"></svg>`
	wantNotFoundBody = `{"error":"not found"}`
)

func testStaticFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":        &fstest.MapFile{Data: []byte(testShell)},
		"assets/app-abc.js": &fstest.MapFile{Data: []byte(testAppScript)},
		"favicon.svg":       &fstest.MapFile{Data: []byte(testFavicon)},
	}
}

func newStaticHarness(t *testing.T) *testutil.Harness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	mountStatic(h, testStaticFS())

	return h
}

func mountStatic(h *testutil.Harness, files fstest.MapFS) {
	s := &Service{Guards: h.Guards, StaticFS: files, OGResolver: og.NewResolver(og.Deps{Settings: h.SettingsService, DeploymentMode: config.DeploymentHosted})}
	RegisterRoutes(h.Engine, s.getAllStaticRoutes())
	h.Engine.NoRoute(s.SPA)
}

func TestStaticController_Assets_ServesTheHashedFile(t *testing.T) {
	// given
	h := newStaticHarness(t)

	// when
	status, body := h.NewRequest(http.MethodGet, "/assets/app-abc.js").Do()

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, testAppScript, string(body))
}

func TestStaticController_Assets_AnswersJSON404ForAMissingFileOrADirectory(t *testing.T) {
	tests := []string{"/assets/missing.js", "/assets/"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			h := newStaticHarness(t)

			// when
			status, body := h.NewRequest(http.MethodGet, path).Do()

			// then a missing hashed file is never answered with the shell or a listing
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, wantNotFoundBody, string(body))
		})
	}
}

func TestStaticController_AnswersJSON404ForADotDotSegment(t *testing.T) {
	tests := []string{"/assets/x/../app-abc.js", "/x/../favicon.svg", "/assets/x\\..\\app-abc.js", "/x/../pricing", "/b/../x"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			h := newStaticHarness(t)

			// when
			rec := h.NewRequest(http.MethodGet, path).Record()

			// then net/http's plain-text 400 never reaches the client
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, wantNotFoundBody, rec.Body.String())
		})
	}
}

func TestStaticController_ServesTheWholeFileForAnUnsatisfiableRange(t *testing.T) {
	tests := []string{"/assets/app-abc.js", "/favicon.svg"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			h := newStaticHarness(t)

			// when
			rec := h.NewRequest(http.MethodGet, path).WithHeader("Range", "bytes=9999-").Record()

			// then the range is ignored rather than answered with net/http's plain-text 416
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.NotEmpty(t, rec.Body.String())
			assert.Empty(t, rec.Header().Get("Content-Range"))
		})
	}
}

func TestStaticController_SPA_ServesAnEmbeddedFileForAPathWithAnExtension(t *testing.T) {
	// given
	h := newStaticHarness(t)

	// when
	status, body := h.NewRequest(http.MethodGet, "/favicon.svg").Do()

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, testFavicon, string(body))
}

func TestStaticController_SPA_AnswersJSON404ForAMissingFileWithAnExtension(t *testing.T) {
	// given
	h := newStaticHarness(t)

	// when
	status, body := h.NewRequest(http.MethodGet, "/missing.png").Do()

	// then
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, wantNotFoundBody, string(body))
}

func TestStaticController_SPA_ServesTheShellForEveryPage(t *testing.T) {
	tests := []string{"/", "/board/x", "/pricing/"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			h := newStaticHarness(t)

			// when
			rec := h.NewRequest(http.MethodGet, path).Record()

			// then NoRoute starts at 404, so the shell must go out with an explicit 200
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
			assert.Equal(t, testShell, rec.Body.String())
		})
	}
}

func TestStaticController_SPA_AnswersJSON500WhenTheShellCannotBeRead(t *testing.T) {
	// given an embedded build with no shell
	h := testutil.NewHarness(t)
	mountStatic(h, fstest.MapFS{"favicon.svg": &fstest.MapFile{Data: []byte(testFavicon)}})

	// when
	status, body := h.NewRequest(http.MethodGet, "/pricing").Do()

	// then a broken build is reported, never answered with a stand-in page
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.JSONEq(t, `{"error":"read the embedded shell"}`, string(body))
}

func TestStaticController_SPA_AnswersJSON500WhenTheBaseURLCannotBeRead(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errServiceDown)
	mountStatic(h, testStaticFS())

	// when
	status, body := h.NewRequest(http.MethodGet, "/pricing").Do()

	// then the shell never goes out with its placeholders unfilled
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.JSONEq(t, `{"error":"failed to resolve the page meta"}`, string(body))
}

func TestStaticController_SPA_AnswersJSON404OutsideThePages(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "a method other than GET or HEAD", method: http.MethodPost, path: "/x"},
		{name: "an unknown api path", method: http.MethodGet, path: "/api/v1/x"},
		{name: "the api root", method: http.MethodGet, path: "/api"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newStaticHarness(t)

			// when
			status, body := h.NewRequest(tc.method, tc.path).Do()

			// then an api caller is never handed html to parse
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, wantNotFoundBody, string(body))
		})
	}
}

func TestInit_RegistersTheWebManifestType(t *testing.T) {
	// given
	ext := ".webmanifest"

	// when
	got := mime.TypeByExtension(ext)

	// then
	assert.Equal(t, "application/manifest+json", got)
}

func TestStaticController_SelfHostCompose_DownloadsTheEmbeddedFile(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)
	compose := "name: retrotide\n"
	s := &Service{Guards: h.Guards, StaticFS: testStaticFS(), SelfHostCompose: []byte(compose)}
	RegisterRoutes(h.Engine, s.getAllStaticRoutes())

	// when
	rec := h.NewRequest(http.MethodGet, "/self-host/docker-compose.yml").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, compose, rec.Body.String())
	assert.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="docker-compose.yml"`, rec.Header().Get("Content-Disposition"))
}
