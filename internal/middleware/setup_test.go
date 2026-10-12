package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dronebl"
	"retrotide/internal/guest"
	"retrotide/internal/model"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	maintenanceFixture struct {
		settings    *settings.MockService
		sessionRepo *repository.MockSessionRepository
		authz       *authz.MockService
		recorded    []error
	}
)

const (
	maintenanceBody = `{"error":"site is under maintenance"}`
)

func newMaintenanceFixture(t *testing.T, enabled bool) *maintenanceFixture {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(enabled, nil).Maybe()
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil).Maybe()

	return &maintenanceFixture{
		settings:    settingsSvc,
		sessionRepo: repository.NewMockSessionRepository(t),
		authz:       authz.NewMockService(t),
	}
}

func (f *maintenanceFixture) engine(t *testing.T) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(func(c *gin.Context) {
		c.Next()

		for _, recorded := range c.Errors {
			f.recorded = append(f.recorded, recorded.Err)
		}
	})
	engine.Use(Maintenance(f.settings, session.NewManager(f.sessionRepo, f.settings), f.authz))

	answer := func(c *gin.Context) {
		c.String(http.StatusOK, "handler reached")
	}
	engine.POST("/api/v1/auth/login", answer)
	engine.GET("/api/v1/site-info", answer)
	engine.GET("/api/v1/x", answer)
	engine.POST("/api/v1/x", answer)
	engine.GET("/pricing", answer)

	return engine
}

func (f *maintenanceFixture) liveSession(t *testing.T) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil)

	return userID
}

func fullChainEngine(t *testing.T, verdict dronebl.Verdict) *gin.Engine {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL+"/", nil).Maybe()
	settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, nil).Maybe()
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil).Maybe()

	checker := dronebl.NewMockService(t)
	checker.EXPECT().Check(mock.Anything, mock.Anything).Return(verdict, nil).Maybe()

	engine := newTestEngine(t)
	require.NoError(t, Setup(engine, fullDeps(t, settingsSvc, checker)))
	answer := func(c *gin.Context) {
		c.String(http.StatusOK, "pricing")
	}
	engine.GET("/pricing", answer)
	engine.GET("/api/v1/x", answer)
	engine.GET("/api/v1/site-info", answer)

	return engine
}

func TestSetup_InstallsThePrivateSiteGate(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "an api route", path: "/api/v1/x", wantStatus: http.StatusUnauthorized},
		{name: "the site info", path: "/api/v1/site-info", wantStatus: http.StatusOK},
		{name: "a page", path: "/pricing", wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the whole global chain on a private site, and a visitor without a session
			engine := fullChainEngine(t, dronebl.Verdict{})
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil)
			req.Host = testHost

			// when
			rec := serve(engine, req)

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

func fullDeps(t *testing.T, settingsSvc settings.Service, checker dronebl.Service) Deps {
	t.Helper()

	return Deps{
		Settings:   settingsSvc,
		BodyLimit:  NewBodyLimit(1 << 20),
		DroneBL:    checker,
		Sessions:   session.NewManager(repository.NewMockSessionRepository(t), settingsSvc),
		Guests:     guest.NewManager(repository.NewMockGuestDeviceRepository(t), settingsSvc),
		Authz:      authz.NewMockService(t),
		LastSeenIP: NewLastSeenIP(repository.NewMockUserRepository(t)),
	}
}

