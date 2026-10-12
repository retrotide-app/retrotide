package admin

import (
	"math"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/authz"
	"retrotide/internal/email"
	"retrotide/internal/entitlements"
	"retrotide/internal/hierarchy"
	"retrotide/internal/identity"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/role"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	testMocks struct {
		users         *repository.MockUserRepository
		roles         *repository.MockRoleRepository
		permissions   *repository.MockPermissionRepository
		auditLog      *repository.MockAuditLogRepository
		verifications *repository.MockEmailVerificationRepository
		invitations   *repository.MockInvitationRepository
		organisations *repository.MockOrganisationRepository
		teams         *repository.MockTeamRepository
		stats         *repository.MockStatsRepository
		sessionRepo   *repository.MockSessionRepository
		authz         *authz.MockService
		accounts      *user.MockService
		settings      *settings.MockService
		email         *email.MockService
		entitlements  *entitlements.MockService
		google        *identity.MockProvider
		disconnector  *session.MockDisconnector
	}
)

const (
	testIP          = "203.0.113.9"
	testSiteName    = "RetroTide"
	testBaseURL     = "https://retrotide.test/"
	testEmail       = "alice@example.com"
	testNewEmail    = "alice@work.example"
	testDisplayName = "Alice"
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		users:         repository.NewMockUserRepository(t),
		roles:         repository.NewMockRoleRepository(t),
		permissions:   repository.NewMockPermissionRepository(t),
		auditLog:      repository.NewMockAuditLogRepository(t),
		verifications: repository.NewMockEmailVerificationRepository(t),
		invitations:   repository.NewMockInvitationRepository(t),
		organisations: repository.NewMockOrganisationRepository(t),
		teams:         repository.NewMockTeamRepository(t),
		stats:         repository.NewMockStatsRepository(t),
		sessionRepo:   repository.NewMockSessionRepository(t),
		authz:         authz.NewMockService(t),
		accounts:      user.NewMockService(t),
		settings:      settings.NewMockService(t),
		email:         email.NewMockService(t),
		entitlements:  entitlements.NewMockService(t),
		google:        identity.NewMockProvider(t),
		disconnector:  session.NewMockDisconnector(t),
	}

	sessions := session.NewManager(m.sessionRepo, m.settings)
	sessions.SetDisconnector(m.disconnector)

	svc := NewService(Deps{
		Users:           m.users,
		Roles:           m.roles,
		Permissions:     m.permissions,
		AuditLog:        m.auditLog,
		Verifications:   m.verifications,
		Invitations:     m.invitations,
		Organisations:   m.organisations,
		Teams:           m.teams,
		Stats:           m.stats,
		Authz:           m.authz,
		Hierarchy:       hierarchy.NewService(hierarchy.Deps{Authz: m.authz}),
		Accounts:        m.accounts,
		Settings:        m.settings,
		Sessions:        sessions,
		Email:           m.email,
		Entitlements:    m.entitlements,
		SignInProviders: []identity.Provider{m.google},
	}).(*service)

	return svc, m
}

func (m *testMocks) ranks(actorID uuid.UUID, actor role.Role, targetID uuid.UUID, target role.Role) {
	m.authz.EXPECT().GetRole(mock.Anything, actorID).Return(actor, nil)
	m.authz.EXPECT().GetRole(mock.Anything, targetID).Return(target, nil)
}

func (m *testMocks) audited(entry audit.NewEntry) {
	entry.ActorKind = audit.ActorStaff
	entry.IP = testIP

	m.auditLog.EXPECT().Create(mock.Anything, entry).Return(nil)
}

func staffAction(actorID uuid.UUID) spec.StaffAction {
	return spec.StaffAction{ActorID: actorID, IP: testIP}
}

func userAction(actorID uuid.UUID, userID uuid.UUID) spec.StaffUserAction {
	return spec.StaffUserAction{StaffAction: staffAction(actorID), UserID: userID}
}

func expectedUserEntry(actorID uuid.UUID, action audit.Action, userID uuid.UUID, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		ActorID:    actorID,
		Action:     action,
		TargetType: audit.TargetUser,
		TargetID:   userID.String(),
		SubjectID:  userID,
		Details:    details,
	}
}

