package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/authctx"
	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
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
	authFixture struct {
		sessionRepo *repository.MockSessionRepository
		accounts    *authz.MockService
		tenants     *orgauthz.MockService
		guards      *Guards
		recorded    []error
	}

	tenantGuard struct {
		name         string
		team         bool
		perm         orgauthz.Permission
		prefix       string
		notFoundBody string
		badIDBody    string
	}
)

const (
	probeRoute          = "/api/v1/probe"
	orgProbeRoute       = "/api/v1/orgs/:orgID/probe"
	teamProbeRoute      = "/api/v1/teams/:teamID/probe"
	notificationRead    = "/api/v1/notifications/:id/read"
	notificationReadURL = "/api/v1/notifications/42/read"
	sessionToken        = "9b2e4c6a8d0f1e3c5a7b9d1f3e5c7a9b1d3f5e7c9a1b3d5f7e9c1a3b5d7f9e1c"
	anonymousBody       = "anonymous"

	authRequiredBody  = `{"error":"authentication required"}`
	invalidBody       = `{"error":"invalid or expired session"}`
	bannedBody        = `{"error":"your account has been banned"}`
	lockedBody        = `{"error":"your account is locked"}`
	unavailableBody   = `{"error":"service unavailable"}`
	insufficientBody  = `{"error":"insufficient permissions"}`
	hostSessionCookie = "__Host-rt_session"
	bareSessionCookie = "rt_session"
)

var (
	tenantGuards = []tenantGuard{
		{
			name:         "OrgPermission",
			perm:         orgauthz.PermOrgUpdate,
			prefix:       "/api/v1/orgs/",
			notFoundBody: `{"error":"organisation not found"}`,
			badIDBody:    `{"error":"invalid orgID"}`,
		},
		{
			name:         "TeamPermission",
			team:         true,
			perm:         orgauthz.PermTeamManageMembers,
			prefix:       "/api/v1/teams/",
			notFoundBody: `{"error":"team not found"}`,
			badIDBody:    `{"error":"invalid teamID"}`,
		},
	}
)

func newAuthFixture(t *testing.T, baseURLErr error) *authFixture {
	t.Helper()

	sessionRepo := repository.NewMockSessionRepository(t)
	accounts := authz.NewMockService(t)
	tenants := orgauthz.NewMockService(t)
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, baseURLErr)

	return &authFixture{
		sessionRepo: sessionRepo,
		accounts:    accounts,
		tenants:     tenants,
		guards: NewGuards(GuardDeps{
			Settings: settingsSvc,
			Sessions: session.NewManager(sessionRepo, settingsSvc),
			Authz:    accounts,
			OrgAuthz: tenants,
		}),
	}
}

func (g tenantGuard) build(guards *Guards) gin.HandlerFunc {
	if g.team {
		return guards.TeamPermission(g.perm)
	}

	return guards.OrgPermission(g.perm)
}

func (g tenantGuard) decides(f *authFixture, userID uuid.UUID, id uuid.UUID, decision orgauthz.Decision, err error) {
	if g.team {
		f.tenants.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: id, UserID: userID, Permission: g.perm}).Return(decision, err)
		return
	}

	f.tenants.EXPECT().CanInOrg(mock.Anything, spec.OrgPermissionCheck{OrgID: id, UserID: userID, Permission: g.perm}).Return(decision, err)
}

func (g tenantGuard) path(id string) string {
	return g.prefix + id + "/probe"
}

func (f *authFixture) engine(t *testing.T, guard gin.HandlerFunc) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(func(c *gin.Context) {
		c.Next()

		for _, recorded := range c.Errors {
			f.recorded = append(f.recorded, recorded.Err)
		}
	})

	for _, route := range []string{probeRoute, orgProbeRoute, teamProbeRoute, notificationRead, "/api/v1/notifications/read"} {
		engine.GET(route, guard, principalProbe)
		engine.POST(route, guard, principalProbe)
		engine.PUT(route, guard, principalProbe)
		engine.PATCH(route, guard, principalProbe)
		engine.DELETE(route, guard, principalProbe)
	}

	return engine
}

func (f *authFixture) liveSession() uuid.UUID {
	userID := uuid.New()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil)

	return userID
}

func principalProbe(c *gin.Context) {
	principal, ok := authctx.Principal(c.Request.Context())
	if !ok {
		c.String(http.StatusOK, anonymousBody)
		return
	}

	c.String(http.StatusOK, string(principal.Kind)+":"+principal.ID.String())
}

