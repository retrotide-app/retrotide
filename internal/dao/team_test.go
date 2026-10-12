package dao_test

import (
	"fmt"
	"sync"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	teamQuotaUnderTest = 3
)

func TestTeam_CreateKeepsToTheTeamQuota(t *testing.T) {
	// given an organisation with its first team, on a plan that allows one
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID

	// when
	_, full := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "Second", MaxTeams: 1})
	second, err := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "Second", MaxTeams: 2})

	// then
	require.ErrorIs(t, full, dao.ErrQuotaExceeded)
	require.NoError(t, err)
	assert.Equal(t, orgID, second.OrgID)
	assert.False(t, second.Archived())

	active, err := repos.Team.CountActive(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, 2, active)
}

func TestTeam_ArchivedTeamsLeaveTheQuota(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	second, err := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "Second", MaxTeams: 2})
	require.NoError(t, err)
	_, err = repos.Team.Archive(t.Context(), spec.TeamKey{OrgID: orgID, TeamID: second.ID})
	require.NoError(t, err)

	// when
	third, err := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "Third", MaxTeams: 2})

	// then
	require.NoError(t, err)
	assert.Equal(t, "Third", third.Name)
	active, err := repos.Team.CountActive(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, 2, active)
}

func TestTeam_NamesAreUniqueInAnOrganisationInAnyCase(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	otherOrgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	_, err := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "Platform", MaxTeams: generousQuota})
	require.NoError(t, err)

	// when
	_, taken := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "PLATFORM", MaxTeams: generousQuota})
	_, elsewhere := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: otherOrgID, Name: "Platform", MaxTeams: generousQuota})

	// then
	require.ErrorIs(t, taken, dao.ErrConflict)
	require.NoError(t, elsewhere)
}

func TestTeam_ADeletedOrganisationTakesNoNewTeams(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	_, err := repos.Organisation.SoftDelete(t.Context(), orgID)
	require.NoError(t, err)

	// when
	_, err = repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "Late", MaxTeams: generousQuota})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTeam_ConcurrentCreatesStayWithinTheQuota(t *testing.T) {
	// given an organisation with its first team
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for i := range concurrentClaims {
		wg.Go(func() {
			_, err := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: fmt.Sprintf("Team %d", i), MaxTeams: teamQuotaUnderTest})

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				succeeded++
				return
			}

			failures = append(failures, err)
		})
	}
	wg.Wait()

	// then
	assert.Equal(t, teamQuotaUnderTest-1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrQuotaExceeded)
	}
	active, err := repos.Team.CountActive(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, teamQuotaUnderTest, active)
}

func TestTeam_ReadsTheHeaderAndListsInCreationOrder(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	second := daotest.CreateTeam(t, repos, orgID)

	// when
	header, err := repos.Team.GetHeader(t.Context(), second.ID)
	byID, byIDErr := repos.Team.GetByID(t.Context(), spec.TeamKey{OrgID: orgID, TeamID: second.ID})
	listed, listErr := repos.Team.ListForOrganisation(t.Context(), orgID)

	// then the header names the organisation, so a team route can find its tenant
	require.NoError(t, err)
	require.NoError(t, byIDErr)
	require.NoError(t, listErr)
	assert.Equal(t, orgID, header.OrgID)
	assert.Equal(t, *second, *byID)
	require.Len(t, listed, 2)
	assert.Equal(t, workspace.Team.ID, listed[0].ID)
	assert.Equal(t, second.ID, listed[1].ID)

	_, unknown := repos.Team.GetHeader(t.Context(), uuid.New())
	require.ErrorIs(t, unknown, dao.ErrNotFound)
}

func TestTeam_RenameArchiveAndDelete(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	orgID := workspace.Organisation.ID
	team := daotest.CreateTeam(t, repos, orgID)
	key := spec.TeamKey{OrgID: orgID, TeamID: team.ID}
	createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, TeamID: &team.ID})

	// when
	renamed, err := repos.Team.Rename(t.Context(), spec.TeamRename{OrgID: orgID, TeamID: team.ID, Name: "Renamed"})
	_, taken := repos.Team.Rename(t.Context(), spec.TeamRename{OrgID: orgID, TeamID: team.ID, Name: workspace.Team.Name})
	archived, archiveErr := repos.Team.Archive(t.Context(), key)
	_, archivedAgain := repos.Team.Archive(t.Context(), key)
	deleted, deleteErr := repos.Team.Delete(t.Context(), key)
	_, deletedAgain := repos.Team.Delete(t.Context(), key)

	// then deleting a team takes the invites bound to it
	require.NoError(t, err)
	assert.Equal(t, "Renamed", renamed.Name)
	require.ErrorIs(t, taken, dao.ErrConflict)
	require.NoError(t, archiveErr)
	assert.True(t, archived.Archived())
	require.ErrorIs(t, archivedAgain, dao.ErrNotFound)
	require.NoError(t, deleteErr)
	assert.Equal(t, team.ID, deleted.ID)
	require.ErrorIs(t, deletedAgain, dao.ErrNotFound)
	assert.Zero(t, countRows(t, repos, "organisation_invites"))
}

