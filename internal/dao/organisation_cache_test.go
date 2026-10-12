package dao_test

import (
	"context"
	"sync"
	"testing"

	"retrotide/internal/cache"
	"retrotide/internal/cache/engines"
	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/store"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	valkeymock "github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

type (
	valkeyRecorder struct {
		mu       sync.Mutex
		commands [][]string
	}
)

func newCachedRepos(t *testing.T) (*repository.Repositories, *valkeyRecorder) {
	t.Helper()

	client := valkeymock.NewClient(gomock.NewController(t))
	recorder := &valkeyRecorder{}

	client.EXPECT().
		Do(gomock.Any(), valkeymock.Match("FLUSHDB")).
		Return(valkeymock.Result(valkeymock.ValkeyString("OK"))).
		MaxTimes(1)

	client.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd valkey.Completed) valkey.ValkeyResult {
			recorder.mu.Lock()
			defer recorder.mu.Unlock()

			recorder.commands = append(recorder.commands, cmd.Commands())
			if cmd.Commands()[0] == "GET" {
				return valkeymock.ErrorResult(valkey.Nil)
			}

			return valkeymock.Result(valkeymock.ValkeyString("OK"))
		}).
		AnyTimes()

	return store.New(daotest.NewDB(t), cache.NewManager(engines.NewValkeyWithClient(client))), recorder
}

func (r *valkeyRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commands = nil
}

func (r *valkeyRecorder) recorded() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.commands
}

func TestOrganisationCache_AMembershipIsReadThroughTheCacheForAMinute(t *testing.T) {
	// given a cold cache
	repos, recorder := newCachedRepos(t)
	owner := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	recorder.reset()

	// when
	member, err := repos.Organisation.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: owner.ID})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.OrgRoleOwner, member.Role)
	commands := recorder.recorded()
	require.Len(t, commands, 2)
	assert.Equal(t, []string{"GET", cache.OrgMembership.Key(orgID.String(), owner.ID.String())}, commands[0])
	assert.Equal(t, []string{"SET", cache.OrgMembership.Key(orgID.String(), owner.ID.String())}, commands[1][:2])
	assert.Equal(t, []string{"NX", "PX", "60000"}, commands[1][3:])
}

func TestOrganisationCache_ANonMemberIsNeverCached(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	stranger := uuid.New()
	recorder.reset()

	// when
	_, err := repos.Organisation.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: stranger})

	// then someone who joins a moment later is let in at once
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, [][]string{{"GET", cache.OrgMembership.Key(orgID.String(), stranger.String())}}, recorder.recorded())
}

func TestOrganisationCache_AddingAMemberClearsTheirKeyAfterCommit(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	alice := daotest.CreateUser(t, repos)
	recorder.reset()

	// when
	_, err := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: alice.ID, Role: model.OrgRoleMember, MaxMembers: generousQuota})

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"DEL", cache.OrgMembership.Key(orgID.String(), alice.ID.String())}}, recorder.recorded())
}

func TestOrganisationCache_ChangingARoleClearsThatMembersKey(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	orgID := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID).Organisation.ID
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	recorder.reset()

	// when
	_, err := repos.Organisation.SetMemberRole(t.Context(), spec.OrganisationMemberRoleChange{OrgID: orgID, UserID: alice.ID, Role: model.OrgRoleAdmin})

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"DEL", cache.OrgMembership.Key(orgID.String(), alice.ID.String())}}, recorder.recorded())
}

func TestOrganisationCache_RemovingAMemberClearsTheirOrganisationAndTeamKeys(t *testing.T) {
	// given alice is on two teams
	repos, recorder := newCachedRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	second := daotest.CreateTeam(t, repos, orgID)
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	daotest.AddTeamMember(t, repos, &workspace.Team, alice.ID, model.TeamRoleMember)
	daotest.AddTeamMember(t, repos, second, alice.ID, model.TeamRoleLead)
	recorder.reset()

	// when
	_, err := repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: alice.ID})

	// then the team places the cascade removed are cleared too
	require.NoError(t, err)
	assert.Equal(t, [][]string{{
		"DEL",
		cache.OrgMembership.Key(orgID.String(), alice.ID.String()),
		cache.TeamMembership.Key(orgID.String(), workspace.Team.ID.String(), alice.ID.String()),
		cache.TeamMembership.Key(orgID.String(), second.ID.String(), alice.ID.String()),
	}}, recorder.recorded())
}

func TestOrganisationCache_ARefusedWriteClearsNothing(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	owner := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	recorder.reset()

	// when
	_, err := repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: owner.ID})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Empty(t, recorder.recorded())
}

func TestOrganisationCache_TransferringOwnershipClearsBothMembers(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	owner := daotest.CreateUser(t, repos)
	orgID := daotest.CreateOrg(t, repos, owner.ID).Organisation.ID
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleAdmin)
	recorder.reset()

	// when
	err := repos.Organisation.TransferOwnership(t.Context(), spec.OwnershipTransfer{OrgID: orgID, FromUserID: owner.ID, ToUserID: alice.ID, MaxOwned: daotest.UnlimitedQuota})

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"DEL", cache.OrgMembership.Key(orgID.String(), owner.ID.String()), cache.OrgMembership.Key(orgID.String(), alice.ID.String())}}, recorder.recorded())
}

func TestOrganisationCache_AcceptingAnInviteClearsTheNewMembersKeys(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID := workspace.Organisation.ID
	_, drawn := createInvite(t, repos, spec.NewOrganisationInvite{OrgID: orgID, TeamID: &workspace.Team.ID})
	alice := daotest.CreateUser(t, repos)
	recorder.reset()

	// when
	_, err := repos.Organisation.AcceptInvite(t.Context(), acceptance(drawn, alice, freeMemberQuota))

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"DEL", cache.OrgMembership.Key(orgID.String(), alice.ID.String()), cache.TeamMembership.Key(orgID.String(), workspace.Team.ID.String(), alice.ID.String())}}, recorder.recorded())
}