func guardedRequest(t *testing.T, engine *gin.Engine, method string, path string, cookieName string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	if cookieName != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: sessionToken})
	}

	return serve(engine, req)
}

func signedIn(userID uuid.UUID) string {
	return string(model.PrincipalUser) + ":" + userID.String()
}

func sessionContext(t *testing.T) *gin.Context {
	t.Helper()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, probeRoute, nil)
	c.Request.AddCookie(&http.Cookie{Name: hostSessionCookie, Value: sessionToken})

	return c
}

func TestUserGuard_LooksUpTheSessionOncePerRequest(t *testing.T) {
	// given one request that the private-site gate and then the route guard both resolve
	f := newAuthFixture(t, nil)
	userID := uuid.New()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil).Once()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil).Once()
	c := sessionContext(t)

	// when
	firstID, firstState := f.guards.users.resolve(c)
	secondID, secondState := f.guards.users.resolve(c)

	// then the second answer comes from the request, which the strict mocks confirm
	assert.Equal(t, userID, firstID)
	assert.Equal(t, sessionLive, firstState)
	assert.Equal(t, firstID, secondID)
	assert.Equal(t, firstState, secondState)
}

func TestUserGuard_KeepsABannedAnswerForTheRestOfTheRequest(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	userID := uuid.New()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil).Once()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil).Once()
	f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil).Once()
	c := sessionContext(t)

	// when
	_, firstState := f.guards.users.resolve(c)
	_, secondState := f.guards.users.resolve(c)

	// then the session is revoked once, and the second guard still sees the ban
	assert.Equal(t, sessionBanned, firstState)
	assert.Equal(t, sessionBanned, secondState)
}

func TestUserGuard_NeverKeepsALookupThatFailed(t *testing.T) {
	// given a session store that fails once and then answers
	f := newAuthFixture(t, nil)
	userID := uuid.New()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, errDatabaseDown).Once()
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: userID}, nil).Once()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil).Once()
	c := sessionContext(t)

	// when
	_, firstState := f.guards.users.resolve(c)
	secondID, secondState := f.guards.users.resolve(c)

	// then a failure is never mistaken for an answer
	assert.Equal(t, sessionUnchecked, firstState)
	assert.Equal(t, userID, secondID)
	assert.Equal(t, sessionLive, secondState)
}

func TestAuth_RefusesARequestWithoutASessionCookie(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, "")

	// then
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, authRequiredBody, rec.Body.String())
}

func TestAuth_IgnoresABareSessionCookieUnderAnHTTPSBaseURL(t *testing.T) {
	// given a sibling subdomain planted the unprefixed name
	f := newAuthFixture(t, nil)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, bareSessionCookie)

	// then
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, authRequiredBody, rec.Body.String())
}

func TestAuth_RefusesAnUnknownOrExpiredSession(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, invalidBody, rec.Body.String())
	assert.Empty(t, f.recorded)
}

func TestAuth_AnswersServiceUnavailableWhenTheSessionCannotBeChecked(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, errDatabaseDown)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, unavailableBody, rec.Body.String())
	require.Len(t, f.recorded, 1)
	assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
}

func TestAuth_AnswersServiceUnavailableWhenTheBaseURLCannotBeRead(t *testing.T) {
	// given
	f := newAuthFixture(t, errDatabaseDown)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, unavailableBody, rec.Body.String())
	require.Len(t, f.recorded, 1)
	assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
}

func TestAuth_RevokesABannedUsersSessionAndRefuses(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
	f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, bannedBody, rec.Body.String())
	assert.Empty(t, f.recorded)
}

func TestAuth_RefusesABannedUserAndRecordsAFailedRevocation(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
	f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(errDatabaseDown)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, bannedBody, rec.Body.String())
	require.Len(t, f.recorded, 1)
	assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
}

func TestAuth_AnswersServiceUnavailableWhenTheBanCannotBeChecked(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, errDatabaseDown)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, unavailableBody, rec.Body.String())
	require.Len(t, f.recorded, 1)
	assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
}

func TestAuth_LetsAReadThroughWithTheUserPrincipal(t *testing.T) {
	// given reads never consult the lock
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, signedIn(userID), rec.Body.String())
}

