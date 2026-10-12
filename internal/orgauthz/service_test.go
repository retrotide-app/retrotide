package orgauthz

import (
	"testing"

	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	serviceMocks struct {
		organisations *repository.MockOrganisationRepository
		teams         *repository.MockTeamRepository
	}
)

func newTestService(t *testing.T) (Service, *serviceMocks) {
	t.Helper()

	m := &serviceMocks{
		organisations: repository.NewMockOrganisationRepository(t),
		teams:         repository.NewMockTeamRepository(t),
	}

	return NewService(m.organisations, m.teams), m
}

func (m *serviceMocks) orgMember(orgID uuid.UUID, userID uuid.UUID, org model.OrgRole) {
	key := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}
	if org == "" {
		m.organisations.EXPECT().GetMember(mock.Anything, key).Return(nil, errNoRow)
		return
	}

	m.organisations.EXPECT().GetMember(mock.Anything, key).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: org}, nil)
}

func (m *serviceMocks) inTeamsOrganisation(orgID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, org model.OrgRole) {
	m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(&model.Team{ID: teamID, OrgID: orgID, Name: "Platform"}, nil)
	m.orgMember(orgID, userID, org)
}

func (m *serviceMocks) teamMember(key spec.TeamMemberKey, team model.TeamRole) {
	if team == "" {
		m.teams.EXPECT().GetMember(mock.Anything, key).Return(nil, errNoRow)
		return
	}

	m.teams.EXPECT().GetMember(mock.Anything, key).Return(&model.TeamMember{OrgID: key.OrgID, TeamID: key.TeamID, UserID: key.UserID, Role: team}, nil)
}

func TestCanInOrg_GrantsTheSpecifiedTableToEveryMember(t *testing.T) {
	for perm, allowed := range orgTable {
		for i, org := range orgRoles {
			t.Run(string(org)+" "+string(perm), func(t *testing.T) {
				// given the table of section 6.5.2, owner then admin then member
				svc, m := newTestService(t)
				orgID, userID := uuid.New(), uuid.New()
				m.orgMember(orgID, userID, org)

				// when
				decision, err := svc.CanInOrg(t.Context(), spec.OrgPermissionCheck{OrgID: orgID, UserID: userID, Permission: perm})

				// then
				require.NoError(t, err)
				assert.Equal(t, Decision{Member: true, Allowed: allowed[i]}, decision)
			})
		}
	}
}

func TestCanInOrg_SomeoneOutsideTheOrganisationIsNoMember(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	m.orgMember(orgID, userID, "")

	// when
	decision, err := svc.CanInOrg(t.Context(), spec.OrgPermissionCheck{OrgID: orgID, UserID: userID, Permission: PermOrgView})

	// then the guard answers 404, so the organisation cannot be told apart from one that does not exist
	require.NoError(t, err)
	assert.Equal(t, Decision{}, decision)
}

func TestCanInOrg_NoUserIsNoMemberAndReadsNothing(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	decision, err := svc.CanInOrg(t.Context(), spec.OrgPermissionCheck{OrgID: uuid.New(), UserID: uuid.Nil, Permission: PermOrgView})

	// then
	require.NoError(t, err)
	assert.Equal(t, Decision{}, decision)
}

func TestCanInOrg_ReturnsAMembershipReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}).Return(nil, errDatabaseDown)

	// when
	decision, err := svc.CanInOrg(t.Context(), spec.OrgPermissionCheck{OrgID: orgID, UserID: userID, Permission: PermOrgView})

	// then a failed read is never a silent denial or a silent grant
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Equal(t, Decision{}, decision)
}

func TestCanInOrg_RefusesAPermissionOutsideTheOrganisationCatalogue(t *testing.T) {
	tests := []struct {
		name string
		perm Permission
		want error
	}{
		{name: "an undeclared permission", perm: "org.teleport", want: ErrUnknownPermission},
		{name: "a site staff permission", perm: "manage_settings", want: ErrUnknownPermission},
		{name: "a team permission", perm: PermTeamView, want: ErrOutOfScope},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the strict mocks prove that no membership is read
			svc, _ := newTestService(t)

			// when
			decision, err := svc.CanInOrg(t.Context(), spec.OrgPermissionCheck{OrgID: uuid.New(), UserID: uuid.New(), Permission: tc.perm})

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Equal(t, Decision{}, decision)
		})
	}
}

