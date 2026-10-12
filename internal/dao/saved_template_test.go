package dao_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	savedTemplateQuotaUnderTest = 3
)

var (
	savedTemplateColumns = []model.SavedTemplateColumn{
		{Title: "Went well", Description: "What should we keep?", Colour: model.ColourGreen},
		{Title: "To improve", Description: "", Colour: model.ColourRed},
	}
)

func personalTemplate(orgID uuid.UUID, ownerID uuid.UUID, name string) spec.NewSavedTemplate {
	return spec.NewSavedTemplate{
		OrgID:        orgID,
		OwnerID:      new(ownerID),
		CreatedBy:    ownerID,
		Name:         name,
		Description:  "Saved from a board",
		Columns:      savedTemplateColumns,
		MaxTemplates: daotest.UnlimitedQuota,
	}
}

func teamTemplate(orgID uuid.UUID, teamID uuid.UUID, creatorID uuid.UUID, name string) spec.NewSavedTemplate {
	return spec.NewSavedTemplate{
		OrgID:        orgID,
		TeamID:       new(teamID),
		CreatedBy:    creatorID,
		Name:         name,
		Columns:      savedTemplateColumns,
		MaxTemplates: daotest.UnlimitedQuota,
	}
}

func createSavedTemplate(t *testing.T, repos *repository.Repositories, s spec.NewSavedTemplate) *model.SavedTemplate {
	t.Helper()

	created, err := repos.SavedTemplate.Create(t.Context(), s)
	require.NoError(t, err)

	return created
}

func savedTemplateNames(templates []model.SavedTemplate) []string {
	names := make([]string, 0, len(templates))
	for _, template := range templates {
		names = append(names, template.Name)
	}

	return names
}

func TestSavedTemplateDAO_APersonalTemplateComesBackWhole(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	ownerID := workspace.Owner.UserID

	// when
	created, err := repos.SavedTemplate.Create(t.Context(), personalTemplate(workspace.Organisation.ID, ownerID, "Our retro"))

	// then
	require.NoError(t, err)
	assert.Equal(t, workspace.Organisation.ID, created.OrgID)
	require.NotNil(t, created.OwnerID)
	assert.Equal(t, ownerID, *created.OwnerID)
	assert.Nil(t, created.TeamID)
	require.NotNil(t, created.CreatedBy)
	assert.Equal(t, ownerID, *created.CreatedBy)
	assert.Equal(t, model.SavedTemplatePersonal, created.Scope())
	assert.Equal(t, "Our retro", created.Name)
	assert.Equal(t, "Saved from a board", created.Description)
	assert.Equal(t, savedTemplateColumns, created.Columns)
	assert.WithinDuration(t, time.Now(), created.CreatedAt, time.Minute)

	got, err := repos.SavedTemplate.Get(t.Context(), spec.SavedTemplateKey{OrgID: workspace.Organisation.ID, ID: created.ID})
	require.NoError(t, err)
	assert.Equal(t, savedTemplateColumns, got.Columns)
	assert.Empty(t, got.TeamName)
}

func TestSavedTemplateDAO_ATeamTemplateCarriesItsTeamName(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	created := createSavedTemplate(t, repos, teamTemplate(workspace.Organisation.ID, workspace.Team.ID, workspace.Owner.UserID, "Team retro"))

	// when
	got, err := repos.SavedTemplate.Get(t.Context(), spec.SavedTemplateKey{OrgID: workspace.Organisation.ID, ID: created.ID})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.SavedTemplateTeam, got.Scope())
	require.NotNil(t, got.TeamID)
	assert.Equal(t, workspace.Team.ID, *got.TeamID)
	assert.Nil(t, got.OwnerID)
	assert.Equal(t, workspace.Team.Name, got.TeamName)
}

func TestSavedTemplateDAO_TheFourthTemplateIsOverTheQuota(t *testing.T) {
	// given three templates, personal and team combined, on a quota of three
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID, ownerID := workspace.Organisation.ID, workspace.Owner.UserID
	for _, s := range []spec.NewSavedTemplate{
		personalTemplate(orgID, ownerID, "First"),
		personalTemplate(orgID, ownerID, "Second"),
		teamTemplate(orgID, workspace.Team.ID, ownerID, "Third"),
	} {
		s.MaxTemplates = savedTemplateQuotaUnderTest
		createSavedTemplate(t, repos, s)
	}
	fourth := teamTemplate(orgID, workspace.Team.ID, ownerID, "Fourth")
	fourth.MaxTemplates = savedTemplateQuotaUnderTest

	// when
	_, err := repos.SavedTemplate.Create(t.Context(), fourth)

	// then
	require.ErrorIs(t, err, dao.ErrQuotaExceeded)
	count, err := repos.SavedTemplate.Count(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, savedTemplateQuotaUnderTest, count)
}

