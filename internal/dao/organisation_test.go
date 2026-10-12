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
	"retrotide/internal/role"
	"retrotide/internal/token"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	inviteLifetime    = 7 * 24 * time.Hour
	restoreWindow     = 30 * 24 * time.Hour
	concurrentClaims  = 8
	generousQuota     = 100
	oneMemberInvite   = 1
	twoMemberInvite   = 2
	freeMemberQuota   = 10
	quotaUnderTest    = 5
	concurrentJoiners = 10
)

func countWhere(t *testing.T, repos *repository.Repositories, query string, args ...any) int {
	t.Helper()

	var rows int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), query, args...).Scan(&rows))

	return rows
}

func createInvite(t *testing.T, repos *repository.Repositories, invite spec.NewOrganisationInvite) (*model.OrganisationInvite, string) {
	t.Helper()

	drawn, err := token.New()
	require.NoError(t, err)

	invite.TokenHash = token.Hash(drawn)
	if invite.ExpiresAt.IsZero() {
		invite.ExpiresAt = time.Now().Add(inviteLifetime)
	}
	if invite.Role == "" {
		invite.Role = model.OrgRoleMember
	}

	created, err := repos.Organisation.CreateInvite(t.Context(), invite)
	require.NoError(t, err)

	return created, drawn
}

func memberRole(t *testing.T, repos *repository.Repositories, orgID uuid.UUID, userID uuid.UUID) model.OrgRole {
	t.Helper()

	member, err := repos.Organisation.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})
	require.NoError(t, err)

	return member.Role
}

func acceptance(drawn string, user *model.User, maxMembers int) spec.InviteAcceptance {
	return spec.InviteAcceptance{TokenHash: token.Hash(drawn), UserID: user.ID, Email: user.Email, MaxMembers: maxMembers}
}

func TestOrganisation_CreateWorkspaceMakesTheOwnerAndTheFirstTeamLead(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)

	// when
	created, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: "acme", Name: "Acme", TeamName: "Platform", MaxOwned: 1})

	// then the organisation, its owner, its first team and that team's lead are written together
	require.NoError(t, err)
	assert.Equal(t, "acme", created.Organisation.Slug)
	assert.Equal(t, "Acme", created.Organisation.Name)
	assert.True(t, created.Organisation.AllowGuests)
	assert.Equal(t, owner.ID, *created.Organisation.CreatedBy)
	assert.False(t, created.Organisation.Deleted())
	assert.Equal(t, model.OrgRoleOwner, created.Owner.Role)
	assert.Equal(t, "Platform", created.Team.Name)
	assert.Equal(t, created.Organisation.ID, created.Team.OrgID)
	assert.Equal(t, model.TeamRoleLead, created.Lead.Role)

	stored, err := repos.Organisation.GetByID(t.Context(), created.Organisation.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Organisation, *stored)
	assert.Equal(t, model.OrgRoleOwner, memberRole(t, repos, created.Organisation.ID, owner.ID))

	lead, err := repos.Team.GetMember(t.Context(), spec.TeamMemberKey{OrgID: created.Organisation.ID, TeamID: created.Team.ID, UserID: owner.ID})
	require.NoError(t, err)
	assert.Equal(t, model.TeamRoleLead, lead.Role)

	owned, err := repos.Organisation.CountOwnedByUser(t.Context(), owner.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, owned)
}

func TestOrganisation_CreateWorkspaceKeepsToThePerOwnerQuota(t *testing.T) {
	// given an owner who already has the one free organisation the quota allows
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	first, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: "first", Name: "First", TeamName: "Team", MaxOwned: 1})
	require.NoError(t, err)

	// when
	_, refused := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: "second", Name: "Second", TeamName: "Team", MaxOwned: 1})

	// then nothing of the second workspace is written, and a deleted organisation stops counting
	require.ErrorIs(t, refused, dao.ErrQuotaExceeded)
	assert.Equal(t, 1, countRows(t, repos, "organisations"))
	assert.Equal(t, 1, countRows(t, repos, "teams"))

	_, err = repos.Organisation.SoftDelete(t.Context(), first.Organisation.ID)
	require.NoError(t, err)
	_, err = repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: "second", Name: "Second", TeamName: "Team", MaxOwned: 1})
	require.NoError(t, err)
}

func TestOrganisation_CreateWorkspaceWritesNothingForATakenSlug(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	holder := daotest.CreateUser(t, repos)
	owner := daotest.CreateUser(t, repos)
	_, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: holder.ID, Slug: "acme", Name: "Acme", TeamName: "Team", MaxOwned: 1})
	require.NoError(t, err)

	// when
	_, taken := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: "acme", Name: "Other Acme", TeamName: "Team", MaxOwned: 1})

	// then
	require.ErrorIs(t, taken, dao.ErrConflict)
	assert.Equal(t, 1, countRows(t, repos, "organisations"))
	assert.Equal(t, 1, countRows(t, repos, "organisation_members"))
	assert.Equal(t, 1, countRows(t, repos, "teams"))
	assert.Equal(t, 1, countRows(t, repos, "team_members"))
}

