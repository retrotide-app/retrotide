package team

import (
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	orgOwner   = hierarchy.Holder{Org: model.OrgRoleOwner}
	orgAdmin   = hierarchy.Holder{Org: model.OrgRoleAdmin}
	offTeam    = hierarchy.Holder{Org: model.OrgRoleMember}
	lead       = hierarchy.Holder{Org: model.OrgRoleMember, Team: model.TeamRoleLead}
	teamMember = hierarchy.Holder{Org: model.OrgRoleMember, Team: model.TeamRoleMember}
)

func (m *testMocks) acts(team *model.Team, action MemberCommand, actor hierarchy.Holder, target hierarchy.Holder) {
	standing := hierarchy.Standing{Actor: actor, Target: target}

	m.hierarchy.EXPECT().ActOnTeamMember(mock.Anything, spec.TeamMemberActionCheck{OrgID: team.OrgID, TeamID: team.ID, ActorID: action.ActorID, TargetID: action.UserID}).Return(&standing, nil)
}

func memberCommand(team *model.Team) MemberCommand {
	return MemberCommand{ActorID: uuid.New(), IP: testIP, TeamID: team.ID, UserID: uuid.New()}
}

func memberRole(team *model.Team, r model.TeamRole) MemberRole {
	return MemberRole{ActorID: uuid.New(), IP: testIP, TeamID: team.ID, UserID: uuid.New(), Role: r}
}

func teamMembership(team *model.Team, userID uuid.UUID, r model.TeamRole) *model.TeamMember {
	return &model.TeamMember{OrgID: team.OrgID, TeamID: team.ID, UserID: userID, Role: r, JoinedAt: testCreated}
}

func TestListMembers_ListsTheTeamsMembersWithinItsOrganisation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team, userID := platform(uuid.New()), uuid.New()
	m.known(team)
	m.teams.EXPECT().ListMembers(mock.Anything, spec.TeamKey{OrgID: team.OrgID, TeamID: team.ID}).Return([]model.TeamMemberProfile{
		{TeamMember: *teamMembership(team, userID, model.TeamRoleLead), Email: "alice@example.com", DisplayName: "Alice", OrgRole: model.OrgRoleMember},
		{TeamMember: *teamMembership(team, uuid.New(), model.TeamRoleMember), Email: "sam@example.com", DisplayName: "Sam", OrgRole: model.OrgRoleAdmin, SiteRole: role.Admin},
	}, nil)

	// when
	got, err := svc.ListMembers(t.Context(), team.ID)

	// then each member carries their organisation role, and their site staff role when they hold one
	require.NoError(t, err)
	require.Len(t, got.Members, 2)
	assert.Equal(t, userID, got.Members[0].UserID)
	assert.Equal(t, "Alice", got.Members[0].DisplayName)
	assert.Equal(t, model.TeamRoleLead, got.Members[0].Role)
	assert.Equal(t, model.OrgRoleMember, got.Members[0].OrgRole)
	assert.Empty(t, got.Members[0].SiteRole)
	assert.Equal(t, model.OrgRoleAdmin, got.Members[1].OrgRole)
	assert.Equal(t, role.Admin, got.Members[1].SiteRole)
}

