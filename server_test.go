package main

import (
	"net/http"
	"testing"
	"testing/fstest"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/controllers"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/health"
	"retrotide/internal/middleware"
	"retrotide/internal/settings"
	"retrotide/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testMetricsToken = "s3cret"
)

var (
	pprofPaths = []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/goroutine?debug=1",
		"/debug/pprof/heap",
		"/debug/pprof/profile?seconds=1",
		"/debug/pprof/trace?seconds=1",
		"/debug/pprof/symbol",
	}
)

func newProductionEngine(t *testing.T, settingsSvc settings.Service) (*gin.Engine, *controllers.Service) {
	t.Helper()

	engine, err := utils.NewEngine(utils.DefaultTrustedProxies, config.DeploymentHosted)
	require.NoError(t, err)

	ctrl := &controllers.Service{
		HealthService: health.NewMockService(t),
		Guards:        middleware.NewGuards(middleware.GuardDeps{Settings: settingsSvc}),
		StaticFS:      fstest.MapFS{},
	}

	return engine, ctrl
}

func newPprofHarness(t *testing.T) *testutil.Harness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	ctrl := &controllers.Service{
		HealthService: health.NewMockService(t),
		Guards:        h.Guards,
		StaticFS:      fstest.MapFS{},
	}
	registerAllRoutes(h.Engine, ctrl, h.Guards)

	return h
}

func routeTable(engine *gin.Engine) []string {
	var registered []string
	for _, route := range engine.Routes() {
		registered = append(registered, route.Method+" "+route.Path)
	}

	return registered
}