func TestOrganisation_CreateWorkspaceForAMissingAccountIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	_, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: uuid.New(), Slug: "ghost", Name: "Ghost", TeamName: "Team", MaxOwned: 1})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Zero(t, countRows(t, repos, "organisations"))
}

func TestOrganisation_ConcurrentWorkspacesForOneOwnerStayWithinTheQuota(t *testing.T) {
	// given one owner onboarding in several tabs at once
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for i := range concurrentClaims {
		wg.Go(func() {
			_, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: fmt.Sprintf("tab-%d", i), Name: "Tab", TeamName: "Team", MaxOwned: 1})

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

	// then the row lock makes the count and the insert one step
	assert.Equal(t, 1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrQuotaExceeded)
	}
	assert.Equal(t, 1, countRows(t, repos, "organisations"))
}

func TestOrganisation_UpdateChangesTheSettingsOfALiveOrganisation(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	other := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID

	// when
	updated, err := repos.Organisation.Update(t.Context(), spec.OrganisationUpdate{OrgID: orgID, Slug: "renamed", Name: "Renamed", AllowGuests: false})

	// then
	require.NoError(t, err)
	assert.Equal(t, "renamed", updated.Slug)
	assert.Equal(t, "Renamed", updated.Name)
	assert.False(t, updated.AllowGuests)
	assert.False(t, updated.UpdatedAt.Before(workspace.Organisation.UpdatedAt))

	_, taken := repos.Organisation.Update(t.Context(), spec.OrganisationUpdate{OrgID: orgID, Slug: other.Organisation.Slug, Name: "Renamed"})
	require.ErrorIs(t, taken, dao.ErrConflict)

	_, err = repos.Organisation.SoftDelete(t.Context(), orgID)
	require.NoError(t, err)
	_, deleted := repos.Organisation.Update(t.Context(), spec.OrganisationUpdate{OrgID: orgID, Slug: "renamed", Name: "Again"})
	require.ErrorIs(t, deleted, dao.ErrNotFound)
}

func TestOrganisation_SoftDeleteLeavesAWindowToRestore(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID

	// when
	deleted, err := repos.Organisation.SoftDelete(t.Context(), orgID)

	// then the row stays, closed to new members, until it is restored within the window
	require.NoError(t, err)
	assert.True(t, deleted.Deleted())

	_, again := repos.Organisation.SoftDelete(t.Context(), orgID)
	require.ErrorIs(t, again, dao.ErrNotFound)
	require.ErrorIs(t, repos.Organisation.Lock(t.Context(), orgID), dao.ErrNotFound)

	_, late := repos.Organisation.Restore(t.Context(), spec.OrganisationRestore{OrgID: orgID, OwnerID: owner.ID, DeletedAfter: time.Now().Add(time.Minute), MaxOwned: 1})
	require.ErrorIs(t, late, dao.ErrNotFound)
	require.NotErrorIs(t, late, dao.ErrQuotaExceeded)

	restored, err := repos.Organisation.Restore(t.Context(), spec.OrganisationRestore{OrgID: orgID, OwnerID: owner.ID, DeletedAfter: time.Now().Add(-restoreWindow), MaxOwned: 1})
	require.NoError(t, err)
	assert.False(t, restored.Deleted())

	_, twice := repos.Organisation.Restore(t.Context(), spec.OrganisationRestore{OrgID: orgID, OwnerID: owner.ID, DeletedAfter: time.Now().Add(-restoreWindow), MaxOwned: 1})
	require.ErrorIs(t, twice, dao.ErrNotFound)
	require.NotErrorIs(t, twice, dao.ErrQuotaExceeded)
	require.NoError(t, repos.Organisation.Lock(t.Context(), orgID))
}

func TestOrganisation_RestoreKeepsToThePerOwnerQuota(t *testing.T) {
	// given an owner who deleted their one free organisation and then made another
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	first, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: "first", Name: "First", TeamName: "Team", MaxOwned: 1})
	require.NoError(t, err)
	_, err = repos.Organisation.SoftDelete(t.Context(), first.Organisation.ID)
	require.NoError(t, err)
	second, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: "second", Name: "Second", TeamName: "Team", MaxOwned: 1})
	require.NoError(t, err)
	restore := spec.OrganisationRestore{OrgID: first.Organisation.ID, OwnerID: owner.ID, DeletedAfter: time.Now().Add(-restoreWindow), MaxOwned: 1}

	// when
	_, refused := repos.Organisation.Restore(t.Context(), restore)

	// then the first stays deleted until the second no longer counts
	require.ErrorIs(t, refused, dao.ErrQuotaExceeded)
	kept, err := repos.Organisation.GetByID(t.Context(), first.Organisation.ID)
	require.NoError(t, err)
	assert.True(t, kept.Deleted())

	_, err = repos.Organisation.SoftDelete(t.Context(), second.Organisation.ID)
	require.NoError(t, err)
	restored, err := repos.Organisation.Restore(t.Context(), restore)
	require.NoError(t, err)
	assert.False(t, restored.Deleted())
}

