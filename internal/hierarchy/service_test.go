package hierarchy

import (
	"testing"

	"retrotide/internal/authz"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	serviceMocks struct {
		authz         *authz.MockService
		organisations *repository.MockOrganisationRepository
		teams         *repository.MockTeamRepository
	}

	person struct {
		site role.Role
		org  model.OrgRole
		team model.TeamRole
	}
)

func newTestService(t *testing.T) (Service, *serviceMocks) {
	t.Helper()

	m := &serviceMocks{
		authz:         authz.NewMockService(t),
		organisations: repository.NewMockOrganisationRepository(t),
		teams:         repository.NewMockTeamRepository(t),
	}

	return NewService(Deps{Authz: m.authz, Organisations: m.organisations, Teams: m.teams}), m
}

func (m *serviceMocks) ranks(actorID uuid.UUID, actor role.Role, targetID uuid.UUID, target role.Role) {
	m.authz.EXPECT().GetRole(mock.Anything, actorID).Return(actor, nil)
	m.authz.EXPECT().GetRole(mock.Anything, targetID).Return(target, nil)
}

func (m *serviceMocks) orgMember(orgID uuid.UUID, userID uuid.UUID, org model.OrgRole) {
	key := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}
	m.organisations.EXPECT().GetMember(mock.Anything, key).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: org}, nil)
}

func (m *serviceMocks) teamMember(key spec.TeamMemberKey, team model.TeamRole) {
	if team == "" {
		m.teams.EXPECT().GetMember(mock.Anything, key).Return(nil, errNoRow)
		return
	}

	m.teams.EXPECT().GetMember(mock.Anything, key).Return(&model.TeamMember{OrgID: key.OrgID, TeamID: key.TeamID, UserID: key.UserID, Role: team}, nil)
}

func (m *serviceMocks) inOrganisation(orgID uuid.UUID, userID uuid.UUID, who person) {
	m.orgMember(orgID, userID, who.org)
	m.authz.EXPECT().GetRole(mock.Anything, userID).Return(who.site, nil)
}

func (m *serviceMocks) inTeam(orgID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, who person) {
	m.inOrganisation(orgID, userID, who)
	m.teamMember(spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: userID}, who.team)
}

func TestActOnUser_ReturnsTheStandingOfAnOutrankingActor(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, role.Support)

	// when
	standing, err := svc.ActOnUser(t.Context(), spec.UserActionCheck{ActorID: actorID, TargetID: targetID})

	// then
	require.NoError(t, err)
	assert.Equal(t, &Standing{Actor: Holder{Site: role.Admin}, Target: Holder{Site: role.Support}}, standing)
}

func TestActOnUser_OnlyAnOutrankingActorMayAct(t *testing.T) {
	tests := []struct {
		name    string
		actor   role.Role
		target  role.Role
		allowed bool
	}{
		{name: "support on an ordinary account", actor: role.Support, target: "", allowed: true},
		{name: "a site admin on support", actor: role.Admin, target: role.Support, allowed: true},
		{name: "the super admin on a site admin", actor: role.SuperAdmin, target: role.Admin, allowed: true},
		{name: "support on support", actor: role.Support, target: role.Support, allowed: false},
		{name: "support on a site admin", actor: role.Support, target: role.Admin, allowed: false},
		{name: "a site admin on the super admin", actor: role.Admin, target: role.SuperAdmin, allowed: false},
		{name: "the super admin on the super admin", actor: role.SuperAdmin, target: role.SuperAdmin, allowed: false},
		{name: "an ordinary account on another", actor: "", target: "", allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			actorID, targetID := uuid.New(), uuid.New()
			m.ranks(actorID, tc.actor, targetID, tc.target)

			// when
			standing, err := svc.ActOnUser(t.Context(), spec.UserActionCheck{ActorID: actorID, TargetID: targetID})

			// then
			if tc.allowed {
				require.NoError(t, err)
				assert.NotNil(t, standing)
				return
			}

			assert.ErrorIs(t, err, ErrOutranked)
			assert.Nil(t, standing)
		})
	}
}

func TestActOnUser_NobodyActsOnThemselves(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID := uuid.New()
	m.authz.EXPECT().GetRole(mock.Anything, actorID).Return(role.Admin, nil)

	// when
	standing, err := svc.ActOnUser(t.Context(), spec.UserActionCheck{ActorID: actorID, TargetID: actorID})

	// then
	assert.ErrorIs(t, err, ErrOutranked)
	assert.Nil(t, standing)
}

func TestActOnUser_ReturnsARoleReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.authz.EXPECT().GetRole(mock.Anything, actorID).Return(role.Admin, nil)
	m.authz.EXPECT().GetRole(mock.Anything, targetID).Return("", errDatabaseDown)

	// when
	standing, err := svc.ActOnUser(t.Context(), spec.UserActionCheck{ActorID: actorID, TargetID: targetID})

	// then a role that cannot be read is never taken for no role
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrOutranked)
	assert.Nil(t, standing)
}