func TestTeam_MembersMustBelongToTheOrganisation(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	stranger := daotest.CreateUser(t, repos)
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)

	// when
	_, outsider := repos.Team.AddMember(t.Context(), spec.NewTeamMember{OrgID: orgID, TeamID: workspace.Team.ID, UserID: stranger.ID, Role: model.TeamRoleMember})
	added, err := repos.Team.AddMember(t.Context(), spec.NewTeamMember{OrgID: orgID, TeamID: workspace.Team.ID, UserID: alice.ID, Role: model.TeamRoleMember})
	_, twice := repos.Team.AddMember(t.Context(), spec.NewTeamMember{OrgID: orgID, TeamID: workspace.Team.ID, UserID: alice.ID, Role: model.TeamRoleLead})

	// then
	require.ErrorIs(t, outsider, dao.ErrNotFound)
	require.NoError(t, err)
	assert.Equal(t, model.TeamRoleMember, added.Role)
	require.ErrorIs(t, twice, dao.ErrConflict)
}

func TestTeam_MemberRolesListsAndRemoval(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	alice := daotest.CreateUser(t, repos, daotest.WithEmail("alice@example.com"), daotest.WithDisplayName("Alice"))
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	orgID := workspace.Organisation.ID
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	daotest.AddTeamMember(t, repos, &workspace.Team, alice.ID, model.TeamRoleMember)
	require.NoError(t, repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: owner.ID, Role: role.Admin}))
	aliceKey := spec.TeamMemberKey{OrgID: orgID, TeamID: workspace.Team.ID, UserID: alice.ID}

	// when
	promoted, err := repos.Team.SetMemberRole(t.Context(), spec.TeamMemberRoleChange{OrgID: orgID, TeamID: workspace.Team.ID, UserID: alice.ID, Role: model.TeamRoleLead})
	members, membersErr := repos.Team.ListMembers(t.Context(), spec.TeamKey{OrgID: orgID, TeamID: workspace.Team.ID})
	teams, teamsErr := repos.Team.ListForMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: alice.ID})

	// then
	require.NoError(t, err)
	require.NoError(t, membersErr)
	require.NoError(t, teamsErr)
	assert.Equal(t, model.TeamRoleLead, promoted.Role)

	require.Len(t, members, 2)
	assert.Equal(t, owner.ID, members[0].UserID)
	assert.Equal(t, model.OrgRoleOwner, members[0].OrgRole)
	assert.Equal(t, role.Admin, members[0].SiteRole)
	assert.Equal(t, alice.ID, members[1].UserID)
	assert.Equal(t, "alice@example.com", members[1].Email)
	assert.Equal(t, "Alice", members[1].DisplayName)
	assert.Equal(t, model.TeamRoleLead, members[1].Role)
	assert.Equal(t, model.OrgRoleMember, members[1].OrgRole)
	assert.Empty(t, members[1].SiteRole)

	require.Len(t, teams, 1)
	assert.Equal(t, workspace.Team.ID, teams[0].ID)
	assert.Equal(t, model.TeamRoleLead, teams[0].Role)

	removed, err := repos.Team.RemoveMember(t.Context(), aliceKey)
	require.NoError(t, err)
	assert.Equal(t, alice.ID, removed.UserID)
	_, again := repos.Team.RemoveMember(t.Context(), aliceKey)
	require.ErrorIs(t, again, dao.ErrNotFound)
	_, missing := repos.Team.SetMemberRole(t.Context(), spec.TeamMemberRoleChange{OrgID: orgID, TeamID: workspace.Team.ID, UserID: alice.ID, Role: model.TeamRoleMember})
	require.ErrorIs(t, missing, dao.ErrNotFound)
	assert.Equal(t, model.OrgRoleMember, memberRole(t, repos, orgID, alice.ID), "leaving a team keeps the organisation membership")
}