func TestOrganisation_ConcurrentRestoresForOneOwnerStayWithinTheQuota(t *testing.T) {
	// given one owner with several deleted organisations and space for one
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	deleted := make([]uuid.UUID, 0, concurrentClaims)
	for i := range concurrentClaims {
		created, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: owner.ID, Slug: fmt.Sprintf("gone-%d", i), Name: "Gone", TeamName: "Team", MaxOwned: 1})
		require.NoError(t, err)
		_, err = repos.Organisation.SoftDelete(t.Context(), created.Organisation.ID)
		require.NoError(t, err)
		deleted = append(deleted, created.Organisation.ID)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for _, orgID := range deleted {
		wg.Go(func() {
			_, err := repos.Organisation.Restore(t.Context(), spec.OrganisationRestore{OrgID: orgID, OwnerID: owner.ID, DeletedAfter: time.Now().Add(-restoreWindow), MaxOwned: 1})

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

	// then the owner's row lock makes the count and the restore one step
	assert.Equal(t, 1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrQuotaExceeded)
	}
	owned, err := repos.Organisation.CountOwnedByUser(t.Context(), owner.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, owned)
}

func TestOrganisation_PurgeRemovesOnlyOrganisationsDeletedBeforeTheCutoff(t *testing.T) {
	// given one organisation deleted long ago, one deleted today and one live
	repos := daotest.NewRepos(t)
	old := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	recent := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	live := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	createInvite(t, repos, spec.NewOrganisationInvite{OrgID: old.Organisation.ID, TeamID: &old.Team.ID})
	for _, workspace := range []*model.Workspace{old, recent} {
		_, err := repos.Organisation.SoftDelete(t.Context(), workspace.Organisation.ID)
		require.NoError(t, err)
	}
	_, err := repos.DB().ExecContext(t.Context(), `UPDATE organisations SET deleted_at = NOW() - INTERVAL '31 days' WHERE id = $1`, old.Organisation.ID)
	require.NoError(t, err)

	// when
	purged, err := repos.Organisation.PurgeDeletedBefore(t.Context(), time.Now().Add(-restoreWindow))

	// then the purge takes the organisation's teams, members and invites with it
	require.NoError(t, err)
	assert.EqualValues(t, 1, purged)

	_, gone := repos.Organisation.GetByID(t.Context(), old.Organisation.ID)
	require.ErrorIs(t, gone, dao.ErrNotFound)
	for _, table := range []string{"organisation_members", "teams", "team_members", "organisation_invites"} {
		assert.Zero(t, countWhere(t, repos, "SELECT COUNT(*) FROM "+table+" WHERE org_id = $1", old.Organisation.ID), table)
	}

	for _, kept := range []*model.Workspace{recent, live} {
		_, err := repos.Organisation.GetByID(t.Context(), kept.Organisation.ID)
		require.NoError(t, err)
	}
}

func TestOrganisation_AddMemberKeepsToTheMemberQuota(t *testing.T) {
	// given an organisation with its owner, allowed two members
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	alice := daotest.CreateUser(t, repos)
	bob := daotest.CreateUser(t, repos)
	carol := daotest.CreateUser(t, repos)

	// when
	added, err := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: alice.ID, Role: model.OrgRoleMember, MaxMembers: 2})
	_, full := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: bob.ID, Role: model.OrgRoleMember, MaxMembers: 2})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.OrgRoleMember, added.Role)
	require.ErrorIs(t, full, dao.ErrQuotaExceeded)

	_, twice := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: alice.ID, Role: model.OrgRoleMember, MaxMembers: generousQuota})
	require.ErrorIs(t, twice, dao.ErrConflict)

	_, secondOwner := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: carol.ID, Role: model.OrgRoleOwner, MaxMembers: generousQuota})
	require.ErrorIs(t, secondOwner, dao.ErrConflict)

	_, ghost := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: uuid.New(), Role: model.OrgRoleMember, MaxMembers: generousQuota})
	require.ErrorIs(t, ghost, dao.ErrNotFound)

	count, err := repos.Organisation.CountMembers(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	_, err = repos.Organisation.SoftDelete(t.Context(), orgID)
	require.NoError(t, err)
	_, deleted := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: carol.ID, Role: model.OrgRoleMember, MaxMembers: generousQuota})
	require.ErrorIs(t, deleted, dao.ErrNotFound)
}

