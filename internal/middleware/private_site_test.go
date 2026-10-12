package middleware

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/guest"
	"retrotide/internal/model"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	privateSiteFixture struct {
		settings    *settings.MockService
		sessionRepo *repository.MockSessionRepository
		guestRepo   *repository.MockGuestDeviceRepository
		authz       *authz.MockService
		recorded    []error
	}
)

const (
	privateSiteBody = `{"error":"this site is private, sign in to continue","code":"sign_in_required"}`
	handlerReached  = "handler reached"
	hostGuestCookie = "__Host-rt_guest"
)

var (
	privateSiteProtectedRoutes = []string{
		"/api/v1/x",
		"/api/v1/admin/users",
		"/api/v1/admin/invitations",
		"/api/v1/notifications",
		"/api/v1/auth/identities",
		"/api/v1/auth/google/start",
		"/api/v1/auth/google/callback",
		"/api/v1/orgs",
		"/api/v1/invites/accept",
		"/api/v1/ws",
	}

	privateSitePostOnlyRoutes = []string{
		"/api/v1/auth/google/start",
		"/api/v1/invites/accept",
	}

	privateSiteGuestRoutes = []string{
		"/api/v1/boards/8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"/api/v1/boards/8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b/cards",
		"/api/v1/ws",
	}
)

func newPrivateSiteFixture(t *testing.T) *privateSiteFixture {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil).Maybe()

	return &privateSiteFixture{
		settings:    settingsSvc,
		sessionRepo: repository.NewMockSessionRepository(t),
		guestRepo:   repository.NewMockGuestDeviceRepository(t),
		authz:       authz.NewMockService(t),
	}
}

func (f *privateSiteFixture) onSite(mode config.SiteMode) *privateSiteFixture {
	f.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(mode), nil)

	return f
}

func (f *privateSiteFixture) engine(t *testing.T) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(func(c *gin.Context) {
		c.Next()

		for _, recorded := range c.Errors {
			f.recorded = append(f.recorded, recorded.Err)
		}
	})
	engine.Use(PrivateSite(f.settings, session.NewManager(f.sessionRepo, f.settings), guest.NewManager(f.guestRepo, f.settings), f.authz))

	answer := func(c *gin.Context) {
		c.String(http.StatusOK, handlerReached)
	}

	for _, open := range sortedOpenRoutes() {
		engine.Handle(open.method, open.route, answer)
	}

	engine.DELETE("/api/v1/auth/session", answer)
	engine.PUT("/api/v1/auth/register", answer)
	engine.PUT("/api/v1/invites/register", answer)
	engine.Any("/api/v1/x", answer)
	engine.GET("/api/v1/admin/users", answer)
	engine.GET("/api/v1/admin/invitations", answer)
	engine.GET("/api/v1/notifications", answer)
	engine.GET("/api/v1/auth/identities", answer)
	engine.POST("/api/v1/auth/:provider/start", answer)
	engine.GET("/api/v1/auth/:provider/callback", answer)
	engine.GET("/api/v1/orgs", answer)
	engine.POST("/api/v1/invites/accept", answer)
	engine.GET(socketRoute, answer)
	engine.GET(boardRoute, answer)
	engine.GET(boardRoute+"/cards", answer)
	engine.GET("/livez", answer)
	engine.GET("/health", answer)
	engine.GET(metricsRoute, answer)
	engine.GET(assetsRoute, answer)
	engine.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/api/v1/nope" {
			c.String(http.StatusNotFound, "not found")
			return
		}

		c.String(http.StatusOK, "the spa shell")
	})

	return engine
}

func sortedOpenRoutes() []guardedRoute {
	return slices.SortedFunc(maps.Keys(privateSiteOpenRoutes), func(a, b guardedRoute) int {
		return cmp.Or(cmp.Compare(a.route, b.route), cmp.Compare(a.method, b.method))
	})
}

func (f *privateSiteFixture) liveSession(t *testing.T) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil)

	return userID
}

func (f *privateSiteFixture) liveGuest(t *testing.T) uuid.UUID {
	t.Helper()

	deviceID := uuid.New()
	f.guestRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.GuestDevice{ID: deviceID, LastSeenAt: time.Now()}, nil)

	return deviceID
}

func TestPrivateSite_DoesNothingOnAPublicSite(t *testing.T) {
	// given a public site, which is the default, and a visitor without a session
	f := newPrivateSiteFixture(t).onSite(config.SiteModePublic)
	engine := f.engine(t)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, "/api/v1/x", "")

	// then no session is looked up, which the strict mocks would report
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, handlerReached, rec.Body.String())
}