func TestSavedTemplateDAO_ConcurrentCreatesStayWithinTheQuota(t *testing.T) {
	// given an organisation with no templates
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID, ownerID := workspace.Organisation.ID, workspace.Owner.UserID

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for i := range concurrentClaims {
		wg.Go(func() {
			s := personalTemplate(orgID, ownerID, fmt.Sprintf("Template %d", i))
			s.MaxTemplates = savedTemplateQuotaUnderTest
			_, err := repos.SavedTemplate.Create(t.Context(), s)

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
	assert.Equal(t, savedTemplateQuotaUnderTest, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrQuotaExceeded)
	}
	count, err := repos.SavedTemplate.Count(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, savedTemplateQuotaUnderTest, count)
}

func TestSavedTemplateDAO_ATemplateBelongsToATeamOrAPersonNeverBoth(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	both := personalTemplate(workspace.Organisation.ID, workspace.Owner.UserID, "Both")
	both.TeamID = new(workspace.Team.ID)
	neither := personalTemplate(workspace.Organisation.ID, workspace.Owner.UserID, "Neither")
	neither.OwnerID = nil

	// when
	_, bothErr := repos.SavedTemplate.Create(t.Context(), both)
	_, neitherErr := repos.SavedTemplate.Create(t.Context(), neither)

	// then
	require.ErrorIs(t, bothErr, dao.ErrConflict)
	require.ErrorIs(t, neitherErr, dao.ErrConflict)
	assert.Zero(t, countRows(t, repos, "saved_templates"))
}

func TestSavedTemplateDAO_ATemplateNeedsATeamAndAnOwnerInsideItsOrganisation(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	other := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	stranger := daotest.CreateUser(t, repos)

	// when
	_, foreignTeam := repos.SavedTemplate.Create(t.Context(), teamTemplate(workspace.Organisation.ID, other.Team.ID, workspace.Owner.UserID, "Foreign team"))
	_, outsider := repos.SavedTemplate.Create(t.Context(), personalTemplate(workspace.Organisation.ID, stranger.ID, "Outsider"))

	// then
	require.ErrorIs(t, foreignTeam, dao.ErrNotFound)
	require.ErrorIs(t, outsider, dao.ErrNotFound)
	assert.Zero(t, countRows(t, repos, "saved_templates"))
}

func TestSavedTemplateDAO_DeletingATeamTakesItsTemplates(t *testing.T) {
	// given a template of a team, and a personal one
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	team := daotest.CreateTeam(t, repos, orgID)
	createSavedTemplate(t, repos, teamTemplate(orgID, team.ID, workspace.Owner.UserID, "Team"))
	createSavedTemplate(t, repos, personalTemplate(orgID, workspace.Owner.UserID, "Personal"))

	// when
	_, err := repos.Team.Delete(t.Context(), spec.TeamKey{OrgID: orgID, TeamID: team.ID})

	// then
	require.NoError(t, err)
	listed, err := repos.SavedTemplate.List(t.Context(), spec.SavedTemplateViewer{OrgID: orgID, UserID: workspace.Owner.UserID})
	require.NoError(t, err)
	assert.Equal(t, []string{"Personal"}, savedTemplateNames(listed))
}

func TestSavedTemplateDAO_LeavingTheOrganisationTakesYourPersonalTemplates(t *testing.T) {
	// given a member with a personal template, who also saved one for the team
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	member := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, member.ID, model.OrgRoleMember)
	createSavedTemplate(t, repos, personalTemplate(orgID, member.ID, "Mine"))
	createSavedTemplate(t, repos, teamTemplate(orgID, workspace.Team.ID, member.ID, "Shared"))

	// when
	_, err := repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: member.ID})

	// then the personal one stops using the quota, and the team one stays
	require.NoError(t, err)
	count, err := repos.SavedTemplate.Count(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	listed, err := repos.SavedTemplate.List(t.Context(), spec.SavedTemplateViewer{OrgID: orgID, UserID: workspace.Owner.UserID})
	require.NoError(t, err)
	assert.Equal(t, []string{"Shared"}, savedTemplateNames(listed))
}

func TestSavedTemplateDAO_DeletingTheCreatorKeepsATeamTemplate(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	member := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, member.ID, model.OrgRoleMember)
	created := createSavedTemplate(t, repos, teamTemplate(orgID, workspace.Team.ID, member.ID, "Shared"))

	// when
	_, err := repos.DB().ExecContext(t.Context(), `DELETE FROM users WHERE id = $1`, member.ID)

	// then
	require.NoError(t, err)
	got, err := repos.SavedTemplate.Get(t.Context(), spec.SavedTemplateKey{OrgID: orgID, ID: created.ID})
	require.NoError(t, err)
	assert.Nil(t, got.CreatedBy)
}