func TestActOnMember_AcrossBothLevels(t *testing.T) {
	tests := []struct {
		name    string
		actor   person
		target  person
		allowed bool
	}{
		{name: "the owner on an admin", actor: person{org: model.OrgRoleOwner}, target: person{org: model.OrgRoleAdmin}, allowed: true},
		{name: "the owner on a member", actor: person{org: model.OrgRoleOwner}, target: person{org: model.OrgRoleMember}, allowed: true},
		{name: "an admin on a member", actor: person{org: model.OrgRoleAdmin}, target: person{org: model.OrgRoleMember}, allowed: true},
		{name: "an admin on an admin", actor: person{org: model.OrgRoleAdmin}, target: person{org: model.OrgRoleAdmin}, allowed: false},
		{name: "an admin on the owner", actor: person{org: model.OrgRoleAdmin}, target: person{org: model.OrgRoleOwner}, allowed: false},
		{name: "a member on a member", actor: person{org: model.OrgRoleMember}, target: person{org: model.OrgRoleMember}, allowed: false},
		{name: "a member on an admin", actor: person{org: model.OrgRoleMember}, target: person{org: model.OrgRoleAdmin}, allowed: false},
		{name: "support who is a member on the owner", actor: person{site: role.Support, org: model.OrgRoleMember}, target: person{org: model.OrgRoleOwner}, allowed: true},
		{name: "the owner on support who is a member", actor: person{org: model.OrgRoleOwner}, target: person{site: role.Support, org: model.OrgRoleMember}, allowed: false},
		{name: "a site admin who is a member on support who is an admin", actor: person{site: role.Admin, org: model.OrgRoleMember}, target: person{site: role.Support, org: model.OrgRoleAdmin}, allowed: true},
		{name: "support who owns the organisation on support who is a member", actor: person{site: role.Support, org: model.OrgRoleOwner}, target: person{site: role.Support, org: model.OrgRoleMember}, allowed: false},
		{name: "the owner on the super admin who is a member", actor: person{org: model.OrgRoleOwner}, target: person{site: role.SuperAdmin, org: model.OrgRoleMember}, allowed: false},
		{name: "a site admin who owns the organisation on the super admin who is a member", actor: person{site: role.Admin, org: model.OrgRoleOwner}, target: person{site: role.SuperAdmin, org: model.OrgRoleMember}, allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID, targetID := uuid.New(), uuid.New(), uuid.New()
			m.inOrganisation(orgID, actorID, tc.actor)
			m.inOrganisation(orgID, targetID, tc.target)

			// when
			standing, err := svc.ActOnMember(t.Context(), spec.MemberActionCheck{OrgID: orgID, ActorID: actorID, TargetID: targetID})

			// then
			if tc.allowed {
				require.NoError(t, err)
				assert.Equal(t, &Standing{Actor: Holder{Site: tc.actor.site, Org: tc.actor.org}, Target: Holder{Site: tc.target.site, Org: tc.target.org}}, standing)
				return
			}

			assert.ErrorIs(t, err, ErrOutranked)
			assert.Nil(t, standing)
		})
	}
}

func TestActOnMember_NobodyActsOnThemselves(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID := uuid.New(), uuid.New()
	m.inOrganisation(orgID, ownerID, person{org: model.OrgRoleOwner})

	// when
	standing, err := svc.ActOnMember(t.Context(), spec.MemberActionCheck{OrgID: orgID, ActorID: ownerID, TargetID: ownerID})

	// then
	assert.ErrorIs(t, err, ErrOutranked)
	assert.Nil(t, standing)
}

func TestActOnMember_SomeoneOutsideTheOrganisationIsNotAMember(t *testing.T) {
	tests := []struct {
		name   string
		expect func(m *serviceMocks, orgID uuid.UUID, actorID uuid.UUID, targetID uuid.UUID)
	}{
		{
			name: "the target",
			expect: func(m *serviceMocks, orgID uuid.UUID, actorID uuid.UUID, targetID uuid.UUID) {
				m.inOrganisation(orgID, actorID, person{org: model.OrgRoleOwner})
				m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: targetID}).Return(nil, errNoRow)
			},
		},
		{
			name: "the actor, whose site role is never read",
			expect: func(m *serviceMocks, orgID uuid.UUID, actorID uuid.UUID, _ uuid.UUID) {
				m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: actorID}).Return(nil, errNoRow)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the strict mocks prove that nothing more is read
			svc, m := newTestService(t)
			orgID, actorID, targetID := uuid.New(), uuid.New(), uuid.New()
			tc.expect(m, orgID, actorID, targetID)

			// when
			standing, err := svc.ActOnMember(t.Context(), spec.MemberActionCheck{OrgID: orgID, ActorID: actorID, TargetID: targetID})

			// then
			assert.ErrorIs(t, err, ErrNotMember)
			assert.ErrorIs(t, err, dao.ErrNotFound)
			assert.NotErrorIs(t, err, ErrOutranked)
			assert.Nil(t, standing)
		})
	}
}