func TestAuth_LetsAnUnlockedWriteThrough(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			// given
			f := newAuthFixture(t, nil)
			userID := f.liveSession()
			f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
			f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			engine := f.engine(t, f.guards.Auth)

			// when
			rec := guardedRequest(t, engine, method, probeRoute, hostSessionCookie)

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, signedIn(userID), rec.Body.String())
		})
	}
}

func TestAuth_RefusesAWriteFromALockedAccount(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
	f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodPost, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, lockedBody, rec.Body.String())
}

func TestAuth_LetsALockedAccountMarkNotificationsRead(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "one notification", path: notificationReadURL},
		{name: "every notification", path: "/api/v1/notifications/read"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the route is exempt from the lock, so it is never consulted
			f := newAuthFixture(t, nil)
			userID := f.liveSession()
			f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
			engine := f.engine(t, f.guards.Auth)

			// when
			rec := guardedRequest(t, engine, http.MethodPost, tc.path, hostSessionCookie)

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, signedIn(userID), rec.Body.String())
		})
	}
}

func TestAuth_ExemptsOnlyTheExemptMethodOnARoute(t *testing.T) {
	// given the exemption covers POST on the mark-read route, not DELETE
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
	f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodDelete, notificationReadURL, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, lockedBody, rec.Body.String())
}

func TestAuth_AnswersServiceUnavailableWhenTheLockCannotBeChecked(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
	f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(false, errDatabaseDown)
	engine := f.engine(t, f.guards.Auth)

	// when
	rec := guardedRequest(t, engine, http.MethodPost, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, unavailableBody, rec.Body.String())
	require.Len(t, f.recorded, 1)
	assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
}

func TestPermission_LetsAHolderThroughWithTheUserPrincipal(t *testing.T) {
	tests := []struct {
		name   string
		method string
		expect func(f *authFixture, userID uuid.UUID)
	}{
		{name: "a read, which never consults the lock", method: http.MethodGet, expect: func(*authFixture, uuid.UUID) {}},
		{
			name:   "a write from an unlocked account",
			method: http.MethodPost,
			expect: func(f *authFixture, userID uuid.UUID) {
				f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newAuthFixture(t, nil)
			userID := f.liveSession()
			f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
			tc.expect(f, userID)
			f.accounts.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(true, nil)
			engine := f.engine(t, f.guards.Permission(authz.PermManageSettings))

			// when
			rec := guardedRequest(t, engine, tc.method, probeRoute, hostSessionCookie)

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, signedIn(userID), rec.Body.String())
		})
	}
}

func TestPermission_RefusesAUserWithoutThePermission(t *testing.T) {
	// given
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
	f.accounts.EXPECT().Can(mock.Anything, userID, authz.PermViewAuditLog).Return(false, nil)
	engine := f.engine(t, f.guards.Permission(authz.PermViewAuditLog))

	// when
	rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, insufficientBody, rec.Body.String())
	assert.Empty(t, f.recorded)
}

func TestPermission_AnswersServiceUnavailableWhenThePermissionCannotBeChecked(t *testing.T) {
	tests := []struct {
		name  string
		cause error
	}{
		{name: "the role store is down", cause: errDatabaseDown},
		{name: "the guard names no declared permission", cause: fmt.Errorf("%w: %q", authz.ErrUnknownPermission, "manage_everything")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newAuthFixture(t, nil)
			userID := f.liveSession()
			f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
			f.accounts.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(false, tc.cause)
			engine := f.engine(t, f.guards.Permission(authz.PermManageSettings))

			// when
			rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

			// then a failed check is never a silent denial or a silent grant
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, unavailableBody, rec.Body.String())
			require.Len(t, f.recorded, 1)
			assert.ErrorIs(t, f.recorded[0], tc.cause)
		})
	}
}

func TestPermission_RefusesBeforeThePermissionIsAsked(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		cookie     string
		expect     func(f *authFixture)
		wantStatus int
		wantBody   string
	}{
		{name: "no session cookie", method: http.MethodGet, expect: func(*authFixture) {}, wantStatus: http.StatusUnauthorized, wantBody: authRequiredBody},
		{
			name:   "an unknown or expired session",
			method: http.MethodGet,
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   invalidBody,
		},
		{
			name:   "a banned user, whose session is revoked",
			method: http.MethodGet,
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				userID := f.liveSession()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
				f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)
			},
			wantStatus: http.StatusForbidden,
			wantBody:   bannedBody,
		},
		{
			name:   "a write from a locked account",
			method: http.MethodPost,
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				userID := f.liveSession()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)
			},
			wantStatus: http.StatusForbidden,
			wantBody:   lockedBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the permission is never asked, which the strict authz mock would report
			f := newAuthFixture(t, nil)
			tc.expect(f)
			engine := f.engine(t, f.guards.Permission(authz.PermManageSettings))

			// when
			rec := guardedRequest(t, engine, tc.method, probeRoute, tc.cookie)

			// then
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.JSONEq(t, tc.wantBody, rec.Body.String())
		})
	}
}