func account(userID uuid.UUID) *model.User {
	return &model.User{ID: userID, Email: testEmail, DisplayName: testDisplayName, EmailVerified: true, CreatedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
}

func TestAuthorise_OnlyAnOutrankingActorMayTouchAnAccount(t *testing.T) {
	tests := []struct {
		name    string
		actor   role.Role
		target  role.Role
		allowed bool
	}{
		{name: "support on an ordinary user", actor: role.Support, target: "", allowed: true},
		{name: "admin on support", actor: role.Admin, target: role.Support, allowed: true},
		{name: "super admin on admin", actor: role.SuperAdmin, target: role.Admin, allowed: true},
		{name: "support on support", actor: role.Support, target: role.Support, allowed: false},
		{name: "support on admin", actor: role.Support, target: role.Admin, allowed: false},
		{name: "admin on admin", actor: role.Admin, target: role.Admin, allowed: false},
		{name: "admin on super admin", actor: role.Admin, target: role.SuperAdmin, allowed: false},
		{name: "super admin on super admin", actor: role.SuperAdmin, target: role.SuperAdmin, allowed: false},
		{name: "an actor with no staff role", actor: "", target: "", allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			actorID, targetID := uuid.New(), uuid.New()
			m.ranks(actorID, tc.actor, targetID, tc.target)
			if tc.allowed {
				m.users.EXPECT().Unban(mock.Anything, targetID).Return(nil)
				m.audited(expectedUserEntry(actorID, audit.ActionUnbanUser, targetID, nil))
			}

			// when
			err := svc.UnbanUser(t.Context(), userAction(actorID, targetID))

			// then the strict mocks prove that a refused action writes nothing
			if tc.allowed {
				assert.NoError(t, err)
				return
			}

			assert.ErrorIs(t, err, ErrProtectedUser)
			assert.ErrorIs(t, err, hierarchy.ErrOutranked)
		})
	}
}

func TestAuthorise_StaffCannotActOnThemselves(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.authz.EXPECT().GetRole(mock.Anything, actorID).Return(role.Admin, nil)

	// when
	err := svc.UnbanUser(t.Context(), userAction(actorID, actorID))

	// then
	assert.ErrorIs(t, err, ErrProtectedUser)
}

func TestAuthorise_ReturnsARoleReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.authz.EXPECT().GetRole(mock.Anything, actorID).Return(role.Admin, nil)
	m.authz.EXPECT().GetRole(mock.Anything, targetID).Return("", errDatabaseDown)

	// when
	err := svc.UnbanUser(t.Context(), userAction(actorID, targetID))

	// then it is never read as "no role", which would let the action through
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrProtectedUser)
}

func TestRecord_AFailedAuditEntryIsReportedAfterTheChange(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.users.EXPECT().Unban(mock.Anything, targetID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	err := svc.UnbanUser(t.Context(), userAction(actorID, targetID))

	// then the unban stands, and the caller learns that it went unrecorded
	assert.ErrorIs(t, err, ErrNotAudited)
	assert.ErrorIs(t, err, errAuditDown)
}

func TestClampPage(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{name: "no limit takes the default", limit: 0, offset: 0, wantLimit: 20, wantOffset: 0},
		{name: "a negative limit takes the default", limit: -5, offset: 10, wantLimit: 20, wantOffset: 10},
		{name: "a large limit is capped", limit: 500, offset: 0, wantLimit: 100, wantOffset: 0},
		{name: "a negative offset starts at the beginning", limit: 10, offset: -3, wantLimit: 10, wantOffset: 0},
		{name: "a deep offset is kept", limit: 10, offset: 5000, wantLimit: 10, wantOffset: 5000},
		{name: "an offset past int32 is capped so the query never truncates it", limit: 10, offset: math.MaxInt32 + 1, wantLimit: 10, wantOffset: math.MaxInt32},
		{name: "an offset that would wrap to zero is capped too", limit: 10, offset: 1 << 32, wantLimit: 10, wantOffset: math.MaxInt32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			limit, offset := clampPage(tc.limit, tc.offset, defaultUserPageSize)

			// then
			assert.Equal(t, tc.wantLimit, limit)
			assert.Equal(t, tc.wantOffset, offset)
		})
	}
}