func TestActOnMember_ReturnsAReadFailure(t *testing.T) {
	tests := []struct {
		name   string
		expect func(m *serviceMocks, orgID uuid.UUID, actorID uuid.UUID)
	}{
		{
			name: "the membership",
			expect: func(m *serviceMocks, orgID uuid.UUID, actorID uuid.UUID) {
				m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: actorID}).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the site role",
			expect: func(m *serviceMocks, orgID uuid.UUID, actorID uuid.UUID) {
				m.orgMember(orgID, actorID, model.OrgRoleOwner)
				m.authz.EXPECT().GetRole(mock.Anything, actorID).Return("", errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			tc.expect(m, orgID, actorID)

			// when
			standing, err := svc.ActOnMember(t.Context(), spec.MemberActionCheck{OrgID: orgID, ActorID: actorID, TargetID: uuid.New()})

			// then a failed read is never a refusal or a grant
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.NotErrorIs(t, err, ErrNotMember)
			assert.NotErrorIs(t, err, ErrOutranked)
			assert.Nil(t, standing)
		})
	}
}

func TestActOnTeamMember_AcrossTheOrganisationAndTheTeam(t *testing.T) {
	tests := []struct {
		name    string
		actor   person
		target  person
		allowed bool
	}{
		{name: "a lead on a team member", actor: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, target: person{org: model.OrgRoleMember, team: model.TeamRoleMember}, allowed: true},
		{name: "a lead on a member being added", actor: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, target: person{org: model.OrgRoleMember}, allowed: true},
		{name: "a lead on another lead", actor: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, target: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, allowed: false},
		{name: "a lead on an organisation admin, who leads every team", actor: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, target: person{org: model.OrgRoleAdmin}, allowed: false},
		{name: "an organisation admin off the team on a lead", actor: person{org: model.OrgRoleAdmin}, target: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, allowed: true},
		{name: "the owner off the team on an organisation admin on it", actor: person{org: model.OrgRoleOwner}, target: person{org: model.OrgRoleAdmin, team: model.TeamRoleMember}, allowed: true},
		{name: "a team member on a team member", actor: person{org: model.OrgRoleMember, team: model.TeamRoleMember}, target: person{org: model.OrgRoleMember, team: model.TeamRoleMember}, allowed: false},
		{name: "a member off the team on another", actor: person{org: model.OrgRoleMember}, target: person{org: model.OrgRoleMember}, allowed: false},
		{name: "support who is a team member on a lead", actor: person{site: role.Support, org: model.OrgRoleMember, team: model.TeamRoleMember}, target: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, allowed: true},
		{name: "a lead on support who is a team member", actor: person{org: model.OrgRoleMember, team: model.TeamRoleLead}, target: person{site: role.Support, org: model.OrgRoleMember, team: model.TeamRoleMember}, allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, teamID, actorID, targetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			m.inTeam(orgID, teamID, actorID, tc.actor)
			m.inTeam(orgID, teamID, targetID, tc.target)

			// when
			standing, err := svc.ActOnTeamMember(t.Context(), spec.TeamMemberActionCheck{OrgID: orgID, TeamID: teamID, ActorID: actorID, TargetID: targetID})

			// then
			if tc.allowed {
				require.NoError(t, err)
				assert.Equal(t, &Standing{Actor: Holder{Site: tc.actor.site, Org: tc.actor.org, Team: tc.actor.team}, Target: Holder{Site: tc.target.site, Org: tc.target.org, Team: tc.target.team}}, standing)
				return
			}

			assert.ErrorIs(t, err, ErrOutranked)
			assert.Nil(t, standing)
		})
	}
}

func TestActOnTeamMember_SomeoneOutsideTheOrganisationIsNotAMember(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, teamID, actorID, targetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	m.inTeam(orgID, teamID, actorID, person{org: model.OrgRoleOwner})
	m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: targetID}).Return(nil, errNoRow)

	// when
	standing, err := svc.ActOnTeamMember(t.Context(), spec.TeamMemberActionCheck{OrgID: orgID, TeamID: teamID, ActorID: actorID, TargetID: targetID})

	// then a team never takes anyone from outside its organisation
	assert.ErrorIs(t, err, ErrNotMember)
	assert.Nil(t, standing)
}

func TestActOnTeamMember_ReturnsATeamMembershipReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, teamID, actorID := uuid.New(), uuid.New(), uuid.New()
	m.inOrganisation(orgID, actorID, person{org: model.OrgRoleMember})
	m.teams.EXPECT().GetMember(mock.Anything, spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: actorID}).Return(nil, errDatabaseDown)

	// when
	standing, err := svc.ActOnTeamMember(t.Context(), spec.TeamMemberActionCheck{OrgID: orgID, TeamID: teamID, ActorID: actorID, TargetID: uuid.New()})

	// then a team role that cannot be read is never taken for no team role
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrOutranked)
	assert.Nil(t, standing)
}
