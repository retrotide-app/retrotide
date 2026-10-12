package team

import (
	"errors"
	"strings"
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func freePlan() *entitlements.Plan {
	return &entitlements.Plan{Code: model.PlanFree, Limits: map[model.Limit]int{model.LimitTeams: 1, model.LimitMembers: 10}}
}

func (m *testMocks) withinTheFreePlan(orgID uuid.UUID, current int) {
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(), nil)
	m.teams.EXPECT().CountActive(mock.Anything, orgID).Return(current, nil)
	m.entitlements.EXPECT().CheckWithin(mock.Anything, spec.LimitCheck{OrgID: orgID, Limit: model.LimitTeams, Current: current}).Return(nil)
}

func createRequest(orgID uuid.UUID, actorID uuid.UUID) CreateRequest {
	return CreateRequest{ActorID: actorID, IP: testIP, OrgID: orgID, Name: "  Platform  "}
}

func TestCreate_CreatesATeamWithinThePlanAndAuditsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	created := platform(orgID)
	m.live(orgID)
	m.orgRole(orgID, actorID, model.OrgRoleAdmin)
	m.withinTheFreePlan(orgID, 0)
	m.teams.EXPECT().Create(mock.Anything, spec.NewTeam{OrgID: orgID, Name: "Platform", MaxTeams: 1}).Return(created, nil)
	m.audited(expectedTeamEntry(created, actorID, audit.ActionTeamCreated, map[string]string{"name": "Platform"}))

	// when
	got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

	// then the creator, an organisation admin, leads it without a membership
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, orgID, got.OrgID)
	assert.Equal(t, "Platform", got.Name)
	assert.Equal(t, model.TeamRoleLead, got.Role)
}

func TestCreate_ASecondTeamOnFreeNeedsAnUpgrade(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	denied := &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitTeams, Allowed: 1, Current: 1}
	m.live(orgID)
	m.orgRole(orgID, actorID, model.OrgRoleOwner)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(), nil)
	m.teams.EXPECT().CountActive(mock.Anything, orgID).Return(1, nil)
	m.entitlements.EXPECT().CheckWithin(mock.Anything, spec.LimitCheck{OrgID: orgID, Limit: model.LimitTeams, Current: 1}).Return(denied)

	// when
	got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

	// then the strict mocks prove nothing was written
	assert.Nil(t, got)
	assert.ErrorIs(t, err, entitlements.ErrUpgradeRequired)
	found, ok := errors.AsType[*entitlements.DeniedError](err)
	require.True(t, ok)
	assert.Equal(t, denied, found)
}

func TestCreate_ADeletedOrganisationIsNotFoundBeforeThePlanIsAsked(t *testing.T) {
	// given an organisation that already holds its one free team
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.gone(orgID)

	// when
	got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

	// then it answers 404, never an upgrade prompt, and the strict mocks prove the plan was never read
	assert.ErrorIs(t, err, ErrOrganisationNotFound)
	assert.NotErrorIs(t, err, entitlements.ErrUpgradeRequired)
	assert.Nil(t, got)
}

func TestCreate_TheConditionalInsertRefusesATeamThatRacedPastThePlan(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.orgRole(orgID, actorID, model.OrgRoleOwner)
	m.withinTheFreePlan(orgID, 0)
	m.teams.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, dao.ErrQuotaExceeded)

	// when
	got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

	// then
	assert.Nil(t, got)
	found, ok := errors.AsType[*entitlements.DeniedError](err)
	require.True(t, ok)
	assert.Equal(t, &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitTeams, Allowed: 1, Current: 1}, found)
	assert.ErrorIs(t, err, entitlements.ErrUpgradeRequired)
}

