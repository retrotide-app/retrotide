package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	tenants struct {
		repos *repository.Repositories
		a     *model.Workspace
		b     *model.Workspace
		alice *model.User
	}
)

func newTenants(t *testing.T) tenants {
	t.Helper()

	repos := daotest.NewRepos(t)
	a := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	b := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, a.Organisation.ID, alice.ID, model.OrgRoleMember)

	return tenants{repos: repos, a: a, b: b, alice: alice}
}

func TestCrossTenant_OrganisationMembersAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given alice is a member of organisation A only
	tt := newTenants(t)
	throughB := spec.OrganisationMemberKey{OrgID: tt.b.Organisation.ID, UserID: tt.alice.ID}

	// when
	_, getErr := tt.repos.Organisation.GetMember(t.Context(), throughB)
	_, roleErr := tt.repos.Organisation.SetMemberRole(t.Context(), spec.OrganisationMemberRoleChange{OrgID: tt.b.Organisation.ID, UserID: tt.alice.ID, Role: model.OrgRoleAdmin})
	_, removeErr := tt.repos.Organisation.RemoveMember(t.Context(), throughB)
	transferErr := tt.repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: tt.b.Organisation.ID, FromUserID: tt.a.Owner.UserID, ToUserID: tt.alice.ID, MaxOwned: daotest.UnlimitedQuota})
	listedB, listErr := tt.repos.Organisation.ListMembers(t.Context(), tt.b.Organisation.ID)

	// then
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, roleErr, dao.ErrNotFound)
	require.ErrorIs(t, removeErr, dao.ErrNotFound)
	require.ErrorIs(t, transferErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	require.Len(t, listedB, 1)
	assert.Equal(t, tt.b.Owner.UserID, listedB[0].UserID)

	assert.Equal(t, model.OrgRoleMember, memberRole(t, tt.repos, tt.a.Organisation.ID, tt.alice.ID))
	assert.Equal(t, model.OrgRoleOwner, memberRole(t, tt.repos, tt.a.Organisation.ID, tt.a.Owner.UserID))
}

func TestCrossTenant_TeamsAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given
	tt := newTenants(t)
	teamA := tt.a.Team
	throughB := spec.TeamKey{OrgID: tt.b.Organisation.ID, TeamID: teamA.ID}

	// when
	_, getErr := tt.repos.Team.GetByID(t.Context(), throughB)
	_, renameErr := tt.repos.Team.Rename(t.Context(), spec.TeamRename{OrgID: tt.b.Organisation.ID, TeamID: teamA.ID, Name: "Taken over"})
	_, archiveErr := tt.repos.Team.Archive(t.Context(), throughB)
	_, deleteErr := tt.repos.Team.Delete(t.Context(), throughB)
	listedB, listErr := tt.repos.Team.ListForOrganisation(t.Context(), tt.b.Organisation.ID)
	activeB, countErr := tt.repos.Team.CountActive(t.Context(), tt.b.Organisation.ID)

	// then
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, renameErr, dao.ErrNotFound)
	require.ErrorIs(t, archiveErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	require.NoError(t, countErr)
	require.Len(t, listedB, 1)
	assert.Equal(t, tt.b.Team.ID, listedB[0].ID)
	assert.Equal(t, 1, activeB)

	unchanged, err := tt.repos.Team.GetByID(t.Context(), spec.TeamKey{OrgID: tt.a.Organisation.ID, TeamID: teamA.ID})
	require.NoError(t, err)
	assert.Equal(t, teamA, *unchanged)
}

func TestCrossTenant_TeamMembersAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given alice is on team A, and bob is a member of organisation B only
	tt := newTenants(t)
	teamA := tt.a.Team
	daotest.AddTeamMember(t, tt.repos, &teamA, tt.alice.ID, model.TeamRoleMember)
	bob := daotest.CreateUser(t, tt.repos)
	daotest.AddOrgMember(t, tt.repos, tt.b.Organisation.ID, bob.ID, model.OrgRoleMember)
	throughB := spec.TeamMemberKey{OrgID: tt.b.Organisation.ID, TeamID: teamA.ID, UserID: tt.alice.ID}

	// when
	_, getErr := tt.repos.Team.GetMember(t.Context(), throughB)
	_, roleErr := tt.repos.Team.SetMemberRole(t.Context(), spec.TeamMemberRoleChange{OrgID: tt.b.Organisation.ID, TeamID: teamA.ID, UserID: tt.alice.ID, Role: model.TeamRoleLead})
	_, removeErr := tt.repos.Team.RemoveMember(t.Context(), throughB)
	_, joinErr := tt.repos.Team.AddMember(t.Context(), spec.NewTeamMember{OrgID: tt.b.Organisation.ID, TeamID: teamA.ID, UserID: bob.ID, Role: model.TeamRoleLead})
	_, crossJoinErr := tt.repos.Team.AddMember(t.Context(), spec.NewTeamMember{OrgID: tt.a.Organisation.ID, TeamID: teamA.ID, UserID: bob.ID, Role: model.TeamRoleLead})
	listed, listErr := tt.repos.Team.ListMembers(t.Context(), spec.TeamKey{OrgID: tt.b.Organisation.ID, TeamID: teamA.ID})
	teams, teamsErr := tt.repos.Team.ListForMember(t.Context(), spec.OrganisationMemberKey{OrgID: tt.b.Organisation.ID, UserID: tt.alice.ID})

	// then the composite keys refuse a team of one organisation holding a member of another
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, roleErr, dao.ErrNotFound)
	require.ErrorIs(t, removeErr, dao.ErrNotFound)
	require.ErrorIs(t, joinErr, dao.ErrNotFound)
	require.ErrorIs(t, crossJoinErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	require.NoError(t, teamsErr)
	assert.Empty(t, listed)
	assert.Empty(t, teams)

	kept, err := tt.repos.Team.GetMember(t.Context(), spec.TeamMemberKey{OrgID: tt.a.Organisation.ID, TeamID: teamA.ID, UserID: tt.alice.ID})
	require.NoError(t, err)
	assert.Equal(t, model.TeamRoleMember, kept.Role)
}