func TestInitApp_RouteTable(t *testing.T) {
	// given
	engine, ctrl := newProductionEngine(t, settings.NewMockService(t))

	// when every list is mounted on one engine exactly as initApp mounts it, so a conflict panics here rather than at boot
	registerAllRoutes(engine, ctrl, ctrl.Guards)

	// then
	assert.ElementsMatch(t, []string{
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/invitation",
		"POST /api/v1/auth/invitation/accept",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"GET /api/v1/auth/session",
		"HEAD /api/v1/auth/session",
		"POST /api/v1/auth/verify-email",
		"POST /api/v1/auth/resend-verification",
		"POST /api/v1/auth/forgot-password",
		"POST /api/v1/auth/reset-password",
		"POST /api/v1/auth/set-email",
		"PUT /api/v1/auth/password",
		"PUT /api/v1/auth/profile",
		"POST /api/v1/auth/sign-out-other-devices",
		"DELETE /api/v1/auth/account",
		"GET /api/v1/auth/identities",
		"HEAD /api/v1/auth/identities",
		"DELETE /api/v1/auth/identities/:provider",
		"POST /api/v1/auth/:provider/start",
		"GET /api/v1/auth/:provider/callback",
		"HEAD /api/v1/auth/:provider/callback",
		"GET /api/v1/auth/two-factor",
		"HEAD /api/v1/auth/two-factor",
		"POST /api/v1/auth/two-factor/enrolment",
		"POST /api/v1/auth/two-factor/confirmation",
		"DELETE /api/v1/auth/two-factor",
		"POST /api/v1/auth/two-factor/verify",
		"GET /api/v1/notifications",
		"HEAD /api/v1/notifications",
		"GET /api/v1/notifications/unread-count",
		"HEAD /api/v1/notifications/unread-count",
		"POST /api/v1/notifications/read",
		"POST /api/v1/notifications/:id/read",
		"GET /api/v1/site-info",
		"HEAD /api/v1/site-info",
		"POST /api/v1/client-errors",
		"GET /api/v1/admin/users",
		"HEAD /api/v1/admin/users",
		"GET /api/v1/admin/users/:id",
		"HEAD /api/v1/admin/users/:id",
		"GET /api/v1/admin/users/:id/audit-log",
		"HEAD /api/v1/admin/users/:id/audit-log",
		"POST /api/v1/admin/users/:id/role",
		"DELETE /api/v1/admin/users/:id/role",
		"POST /api/v1/admin/users/:id/ban",
		"POST /api/v1/admin/users/:id/unban",
		"POST /api/v1/admin/users/:id/lock",
		"POST /api/v1/admin/users/:id/unlock",
		"POST /api/v1/admin/users/:id/force-logout",
		"DELETE /api/v1/admin/users/:id",
		"POST /api/v1/admin/users/:id/reset-password",
		"PUT /api/v1/admin/users/:id/email",
		"POST /api/v1/admin/users/:id/verify-email",
		"POST /api/v1/admin/users/:id/unverify-email",
		"PUT /api/v1/admin/users/:id/display-name",
		"GET /api/v1/admin/invitations",
		"HEAD /api/v1/admin/invitations",
		"POST /api/v1/admin/invitations",
		"POST /api/v1/admin/invitations/:id/resend",
		"DELETE /api/v1/admin/invitations/:id",
		"GET /api/v1/admin/settings",
		"HEAD /api/v1/admin/settings",
		"PUT /api/v1/admin/settings",
		"POST /api/v1/admin/settings/test-email",
		"GET /api/v1/admin/permissions",
		"HEAD /api/v1/admin/permissions",
		"PUT /api/v1/admin/permissions/roles/:role",
		"GET /api/v1/admin/audit-log",
		"HEAD /api/v1/admin/audit-log",
		"GET /api/v1/admin/stats",
		"HEAD /api/v1/admin/stats",
		"GET /api/v1/admin/organisations",
		"HEAD /api/v1/admin/organisations",
		"GET /api/v1/admin/organisations/:orgID",
		"HEAD /api/v1/admin/organisations/:orgID",
		"POST /api/v1/orgs",
		"GET /api/v1/orgs",
		"HEAD /api/v1/orgs",
		"GET /api/v1/orgs/:orgID",
		"HEAD /api/v1/orgs/:orgID",
		"PUT /api/v1/orgs/:orgID",
		"DELETE /api/v1/orgs/:orgID",
		"POST /api/v1/orgs/:orgID/restore",
		"POST /api/v1/orgs/:orgID/transfer",
		"POST /api/v1/orgs/:orgID/leave",
		"GET /api/v1/orgs/:orgID/members",
		"HEAD /api/v1/orgs/:orgID/members",
		"PUT /api/v1/orgs/:orgID/members/:userID/role",
		"DELETE /api/v1/orgs/:orgID/members/:userID",
		"GET /api/v1/orgs/:orgID/entitlements",
		"HEAD /api/v1/orgs/:orgID/entitlements",
		"GET /api/v1/orgs/:orgID/teams",
		"HEAD /api/v1/orgs/:orgID/teams",
		"POST /api/v1/orgs/:orgID/teams",
		"POST /api/v1/orgs/:orgID/teams/:teamID/archive",
		"DELETE /api/v1/orgs/:orgID/teams/:teamID",
		"GET /api/v1/teams/:teamID",
		"HEAD /api/v1/teams/:teamID",
		"PUT /api/v1/teams/:teamID",
		"GET /api/v1/teams/:teamID/members",
		"HEAD /api/v1/teams/:teamID/members",
		"POST /api/v1/teams/:teamID/members",
		"PUT /api/v1/teams/:teamID/members/:userID/role",
		"DELETE /api/v1/teams/:teamID/members/:userID",
		"GET /api/v1/orgs/:orgID/invites",
		"HEAD /api/v1/orgs/:orgID/invites",
		"POST /api/v1/orgs/:orgID/invites",
		"POST /api/v1/orgs/:orgID/invites/:inviteID/resend",
		"DELETE /api/v1/orgs/:orgID/invites/:inviteID",
		"POST /api/v1/invites/lookup",
		"POST /api/v1/invites/accept",
		"POST /api/v1/invites/register",
		"GET /api/v1/orgs/:orgID/boards",
		"HEAD /api/v1/orgs/:orgID/boards",
		"POST /api/v1/orgs/:orgID/boards",
		"GET /api/v1/teams/:teamID/boards",
		"HEAD /api/v1/teams/:teamID/boards",
		"POST /api/v1/teams/:teamID/boards",
		"GET /api/v1/boards/:boardID",
		"HEAD /api/v1/boards/:boardID",
		"POST /api/v1/boards/:boardID/participants/me",
		"PUT /api/v1/boards/:boardID/context",
		"GET /api/v1/boards/:boardID/sharing",
		"HEAD /api/v1/boards/:boardID/sharing",
		"PUT /api/v1/boards/:boardID/sharing/link-access",
		"PUT /api/v1/boards/:boardID/sharing/password",
		"POST /api/v1/boards/:boardID/sharing/rotate",
		"POST /api/v1/boards/:boardID/archive",
		"POST /api/v1/boards/:boardID/unarchive",
		"PUT /api/v1/boards/:boardID/pin",
		"DELETE /api/v1/boards/:boardID/pin",
		"PUT /api/v1/boards/:boardID/background",
		"PUT /api/v1/boards/:boardID/team",
		"DELETE /api/v1/boards/:boardID",
		"POST /api/v1/boards/:boardID/import",
		"POST /api/v1/orgs/:orgID/boards/bulk",
		"GET /api/v1/orgs/:orgID/boards/deleted",
		"HEAD /api/v1/orgs/:orgID/boards/deleted",
		"POST /api/v1/orgs/:orgID/boards/:boardID/restore",
		"GET /api/v1/orgs/:orgID/boards/export",
		"HEAD /api/v1/orgs/:orgID/boards/export",
		"GET /api/v1/b/:slug",
		"HEAD /api/v1/b/:slug",
		"POST /api/v1/b/:slug/join",
		"GET /api/v1/auth/guest-claim",
		"HEAD /api/v1/auth/guest-claim",
		"POST /api/v1/auth/guest-claim",
		"DELETE /api/v1/auth/guest-claim",
		"POST /api/v1/boards/:boardID/columns",
		"PUT /api/v1/boards/:boardID/columns/:columnID",
		"POST /api/v1/boards/:boardID/columns/:columnID/move",
		"POST /api/v1/boards/:boardID/columns/:columnID/clear",
		"DELETE /api/v1/boards/:boardID/columns/:columnID",
		"POST /api/v1/boards/:boardID/columns/:columnID/reveal",
		"POST /api/v1/boards/:boardID/columns/:columnID/hide",
		"POST /api/v1/boards/:boardID/columns/:columnID/show",
		"POST /api/v1/boards/:boardID/columns/:columnID/votes/reset",
		"PUT /api/v1/boards/:boardID/columns/:columnID/vote-limit",
		"POST /api/v1/boards/:boardID/cards",
		"PUT /api/v1/boards/:boardID/cards/:cardID",
		"PUT /api/v1/boards/:boardID/cards/:cardID/colour",
		"POST /api/v1/boards/:boardID/cards/:cardID/move",
		"DELETE /api/v1/boards/:boardID/cards/:cardID",
		"POST /api/v1/boards/:boardID/cards/:cardID/merge",
		"POST /api/v1/boards/:boardID/cards/:cardID/unmerge",
		"PUT /api/v1/boards/:boardID/cards/:cardID/reactions/:reaction",
		"DELETE /api/v1/boards/:boardID/cards/:cardID/reactions/:reaction",
		"PUT /api/v1/boards/:boardID/cards/:cardID/pin",
		"PUT /api/v1/boards/:boardID/cards/:cardID/image",
		"POST /api/v1/boards/:boardID/cards/:cardID/votes",
		"DELETE /api/v1/boards/:boardID/cards/:cardID/votes",
		"POST /api/v1/boards/:boardID/votes/reset",
		"PUT /api/v1/boards/:boardID/settings",
		"POST /api/v1/boards/:boardID/cards/:cardID/comments",
		"PUT /api/v1/boards/:boardID/comments/:commentID",
		"DELETE /api/v1/boards/:boardID/comments/:commentID",
		"POST /api/v1/boards/:boardID/reveal",
		"POST /api/v1/boards/:boardID/hide",
		"POST /api/v1/boards/:boardID/timer",
		"POST /api/v1/boards/:boardID/close",
		"POST /api/v1/boards/:boardID/stage",
		"POST /api/v1/boards/:boardID/presentation",
		"POST /api/v1/boards/:boardID/highlight",
		"POST /api/v1/boards/:boardID/focus",
		"POST /api/v1/boards/:boardID/surveys",
		"POST /api/v1/boards/:boardID/surveys/:surveyID/close",
		"DELETE /api/v1/boards/:boardID/surveys/:surveyID",
		"PUT /api/v1/boards/:boardID/surveys/:surveyID/responses",
		"GET /api/v1/survey-templates",
		"HEAD /api/v1/survey-templates",
		"POST /api/v1/boards/:boardID/media",
		"GET /api/v1/orgs/:orgID/saved-templates",
		"HEAD /api/v1/orgs/:orgID/saved-templates",
		"POST /api/v1/boards/:boardID/saved-templates",
		"DELETE /api/v1/orgs/:orgID/saved-templates/:templateID",
		"PUT /api/v1/boards/:boardID/participants/:participantID/role",
		"DELETE /api/v1/boards/:boardID/participants/:participantID",
		"GET /api/v1/orgs/:orgID/action-items",
		"HEAD /api/v1/orgs/:orgID/action-items",
		"GET /api/v1/auth/mentions",
		"HEAD /api/v1/auth/mentions",
		"GET /api/v1/boards/:boardID/people",
		"HEAD /api/v1/boards/:boardID/people",
		"POST /api/v1/boards/:boardID/action-items",
		"PUT /api/v1/boards/:boardID/action-items/:itemID",
		"POST /api/v1/boards/:boardID/action-items/:itemID/status",
		"DELETE /api/v1/boards/:boardID/action-items/:itemID",
		"POST /api/v1/boards/:boardID/previous-actions/:itemID/review",
		"GET /api/v1/boards/:boardID/export",
		"HEAD /api/v1/boards/:boardID/export",
		"GET /api/v1/orgs/:orgID/export",
		"HEAD /api/v1/orgs/:orgID/export",
		"GET /api/v1/auth/export",
		"HEAD /api/v1/auth/export",
		"GET /api/v1/templates",
		"HEAD /api/v1/templates",
		"GET /api/v1/templates/:key",
		"HEAD /api/v1/templates/:key",
		"GET /api/v1/ws",
		"HEAD /api/v1/ws",
		"GET /livez",
		"HEAD /livez",
		"GET /health",
		"HEAD /health",
		"GET /metrics",
		"HEAD /metrics",
		"GET /robots.txt",
		"HEAD /robots.txt",
		"GET /sitemap.xml",
		"HEAD /sitemap.xml",
		"GET /uploads/:boardID/:file",
		"HEAD /uploads/:boardID/:file",
		"GET /assets/*filepath",
		"HEAD /assets/*filepath",
		"GET /self-host/docker-compose.yml",
		"HEAD /self-host/docker-compose.yml",
		"GET /debug/pprof/",
		"HEAD /debug/pprof/",
		"GET /debug/pprof/cmdline",
		"HEAD /debug/pprof/cmdline",
		"GET /debug/pprof/profile",
		"HEAD /debug/pprof/profile",
		"GET /debug/pprof/symbol",
		"HEAD /debug/pprof/symbol",
		"POST /debug/pprof/symbol",
		"GET /debug/pprof/trace",
		"HEAD /debug/pprof/trace",
		"GET /debug/pprof/:profile",
		"HEAD /debug/pprof/:profile",
	}, routeTable(engine))
}

