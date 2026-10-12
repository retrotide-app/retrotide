package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dronebl"
	"retrotide/internal/model"
	"retrotide/internal/repository"
	"retrotide/internal/role"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	restrictedFixture struct {
		userID      uuid.UUID
		checker     *dronebl.MockService
		sessionRepo *repository.MockSessionRepository
		roles       *repository.MockRoleRepository
		users       *repository.MockUserRepository
		engine      *gin.Engine
	}
)

const (
	restrictedWriteRoute = "/api/v1/probe"
	restrictedStaffRoute = "/api/v1/admin/probe"
)

func newRestrictedFixture(t *testing.T, held role.Role) *restrictedFixture {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil).Maybe()
	settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(true, nil)

	f := &restrictedFixture{
		userID:      uuid.New(),
		checker:     dronebl.NewMockService(t),
		sessionRepo: repository.NewMockSessionRepository(t),
		roles:       repository.NewMockRoleRepository(t),
		users:       repository.NewMockUserRepository(t),
	}
	f.sessionRepo.EXPECT().Get(mock.Anything, sessionToken).Return(&model.Session{UserID: f.userID}, nil)
	f.roles.EXPECT().GetRole(mock.Anything, f.userID).Return(held, nil)
	f.users.EXPECT().IsBanned(mock.Anything, f.userID).Return(true, nil)
	f.users.EXPECT().IsLocked(mock.Anything, f.userID).Return(true, nil).Maybe()

	sessions := session.NewManager(f.sessionRepo, settingsSvc)
	authzSvc := authz.NewService(f.roles, repository.NewMockPermissionRepository(t), f.users)
	guards := NewGuards(GuardDeps{Settings: settingsSvc, Sessions: sessions, Authz: authzSvc})

	f.engine = newTestEngine(t)
	f.engine.Use(Maintenance(settingsSvc, sessions, authzSvc), RequireCleanIP(f.checker, sessions))
	f.engine.POST(restrictedWriteRoute, guards.Auth, principalProbe)
	f.engine.POST(restrictedStaffRoute, guards.Permission(authz.PermManageSettings), principalProbe)

	return f
}

func (f *restrictedFixture) post(t *testing.T, route string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, route, nil)
	req.RemoteAddr = listedPeer
	req.AddCookie(&http.Cookie{Name: hostSessionCookie, Value: sessionToken})

	return serve(f.engine, req)
}

func TestSuperAdmin_PassesEveryAccountRestrictionOnASignedInSession(t *testing.T) {
	for _, route := range []string{restrictedWriteRoute, restrictedStaffRoute} {
		t.Run(route, func(t *testing.T) {
			// given a banned, locked super admin writing from a listed address while the site is under maintenance
			f := newRestrictedFixture(t, role.SuperAdmin)
			f.checker.EXPECT().Check(mock.Anything, listedClientIP).Return(listedVerdict, nil).Once()

			// when
			rec := f.post(t, route)

			// then the ban, maintenance, DroneBL, lock and permission checks all let the session through, and it is never revoked
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, signedIn(f.userID), rec.Body.String())
		})
	}
}

func TestSuperAdmin_TheSameRestrictionsStillHoldForAnAdmin(t *testing.T) {
	// given an admin, who holds every permission but no exemption, in the same position
	f := newRestrictedFixture(t, role.Admin)
	f.sessionRepo.EXPECT().Delete(mock.Anything, sessionToken).Return(nil)

	// when
	rec := f.post(t, restrictedWriteRoute)

	// then the ban revokes the session, so maintenance closes the api to it before the blocklist is asked
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, maintenanceBody, rec.Body.String())
}