func TestCreate_APlanThatDoesNotNameTheTeamLimitIsAnError(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.orgRole(orgID, actorID, model.OrgRoleOwner)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(&entitlements.Plan{Code: "team", Limits: map[model.Limit]int{}}, nil)

	// when
	got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

	// then a limit the plan does not name is never unlimited, and the strict mocks prove nothing was counted or written
	assert.ErrorIs(t, err, entitlements.ErrUnknownLimit)
	assert.NotErrorIs(t, err, entitlements.ErrUpgradeRequired)
	assert.Nil(t, got)
}

func TestCreate_RefusesAnInvalidNameBeforeReadingAnything(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{name: "no name", in: "  ", want: ErrNameRequired},
		{name: "a name over 80 characters", in: strings.Repeat("t", 81), want: ErrNameTooLong},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.Create(t.Context(), CreateRequest{ActorID: uuid.New(), IP: testIP, OrgID: uuid.New(), Name: tc.in})

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestCreate_MapsWhatTheWriteRefused(t *testing.T) {
	tests := []struct {
		name     string
		writeErr error
		want     error
	}{
		{name: "a name the organisation already uses, in any case", writeErr: errRowConflicted, want: ErrNameTaken},
		{name: "an organisation deleted meanwhile", writeErr: errRowMissing, want: ErrOrganisationNotFound},
		{name: "a failed write", writeErr: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			m.live(orgID)
			m.orgRole(orgID, actorID, model.OrgRoleOwner)
			m.withinTheFreePlan(orgID, 0)
			m.teams.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, tc.writeErr)

			// when
			got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestCreate_ReportsAFailedRead(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID)
		want  error
	}{
		{name: "the organisation", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a creator outside the organisation", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.live(orgID)
			m.organisations.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrOrganisationNotFound},
		{name: "the creator's membership", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.live(orgID)
			m.organisations.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "the plan", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.live(orgID)
			m.orgRole(orgID, actorID, model.OrgRoleOwner)
			m.entitlements.EXPECT().For(mock.Anything, orgID).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "the team count", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.live(orgID)
			m.orgRole(orgID, actorID, model.OrgRoleOwner)
			m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(), nil)
			m.teams.EXPECT().CountActive(mock.Anything, orgID).Return(0, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "the allowance check", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.live(orgID)
			m.orgRole(orgID, actorID, model.OrgRoleOwner)
			m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(), nil)
			m.teams.EXPECT().CountActive(mock.Anything, orgID).Return(0, nil)
			m.entitlements.EXPECT().CheckWithin(mock.Anything, mock.Anything).Return(errDatabaseDown)
		}, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			tc.given(m, orgID, actorID)

			// when
			got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.NotErrorIs(t, err, entitlements.ErrUpgradeRequired)
			assert.Nil(t, got)
		})
	}
}