func TestListMembers_ReportsWhatTheReadsFound(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team)
		want  error
	}{
		{name: "an unknown team", given: func(m *testMocks, team *model.Team) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "a team of a deleted organisation", given: func(m *testMocks, team *model.Team) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(team, nil)
			m.gone(team.OrgID)
		}, want: ErrOrganisationNotFound},
		{name: "a failed list", given: func(m *testMocks, team *model.Team) {
			m.known(team)
			m.teams.EXPECT().ListMembers(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			tc.given(m, team)

			// when
			got, err := svc.ListMembers(t.Context(), team.ID)

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestMemberWrites_AnArchivedTeamOrADeletedOrganisationIsRefused(t *testing.T) {
	writes := []struct {
		name  string
		write func(t *testing.T, svc *service, team *model.Team) error
	}{
		{name: "adding", write: func(t *testing.T, svc *service, team *model.Team) error {
			return svc.AddMember(t.Context(), memberRole(team, model.TeamRoleMember))
		}},
		{name: "changing a role", write: func(t *testing.T, svc *service, team *model.Team) error {
			return svc.SetMemberRole(t.Context(), memberRole(team, model.TeamRoleLead))
		}},
		{name: "removing", write: func(t *testing.T, svc *service, team *model.Team) error {
			return svc.RemoveMember(t.Context(), memberCommand(team))
		}},
	}

	states := []struct {
		name  string
		given func(m *testMocks, team *model.Team)
		want  error
	}{
		{name: "an archived team", given: func(m *testMocks, team *model.Team) {
			m.known(archived(team))
		}, want: ErrTeamArchived},
		{name: "a team of a deleted organisation", given: func(m *testMocks, team *model.Team) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(team, nil)
			m.gone(team.OrgID)
		}, want: ErrOrganisationNotFound},
	}

	for _, write := range writes {
		for _, state := range states {
			t.Run(write.name+" on "+state.name, func(t *testing.T) {
				// given
				svc, m := newTestService(t)
				team := platform(uuid.New())
				state.given(m, team)

				// when
				err := write.write(t, svc, team)

				// then the strict mocks prove the hierarchy was never asked and nothing was written
				assert.ErrorIs(t, err, state.want)
			})
		}
	}
}

func TestAddMember_AddsAnOrganisationMemberBelowTheActor(t *testing.T) {
	tests := []struct {
		name  string
		actor hierarchy.Holder
		role  model.TeamRole
	}{
		{name: "a lead adds a member", actor: lead, role: model.TeamRoleMember},
		{name: "an admin adds a lead", actor: orgAdmin, role: model.TeamRoleLead},
		{name: "the owner adds a lead", actor: orgOwner, role: model.TeamRoleLead},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			change := memberRole(team, tc.role)
			m.known(team)
			m.acts(team, change.MemberCommand, tc.actor, offTeam)
			m.teams.EXPECT().AddMember(mock.Anything, spec.NewTeamMember{OrgID: team.OrgID, TeamID: team.ID, UserID: change.UserID, Role: tc.role}).Return(teamMembership(team, change.UserID, tc.role), nil)
			m.audited(expectedMemberEntry(team, change.ActorID, audit.ActionTeamMemberAdded, change.UserID, map[string]string{"role": string(tc.role)}))

			// when
			err := svc.AddMember(t.Context(), change)

			// then
			assert.NoError(t, err)
		})
	}
}

func TestAddMember_RefusesARoleOtherThanLeadOrMember(t *testing.T) {
	for _, r := range []model.TeamRole{"", "owner", "facilitator"} {
		t.Run(string(r), func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			err := svc.AddMember(t.Context(), memberRole(platform(uuid.New()), r))

			// then
			assert.ErrorIs(t, err, ErrInvalidRole)
		})
	}
}

func TestAddMember_ALeadCannotMakeAnotherLead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team := platform(uuid.New())
	change := memberRole(team, model.TeamRoleLead)
	m.known(team)
	m.acts(team, change.MemberCommand, lead, offTeam)

	// when
	err := svc.AddMember(t.Context(), change)

	// then the strict mocks prove nothing was added
	assert.ErrorIs(t, err, hierarchy.ErrOutranked)
}

func TestAddMember_RefusesSomeoneAlreadyOnTheTeam(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team := platform(uuid.New())
	change := memberRole(team, model.TeamRoleMember)
	m.known(team)
	m.acts(team, change.MemberCommand, orgAdmin, teamMember)

	// when
	err := svc.AddMember(t.Context(), change)

	// then
	assert.ErrorIs(t, err, ErrAlreadyTeamMember)
}