func TestPprof_RefusesEverySessionWithoutManageSettings(t *testing.T) {
	routes := []testutil.AuthRoute{{Method: http.MethodPost, Path: "/debug/pprof/symbol"}}
	for _, path := range pprofPaths {
		routes = append(routes, testutil.AuthRoute{Method: http.MethodGet, Path: path})
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPermissionFailureSuite(t, newPprofHarness, testutil.PermissionRoute{AuthRoute: route, Permission: authz.PermManageSettings})
		})
	}
}

func TestPprof_RefusesARequestCarryingOnlyTheMetricsToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{name: "a bearer token in the Authorization header", header: "Authorization", value: "Bearer " + testMetricsToken},
		{name: "the metrics header", header: "X-Metrics-Token", value: testMetricsToken},
	}

	for _, tc := range tests {
		for _, path := range pprofPaths {
			t.Run(tc.name+" "+path, func(t *testing.T) {
				// given the metrics token is never read, which the strict settings mock would report
				h := newPprofHarness(t)

				// when
				rec := h.NewRequest(http.MethodGet, path).WithHeader(tc.header, tc.value).Record()

				// then the token that opens /metrics opens no profile
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.JSONEq(t, `{"error":"authentication required"}`, rec.Body.String())
			})
		}
	}
}

func TestPprof_ServesASessionHoldingManageSettings(t *testing.T) {
	for _, path := range pprofPaths {
		t.Run(path, func(t *testing.T) {
			// given
			h := newPprofHarness(t)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			h.Authz.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(true, nil)

			// when
			rec := h.NewRequest(http.MethodGet, path).WithCookie(cookie).Record()

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.NotEmpty(t, rec.Body.Bytes())
		})
	}
}

func TestPprof_ServesASymbolLookupToASessionHoldingManageSettings(t *testing.T) {
	// given the one pprof write passes the lock gate first
	h := newPprofHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.Authz.EXPECT().Can(mock.Anything, userID, authz.PermManageSettings).Return(true, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/debug/pprof/symbol").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "num_symbols")
}
