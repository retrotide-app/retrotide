package team

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/entitlements"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	testMocks struct {
		teams         *repository.MockTeamRepository
		organisations *repository.MockOrganisationRepository
		auditLog      *repository.MockAuditLogRepository
		hierarchy     *hierarchy.MockService
		entitlements  *entitlements.MockService
	}
)

const (
	testIP = "203.0.113.9"
)

var (
	testCreated = time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		teams:         repository.NewMockTeamRepository(t),
		organisations: repository.NewMockOrganisationRepository(t),
		auditLog:      repository.NewMockAuditLogRepository(t),
		hierarchy:     hierarchy.NewMockService(t),
		entitlements:  entitlements.NewMockService(t),
	}

	svc := NewService(Deps{
		Teams:         m.teams,
		Organisations: m.organisations,
		AuditLog:      m.auditLog,
		Hierarchy:     m.hierarchy,
		Entitlements:  m.entitlements,
	}).(*service)

	return svc, m
}

func (m *testMocks) audited(entry audit.NewEntry) {
	entry.ActorKind = audit.ActorUser
	entry.IP = testIP

	m.auditLog.EXPECT().Create(mock.Anything, entry).Return(nil)
}

func (m *testMocks) orgRole(orgID uuid.UUID, userID uuid.UUID, r model.OrgRole) {
	key := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}

	m.organisations.EXPECT().GetMember(mock.Anything, key).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: r, JoinedAt: testCreated}, nil)
}

func (m *testMocks) teamRole(team *model.Team, userID uuid.UUID, r model.TeamRole) {
	key := spec.TeamMemberKey{OrgID: team.OrgID, TeamID: team.ID, UserID: userID}

	m.teams.EXPECT().GetMember(mock.Anything, key).Return(&model.TeamMember{OrgID: team.OrgID, TeamID: team.ID, UserID: userID, Role: r, JoinedAt: testCreated}, nil)
}

func (m *testMocks) live(orgID uuid.UUID) {
	found := &model.Organisation{ID: orgID, Name: "Acme Retros"}

	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(found, nil)
}

func (m *testMocks) gone(orgID uuid.UUID) {
	found := &model.Organisation{ID: orgID, Name: "Acme Retros", DeletedAt: new(testCreated)}

	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(found, nil)
}

func (m *testMocks) known(team *model.Team) {
	found := *team

	m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(&found, nil)
	m.live(team.OrgID)
}

func archived(team *model.Team) *model.Team {
	found := *team
	found.ArchivedAt = new(testCreated)

	return &found
}

func platform(orgID uuid.UUID) *model.Team {
	return &model.Team{ID: uuid.New(), OrgID: orgID, Name: "Platform", CreatedAt: testCreated}
}

func expectedTeamEntry(team *model.Team, actorID uuid.UUID, action audit.Action, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      team.OrgID,
		ActorID:    actorID,
		Action:     action,
		TargetType: audit.TargetTeam,
		TargetID:   team.ID.String(),
		Details:    details,
	}
}

func expectedMemberEntry(team *model.Team, actorID uuid.UUID, action audit.Action, userID uuid.UUID, details map[string]string) audit.NewEntry {
	entry := expectedTeamEntry(team, actorID, action, details)
	entry.SubjectID = userID

	return entry
}

func TestCleanName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "trimmed", in: "  Platform  ", want: "Platform"},
		{name: "stored as written", in: "<b>R&D</b>", want: "<b>R&D</b>"},
		{name: "eighty characters", in: strings.Repeat("é", 80), want: strings.Repeat("é", 80)},
		{name: "empty", in: "", wantErr: ErrNameRequired},
		{name: "only spaces", in: " \t ", wantErr: ErrNameRequired},
		{name: "eighty-one characters", in: strings.Repeat("é", 81), wantErr: ErrNameTooLong},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := CleanName(tc.in)

			// then
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestViewerRole_OwnersAndAdminsLeadEveryTeamWithoutAMembership(t *testing.T) {
	for _, r := range []model.OrgRole{model.OrgRoleOwner, model.OrgRoleAdmin} {
		t.Run(string(r), func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team, userID := platform(uuid.New()), uuid.New()
			m.orgRole(team.OrgID, userID, r)

			// when
			got, err := svc.viewerRole(t.Context(), *team, userID)

			// then the strict mocks prove the team membership was never read
			require.NoError(t, err)
			assert.Equal(t, model.TeamRoleLead, got)
		})
	}
}

func TestViewerRole_AMemberHoldsTheirOwnTeamRole(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, userID uuid.UUID)
		want  model.TeamRole
	}{
		{name: "a lead", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.teamRole(team, userID, model.TeamRoleLead)
		}, want: model.TeamRoleLead},
		{name: "a team member", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.teamRole(team, userID, model.TeamRoleMember)
		}, want: model.TeamRoleMember},
		{name: "not on the team", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.teams.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team, userID := platform(uuid.New()), uuid.New()
			m.orgRole(team.OrgID, userID, model.OrgRoleMember)
			tc.given(m, team, userID)

			// when
			got, err := svc.viewerRole(t.Context(), *team, userID)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestViewerRole_ReportsWhatTheReadsFound(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, userID uuid.UUID)
		want  error
	}{
		{name: "someone outside the organisation", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a failed organisation read", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a failed team read", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.orgRole(team.OrgID, userID, model.OrgRoleMember)
			m.teams.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team, userID := platform(uuid.New()), uuid.New()
			tc.given(m, team, userID)

			// when
			got, err := svc.viewerRole(t.Context(), *team, userID)

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Empty(t, got)
		})
	}
}

func TestRequireLiveOrganisation_RefusesWhatIsNoLongerThere(t *testing.T) {
	tests := []struct {
		name  string
		found *model.Organisation
		err   error
		want  error
	}{
		{name: "a soft-deleted organisation", found: &model.Organisation{DeletedAt: new(testCreated)}, want: ErrOrganisationNotFound},
		{name: "a purged organisation", err: errRowMissing, want: ErrOrganisationNotFound},
		{name: "a failed read", err: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(tc.found, tc.err)

			// when
			err := svc.requireLiveOrganisation(t.Context(), orgID)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestSettle_AStaleCacheOutranksTheAuditOutcome(t *testing.T) {
	// given
	auditErr := fmt.Errorf("%w: %w", ErrNotAudited, errAuditDown)

	// when
	both, onlyAudit, neither := settle(errCacheDown, auditErr), settle(nil, auditErr), settle(nil, nil)

	// then
	assert.ErrorIs(t, both, ErrCacheStale)
	assert.ErrorIs(t, both, ErrNotAudited)
	assert.ErrorIs(t, both, repository.ErrStaleTenantCache)
	assert.NotErrorIs(t, onlyAudit, ErrCacheStale)
	assert.ErrorIs(t, onlyAudit, ErrNotAudited)
	assert.NoError(t, neither)
}