func TestAddMember_HandsOnTheHierarchysAnswer(t *testing.T) {
	tests := []struct {
		name    string
		refusal error
		want    error
	}{
		{name: "someone outside the organisation", refusal: errNotMember, want: ErrMemberNotFound},
		{name: "someone at the actor's level or above", refusal: errOutranked, want: hierarchy.ErrOutranked},
		{name: "a failed read", refusal: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			m.known(team)
			m.hierarchy.EXPECT().ActOnTeamMember(mock.Anything, mock.Anything).Return(nil, tc.refusal)

			// when
			err := svc.AddMember(t.Context(), memberRole(team, model.TeamRoleMember))

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAddMember_ReportsWhatHappenedToTheWrite(t *testing.T) {
	tests := []struct {
		name     string
		added    bool
		writeErr error
		audited  bool
		want     error
	}{
		{name: "added meanwhile", writeErr: errRowConflicted, want: ErrAlreadyTeamMember},
		{name: "removed from the organisation meanwhile", writeErr: errRowMissing, want: ErrMemberNotFound},
		{name: "a failed write", writeErr: errDatabaseDown, want: errDatabaseDown},
		{name: "a membership whose cache was not cleared", added: true, writeErr: errCacheDown, audited: true, want: ErrCacheStale},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			change := memberRole(team, model.TeamRoleMember)
			var added *model.TeamMember
			if tc.added {
				added = teamMembership(team, change.UserID, model.TeamRoleMember)
			}
			m.known(team)
			m.acts(team, change.MemberCommand, lead, offTeam)
			m.teams.EXPECT().AddMember(mock.Anything, mock.Anything).Return(added, tc.writeErr)
			if tc.audited {
				m.audited(expectedMemberEntry(team, change.ActorID, audit.ActionTeamMemberAdded, change.UserID, map[string]string{"role": "member"}))
			}

			// when
			err := svc.AddMember(t.Context(), change)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAddMember_AnUnknownTeamIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	team := platform(uuid.New())
	m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(nil, errRowMissing)

	// when
	err := svc.AddMember(t.Context(), memberRole(team, model.TeamRoleMember))

	// then
	assert.ErrorIs(t, err, ErrTeamNotFound)
}

func TestSetMemberRole_SetsALeadAndAuditsThePreviousRole(t *testing.T) {
	tests := []struct {
		name   string
		actor  hierarchy.Holder
		target hierarchy.Holder
		role   model.TeamRole
	}{
		{name: "an admin makes a member lead", actor: orgAdmin, target: teamMember, role: model.TeamRoleLead},
		{name: "the owner makes a lead a member", actor: orgOwner, target: lead, role: model.TeamRoleMember},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			change := memberRole(team, tc.role)
			m.known(team)
			m.acts(team, change.MemberCommand, tc.actor, tc.target)
			m.teams.EXPECT().SetMemberRole(mock.Anything, spec.TeamMemberRoleChange{OrgID: team.OrgID, TeamID: team.ID, UserID: change.UserID, Role: tc.role}).Return(teamMembership(team, change.UserID, tc.role), nil)
			m.audited(expectedMemberEntry(team, change.ActorID, audit.ActionTeamMemberRoleChanged, change.UserID, map[string]string{"role": string(tc.role), "previous_role": string(tc.target.Team)}))

			// when
			err := svc.SetMemberRole(t.Context(), change)

			// then
			assert.NoError(t, err)
		})
	}
}

func TestSetMemberRole_RefusesWhatTheActorMayNotDo(t *testing.T) {
	tests := []struct {
		name   string
		actor  hierarchy.Holder
		target hierarchy.Holder
		role   model.TeamRole
		want   error
	}{
		{name: "a lead makes a member lead", actor: lead, target: teamMember, role: model.TeamRoleLead, want: hierarchy.ErrOutranked},
		{name: "someone not on the team", actor: orgAdmin, target: offTeam, role: model.TeamRoleLead, want: ErrTeamMemberNotFound},
		{name: "a standing the ladder does not permit", actor: orgOwner, target: hierarchy.Holder{Site: role.Support, Org: model.OrgRoleMember, Team: model.TeamRoleMember}, role: model.TeamRoleMember, want: hierarchy.ErrOutranked},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			change := memberRole(team, tc.role)
			m.known(team)
			m.acts(team, change.MemberCommand, tc.actor, tc.target)

			// when
			err := svc.SetMemberRole(t.Context(), change)

			// then the strict mocks prove nothing was written
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestSetMemberRole_RefusesARoleOtherThanLeadOrMember(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.SetMemberRole(t.Context(), memberRole(platform(uuid.New()), "owner"))

	// then
	assert.ErrorIs(t, err, ErrInvalidRole)
}

func TestSetMemberRole_ReportsWhatHappened(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, change MemberRole)
		want  error
	}{
		{name: "an unknown team", given: func(m *testMocks, team *model.Team, change MemberRole) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "someone outside the organisation", given: func(m *testMocks, team *model.Team, change MemberRole) {
			m.known(team)
			m.hierarchy.EXPECT().ActOnTeamMember(mock.Anything, mock.Anything).Return(nil, errNotMember)
		}, want: ErrTeamMemberNotFound},
		{name: "a failed hierarchy read", given: func(m *testMocks, team *model.Team, change MemberRole) {
			m.known(team)
			m.hierarchy.EXPECT().ActOnTeamMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "removed meanwhile", given: func(m *testMocks, team *model.Team, change MemberRole) {
			m.known(team)
			m.acts(team, change.MemberCommand, orgAdmin, teamMember)
			m.teams.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamMemberNotFound},
		{name: "a failed write", given: func(m *testMocks, team *model.Team, change MemberRole) {
			m.known(team)
			m.acts(team, change.MemberCommand, orgAdmin, teamMember)
			m.teams.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a membership whose cache was not cleared", given: func(m *testMocks, team *model.Team, change MemberRole) {
			m.known(team)
			m.acts(team, change.MemberCommand, orgAdmin, teamMember)
			m.teams.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(teamMembership(team, change.UserID, model.TeamRoleLead), errCacheDown)
			m.audited(expectedMemberEntry(team, change.ActorID, audit.ActionTeamMemberRoleChanged, change.UserID, map[string]string{"role": "lead", "previous_role": "member"}))
		}, want: ErrCacheStale},
		{name: "a failed audit entry", given: func(m *testMocks, team *model.Team, change MemberRole) {
			m.known(team)
			m.acts(team, change.MemberCommand, orgAdmin, teamMember)
			m.teams.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(teamMembership(team, change.UserID, model.TeamRoleLead), nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
		}, want: ErrNotAudited},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			change := memberRole(team, model.TeamRoleLead)
			tc.given(m, team, change)

			// when
			err := svc.SetMemberRole(t.Context(), change)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRemoveMember_RemovesAndAuditsTheirRole(t *testing.T) {
	tests := []struct {
		name   string
		actor  hierarchy.Holder
		target hierarchy.Holder
	}{
		{name: "a lead removes a member", actor: lead, target: teamMember},
		{name: "an admin removes a lead", actor: orgAdmin, target: lead},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			action := memberCommand(team)
			m.known(team)
			m.acts(team, action, tc.actor, tc.target)
			m.teams.EXPECT().RemoveMember(mock.Anything, spec.TeamMemberKey{OrgID: team.OrgID, TeamID: team.ID, UserID: action.UserID}).Return(teamMembership(team, action.UserID, tc.target.Team), nil)
			m.audited(expectedMemberEntry(team, action.ActorID, audit.ActionTeamMemberRemoved, action.UserID, map[string]string{"role": string(tc.target.Team)}))

			// when
			err := svc.RemoveMember(t.Context(), action)

			// then
			assert.NoError(t, err)
		})
	}
}

func TestRemoveMember_ReportsWhatHappened(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, team *model.Team, action MemberCommand)
		want  error
	}{
		{name: "an unknown team", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.teams.EXPECT().GetHeader(mock.Anything, team.ID).Return(nil, errRowMissing)
		}, want: ErrTeamNotFound},
		{name: "someone not on the team", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.known(team)
			m.acts(team, action, orgAdmin, offTeam)
		}, want: ErrTeamMemberNotFound},
		{name: "site staff not on the team", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.known(team)
			m.hierarchy.EXPECT().ActOnTeamMember(mock.Anything, mock.Anything).Return(nil, errOutranked)
		}, want: hierarchy.ErrOutranked},
		{name: "someone outside the organisation", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.known(team)
			m.hierarchy.EXPECT().ActOnTeamMember(mock.Anything, mock.Anything).Return(nil, errNotMember)
		}, want: ErrTeamMemberNotFound},
		{name: "removed meanwhile", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.known(team)
			m.acts(team, action, lead, teamMember)
			m.teams.EXPECT().RemoveMember(mock.Anything, mock.Anything).Return(nil, errRowMissing)
		}, want: ErrTeamMemberNotFound},
		{name: "a failed write", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.known(team)
			m.acts(team, action, lead, teamMember)
			m.teams.EXPECT().RemoveMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a membership whose cache was not cleared", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.known(team)
			m.acts(team, action, lead, teamMember)
			m.teams.EXPECT().RemoveMember(mock.Anything, mock.Anything).Return(teamMembership(team, action.UserID, model.TeamRoleMember), errCacheDown)
			m.audited(expectedMemberEntry(team, action.ActorID, audit.ActionTeamMemberRemoved, action.UserID, map[string]string{"role": "member"}))
		}, want: ErrCacheStale},
		{name: "a failed audit entry", given: func(m *testMocks, team *model.Team, action MemberCommand) {
			m.known(team)
			m.acts(team, action, lead, teamMember)
			m.teams.EXPECT().RemoveMember(mock.Anything, mock.Anything).Return(teamMembership(team, action.UserID, model.TeamRoleMember), nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
		}, want: ErrNotAudited},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			team := platform(uuid.New())
			action := memberCommand(team)
			tc.given(m, team, action)

			// when
			err := svc.RemoveMember(t.Context(), action)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