func TestOrganisation_ConcurrentJoinsStayWithinTheMemberQuota(t *testing.T) {
	// given an organisation with its owner, and more people joining at once than the quota has space for
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	joiners := make([]*model.User, 0, concurrentJoiners)
	for range concurrentJoiners {
		joiners = append(joiners, daotest.CreateUser(t, repos))
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for _, joiner := range joiners {
		wg.Go(func() {
			_, err := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: joiner.ID, Role: model.OrgRoleMember, MaxMembers: quotaUnderTest})

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
	assert.Equal(t, quotaUnderTest-1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrQuotaExceeded)
	}

	count, err := repos.Organisation.CountMembers(t.Context(), orgID)
	require.NoError(t, err)
	assert.Equal(t, quotaUnderTest, count)
}

func TestOrganisation_ListsMembersWithTheirProfiles(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos, daotest.WithEmail("olive@example.com"), daotest.WithDisplayName("Olive"))
	alice := daotest.CreateUser(t, repos, daotest.WithEmail("alice@example.com"), daotest.WithDisplayName("Alice"))
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleAdmin)
	require.NoError(t, repos.Role.SetRole(t.Context(), spec.RoleAssignment{UserID: alice.ID, Role: role.Support}))

	// when
	members, err := repos.Organisation.ListMembers(t.Context(), orgID)

	// then in the order they joined, each with their site staff role when they hold one
	require.NoError(t, err)
	require.Len(t, members, 2)
	assert.Equal(t, owner.ID, members[0].UserID)
	assert.Equal(t, "olive@example.com", members[0].Email)
	assert.Equal(t, "Olive", members[0].DisplayName)
	assert.Equal(t, model.OrgRoleOwner, members[0].Role)
	assert.Empty(t, members[0].SiteRole)
	assert.Equal(t, alice.ID, members[1].UserID)
	assert.Equal(t, model.OrgRoleAdmin, members[1].Role)
	assert.Equal(t, role.Support, members[1].SiteRole)

	_, stranger := repos.Organisation.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: daotest.CreateUser(t, repos).ID})
	require.ErrorIs(t, stranger, dao.ErrNotFound)
}

func TestOrganisation_SetMemberRoleNeverTouchesTheOwner(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	alice := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)

	// when
	promoted, err := repos.Organisation.SetMemberRole(t.Context(), spec.OrganisationMemberRoleChange{OrgID: orgID, UserID: alice.ID, Role: model.OrgRoleAdmin})
	_, demoteOwner := repos.Organisation.SetMemberRole(t.Context(), spec.OrganisationMemberRoleChange{OrgID: orgID, UserID: owner.ID, Role: model.OrgRoleMember})
	_, makeOwner := repos.Organisation.SetMemberRole(t.Context(), spec.OrganisationMemberRoleChange{OrgID: orgID, UserID: alice.ID, Role: model.OrgRoleOwner})
	_, stranger := repos.Organisation.SetMemberRole(t.Context(), spec.OrganisationMemberRoleChange{OrgID: orgID, UserID: uuid.New(), Role: model.OrgRoleAdmin})

	// then ownership only ever moves by a transfer
	require.NoError(t, err)
	assert.Equal(t, model.OrgRoleAdmin, promoted.Role)
	require.ErrorIs(t, demoteOwner, dao.ErrNotFound)
	require.ErrorIs(t, makeOwner, dao.ErrNotFound)
	require.ErrorIs(t, stranger, dao.ErrNotFound)
	assert.Equal(t, model.OrgRoleOwner, memberRole(t, repos, orgID, owner.ID))
	assert.Equal(t, model.OrgRoleAdmin, memberRole(t, repos, orgID, alice.ID))
}

func TestOrganisation_RemoveMemberTakesTheirTeamPlacesAndNeverTheOwner(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	alice := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	orgID := workspace.Organisation.ID
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	daotest.AddTeamMember(t, repos, &workspace.Team, alice.ID, model.TeamRoleMember)

	// when
	removed, err := repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: alice.ID})

	// then
	require.NoError(t, err)
	assert.Equal(t, alice.ID, removed.UserID)

	_, gone := repos.Organisation.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: alice.ID})
	require.ErrorIs(t, gone, dao.ErrNotFound)
	_, teamGone := repos.Team.GetMember(t.Context(), spec.TeamMemberKey{OrgID: orgID, TeamID: workspace.Team.ID, UserID: alice.ID})
	require.ErrorIs(t, teamGone, dao.ErrNotFound)

	_, again := repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: alice.ID})
	require.ErrorIs(t, again, dao.ErrNotFound)
	_, ownerErr := repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: owner.ID})
	require.ErrorIs(t, ownerErr, dao.ErrNotFound)
	assert.Equal(t, model.OrgRoleOwner, memberRole(t, repos, orgID, owner.ID))
}

