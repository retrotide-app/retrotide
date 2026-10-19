package controllers

import (
	"net/http"
	"testing"
	"testing/fstest"

	"retrotide/internal/controllers/utils/testutil"

	"github.com/stretchr/testify/assert"
)

const (
	marketingShell = `<!doctype html>
<html lang="en-GB">
    <head>
        <title>RetroTide</title>
        <meta
            name="description"
            content="Sprint retrospectives for teams."
        />
        <meta name="robots" content="index, follow" />
        <link rel="canonical" href="__BASE_URL__/" />
        <meta property="og:url" content="__BASE_URL__/" />
        <meta property="og:image" content="__BASE_URL__/logo.png" />
        <meta property="og:title" content="RetroTide" />
        <meta
            property="og:description"
            content="Sprint retrospectives for teams."
        />
        <meta name="twitter:title" content="RetroTide" />
        <meta name="twitter:description" content="Sprint retrospectives for teams." />
    </head>
    <body><div id="root"></div></body>
</html>`
)

func newSharedShellHarness(t *testing.T) *testutil.Harness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	mountStatic(h, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte(marketingShell)}})

	return h
}

func TestStaticController_SharedPathsGetGenericMetaAndNoIndex(t *testing.T) {
	tests := []string{"/b/k3v9x2QpLmNoRsTu", "/boards/8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b", "/o/acme", "/invite", "/B/k3v9x2QpLmNoRsTu"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given
			h := newSharedShellHarness(t)

			// when
			rec := h.NewRequest(http.MethodGet, path).Record()

			// then a link unfurled by a third party shows only the generic invitation
			body := rec.Body.String()
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
			assert.Contains(t, body, "<title>Join a retrospective on RetroTide</title>")
			assert.Contains(t, body, `content="noindex, nofollow"`)
			assert.Contains(t, body, `property="og:title" content="Join a retrospective on RetroTide"`)
			assert.Contains(t, body, `name="twitter:title" content="Join a retrospective on RetroTide"`)
			assert.NotContains(t, body, "Sprint retrospectives for teams.")
			assert.NotContains(t, body, "index, follow")
			assert.NotContains(t, body, "__BASE_URL__")
			assert.Contains(t, body, `<div id="root"></div>`)
		})
	}
}

func TestStaticController_OtherPagesKeepTheShellsDefaultMeta(t *testing.T) {
	tests := []struct {
		path      string
		canonical string
	}{
		{path: "/bo", canonical: "https://retrotide.test/bo"},
		{path: "/board/x", canonical: "https://retrotide.test/board/x"},
		{path: "/dashboard", canonical: "https://retrotide.test/dashboard"},
		{path: "/templates/blank", canonical: "https://retrotide.test/templates/blank"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given
			h := newSharedShellHarness(t)

			// when
			rec := h.NewRequest(http.MethodGet, tc.path).Record()

			// then the copy is the shell's own, with only the base url filled
			body := rec.Body.String()
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, body, "<title>RetroTide</title>")
			assert.Contains(t, body, `content="Sprint retrospectives for teams."`)
			assert.Contains(t, body, `<meta name="robots" content="index, follow" />`)
			assert.Contains(t, body, `<link rel="canonical" href="`+tc.canonical+`" />`)
			assert.Contains(t, body, `<meta property="og:url" content="`+tc.canonical+`" />`)
			assert.Contains(t, body, `<meta property="og:image" content="https://retrotide.test/logo.png" />`)
			assert.NotContains(t, body, "__BASE_URL__")
		})
	}
}

func TestStaticController_MarketingPagesCarryTheirOwnMeta(t *testing.T) {
	tests := []struct {
		path      string
		title     string
		canonical string
	}{
		{path: "/", title: "RetroTide: sprint retrospectives your team will use", canonical: "https://retrotide.test/"},
		{path: "/compare/easyretro", title: "RetroTide and EasyRetro compared | RetroTide", canonical: "https://retrotide.test/compare/easyretro"},
		{path: "/self-host", title: "Self-host RetroTide | RetroTide", canonical: "https://retrotide.test/self-host"},
		{path: "/templates/sailboat", title: "Sailboat retrospective template | RetroTide", canonical: "https://retrotide.test/templates/sailboat"},
		{path: "/legal/privacy", title: "Privacy notice | RetroTide", canonical: "https://retrotide.test/legal/privacy"},
		{path: "/Self-Host/?utm_source=slack", title: "Self-host RetroTide | RetroTide", canonical: "https://retrotide.test/self-host"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given
			h := newSharedShellHarness(t)

			// when
			rec := h.NewRequest(http.MethodGet, tc.path).Record()

			// then the page source names the page itself, never the query string it was reached through
			body := rec.Body.String()
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, body, "<title>"+tc.title+"</title>")
			assert.Contains(t, body, `<meta property="og:title" content="`+tc.title+`" />`)
			assert.Contains(t, body, `<meta name="twitter:title" content="`+tc.title+`" />`)
			assert.Contains(t, body, `<link rel="canonical" href="`+tc.canonical+`" />`)
			assert.Contains(t, body, `<meta property="og:url" content="`+tc.canonical+`" />`)
			assert.NotContains(t, body, "Sprint retrospectives for teams.")
			assert.NotContains(t, body, "utm_source")
			assert.NotContains(t, body, "__BASE_URL__")
		})
	}
}