func TestCanInTeam_GrantsTheEffectiveTeamRole(t *testing.T) {
	const (
		leadColumn   = 0
		memberColumn = 1
		noColumn     = -1
	)

	tests := []struct {
		name   string
		org    model.OrgRole
		team   model.TeamRole
		reads  bool
		column int
	}{
		{name: "the owner, who leads every team", org: model.OrgRoleOwner, column: leadColumn},
		{name: "an admin, who leads every team", org: model.OrgRoleAdmin, column: leadColumn},
		{name: "a member who leads the team", org: model.OrgRoleMember, team: model.TeamRoleLead, reads: true, column: leadColumn},
		{name: "a member of the team", org: model.OrgRoleMember, team: model.TeamRoleMember, reads: true, column: memberColumn},
		{name: "a member who is not on the team", org: model.OrgRoleMember, reads: true, column: noColumn},
	}

	for _, tc := range tests {
		for perm, allowed := range teamTable {
			t.Run(tc.name+" "+string(perm), func(t *testing.T) {
				// given the table of section 6.5.2, and the owner and admins are never looked up on the team
				svc, m := newTestService(t)
				orgID, teamID, userID := uuid.New(), uuid.New(), uuid.New()
				m.inTeamsOrganisation(orgID, teamID, userID, tc.org)
				if tc.reads {
					m.teamMember(spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: userID}, tc.team)
				}

				want := tc.column != noColumn && allowed[tc.column]

				// when
				decision, err := svc.CanInTeam(t.Context(), spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: perm})

				// then an organisation member always knows the team exists, so a refusal is a 403
				require.NoError(t, err)
				assert.Equal(t, Decision{Member: true, Allowed: want}, decision)
			})
		}
	}
}

func TestCanInTeam_SomeoneWhoCannotSeeTheTeamIsNoMember(t *testing.T) {
	tests := []struct {
		name   string
		expect func(m *serviceMocks, orgID uuid.UUID, teamID uuid.UUID, userID uuid.UUID)
	}{
		{
			name: "an unknown team",
			expect: func(m *serviceMocks, _ uuid.UUID, teamID uuid.UUID, _ uuid.UUID) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(nil, errNoRow)
			},
		},
		{
			name: "someone outside the team's organisation",
			expect: func(m *serviceMocks, orgID uuid.UUID, teamID uuid.UUID, userID uuid.UUID) {
				m.inTeamsOrganisation(orgID, teamID, userID, "")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the strict mocks prove that no team membership is read
			svc, m := newTestService(t)
			orgID, teamID, userID := uuid.New(), uuid.New(), uuid.New()
			tc.expect(m, orgID, teamID, userID)

			// when
			decision, err := svc.CanInTeam(t.Context(), spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: PermTeamView})

			// then
			require.NoError(t, err)
			assert.Equal(t, Decision{}, decision)
		})
	}
}

func TestCanInTeam_NoUserIsNoMemberAndReadsNothing(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	decision, err := svc.CanInTeam(t.Context(), spec.TeamPermissionCheck{TeamID: uuid.New(), UserID: uuid.Nil, Permission: PermTeamView})

	// then
	require.NoError(t, err)
	assert.Equal(t, Decision{}, decision)
}

func TestCanInTeam_ReturnsAReadFailure(t *testing.T) {
	tests := []struct {
		name   string
		expect func(m *serviceMocks, orgID uuid.UUID, teamID uuid.UUID, userID uuid.UUID)
	}{
		{
			name: "the team header",
			expect: func(m *serviceMocks, _ uuid.UUID, teamID uuid.UUID, _ uuid.UUID) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the organisation membership",
			expect: func(m *serviceMocks, orgID uuid.UUID, teamID uuid.UUID, userID uuid.UUID) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(&model.Team{ID: teamID, OrgID: orgID, Name: "Platform"}, nil)
				m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the team membership",
			expect: func(m *serviceMocks, orgID uuid.UUID, teamID uuid.UUID, userID uuid.UUID) {
				m.inTeamsOrganisation(orgID, teamID, userID, model.OrgRoleMember)
				m.teams.EXPECT().GetMember(mock.Anything, spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: userID}).Return(nil, errDatabaseDown)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, teamID, userID := uuid.New(), uuid.New(), uuid.New()
			tc.expect(m, orgID, teamID, userID)

			// when
			decision, err := svc.CanInTeam(t.Context(), spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: PermTeamView})

			// then a failed read is never a silent denial or a silent grant
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.Equal(t, Decision{}, decision)
		})
	}
}

func TestCanInTeam_RefusesAPermissionOutsideTheTeamCatalogue(t *testing.T) {
	tests := []struct {
		name string
		perm Permission
		want error
	}{
		{name: "an undeclared permission", perm: "team.teleport", want: ErrUnknownPermission},
		{name: "an organisation permission", perm: PermOrgManageTeams, want: ErrOutOfScope},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the strict mocks prove that nothing is read
			svc, _ := newTestService(t)

			// when
			decision, err := svc.CanInTeam(t.Context(), spec.TeamPermissionCheck{TeamID: uuid.New(), UserID: uuid.New(), Permission: tc.perm})

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Equal(t, Decision{}, decision)
		})
	}
}