func TestCreate_AnswersWithTheTeamWhenItsAuditEntryFails(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	created := platform(orgID)
	m.live(orgID)
	m.orgRole(orgID, actorID, model.OrgRoleOwner)
	m.withinTheFreePlan(orgID, 0)
	m.teams.EXPECT().Create(mock.Anything, mock.Anything).Return(created, nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	got, err := svc.Create(t.Context(), createRequest(orgID, actorID))

	// then
	assert.ErrorIs(t, err, ErrNotAudited)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
}

func TestList_ListsTheTeamsTheViewerMaySeeWithTheirEffectiveRole(t *testing.T) {
	tests := []struct {
		name    string
		orgRole model.OrgRole
		want    []model.TeamRole
	}{
		{name: "a member sees only the teams they are on, never another team's name", orgRole: model.OrgRoleMember, want: []model.TeamRole{model.TeamRoleLead, model.TeamRoleMember}},
		{name: "an admin leads every team", orgRole: model.OrgRoleAdmin, want: []model.TeamRole{model.TeamRoleLead, model.TeamRoleLead, model.TeamRoleLead}},
		{name: "the owner leads every team", orgRole: model.OrgRoleOwner, want: []model.TeamRole{model.TeamRoleLead, model.TeamRoleLead, model.TeamRoleLead}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			viewer := spec.OrganisationMemberKey{OrgID: uuid.New(), UserID: uuid.New()}
			led, joined, other := platform(viewer.OrgID), platform(viewer.OrgID), platform(viewer.OrgID)
			m.live(viewer.OrgID)
			m.orgRole(viewer.OrgID, viewer.UserID, tc.orgRole)
			m.teams.EXPECT().ListForOrganisation(mock.Anything, viewer.OrgID).Return([]model.Team{*led, *joined, *other}, nil)
			m.teams.EXPECT().ListForMember(mock.Anything, viewer).Return([]model.MemberTeam{
				{Team: *led, Role: model.TeamRoleLead},
				{Team: *joined, Role: model.TeamRoleMember},
			}, nil)

			// when
			got, err := svc.List(t.Context(), viewer)

			// then
			require.NoError(t, err)
			require.Len(t, got.Teams, len(tc.want))
			for i, want := range tc.want {
				assert.Equal(t, want, got.Teams[i].Role)
			}
			assert.Equal(t, led.ID, got.Teams[0].ID)
			assert.Equal(t, joined.ID, got.Teams[1].ID)
			if len(tc.want) == 3 {
				assert.Equal(t, other.ID, got.Teams[2].ID)
			}
		})
	}
}