func TestPrivateSite_AnswersUnauthorisedToAVisitorWithoutASession(t *testing.T) {
	paths := append(slices.Clone(privateSiteProtectedRoutes), "/api/v1/nope")

	for _, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method+" "+path, func(t *testing.T) {
				// given a private site
				f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
				engine := f.engine(t)

				// when
				rec := guardedRequest(t, engine, method, path, "")

				// then every other api route, matched or not, is closed before its handler or its 404 and 405
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.JSONEq(t, privateSiteBody, rec.Body.String())
				assert.Empty(t, f.recorded)
			})
		}
	}
}

func TestPrivateSite_LetsEveryExemptRouteThroughWithAndWithoutASession(t *testing.T) {
	for _, open := range sortedOpenRoutes() {
		for _, cookie := range []string{"", hostSessionCookie} {
			t.Run(fmt.Sprintf("%s %s with cookie %q", open.method, open.route, cookie), func(t *testing.T) {
				// given the site mode is never read and no session is looked up, which the strict mocks would report
				f := newPrivateSiteFixture(t)
				engine := f.engine(t)

				// when
				rec := guardedRequest(t, engine, open.method, open.route, cookie)

				// then signing in, recovering an account and accepting an invitation stay possible
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, handlerReached, rec.Body.String())
			})
		}
	}
}

func TestPrivateSite_ClosesAnotherMethodOnAnExemptRoute(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodDelete, path: "/api/v1/auth/session"},
		{method: http.MethodPut, path: "/api/v1/auth/register"},
		{method: http.MethodPut, path: "/api/v1/invites/register"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			// given a private site whose open list names a route with another method
			f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, tc.method, tc.path, "")

			// then only the listed method is exempt, so a route added later on an open path is closed by default
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.JSONEq(t, privateSiteBody, rec.Body.String())
		})
	}
}

func TestPrivateSite_ExemptsExactlyTheSignInRoutes(t *testing.T) {
	// given
	want := []string{
		"POST /api/v1/auth/forgot-password",
		"POST /api/v1/auth/invitation",
		"POST /api/v1/auth/invitation/accept",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/resend-verification",
		"POST /api/v1/auth/reset-password",
		"GET /api/v1/auth/session",
		"HEAD /api/v1/auth/session",
		"POST /api/v1/auth/two-factor/verify",
		"POST /api/v1/auth/verify-email",
		"GET /api/v1/b/:slug",
		"HEAD /api/v1/b/:slug",
		"POST /api/v1/b/:slug/join",
		"POST /api/v1/invites/lookup",
		"POST /api/v1/invites/register",
		"GET /api/v1/site-info",
		"HEAD /api/v1/site-info",
	}

	// when
	got := make([]string, 0, len(privateSiteOpenRoutes))
	for _, open := range sortedOpenRoutes() {
		got = append(got, open.method+" "+open.route)
	}

	// then registration stays open for the first account, the service refuses it once one exists, someone invited to an organisation creates their account while accepting, someone signing in finishes their second factor, and a guest reaches a board's share link
	assert.Equal(t, want, got)
}

func TestPrivateSite_LetsAValidGuestThroughOnTheBoardRoutesAndTheSocket(t *testing.T) {
	for _, path := range privateSiteGuestRoutes {
		t.Run(path, func(t *testing.T) {
			// given a private site and a browser holding only a live guest device
			f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
			f.liveGuest(t)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, path, hostGuestCookie)

			// then the route's own guards decide from here, as they do for a live session
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, handlerReached, rec.Body.String())
			assert.Empty(t, f.recorded)
		})
	}
}

func TestPrivateSite_RefusesAGuestThatIsNotLiveOnTheBoardRoutes(t *testing.T) {
	for _, path := range privateSiteGuestRoutes {
		t.Run(path, func(t *testing.T) {
			// given a private site and an unknown or expired guest device
			f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
			f.guestRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get guest device: %w", dao.ErrNotFound))
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, path, hostGuestCookie)

			// then
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.JSONEq(t, privateSiteBody, rec.Body.String())
			assert.Empty(t, f.recorded)
		})
	}
}