func TestOrganisation_TransferOwnershipSwapsBothRolesOrNeither(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	alice := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)

	// when
	err := repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: orgID, FromUserID: owner.ID, ToUserID: alice.ID, MaxOwned: daotest.UnlimitedQuota})

	// then the previous owner stays on as an admin
	require.NoError(t, err)
	assert.Equal(t, model.OrgRoleAdmin, memberRole(t, repos, orgID, owner.ID))
	assert.Equal(t, model.OrgRoleOwner, memberRole(t, repos, orgID, alice.ID))

	notOwner := repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: orgID, FromUserID: owner.ID, ToUserID: alice.ID, MaxOwned: daotest.UnlimitedQuota})
	require.ErrorIs(t, notOwner, dao.ErrNotFound)

	toStranger := repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: orgID, FromUserID: alice.ID, ToUserID: uuid.New(), MaxOwned: daotest.UnlimitedQuota})
	require.ErrorIs(t, toStranger, dao.ErrNotFound)

	notMember := repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: orgID, FromUserID: alice.ID, ToUserID: daotest.CreateUser(t, repos).ID, MaxOwned: daotest.UnlimitedQuota})
	require.ErrorIs(t, notMember, dao.ErrNotFound)
	assert.Equal(t, model.OrgRoleOwner, memberRole(t, repos, orgID, alice.ID), "the demotion rolls back with the failed promotion")
}

func TestOrganisation_TransferOwnershipKeepsTheNewOwnerWithinTheirAllowance(t *testing.T) {
	// given alice already owns one live organisation and one deleted one, and is a member of owner's organisation
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	alice := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	daotest.CreateOrg(t, repos, alice.ID)
	_, err := repos.Organisation.SoftDelete(t.Context(), daotest.CreateOrg(t, repos, alice.ID).Organisation.ID)
	require.NoError(t, err)

	// when
	refused := repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: orgID, FromUserID: owner.ID, ToUserID: alice.ID, MaxOwned: 1})
	allowed := repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: orgID, FromUserID: owner.ID, ToUserID: alice.ID, MaxOwned: 2})

	// then only her live organisations count, and a refusal changes nobody's role
	require.ErrorIs(t, refused, dao.ErrQuotaExceeded)
	require.NoError(t, allowed)
	assert.Equal(t, model.OrgRoleOwner, memberRole(t, repos, orgID, alice.ID))
	assert.Equal(t, model.OrgRoleAdmin, memberRole(t, repos, orgID, owner.ID))
}