func TestList_ReportsWhatTheReadsFound(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, viewer spec.OrganisationMemberKey)
		want  error
	}{
		{name: "a deleted organisation", given: func(m *testMocks, viewer spec.OrganisationMemberKey) {
			m.gone(viewer.OrgID)
		}, want: ErrOrganisationNotFound},
		{name: "a viewer outside the organisation", given: func(m *testMocks, viewer spec.OrganisationMemberKey) {
			m.live(viewer.OrgID)
			m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(nil, errRowMissing)
		}, want: ErrOrganisationNotFound},
		{name: "a failed membership read", given: func(m *testMocks, viewer spec.OrganisationMemberKey) {
			m.live(viewer.OrgID)
			m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a failed team list", given: func(m *testMocks, viewer spec.OrganisationMemberKey) {
			m.live(viewer.OrgID)
			m.orgRole(viewer.OrgID, viewer.UserID, model.OrgRoleMember)
			m.teams.EXPECT().ListForOrganisation(mock.Anything, viewer.OrgID).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a failed list of the viewer's teams", given: func(m *testMocks, viewer spec.OrganisationMemberKey) {
			m.live(viewer.OrgID)
			m.orgRole(viewer.OrgID, viewer.UserID, model.OrgRoleMember)
			m.teams.EXPECT().ListForOrganisation(mock.Anything, viewer.OrgID).Return(nil, nil)
			m.teams.EXPECT().ListForMember(mock.Anything, viewer).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			viewer := spec.OrganisationMemberKey{OrgID: uuid.New(), UserID: uuid.New()}
			tc.given(m, viewer)

			// when
			got, err := svc.List(t.Context(), viewer)

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestGet_ShowsTheTeamWithTheViewersRole(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team, userID := platform(uuid.New()), uuid.New()
	m.known(team)
	m.orgRole(team.OrgID, userID, model.OrgRoleMember)
	m.teamRole(team, userID, model.TeamRoleMember)

	// when
	got, err := svc.Get(t.Context(), Viewer{TeamID: team.ID, UserID: userID})

	// then
	require.NoError(t, err)
	assert.Equal(t, team.ID, got.ID)
	assert.Equal(t, model.TeamRoleMember, got.Role)
}

func TestGet_AnArchivedTeamCanStillBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team, userID := archived(platform(uuid.New())), uuid.New()
	m.known(team)
	m.orgRole(team.OrgID, userID, model.OrgRoleAdmin)

	// when
	got, err := svc.Get(t.Context(), Viewer{TeamID: team.ID, UserID: userID})

	// then
	require.NoError(t, err)
	assert.NotEmpty(t, got.ArchivedAt)
}

func TestGet_ReportsWhatTheReadsFound(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, userID uuid.UUID)
		want  error
	}{
		{name: "an unknown team", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a failed header read", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a team of a deleted organisation", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(team, nil)
			m.gone(team.OrgID)
		}, want: ErrOrganisationNotFound},
		{name: "a viewer outside the organisation", given: func(m *testMocks, team *model.Team, userID uuid.UUID) {
			m.known(team)
			m.organisations.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team, userID := platform(uuid.New()), uuid.New()
			tc.given(m, team, userID)

			// when
			got, err := svc.Get(t.Context(), Viewer{TeamID: team.ID, UserID: userID})

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func renameRequest(team *model.Team, actorID uuid.UUID) RenameRequest {
	return RenameRequest{ActorID: actorID, IP: testIP, TeamID: team.ID, Name: " Payments "}
}

func TestRename_RenamesWithinTheTeamsOrganisation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team, actorID := platform(uuid.New()), uuid.New()
	renamed := *team
	renamed.Name = "Payments"
	m.known(team)
	m.orgRole(team.OrgID, actorID, model.OrgRoleMember)
	m.teamRole(team, actorID, model.TeamRoleLead)
	m.teams.EXPECT().Rename(mock.Anything, spec.TeamRename{OrgID: team.OrgID, TeamID: team.ID, Name: "Payments"}).Return(&renamed, nil)

	// when
	got, err := svc.Rename(t.Context(), renameRequest(team, actorID))

	// then there is no rename entry in the audit vocabulary, so nothing is audited
	require.NoError(t, err)
	assert.Equal(t, "Payments", got.Name)
	assert.Equal(t, model.TeamRoleLead, got.Role)
}

func TestRename_AnswersWithTheTeamWhenItsCacheWasNotCleared(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team, actorID := platform(uuid.New()), uuid.New()
	renamed := *team
	renamed.Name = "Payments"
	m.known(team)
	m.orgRole(team.OrgID, actorID, model.OrgRoleAdmin)
	m.teams.EXPECT().Rename(mock.Anything, mock.Anything).Return(&renamed, errCacheDown)

	// when
	got, err := svc.Rename(t.Context(), renameRequest(team, actorID))

	// then
	assert.ErrorIs(t, err, ErrCacheStale)
	require.NotNil(t, got)
	assert.Equal(t, "Payments", got.Name)
}

func TestRename_RefusesWhatCannotBeRenamed(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, actorID uuid.UUID)
		want  error
	}{
		{name: "an unknown team", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a team of a deleted organisation", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(team, nil)
			m.gone(team.OrgID)
		}, want: ErrOrganisationNotFound},
		{name: "an archived team", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.known(archived(team))
		}, want: ErrTeamArchived},
		{name: "a name another team holds", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.known(team)
			m.orgRole(team.OrgID, actorID, model.OrgRoleOwner)
			m.teams.EXPECT().Rename(mock.Anything, mock.Anything).Return(nil, errRowConflicted)
		}, want: ErrNameTaken},
		{name: "a team deleted meanwhile", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.known(team)
			m.orgRole(team.OrgID, actorID, model.OrgRoleOwner)
			m.teams.EXPECT().Rename(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a failed write", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.known(team)
			m.orgRole(team.OrgID, actorID, model.OrgRoleOwner)
			m.teams.EXPECT().Rename(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a failed role read", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.known(team)
			m.organisations.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team, actorID := platform(uuid.New()), uuid.New()
			tc.given(m, team, actorID)

			// when
			got, err := svc.Rename(t.Context(), renameRequest(team, actorID))

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestRename_RefusesAnInvalidNameBeforeReadingAnything(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.Rename(t.Context(), RenameRequest{ActorID: uuid.New(), IP: testIP, TeamID: uuid.New(), Name: ""})

	// then
	assert.ErrorIs(t, err, ErrNameRequired)
	assert.Nil(t, got)
}

func orgTeamCommand(team *model.Team, actorID uuid.UUID) OrgTeamCommand {
	return OrgTeamCommand{ActorID: actorID, IP: testIP, OrgID: team.OrgID, TeamID: team.ID}
}

func TestArchive_ArchivesAndAudits(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team, actorID := platform(uuid.New()), uuid.New()
	key := spec.TeamKey{OrgID: team.OrgID, TeamID: team.ID}
	m.live(team.OrgID)
	m.teams.EXPECT().GetByID(mock.Anything, key).Return(team, nil)
	m.teams.EXPECT().Archive(mock.Anything, key).Return(archived(team), nil)
	m.audited(expectedTeamEntry(team, actorID, audit.ActionTeamArchived, map[string]string{"name": "Platform"}))

	// when
	err := svc.Archive(t.Context(), orgTeamCommand(team, actorID))

	// then
	assert.NoError(t, err)
}

func TestArchive_ReportsWhatHappened(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, actorID uuid.UUID)
		want  error
	}{
		{name: "a deleted organisation", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.gone(team.OrgID)
		}, want: ErrOrganisationNotFound},
		{name: "an unknown team, or one in another organisation", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a failed read", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "an archived team", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(archived(team), nil)
		}, want: ErrTeamArchived},
		{name: "archived or deleted meanwhile", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(team, nil)
			m.teams.EXPECT().Archive(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a failed write", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(team, nil)
			m.teams.EXPECT().Archive(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a header whose cache was not cleared", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(team, nil)
			m.teams.EXPECT().Archive(mock.Anything, mock.Anything).Return(team, errCacheDown)
			m.audited(expectedTeamEntry(team, actorID, audit.ActionTeamArchived, map[string]string{"name": "Platform"}))
		}, want: ErrCacheStale},
		{name: "a failed audit entry", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(team, nil)
			m.teams.EXPECT().Archive(mock.Anything, mock.Anything).Return(team, nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
		}, want: ErrNotAudited},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team, actorID := platform(uuid.New()), uuid.New()
			tc.given(m, team, actorID)

			// when
			err := svc.Archive(t.Context(), orgTeamCommand(team, actorID))

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestDelete_DeletesAndAudits(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team, actorID := platform(uuid.New()), uuid.New()
	m.live(team.OrgID)
	m.teams.EXPECT().Delete(mock.Anything, spec.TeamKey{OrgID: team.OrgID, TeamID: team.ID}).Return(team, nil)
	m.audited(expectedTeamEntry(team, actorID, audit.ActionTeamDeleted, map[string]string{"name": "Platform"}))

	// when
	err := svc.Delete(t.Context(), orgTeamCommand(team, actorID))

	// then
	assert.NoError(t, err)
}

func TestDelete_ReportsWhatHappened(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, actorID uuid.UUID)
		want  error
	}{
		{name: "a deleted organisation", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.gone(team.OrgID)
		}, want: ErrOrganisationNotFound},
		{name: "an unknown team, or one in another organisation", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a failed write", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "keys that were not cleared", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().Delete(mock.Anything, mock.Anything).Return(team, errCacheDown)
			m.audited(expectedTeamEntry(team, actorID, audit.ActionTeamDeleted, map[string]string{"name": "Platform"}))
		}, want: ErrCacheStale},
		{name: "a failed audit entry", given: func(m *testMocks, team *model.Team, actorID uuid.UUID) {
			m.live(team.OrgID)
			m.teams.EXPECT().Delete(mock.Anything, mock.Anything).Return(team, nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
		}, want: ErrNotAudited},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team, actorID := platform(uuid.New()), uuid.New()
			tc.given(m, team, actorID)

			// when
			err := svc.Delete(t.Context(), orgTeamCommand(team, actorID))

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