func TestPrivateSite_NeverLetsAGuestOffTheBoardRoutes(t *testing.T) {
	for _, path := range []string{"/api/v1/x", "/api/v1/orgs", "/api/v1/notifications", "/api/v1/admin/users"} {
		t.Run(path, func(t *testing.T) {
			// given a private site and a guest cookie, which the strict guest mock proves is never looked up
			f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, path, hostGuestCookie)

			// then a guest exists only on boards
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.JSONEq(t, privateSiteBody, rec.Body.String())
		})
	}
}

func TestPrivateSite_LetsAGuestThroughBehindAStaleSession(t *testing.T) {
	// given a private site, a session cookie whose row is gone and a live guest device
	f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
	f.liveGuest(t)
	engine := f.engine(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, privateSiteGuestRoutes[0], nil)
	req.AddCookie(&http.Cookie{Name: hostSessionCookie, Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: hostGuestCookie, Value: sessionToken})

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, handlerReached, rec.Body.String())
}

func TestPrivateSite_AnswersServiceUnavailableWhenTheGuestCannotBeChecked(t *testing.T) {
	// given
	f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
	f.guestRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, errDatabaseDown)
	engine := f.engine(t)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, privateSiteGuestRoutes[0], hostGuestCookie)

	// then a failed check never passes and never reads as a missing guest
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, unavailableBody, rec.Body.String())
	require.Len(t, f.recorded, 1)
	assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
}

func TestPrivateSite_LeavesEverythingOutsideTheAPIAlone(t *testing.T) {
	tests := []string{"/", "/login", "/register", "/accept-invitation", "/reset-password", "/assets/index-a1b2c3.js", "/livez", "/health", "/metrics", "/api"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			// given the site mode is never read, which the strict settings mock would report
			f := newPrivateSiteFixture(t)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, path, "")

			// then the shell loads, so the SPA can send the visitor to the login page, and the probes answer
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestPrivateSite_LetsALiveSessionThrough(t *testing.T) {
	for _, path := range privateSiteProtectedRoutes {
		t.Run(path, func(t *testing.T) {
			// given
			f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
			userID := f.liveSession(t)
			f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
			engine := f.engine(t)
			method := http.MethodGet
			if slices.Contains(privateSitePostOnlyRoutes, path) {
				method = http.MethodPost
			}

			// when
			rec := guardedRequest(t, engine, method, path, hostSessionCookie)

			// then the route's own guards decide from here
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, handlerReached, rec.Body.String())
			assert.Empty(t, f.recorded)
		})
	}
}

func TestPrivateSite_RefusesASessionThatIsNotLive(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, f *privateSiteFixture)
	}{
		{
			name: "an unknown or expired session",
			expect: func(_ *testing.T, f *privateSiteFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
			},
		},
		{
			name: "a banned user, whose session is revoked",
			expect: func(t *testing.T, f *privateSiteFixture) {
				userID := f.liveSession(t)
				f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
				f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newPrivateSiteFixture(t).onSite(config.SiteModePrivate)
			tc.expect(t, f)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, "/api/v1/x", hostSessionCookie)

			// then
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.JSONEq(t, privateSiteBody, rec.Body.String())
			assert.Empty(t, f.recorded)
		})
	}
}

func TestPrivateSite_AnswersServiceUnavailableWhenItCannotCheck(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, f *privateSiteFixture)
	}{
		{
			name: "the site mode",
			expect: func(_ *testing.T, f *privateSiteFixture) {
				f.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)
			},
		},
		{
			name: "the session",
			expect: func(_ *testing.T, f *privateSiteFixture) {
				f.onSite(config.SiteModePrivate)
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the ban",
			expect: func(t *testing.T, f *privateSiteFixture) {
				f.onSite(config.SiteModePrivate)
				userID := f.liveSession(t)
				f.authz.EXPECT().IsBanned(mock.Anything, userID).Return(true, errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newPrivateSiteFixture(t)
			tc.expect(t, f)
			engine := f.engine(t)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, "/api/v1/x", hostSessionCookie)

			// then a failed check neither opens a private site nor passes for a missing session
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, unavailableBody, rec.Body.String())
			require.Len(t, f.recorded, 1)
			assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
		})
	}
}

func TestPrivateSite_AnUnknownStoredModeClosesTheAPI(t *testing.T) {
	// given a row edited by hand, which validation on write never lets through
	f := newPrivateSiteFixture(t)
	f.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("friends", nil)
	engine := f.engine(t)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/x", nil))

	// then it is read as neither mode, so nothing opens up
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Len(t, f.recorded, 1)
	assert.EqualError(t, f.recorded[0], `unknown site mode "friends"`)
}