func TestSetup_InstallsTheWholeChain(t *testing.T) {
	// given
	engine := fullChainEngine(t, dronebl.Verdict{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil)
	req.Host = testHost

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "pricing", rec.Body.String())
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "no-cache, no-store, must-revalidate", rec.Header().Get("Cache-Control"))
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestSetup_RefusesAForeignOrigin(t *testing.T) {
	// given
	engine := fullChainEngine(t, dronebl.Verdict{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil)
	req.Host = testHost
	req.Header.Set("Origin", "https://evil.test")

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestSetup_RefusesAnotherHost(t *testing.T) {
	// given
	engine := fullChainEngine(t, dronebl.Verdict{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil)
	req.Host = "evil.test"

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"forbidden"}`, rec.Body.String())
}

func TestSetup_RefusesAListedAddress(t *testing.T) {
	// given
	engine := fullChainEngine(t, dronebl.Verdict{Blocked: true, Classes: []int{13}})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil)
	req.Host = testHost
	req.RemoteAddr = listedPeer

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"your network is listed on a public abuse blocklist"}`, rec.Body.String())
}

func TestSetup_RequiresItsDependencies(t *testing.T) {
	tests := []struct {
		name  string
		strip func(deps *Deps)
		want  error
	}{
		{name: "no settings service", strip: func(deps *Deps) { deps.Settings = nil }, want: errMissingSettings},
		{name: "no body limit", strip: func(deps *Deps) { deps.BodyLimit = nil }, want: errMissingBodyLimit},
		{name: "no dronebl checker", strip: func(deps *Deps) { deps.DroneBL = nil }, want: errMissingDroneBL},
		{name: "no session manager", strip: func(deps *Deps) { deps.Sessions = nil }, want: errMissingSessions},
		{name: "no guest device manager", strip: func(deps *Deps) { deps.Guests = nil }, want: errMissingGuests},
		{name: "no staff permission service", strip: func(deps *Deps) { deps.Authz = nil }, want: errMissingAuthz},
		{name: "no last seen ip recorder", strip: func(deps *Deps) { deps.LastSeenIP = nil }, want: errMissingLastSeenIP},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := newTestEngine(t)
			deps := fullDeps(t, settings.NewMockService(t), dronebl.NewMockService(t))
			tc.strip(&deps)

			// when
			err := Setup(engine, deps)

			// then
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestMaintenance(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		method     string
		path       string
		wantStatus int
	}{
		{name: "the api is closed", enabled: true, method: http.MethodGet, path: "/api/v1/x", wantStatus: http.StatusServiceUnavailable},
		{name: "an unmatched api path is closed", enabled: true, method: http.MethodGet, path: "/api/v1/nope", wantStatus: http.StatusServiceUnavailable},
		{name: "site info stays reachable so the banner can render", enabled: true, method: http.MethodGet, path: "/api/v1/site-info", wantStatus: http.StatusOK},
		{name: "signing in stays reachable", enabled: true, method: http.MethodPost, path: "/api/v1/auth/login", wantStatus: http.StatusOK},
		{name: "pages still load", enabled: true, method: http.MethodGet, path: "/pricing", wantStatus: http.StatusOK},
		{name: "nothing is closed while it is off", enabled: false, method: http.MethodGet, path: "/api/v1/x", wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given no session cookie, and no session lookup outside maintenance, which the strict mocks would report
			f := newMaintenanceFixture(t, tc.enabled)
			engine := f.engine(t)

			// when
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil))

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantStatus == http.StatusServiceUnavailable {
				assert.JSONEq(t, maintenanceBody, rec.Body.String())
			}
		})
	}
}

func TestMaintenance_ClosesTheApiWhenTheSwitchCannotBeRead(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "the api", path: "/api/v1/x", wantStatus: http.StatusServiceUnavailable},
		{name: "a page, which never reads the switch", path: "/pricing", wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			settingsSvc := settings.NewMockService(t)
			settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, errors.New("db down")).Maybe()

			engine := newTestEngine(t)
			engine.Use(Maintenance(settingsSvc, session.NewManager(repository.NewMockSessionRepository(t), settingsSvc), authz.NewMockService(t)))
			answer := func(c *gin.Context) {
				c.String(http.StatusOK, "handler reached")
			}
			engine.GET("/api/v1/x", answer)
			engine.GET("/pricing", answer)

			// when
			rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))

			// then a failed read never reopens an api the operator may have closed
			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

func TestMaintenance_LetsStaffWithManageSettingsThrough(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			// given
			f := newMaintenanceFixture(t, true)
			userID := f.liveSession(t)
			f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
			f.authz.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(true, nil)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, method, "/api/v1/x", hostSessionCookie)

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "handler reached", rec.Body.String())
			assert.Empty(t, f.recorded)
		})
	}
}

