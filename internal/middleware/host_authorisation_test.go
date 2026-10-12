package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHostAuthorisation(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		host       string
		path       string
		wantStatus int
	}{
		{name: "the site's own host", baseURL: testBaseURL, host: testHost, path: "/pricing", wantStatus: http.StatusOK},
		{name: "the site's own host with a port", baseURL: testBaseURL, host: testHost + ":4323", path: "/pricing", wantStatus: http.StatusOK},
		{name: "the site's own host in capitals", baseURL: testBaseURL, host: "RetroTide.TEST", path: "/pricing", wantStatus: http.StatusOK},
		{name: "a base url with a port and a path", baseURL: "http://localhost:4323/app/", host: "localhost:4323", path: "/pricing", wantStatus: http.StatusOK},
		{name: "another host", baseURL: testBaseURL, host: "evil.test", path: "/pricing", wantStatus: http.StatusForbidden},
		{name: "another host on an unmatched path", baseURL: testBaseURL, host: "evil.test", path: "/b/share-slug", wantStatus: http.StatusForbidden},
		{name: "the healthcheck from any host", baseURL: testBaseURL, host: "10.0.0.5:4323", path: "/livez", wantStatus: http.StatusOK},
		{name: "the scraper from any host", baseURL: testBaseURL, host: "retrotide:4323", path: "/metrics", wantStatus: http.StatusOK},
		{name: "a base url without a host refuses everyone", baseURL: "/relative", host: testHost, path: "/pricing", wantStatus: http.StatusForbidden},
		{name: "a base url that does not parse refuses everyone", baseURL: "https://retrotide.test/%zz", host: testHost, path: "/pricing", wantStatus: http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			settingsSvc := settings.NewMockService(t)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(tc.baseURL, nil).Maybe()

			engine := newTestEngine(t)
			engine.Use(HostAuthorisation(settingsSvc))
			answer := func(c *gin.Context) {
				c.String(http.StatusOK, "handler reached")
			}
			engine.GET("/pricing", answer)
			engine.GET("/livez", answer)
			engine.GET("/metrics", answer)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil)
			req.Host = tc.host

			// when
			rec := serve(engine, req)

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantStatus == http.StatusForbidden {
				assert.JSONEq(t, `{"error":"forbidden"}`, rec.Body.String())
			}
		})
	}
}

func TestHostAuthorisation_RefusesEveryoneWhenBaseURLCannotBeRead(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errors.New("db down"))

	engine := newTestEngine(t)
	engine.Use(HostAuthorisation(settingsSvc))
	engine.GET("/pricing", func(c *gin.Context) {
		c.String(http.StatusOK, "handler reached")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil)
	req.Host = testHost

	// when
	rec := serve(engine, req)

	// then a failed read is never treated as a host match
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"error":"service unavailable"}`, rec.Body.String())
}