func TestOrganisation_ConcurrentTransfersToOneNewOwnerStayWithinTheAllowance(t *testing.T) {
	// given alice, who owns nothing, is a member of several organisations whose owners all hand them to her at once
	repos := daotest.NewRepos(t)
	alice := daotest.CreateUser(t, repos)

	transfers := make([]spec.OwnershipTransfer, 0, concurrentClaims)
	for range concurrentClaims {
		workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
		daotest.AddOrgMember(t, repos, workspace.Organisation.ID, alice.ID, model.OrgRoleMember)
		transfers = append(transfers, spec.OwnershipTransfer{OrgID: workspace.Organisation.ID, FromUserID: workspace.Owner.UserID, ToUserID: alice.ID, MaxOwned: 1})
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for _, transfer := range transfers {
		wg.Go(func() {
			err := repos.Organisation.TransferOwnership(t.Context(), transfer)

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

	// then her row lock makes the count and the promotion one step
	assert.Equal(t, 1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrQuotaExceeded)
	}

	owned, err := repos.Organisation.CountOwnedByUser(t.Context(), alice.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, owned)
}

func TestOrganisation_ListForUserShowsADeletedOrganisationOnlyToItsOwner(t *testing.T) {
	// given a user who owns one organisation, deleted since, is a member of another, and was a member of a third, deleted since
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	owned := daotest.CreateOrg(t, repos, user.ID).Organisation.ID
	joined := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	daotest.AddOrgMember(t, repos, joined, user.ID, model.OrgRoleMember)
	left := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	daotest.AddOrgMember(t, repos, left, user.ID, model.OrgRoleAdmin)
	_, err := repos.Organisation.SoftDelete(t.Context(), owned)
	require.NoError(t, err)
	_, err = repos.Organisation.SoftDelete(t.Context(), left)
	require.NoError(t, err)

	// when
	listed, err := repos.Organisation.ListForUser(t.Context(), user.ID)
	nothing, strangerErr := repos.Organisation.ListForUser(t.Context(), daotest.CreateUser(t, repos).ID)

	// then the owner still sees their deleted organisation, so they can restore it, and nobody else sees one
	require.NoError(t, err)
	require.NoError(t, strangerErr)
	require.Len(t, listed, 2)

	roles := map[uuid.UUID]model.OrgRole{}
	deleted := map[uuid.UUID]bool{}
	for _, organisation := range listed {
		roles[organisation.ID] = organisation.Role
		deleted[organisation.ID] = organisation.Deleted()
	}
	assert.Equal(t, map[uuid.UUID]model.OrgRole{owned: model.OrgRoleOwner, joined: model.OrgRoleMember}, roles)
	assert.Equal(t, map[uuid.UUID]bool{owned: true, joined: false}, deleted)

	assert.NotNil(t, nothing)
	assert.Empty(t, nothing)
}

func TestOrganisation_OwnedOrganisationsSharedWithOthersAreFound(t *testing.T) {
	// given a user who owns one organisation alone, one shared with a member, and one shared but deleted
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	daotest.CreateOrg(t, repos, user.ID)
	shared := daotest.CreateOrg(t, repos, user.ID).Organisation.ID
	daotest.AddOrgMember(t, repos, shared, daotest.CreateUser(t, repos).ID, model.OrgRoleMember)
	deleted := daotest.CreateOrg(t, repos, user.ID).Organisation.ID
	daotest.AddOrgMember(t, repos, deleted, daotest.CreateUser(t, repos).ID, model.OrgRoleMember)
	_, err := repos.Organisation.SoftDelete(t.Context(), deleted)
	require.NoError(t, err)

	// when
	found, err := repos.Organisation.ListOwnedWithOtherMembers(t.Context(), user.ID)
	owned, countErr := repos.Organisation.CountOwnedByUser(t.Context(), user.ID)

	// then only live organisations count
	require.NoError(t, err)
	require.NoError(t, countErr)
	require.Len(t, found, 1)
	assert.Equal(t, shared, found[0].ID)
	assert.Equal(t, 2, owned)
}

func TestOrganisation_InviteStoresTheHashAndReadsBackWithItsOrganisation(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	inviter := daotest.CreateUser(t, repos, daotest.WithDisplayName("Ines"))
	workspace := daotest.CreateOrg(t, repos, inviter.ID)
	orgID := workspace.Organisation.ID

	// when
	invite, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, TeamID: &workspace.Team.ID, Email: new("dana@example.com"), MaxUses: new(oneMemberInvite), InvitedBy: inviter.ID})

	// then only the hash is stored, and the acceptance page can name the organisation, team and inviter
	assert.Equal(t, token.Hash(drawn), invite.TokenHash)
	assert.NotEqual(t, drawn, invite.TokenHash)
	assert.Equal(t, model.OrgRoleMember, invite.Role)
	assert.Equal(t, "dana@example.com", *invite.Email)
	assert.True(t, invite.BoundToEmail())
	assert.Equal(t, oneMemberInvite, *invite.MaxUses)
	assert.Zero(t, invite.UseCount)
	assert.Equal(t, inviter.ID, *invite.InvitedBy)

	details, err := repos.Organisation.GetLiveInviteByTokenHash(t.Context(), token.Hash(drawn))
	require.NoError(t, err)
	assert.Equal(t, invite.ID, details.ID)
	assert.Equal(t, workspace.Organisation.Name, details.OrgName)
	assert.Equal(t, workspace.Organisation.Slug, details.OrgSlug)
	assert.Equal(t, workspace.Team.Name, *details.TeamName)
	assert.Equal(t, "Ines", details.InviterName)

	byKey, err := repos.Organisation.GetInvite(t.Context(), spec.OrganisationInviteKey{OrgID: orgID, InviteID: invite.ID})
	require.NoError(t, err)
	assert.Equal(t, *invite, *byKey)

	listed, err := repos.Organisation.ListInvites(t.Context(), orgID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, invite.ID, listed[0].ID)
}

func TestOrganisation_AcceptingALinkInviteJoinsTheOrganisationAndItsTeamUntilItIsUsedUp(t *testing.T) {
	// given a link invite for two people that makes them admins and puts them on the first team
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	_, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, TeamID: &workspace.Team.ID, Role: model.OrgRoleAdmin, MaxUses: new(twoMemberInvite)})
	alice := daotest.CreateUser(t, repos)
	bob := daotest.CreateUser(t, repos)
	carol := daotest.CreateUser(t, repos)

	// when
	accepted, err := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, alice, freeMemberQuota))
	second, secondErr := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, bob, freeMemberQuota))
	_, usedUp := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, carol, freeMemberQuota))

	// then
	require.NoError(t, err)
	require.NoError(t, secondErr)
	assert.Equal(t, model.OrgRoleAdmin, accepted.Member.Role)
	require.NotNil(t, accepted.TeamMember)
	assert.Equal(t, workspace.Team.ID, accepted.TeamMember.TeamID)
	assert.Equal(t, model.TeamRoleMember, accepted.TeamMember.Role)
	assert.Equal(t, 1, accepted.Invite.UseCount)
	assert.Equal(t, 2, second.Invite.UseCount)
	require.ErrorIs(t, usedUp, dao.ErrNotFound)

	assert.Equal(t, model.OrgRoleAdmin, memberRole(t, repos, orgID, alice.ID))
	_, carolErr := repos.Organisation.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: carol.ID})
	require.ErrorIs(t, carolErr, dao.ErrNotFound)

	listed, err := repos.Organisation.ListInvites(t.Context(), orgID)
	require.NoError(t, err)
	assert.Empty(t, listed, "a used-up invite is no longer pending")
	_, liveErr := repos.Organisation.GetLiveInviteByTokenHash(t.Context(), token.Hash(drawn))
	require.ErrorIs(t, liveErr, dao.ErrNotFound)
}