func TestCrossTenant_MemberListsCarryOnlyTheRolesHeldInTheirOwnOrganisation(t *testing.T) {
	// given alice is a member of organisation A and on its team, and an admin of organisation B
	tt := newTenants(t)
	teamA := tt.a.Team
	daotest.AddTeamMember(t, tt.repos, &teamA, tt.alice.ID, model.TeamRoleMember)
	daotest.AddOrgMember(t, tt.repos, tt.b.Organisation.ID, tt.alice.ID, model.OrgRoleAdmin)

	// when
	teamMembers, teamErr := tt.repos.Team.ListMembers(t.Context(), spec.TeamKey{OrgID: tt.a.Organisation.ID, TeamID: teamA.ID})
	membersA, listAErr := tt.repos.Organisation.ListMembers(t.Context(), tt.a.Organisation.ID)
	membersB, listBErr := tt.repos.Organisation.ListMembers(t.Context(), tt.b.Organisation.ID)

	// then her team place shows her role in A, never the one she holds in B
	require.NoError(t, teamErr)
	require.NoError(t, listAErr)
	require.NoError(t, listBErr)

	teamRoles := map[uuid.UUID]model.OrgRole{}
	for _, member := range teamMembers {
		teamRoles[member.UserID] = member.OrgRole
	}
	assert.Equal(t, map[uuid.UUID]model.OrgRole{tt.a.Owner.UserID: model.OrgRoleOwner, tt.alice.ID: model.OrgRoleMember}, teamRoles)

	assert.Equal(t, map[uuid.UUID]model.OrgRole{tt.a.Owner.UserID: model.OrgRoleOwner, tt.alice.ID: model.OrgRoleMember}, orgRoles(membersA))
	assert.Equal(t, map[uuid.UUID]model.OrgRole{tt.b.Owner.UserID: model.OrgRoleOwner, tt.alice.ID: model.OrgRoleAdmin}, orgRoles(membersB))
}

func orgRoles(members []model.OrganisationMemberProfile) map[uuid.UUID]model.OrgRole {
	roles := make(map[uuid.UUID]model.OrgRole, len(members))
	for _, member := range members {
		roles[member.UserID] = member.Role
	}

	return roles
}

func TestCrossTenant_InvitesAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given
	tt := newTenants(t)
	invite, _ := createInvite(t, tt.repos, spec.NewOrganisationInvite{OrgID: tt.a.Organisation.ID, TeamID: &tt.a.Team.ID})
	throughB := spec.OrganisationInviteKey{OrgID: tt.b.Organisation.ID, InviteID: invite.ID}

	// when
	_, getErr := tt.repos.Organisation.GetInvite(t.Context(), throughB)
	_, revokeErr := tt.repos.Organisation.RevokeInvite(t.Context(), throughB)
	listedB, listErr := tt.repos.Organisation.ListInvites(t.Context(), tt.b.Organisation.ID)
	_, crossTeamErr := tt.repos.Organisation.CreateInvite(t.Context(), spec.NewOrganisationInvite{
		OrgID:     tt.b.Organisation.ID,
		TeamID:    &tt.a.Team.ID,
		TokenHash: uuid.NewString(),
		Role:      model.OrgRoleMember,
		ExpiresAt: invite.ExpiresAt,
	})

	// then an invite of organisation B can never name a team of organisation A
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, revokeErr, dao.ErrNotFound)
	require.ErrorIs(t, crossTeamErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	assert.Empty(t, listedB)

	unchanged, err := tt.repos.Organisation.GetInvite(t.Context(), spec.OrganisationInviteKey{OrgID: tt.a.Organisation.ID, InviteID: invite.ID})
	require.NoError(t, err)
	assert.Nil(t, unchanged.RevokedAt)
}