func TestTenantPermission_LetsAMemberWithThePermissionThrough(t *testing.T) {
	methods := []struct {
		name   string
		method string
		expect func(f *authFixture, userID uuid.UUID)
	}{
		{name: "a read, which never consults the lock", method: http.MethodGet, expect: func(*authFixture, uuid.UUID) {}},
		{
			name:   "a write from an unlocked account",
			method: http.MethodPost,
			expect: func(f *authFixture, userID uuid.UUID) {
				f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			},
		},
	}

	for _, guard := range tenantGuards {
		for _, tc := range methods {
			t.Run(guard.name+" "+tc.name, func(t *testing.T) {
				// given
				f := newAuthFixture(t, nil)
				userID, tenantID := f.liveSession(), uuid.New()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				tc.expect(f, userID)
				guard.decides(f, userID, tenantID, orgauthz.Decision{Member: true, Allowed: true}, nil)
				engine := f.engine(t, guard.build(f.guards))

				// when
				rec := guardedRequest(t, engine, tc.method, guard.path(tenantID.String()), hostSessionCookie)

				// then
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, signedIn(userID), rec.Body.String())
			})
		}
	}
}

func TestTenantPermission_AnswersNotFoundToANonMemberAndForbiddenToAMemberWithoutThePermission(t *testing.T) {
	decisions := []struct {
		name       string
		decision   orgauthz.Decision
		wantStatus int
		forbidden  bool
	}{
		{name: "someone outside, who must not learn the tenant exists", decision: orgauthz.Decision{}, wantStatus: http.StatusNotFound},
		{name: "a member without the permission", decision: orgauthz.Decision{Member: true}, wantStatus: http.StatusForbidden, forbidden: true},
	}

	for _, guard := range tenantGuards {
		for _, tc := range decisions {
			t.Run(guard.name+" "+tc.name, func(t *testing.T) {
				// given
				f := newAuthFixture(t, nil)
				userID, tenantID := f.liveSession(), uuid.New()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				guard.decides(f, userID, tenantID, tc.decision, nil)
				engine := f.engine(t, guard.build(f.guards))

				wantBody := guard.notFoundBody
				if tc.forbidden {
					wantBody = insufficientBody
				}

				// when
				rec := guardedRequest(t, engine, http.MethodGet, guard.path(tenantID.String()), hostSessionCookie)

				// then the handler never runs
				assert.Equal(t, tc.wantStatus, rec.Code)
				assert.JSONEq(t, wantBody, rec.Body.String())
				assert.Empty(t, f.recorded)
			})
		}
	}
}

func TestTenantPermission_AnswersServiceUnavailableWhenThePermissionCannotBeChecked(t *testing.T) {
	causes := []struct {
		name  string
		cause error
	}{
		{name: "the membership store is down", cause: errDatabaseDown},
		{name: "the guard names no declared permission", cause: fmt.Errorf("%w: %q", orgauthz.ErrUnknownPermission, "org.teleport")},
		{name: "the guard names a permission of the other scope", cause: fmt.Errorf("%w: %q", orgauthz.ErrOutOfScope, "org.view")},
	}

	for _, guard := range tenantGuards {
		for _, tc := range causes {
			t.Run(guard.name+" "+tc.name, func(t *testing.T) {
				// given
				f := newAuthFixture(t, nil)
				userID, tenantID := f.liveSession(), uuid.New()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				guard.decides(f, userID, tenantID, orgauthz.Decision{}, tc.cause)
				engine := f.engine(t, guard.build(f.guards))

				// when
				rec := guardedRequest(t, engine, http.MethodGet, guard.path(tenantID.String()), hostSessionCookie)

				// then a failed check is never a silent denial, a 404 or a silent grant
				assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
				assert.JSONEq(t, unavailableBody, rec.Body.String())
				require.Len(t, f.recorded, 1)
				assert.ErrorIs(t, f.recorded[0], tc.cause)
			})
		}
	}
}