func TestOrganisation_AnInviteWhoseTeamWasArchivedJoinsOnlyTheOrganisation(t *testing.T) {
	// given a link invite to a team that was archived after the invite was sent
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	_, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, TeamID: &workspace.Team.ID})
	_, err := repos.Team.Archive(t.Context(), spec.TeamKey{OrgID: orgID, TeamID: workspace.Team.ID})
	require.NoError(t, err)
	alice := daotest.CreateUser(t, repos)

	// when
	details, lookupErr := repos.Organisation.GetLiveInviteByTokenHash(t.Context(), token.Hash(drawn))
	accepted, acceptErr := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, alice, freeMemberQuota))

	// then the page names no team, and the archived team takes no new member
	require.NoError(t, lookupErr)
	assert.Nil(t, details.TeamName)
	require.NoError(t, acceptErr)
	assert.Nil(t, accepted.TeamMember)
	assert.Equal(t, model.OrgRoleMember, memberRole(t, repos, orgID, alice.ID))
	_, teamErr := repos.Team.GetMember(t.Context(), spec.TeamMemberKey{OrgID: orgID, TeamID: workspace.Team.ID, UserID: alice.ID})
	require.ErrorIs(t, teamErr, dao.ErrNotFound)
}

func TestOrganisation_AnEmailBoundInviteIsAcceptedOnlyByThatAddress(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	invite, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, Email: new("dana@example.com"), MaxUses: new(oneMemberInvite)})
	eve := daotest.CreateUser(t, repos, daotest.WithEmail("eve@example.com"))
	dana := daotest.CreateUser(t, repos, daotest.WithEmail("Dana@Example.com"))

	// when
	_, wrongAddress := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, eve, freeMemberQuota))
	accepted, err := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, dana, freeMemberQuota))

	// then the address matches in any case, and a refusal spends nothing
	require.ErrorIs(t, wrongAddress, dao.ErrNotFound)
	require.NoError(t, err)
	assert.Equal(t, dana.ID, accepted.Member.UserID)
	assert.Nil(t, accepted.TeamMember)

	_, eveErr := repos.Organisation.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: eve.ID})
	require.ErrorIs(t, eveErr, dao.ErrNotFound)
	spent, err := repos.Organisation.GetInvite(t.Context(), spec.OrganisationInviteKey{OrgID: orgID, InviteID: invite.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, spent.UseCount)
}

func TestOrganisation_AnExpiredRevokedOrDeletedInviteIsNeverAccepted(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, repos *repository.Repositories, orgID uuid.UUID) string
	}{
		{
			name: "expired",
			prepare: func(t *testing.T, repos *repository.Repositories, orgID uuid.UUID) string {
				_, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, ExpiresAt: time.Now().Add(-time.Minute)})
				return drawn
			},
		},
		{
			name: "revoked",
			prepare: func(t *testing.T, repos *repository.Repositories, orgID uuid.UUID) string {
				invite, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID})
				_, err := repos.Organisation.RevokeInvite(t.Context(), spec.OrganisationInviteKey{OrgID: orgID, InviteID: invite.ID})
				require.NoError(t, err)
				return drawn
			},
		},
		{
			name: "its organisation deleted",
			prepare: func(t *testing.T, repos *repository.Repositories, orgID uuid.UUID) string {
				_, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID})
				_, err := repos.Organisation.SoftDelete(t.Context(), orgID)
				require.NoError(t, err)
				return drawn
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)
			orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
			drawn := tc.prepare(t, repos, orgID)
			joiner := daotest.CreateUser(t, repos)

			// when
			_, lookupErr := repos.Organisation.GetLiveInviteByTokenHash(t.Context(), token.Hash(drawn))
			_, acceptErr := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, joiner, freeMemberQuota))

			// then
			require.ErrorIs(t, lookupErr, dao.ErrNotFound)
			require.ErrorIs(t, acceptErr, dao.ErrNotFound)
			assert.Equal(t, 1, countRows(t, repos, "organisation_members"))
		})
	}
}

func TestOrganisation_AnInviteThatWouldPassTheMemberQuotaIsNotSpent(t *testing.T) {
	// given an organisation whose one allowed place its owner holds
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	invite, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID})

	// when
	_, err := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, daotest.CreateUser(t, repos), 1))

	// then the use rolls back with the refused insert
	require.ErrorIs(t, err, dao.ErrQuotaExceeded)
	unspent, err := repos.Organisation.GetInvite(t.Context(), spec.OrganisationInviteKey{OrgID: orgID, InviteID: invite.ID})
	require.NoError(t, err)
	assert.Zero(t, unspent.UseCount)
}