func TestSavedTemplateDAO_ListShowsYourPersonalTemplatesAndEveryTeamTemplateOldestFirst(t *testing.T) {
	// given the owner's personal template, a team template, and another member's personal template
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID, ownerID := workspace.Organisation.ID, workspace.Owner.UserID
	member := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, member.ID, model.OrgRoleMember)
	createSavedTemplate(t, repos, personalTemplate(orgID, ownerID, "Owner's"))
	createSavedTemplate(t, repos, teamTemplate(orgID, workspace.Team.ID, member.ID, "Team"))
	createSavedTemplate(t, repos, personalTemplate(orgID, member.ID, "Member's"))

	// when
	forOwner, ownerErr := repos.SavedTemplate.List(t.Context(), spec.SavedTemplateViewer{OrgID: orgID, UserID: ownerID})
	forMember, memberErr := repos.SavedTemplate.List(t.Context(), spec.SavedTemplateViewer{OrgID: orgID, UserID: member.ID})

	// then
	require.NoError(t, ownerErr)
	require.NoError(t, memberErr)
	assert.Equal(t, []string{"Owner's", "Team"}, savedTemplateNames(forOwner))
	assert.Equal(t, []string{"Team", "Member's"}, savedTemplateNames(forMember))
	assert.Equal(t, workspace.Team.Name, forOwner[1].TeamName)
}

func TestSavedTemplateDAO_DeleteRemovesTheTemplateOnce(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	created := createSavedTemplate(t, repos, personalTemplate(orgID, workspace.Owner.UserID, "Gone"))
	key := spec.SavedTemplateKey{OrgID: orgID, ID: created.ID}

	// when
	deleted, firstErr := repos.SavedTemplate.Delete(t.Context(), key)
	_, secondErr := repos.SavedTemplate.Delete(t.Context(), key)

	// then
	require.NoError(t, firstErr)
	assert.Equal(t, created.ID, deleted.ID)
	require.ErrorIs(t, secondErr, dao.ErrNotFound)
	_, err := repos.SavedTemplate.Get(t.Context(), key)
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestCrossTenant_SavedTemplatesAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given a personal and a team template in organisation A
	tt := newTenants(t)
	orgA, orgB := tt.a.Organisation.ID, tt.b.Organisation.ID
	personal := createSavedTemplate(t, tt.repos, personalTemplate(orgA, tt.alice.ID, "Alice's"))
	team := createSavedTemplate(t, tt.repos, teamTemplate(orgA, tt.a.Team.ID, tt.a.Owner.UserID, "Team A"))

	// when
	_, getPersonalErr := tt.repos.SavedTemplate.Get(t.Context(), spec.SavedTemplateKey{OrgID: orgB, ID: personal.ID})
	_, getTeamErr := tt.repos.SavedTemplate.Get(t.Context(), spec.SavedTemplateKey{OrgID: orgB, ID: team.ID})
	_, deletePersonalErr := tt.repos.SavedTemplate.Delete(t.Context(), spec.SavedTemplateKey{OrgID: orgB, ID: personal.ID})
	_, deleteTeamErr := tt.repos.SavedTemplate.Delete(t.Context(), spec.SavedTemplateKey{OrgID: orgB, ID: team.ID})
	listedB, listBErr := tt.repos.SavedTemplate.List(t.Context(), spec.SavedTemplateViewer{OrgID: orgB, UserID: tt.alice.ID})
	listedByBOwner, listBOwnerErr := tt.repos.SavedTemplate.List(t.Context(), spec.SavedTemplateViewer{OrgID: orgB, UserID: tt.b.Owner.UserID})
	countB, countErr := tt.repos.SavedTemplate.Count(t.Context(), orgB)
	_, intoBErr := tt.repos.SavedTemplate.Create(t.Context(), teamTemplate(orgB, tt.a.Team.ID, tt.b.Owner.UserID, "Hijack"))

	// then
	require.ErrorIs(t, getPersonalErr, dao.ErrNotFound)
	require.ErrorIs(t, getTeamErr, dao.ErrNotFound)
	require.ErrorIs(t, deletePersonalErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteTeamErr, dao.ErrNotFound)
	require.NoError(t, listBErr)
	assert.Empty(t, listedB)
	require.NoError(t, listBOwnerErr)
	assert.Empty(t, listedByBOwner)
	require.NoError(t, countErr)
	assert.Zero(t, countB)
	require.ErrorIs(t, intoBErr, dao.ErrNotFound)

	listedA, err := tt.repos.SavedTemplate.List(t.Context(), spec.SavedTemplateViewer{OrgID: orgA, UserID: tt.alice.ID})
	require.NoError(t, err)
	assert.Equal(t, []string{"Alice's", "Team A"}, savedTemplateNames(listedA))
}