func TestTenantPermission_RefusesAMalformedIDBeforeThePermissionIsAsked(t *testing.T) {
	for _, guard := range tenantGuards {
		t.Run(guard.name, func(t *testing.T) {
			// given the strict orgauthz mock would report any question
			f := newAuthFixture(t, nil)
			userID := f.liveSession()
			f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
			engine := f.engine(t, guard.build(f.guards))

			// when
			rec := guardedRequest(t, engine, http.MethodGet, guard.path("not-a-uuid"), hostSessionCookie)

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, guard.badIDBody, rec.Body.String())
		})
	}
}

func TestTenantPermission_RefusesBeforeThePermissionIsAsked(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		cookie     string
		expect     func(f *authFixture)
		wantStatus int
		wantBody   string
	}{
		{name: "no session cookie", method: http.MethodGet, expect: func(*authFixture) {}, wantStatus: http.StatusUnauthorized, wantBody: authRequiredBody},
		{
			name:   "an unknown or expired session",
			method: http.MethodGet,
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   invalidBody,
		},
		{
			name:   "a banned user, whose session is revoked",
			method: http.MethodGet,
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				userID := f.liveSession()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
				f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)
			},
			wantStatus: http.StatusForbidden,
			wantBody:   bannedBody,
		},
		{
			name:   "a write from a locked account",
			method: http.MethodPost,
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				userID := f.liveSession()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				f.accounts.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)
			},
			wantStatus: http.StatusForbidden,
			wantBody:   lockedBody,
		},
	}

	for _, guard := range tenantGuards {
		for _, tc := range tests {
			t.Run(guard.name+" "+tc.name, func(t *testing.T) {
				// given the permission is never asked, which the strict orgauthz mock would report
				f := newAuthFixture(t, nil)
				tc.expect(f)
				engine := f.engine(t, guard.build(f.guards))

				// when
				rec := guardedRequest(t, engine, tc.method, guard.path(uuid.NewString()), tc.cookie)

				// then
				assert.Equal(t, tc.wantStatus, rec.Code)
				assert.JSONEq(t, tc.wantBody, rec.Body.String())
			})
		}
	}
}

func TestOptionalAuth_ContinuesWithoutAPrincipal(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		expect func(f *authFixture)
	}{
		{name: "no session cookie", expect: func(*authFixture) {}},
		{
			name:   "an unknown or expired session",
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))
			},
		},
		{
			name:   "a banned user, whose session is revoked",
			cookie: hostSessionCookie,
			expect: func(f *authFixture) {
				userID := f.liveSession()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
				f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newAuthFixture(t, nil)
			tc.expect(f)
			engine := f.engine(t, f.guards.OptionalAuth)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, probeRoute, tc.cookie)

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, anonymousBody, rec.Body.String())
			assert.Empty(t, f.recorded)
		})
	}
}

func TestOptionalAuth_LoadsTheUserWhenThereIsOne(t *testing.T) {
	// given writes are not gated, because the handler decides what an anonymous caller may do
	f := newAuthFixture(t, nil)
	userID := f.liveSession()
	f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
	engine := f.engine(t, f.guards.OptionalAuth)

	// when
	rec := guardedRequest(t, engine, http.MethodPost, probeRoute, hostSessionCookie)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, signedIn(userID), rec.Body.String())
}

func TestOptionalAuth_NeverTreatsAFailedCheckAsAnonymous(t *testing.T) {
	tests := []struct {
		name       string
		baseURLErr error
		expect     func(f *authFixture)
	}{
		{name: "the base url", baseURLErr: errDatabaseDown, expect: func(*authFixture) {}},
		{
			name: "the session",
			expect: func(f *authFixture) {
				f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the ban",
			expect: func(f *authFixture) {
				userID := f.liveSession()
				f.accounts.EXPECT().IsBanned(mock.Anything, userID).Return(false, errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newAuthFixture(t, tc.baseURLErr)
			tc.expect(f)
			engine := f.engine(t, f.guards.OptionalAuth)

			// when
			rec := guardedRequest(t, engine, http.MethodGet, probeRoute, hostSessionCookie)

			// then
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, unavailableBody, rec.Body.String())
			require.Len(t, f.recorded, 1)
			assert.ErrorIs(t, f.recorded[0], errDatabaseDown)
		})
	}
}