func TestOrganisation_AnExistingMemberAcceptingAnInviteSpendsNothing(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	invite, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, Role: model.OrgRoleAdmin})

	// when
	_, err := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, alice, freeMemberQuota))

	// then an invite never changes the role of someone already in
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Equal(t, model.OrgRoleMember, memberRole(t, repos, orgID, alice.ID))
	unspent, err := repos.Organisation.GetInvite(t.Context(), spec.OrganisationInviteKey{OrgID: orgID, InviteID: invite.ID})
	require.NoError(t, err)
	assert.Zero(t, unspent.UseCount)
}

func TestOrganisation_ConcurrentAcceptancesOfASingleUseInviteJoinOnce(t *testing.T) {
	// given one single-use link opened by several people at once
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	_, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, MaxUses: new(oneMemberInvite)})
	joiners := make([]*model.User, 0, concurrentClaims)
	for range concurrentClaims {
		joiners = append(joiners, daotest.CreateUser(t, repos))
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for _, joiner := range joiners {
		wg.Go(func() {
			_, err := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, joiner, freeMemberQuota))

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
	assert.Equal(t, 1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrNotFound)
	}
	assert.Equal(t, 2, countRows(t, repos, "organisation_members"))
}

func TestOrganisation_RevokingAnInviteStopsItAndTakesItOffTheList(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	invite, _ := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID})
	key := spec.OrganisationInviteKey{OrgID: orgID, InviteID: invite.ID}

	// when
	revoked, err := repos.Organisation.RevokeInvite(t.Context(), key)
	_, again := repos.Organisation.RevokeInvite(t.Context(), key)

	// then
	require.NoError(t, err)
	assert.NotNil(t, revoked.RevokedAt)
	require.ErrorIs(t, again, dao.ErrNotFound)

	listed, err := repos.Organisation.ListInvites(t.Context(), orgID)
	require.NoError(t, err)
	assert.Empty(t, listed)
	_, unknown := repos.Organisation.RevokeInvite(t.Context(), spec.OrganisationInviteKey{OrgID: orgID, InviteID: uuid.New()})
	require.ErrorIs(t, unknown, dao.ErrNotFound)
}

func TestOrganisation_TheExpirySweepDeletesOnlyExpiredInvites(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, ExpiresAt: time.Now().Add(-time.Minute)})
	live, _ := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID})

	// when
	deleted, err := repos.Organisation.DeleteExpiredInvites(t.Context())

	// then
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	_, err = repos.Organisation.GetInvite(t.Context(), spec.OrganisationInviteKey{OrgID: orgID, InviteID: live.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, repos, "organisation_invites"))
}

func TestOrganisationAccountDeletion_RefusesTheOwnerOfASharedOrganisation(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	daotest.AddOrgMember(t, repos, orgID, daotest.CreateUser(t, repos).ID, model.OrgRoleMember)

	// when
	err := repos.User.DeleteAccount(t.Context(), owner.ID)

	// then ownership has to be transferred, or the organisation deleted, first
	require.ErrorIs(t, err, repository.ErrOwnsSharedOrganisation)
	_, err = repos.User.GetByID(t.Context(), owner.ID)
	require.NoError(t, err)
	kept, err := repos.Organisation.GetByID(t.Context(), orgID)
	require.NoError(t, err)
	assert.False(t, kept.Deleted())
}

func TestOrganisationAccountDeletion_SoftDeletesTheOrganisationsOwnedAlone(t *testing.T) {
	// given a user who owns an organisation alone and is a member of someone else's
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	solo := daotest.CreateOrg(t, repos, user.ID).Organisation.ID
	other := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	daotest.AddOrgMember(t, repos, other, user.ID, model.OrgRoleAdmin)

	// when
	err := repos.User.DeleteAccount(t.Context(), user.ID)

	// then
	require.NoError(t, err)
	_, gone := repos.User.GetByID(t.Context(), user.ID)
	require.ErrorIs(t, gone, dao.ErrNotFound)

	deleted, err := repos.Organisation.GetByID(t.Context(), solo)
	require.NoError(t, err)
	assert.True(t, deleted.Deleted())

	kept, err := repos.Organisation.GetByID(t.Context(), other)
	require.NoError(t, err)
	assert.False(t, kept.Deleted())
	count, err := repos.Organisation.CountMembers(t.Context(), other)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestOrganisationAccountDeletion_ADeletedSharedOrganisationDoesNotHoldTheAccount(t *testing.T) {
	// given an owner who deleted their shared organisation already
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	_, err := repos.Organisation.SoftDelete(t.Context(), orgID)
	require.NoError(t, err)

	// when
	err = repos.User.DeleteAccount(t.Context(), owner.ID)

	// then
	require.NoError(t, err)
	assert.Equal(t, model.OrgRoleMember, memberRole(t, repos, orgID, alice.ID))
}