func TestMaintenance_ClosesTheApiToEveryOtherSession(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, f *maintenanceFixture)
	}{
		{
			name: "a user without manage_settings",
			expect: func(t *testing.T, f *maintenanceFixture) {
				userID := f.liveSession(t)
				f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				f.authz.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(false, nil)
			},
		},
		{
			name: "an unknown or expired session",
			expect: func(_ *testing.T, f *maintenanceFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
			},
		},
		{
			name: "a banned user, whose session is revoked",
			expect: func(t *testing.T, f *maintenanceFixture) {
				userID := f.liveSession(t)
				f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
				f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newMaintenanceFixture(t, true)
			tc.expect(t, f)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, "/api/v1/x", hostSessionCookie)

			// then
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, maintenanceBody, rec.Body.String())
			assert.Empty(t, f.recorded)
		})
	}
}

func TestMaintenance_AnswersServiceUnavailableWhenStaffCannotBeChecked(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, f *maintenanceFixture)
	}{
		{
			name: "the session",
			expect: func(_ *testing.T, f *maintenanceFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the ban",
			expect: func(t *testing.T, f *maintenanceFixture) {
				userID := f.liveSession(t)
				f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(true, errDatabaseDown)
			},
		},
		{
			name: "the permission",
			expect: func(t *testing.T, f *maintenanceFixture) {
				userID := f.liveSession(t)
				f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				f.authz.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(false, errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newMaintenanceFixture(t, true)
			tc.expect(t, f)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, "/api/v1/x", hostSessionCookie)

			// then a failed check is neither a way past maintenance nor mistaken for it
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, unavailableBody, rec.Body.String())
			require.Len(t, f.recorded, 1)
			assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
		})
	}
}

func TestSetup_LetsStaffWithManageSettingsThroughMaintenance(t *testing.T) {
	tests := []struct {
		name       string
		cookie     string
		staff      bool
		wantStatus int
		wantBody   string
	}{
		{name: "a session holding manage_settings", cookie: hostSessionCookie, staff: true, wantStatus: http.StatusOK, wantBody: "handler reached"},
		{name: "no session", cookie: "", wantStatus: http.StatusServiceUnavailable, wantBody: maintenanceBody},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the whole global chain with maintenance switched on
			settingsSvc := settings.NewMockService(t)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL+"/", nil).Maybe()
			settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(true, nil)
			settingsSvc.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil).Maybe()

			checker := dronebl.NewMockService(t)
			checker.EXPECT().Check(mock.Anything, mock.Anything).Return(dronebl.Verdict{}, nil).Maybe()

			sessionRepo := repository.NewMockSessionRepository(t)
			authzSvc := authz.NewMockService(t)
			if tc.staff {
				userID := uuid.New()
				sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil)
				authzSvc.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				authzSvc.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(true, nil)
			}

			deps := fullDeps(t, settingsSvc, checker)
			deps.Sessions = session.NewManager(sessionRepo, settingsSvc)
			deps.Authz = authzSvc

			engine := newTestEngine(t)
			require.NoError(t, Setup(engine, deps))
			engine.GET("/api/v1/x", func(c *gin.Context) {
				c.String(http.StatusOK, "handler reached")
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/x", nil)
			req.Host = testHost
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: tc.cookie, Value: sessionToken})
			}

			// when
			rec := serve(engine, req)

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantStatus == http.StatusOK {
				assert.Equal(t, tc.wantBody, rec.Body.String())
				return
			}

			assert.JSONEq(t, tc.wantBody, rec.Body.String())
		})
	}
}

func TestSetup_RefusesAnOriginWhenBaseURLCannotBeRead(t *testing.T) {
	// given
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errors.New("db down"))

	engine := newTestEngine(t)
	engine.Use(cors.New(corsConfig(settingsSvc)))
	engine.GET("/pricing", func(c *gin.Context) {
		c.String(http.StatusOK, "pricing")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pricing", nil)
	req.Header.Set("Origin", testBaseURL)

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