func TestTeamCache_TheHeaderAndMembershipsAreReadThroughTheCache(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	owner := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	orgID, teamID := workspace.Organisation.ID, workspace.Team.ID
	recorder.reset()

	// when
	header, headerErr := repos.Team.GetHeader(t.Context(), teamID)
	lead, leadErr := repos.Team.GetMember(t.Context(), spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: owner.ID})

	// then the membership key names the organisation, so another organisation's id never shares it
	require.NoError(t, headerErr)
	require.NoError(t, leadErr)
	assert.Equal(t, orgID, header.OrgID)
	assert.Equal(t, model.TeamRoleLead, lead.Role)

	commands := recorder.recorded()
	require.Len(t, commands, 4)
	assert.Equal(t, []string{"GET", cache.TeamHeader.Key(teamID.String())}, commands[0])
	assert.Equal(t, []string{"SET", cache.TeamHeader.Key(teamID.String())}, commands[1][:2])
	assert.Equal(t, []string{"GET", cache.TeamMembership.Key(orgID.String(), teamID.String(), owner.ID.String())}, commands[2])
	assert.Equal(t, []string{"SET", cache.TeamMembership.Key(orgID.String(), teamID.String(), owner.ID.String())}, commands[3][:2])
}

func TestTeamCache_WritesClearTheKeysTheyChange(t *testing.T) {
	tests := []struct {
		name  string
		write func(t *testing.T, repos *repository.Repositories, team *model.Team, userID uuid.UUID) error
		keys  func(team *model.Team, userID uuid.UUID) []string
	}{
		{
			name: "renaming",
			write: func(t *testing.T, repos *repository.Repositories, team *model.Team, _ uuid.UUID) error {
				_, err := repos.Team.Rename(t.Context(), spec.TeamRename{OrgID: team.OrgID, TeamID: team.ID, Name: "Renamed"})
				return err
			},
			keys: func(team *model.Team, _ uuid.UUID) []string {
				return []string{cache.TeamHeader.Key(team.ID.String())}
			},
		},
		{
			name: "archiving",
			write: func(t *testing.T, repos *repository.Repositories, team *model.Team, _ uuid.UUID) error {
				_, err := repos.Team.Archive(t.Context(), spec.TeamKey{OrgID: team.OrgID, TeamID: team.ID})
				return err
			},
			keys: func(team *model.Team, _ uuid.UUID) []string {
				return []string{cache.TeamHeader.Key(team.ID.String())}
			},
		},
		{
			name: "adding a member",
			write: func(t *testing.T, repos *repository.Repositories, team *model.Team, userID uuid.UUID) error {
				_, err := repos.Team.AddMember(t.Context(), spec.NewTeamMember{OrgID: team.OrgID, TeamID: team.ID, UserID: userID, Role: model.TeamRoleMember})
				return err
			},
			keys: func(team *model.Team, userID uuid.UUID) []string {
				return []string{cache.TeamMembership.Key(team.OrgID.String(), team.ID.String(), userID.String())}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos, recorder := newCachedRepos(t)
			workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
			alice := daotest.CreateUser(t, repos)
			daotest.AddOrgMember(t, repos, workspace.Organisation.ID, alice.ID, model.OrgRoleMember)
			recorder.reset()

			// when
			err := tc.write(t, repos, &workspace.Team, alice.ID)

			// then
			require.NoError(t, err)
			assert.Equal(t, [][]string{append([]string{"DEL"}, tc.keys(&workspace.Team, alice.ID)...)}, recorder.recorded())
		})
	}
}

func TestTeamCache_MemberRoleChangesAndRemovalsClearThatMembersKey(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	orgID, teamID := workspace.Organisation.ID, workspace.Team.ID
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	daotest.AddTeamMember(t, repos, &workspace.Team, alice.ID, model.TeamRoleMember)
	recorder.reset()

	// when
	_, roleErr := repos.Team.SetMemberRole(t.Context(), spec.TeamMemberRoleChange{OrgID: orgID, TeamID: teamID, UserID: alice.ID, Role: model.TeamRoleLead})
	_, removeErr := repos.Team.RemoveMember(t.Context(), spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: alice.ID})

	// then
	require.NoError(t, roleErr)
	require.NoError(t, removeErr)
	aliceKey := cache.TeamMembership.Key(orgID.String(), teamID.String(), alice.ID.String())
	assert.Equal(t, [][]string{{"DEL", aliceKey}, {"DEL", aliceKey}}, recorder.recorded())
}

func TestTeamCache_DeletingATeamClearsItsHeaderAndEveryMembersKey(t *testing.T) {
	// given
	repos, recorder := newCachedRepos(t)
	owner := daotest.CreateUser(t, repos)
	workspace := daotest.CreateOrg(t, repos, owner.ID)
	orgID, teamID := workspace.Organisation.ID, workspace.Team.ID
	alice := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, orgID, alice.ID, model.OrgRoleMember)
	daotest.AddTeamMember(t, repos, &workspace.Team, alice.ID, model.TeamRoleMember)
	recorder.reset()

	// when
	_, err := repos.Team.Delete(t.Context(), spec.TeamKey{OrgID: orgID, TeamID: teamID})

	// then
	require.NoError(t, err)
	assert.Equal(t, [][]string{{
		"DEL",
		cache.TeamHeader.Key(teamID.String()),
		cache.TeamMembership.Key(orgID.String(), teamID.String(), owner.ID.String()),
		cache.TeamMembership.Key(orgID.String(), teamID.String(), alice.ID.String()),
	}}, recorder.recorded())
}
