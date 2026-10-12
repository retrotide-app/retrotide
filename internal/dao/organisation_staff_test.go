package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func staffSlugs(organisations []model.StaffOrganisation) []string {
	slugs := make([]string, 0, len(organisations))
	for _, listed := range organisations {
		slugs = append(slugs, listed.Slug)
	}

	return slugs
}

func TestOrganisationDAO_ListForStaff_PagesEveryOrganisationNewestFirstWithItsCounts(t *testing.T) {
	// given an organisation with a second member, a second team and a deleted board, and an older deleted one
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos, daotest.WithEmail("owner@example.com"), daotest.WithDisplayName("Olive Owner"))
	member := daotest.CreateUser(t, repos)
	busy := daotest.CreateOrg(t, repos, owner.ID)
	daotest.AddOrgMember(t, repos, busy.Organisation.ID, member.ID, model.OrgRoleMember)
	daotest.CreateTeam(t, repos, busy.Organisation.ID)
	daotest.CreateBoard(t, repos, busy.Team, owner.ID)
	deletedBoard := daotest.CreateBoard(t, repos, busy.Team, owner.ID)
	_, err := repos.DB().ExecContext(t.Context(), "UPDATE boards SET deleted_at = NOW() WHERE id = $1", deletedBoard.Header.ID)
	require.NoError(t, err)

	quiet := daotest.CreateOrg(t, repos, member.ID)
	backdate(t, repos, "organisations", quiet.Organisation.ID, time.Now().Add(-48*time.Hour))
	_, err = repos.Organisation.SoftDelete(t.Context(), quiet.Organisation.ID)
	require.NoError(t, err)

	// when
	first, firstErr := repos.Organisation.ListForStaff(t.Context(), spec.OrganisationListing{Limit: 1})
	second, secondErr := repos.Organisation.ListForStaff(t.Context(), spec.OrganisationListing{Limit: 1, Offset: 1})
	total, countErr := repos.Organisation.CountForStaff(t.Context(), "")

	// then the deleted organisation is still listed, marked deleted, and the deleted board is not counted
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, countErr)
	assert.Equal(t, 2, total)
	require.Len(t, first, 1)
	require.Len(t, second, 1)

	listed := first[0]
	assert.Equal(t, busy.Organisation.ID, listed.ID)
	assert.Equal(t, &owner.ID, listed.OwnerID)
	assert.Equal(t, new("Olive Owner"), listed.OwnerDisplayName)
	assert.Equal(t, new("owner@example.com"), listed.OwnerEmail)
	assert.Equal(t, 2, listed.MemberCount)
	assert.Equal(t, 2, listed.TeamCount)
	assert.Equal(t, 1, listed.BoardCount)
	assert.Nil(t, listed.DeletedAt)

	assert.Equal(t, quiet.Organisation.ID, second[0].ID)
	assert.NotNil(t, second[0].DeletedAt)
	assert.Equal(t, 1, second[0].MemberCount)
	assert.Equal(t, 0, second[0].BoardCount)
}

func TestOrganisationDAO_ListForStaff_SearchesTheNameAndTheSlug(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	acme := daotest.CreateOrg(t, repos, owner.ID)
	daotest.CreateOrg(t, repos, owner.ID)
	renamed, err := repos.Organisation.Update(t.Context(), spec.OrganisationUpdate{OrgID: acme.Organisation.ID, Slug: "rocket-co", Name: "Acme_Widgets"})
	require.NoError(t, err)

	// when
	byName, byNameErr := repos.Organisation.ListForStaff(t.Context(), spec.OrganisationListing{Search: "acme_w", Limit: 10})
	bySlug, bySlugErr := repos.Organisation.ListForStaff(t.Context(), spec.OrganisationListing{Search: "ROCKET", Limit: 10})
	wildcard, wildcardErr := repos.Organisation.ListForStaff(t.Context(), spec.OrganisationListing{Search: "%", Limit: 10})
	counted, countErr := repos.Organisation.CountForStaff(t.Context(), "rocket")

	// then a LIKE wildcard in the search is matched literally
	require.NoError(t, byNameErr)
	require.NoError(t, bySlugErr)
	require.NoError(t, wildcardErr)
	require.NoError(t, countErr)
	assert.Equal(t, []string{renamed.Slug}, staffSlugs(byName))
	assert.Equal(t, []string{renamed.Slug}, staffSlugs(bySlug))
	assert.Empty(t, wildcard)
	assert.Equal(t, 1, counted)
}

func TestOrganisationDAO_GetForStaff_ReadsOneOrganisationAndKnowsAnUnknownOne(t *testing.T) {
	// given an organisation whose owner deleted their account, so it has no owner left
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	_, err := repos.DB().ExecContext(t.Context(), "DELETE FROM organisation_members WHERE org_id = $1", workspace.Organisation.ID)
	require.NoError(t, err)

	// when
	found, foundErr := repos.Organisation.GetForStaff(t.Context(), workspace.Organisation.ID)
	_, missingErr := repos.Organisation.GetForStaff(t.Context(), uuid.New())

	// then
	require.NoError(t, foundErr)
	assert.Equal(t, workspace.Organisation.ID, found.ID)
	assert.Nil(t, found.OwnerID)
	assert.Nil(t, found.OwnerDisplayName)
	assert.Nil(t, found.OwnerEmail)
	assert.Equal(t, 0, found.MemberCount)
	assert.Equal(t, 1, found.TeamCount)
	require.ErrorIs(t, missingErr, dao.ErrNotFound)
}

func TestTeamDAO_ListWithCounts_CountsMembersAndLiveBoardsOfOneOrganisationOnly(t *testing.T) {
	// given two organisations, the first with a lead on its first team, a live board and a deleted one
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	member := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	daotest.AddOrgMember(t, repos, workspace.Organisation.ID, member.ID, model.OrgRoleMember)
	daotest.AddTeamMember(t, repos, &workspace.Team, member.ID, model.TeamRoleMember)
	second := daotest.CreateTeam(t, repos, workspace.Organisation.ID)
	daotest.CreateBoard(t, repos, workspace.Team, owner.ID)
	deletedBoard := daotest.CreateBoard(t, repos, workspace.Team, owner.ID)
	_, err := repos.DB().ExecContext(t.Context(), "UPDATE boards SET deleted_at = NOW() WHERE id = $1", deletedBoard.Header.ID)
	require.NoError(t, err)

	other := daotest.CreateOrg(t, repos, owner.ID)
	daotest.CreateBoard(t, repos, other.Team, owner.ID)

	// when
	teams, err := repos.Team.ListWithCounts(t.Context(), workspace.Organisation.ID)

	// then
	require.NoError(t, err)
	require.Len(t, teams, 2)
	assert.Equal(t, workspace.Team.ID, teams[0].ID)
	assert.Equal(t, 2, teams[0].MemberCount)
	assert.Equal(t, 1, teams[0].BoardCount)
	assert.Equal(t, second.ID, teams[1].ID)
	assert.Equal(t, 0, teams[1].MemberCount)
	assert.Equal(t, 0, teams[1].BoardCount)
}
