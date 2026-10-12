package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"retrotide/internal/cache"
	"retrotide/internal/cache/engines"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	valkeymock "github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

type (
	organisationRepoMocks struct {
		organisations *dao.MockOrganisationDAO
		users         *dao.MockUserDAO
		teams         *dao.MockTeamDAO
		client        *valkeymock.Client
	}
)

func newMockedOrganisationRepo(t *testing.T) (OrganisationRepository, organisationRepoMocks) {
	t.Helper()

	mocks := organisationRepoMocks{
		organisations: dao.NewMockOrganisationDAO(t),
		users:         dao.NewMockUserDAO(t),
		teams:         dao.NewMockTeamDAO(t),
		client:        valkeymock.NewClient(gomock.NewController(t)),
	}

	deps := OrganisationRepoDeps{Users: mocks.users, Teams: mocks.teams}

	return NewOrganisationRepo(nil, mocks.organisations, deps, cache.NewManager(engines.NewValkeyWithClient(mocks.client))), mocks
}

func TestOrganisationCreateWorkspace_WritesEveryPartInTheCallersTransaction(t *testing.T) {
	// given a cache mock with no expectations, so touching the cache fails the test
	repo, mocks := newMockedOrganisationRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	ownerID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	workspace := spec.NewWorkspace{OwnerID: ownerID, Slug: "acme", Name: "Acme", TeamName: "Platform", MaxOwned: 1}
	var order []string
	mocks.users.EXPECT().GetForUpdate(mock.Anything, ownerID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (*model.User, error) {
		order = append(order, "lock the owner")
		return &model.User{ID: ownerID}, nil
	}).Once()
	mocks.organisations.EXPECT().Create(mock.Anything, spec.NewOrganisation{Slug: "acme", Name: "Acme", CreatedBy: ownerID, MaxOwned: 1}, inTx).RunAndReturn(func(context.Context, spec.NewOrganisation, ...*sql.Tx) (*model.Organisation, error) {
		order = append(order, "create the organisation")
		return &model.Organisation{ID: orgID, Slug: "acme"}, nil
	}).Once()
	mocks.organisations.EXPECT().AddMember(mock.Anything, spec.NewOrganisationMember{OrgID: orgID, UserID: ownerID, Role: model.OrgRoleOwner, MaxMembers: 1}, inTx).RunAndReturn(func(context.Context, spec.NewOrganisationMember, ...*sql.Tx) (*model.OrganisationMember, error) {
		order = append(order, "add the owner")
		return &model.OrganisationMember{OrgID: orgID, UserID: ownerID, Role: model.OrgRoleOwner}, nil
	}).Once()
	mocks.teams.EXPECT().Create(mock.Anything, spec.NewTeam{OrgID: orgID, Name: "Platform", MaxTeams: 1}, inTx).RunAndReturn(func(context.Context, spec.NewTeam, ...*sql.Tx) (*model.Team, error) {
		order = append(order, "create the first team")
		return &model.Team{ID: teamID, OrgID: orgID, Name: "Platform"}, nil
	}).Once()
	mocks.teams.EXPECT().AddMember(mock.Anything, spec.NewTeamMember{OrgID: orgID, TeamID: teamID, UserID: ownerID, Role: model.TeamRoleLead}, inTx).RunAndReturn(func(context.Context, spec.NewTeamMember, ...*sql.Tx) (*model.TeamMember, error) {
		order = append(order, "make the owner its lead")
		return &model.TeamMember{OrgID: orgID, TeamID: teamID, UserID: ownerID, Role: model.TeamRoleLead}, nil
	}).Once()

	// when
	created, err := repo.CreateWorkspace(t.Context(), workspace, tx)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"lock the owner", "create the organisation", "add the owner", "create the first team", "make the owner its lead"}, order)
	assert.Equal(t, orgID, created.Organisation.ID)
	assert.Equal(t, teamID, created.Team.ID)
	assert.Equal(t, model.OrgRoleOwner, created.Owner.Role)
	assert.Equal(t, model.TeamRoleLead, created.Lead.Role)
}

func TestOrganisationCreateWorkspace_StopsAtTheQuota(t *testing.T) {
	// given team mocks with no expectations, so writing past the refusal fails the test
	repo, mocks := newMockedOrganisationRepo(t)
	tx := new(sql.Tx)
	ownerID := uuid.New()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, ownerID, []*sql.Tx{tx}).Return(&model.User{ID: ownerID}, nil).Once()
	mocks.organisations.EXPECT().Create(mock.Anything, mock.Anything, []*sql.Tx{tx}).Return(nil, dao.ErrQuotaExceeded).Once()

	// when
	_, err := repo.CreateWorkspace(t.Context(), spec.NewWorkspace{OwnerID: ownerID, Slug: "acme", Name: "Acme", TeamName: "Team", MaxOwned: 1}, tx)

	// then
	require.ErrorIs(t, err, dao.ErrQuotaExceeded)
}

func TestOrganisationRestore_LocksTheOwnerBeforeTheConditionalRestore(t *testing.T) {
	// given
	repo, mocks := newMockedOrganisationRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	restore := spec.OrganisationRestore{OrgID: uuid.New(), OwnerID: uuid.New(), DeletedAfter: time.Now().Add(-time.Hour), MaxOwned: 1}
	var order []string
	mocks.users.EXPECT().GetForUpdate(mock.Anything, restore.OwnerID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (*model.User, error) {
		order = append(order, "lock the owner")
		return &model.User{ID: restore.OwnerID}, nil
	}).Once()
	mocks.organisations.EXPECT().Restore(mock.Anything, restore, inTx).RunAndReturn(func(context.Context, spec.OrganisationRestore, ...*sql.Tx) (*model.Organisation, error) {
		order = append(order, "restore within the quota")
		return &model.Organisation{ID: restore.OrgID}, nil
	}).Once()

	// when
	restored, err := repo.Restore(t.Context(), restore, tx)

	// then a workspace or another restore for the same owner waits for the lock
	require.NoError(t, err)
	assert.Equal(t, restore.OrgID, restored.ID)
	assert.Equal(t, []string{"lock the owner", "restore within the quota"}, order)
}

func TestOrganisationRestore_TellsTheQuotaFromAnOrganisationThatCannotBeRestored(t *testing.T) {
	deletedAfter := time.Now().Add(-time.Hour)

	tests := []struct {
		name  string
		found *model.Organisation
		want  error
	}{
		{name: "still deleted within the window", found: &model.Organisation{DeletedAt: new(time.Now())}, want: dao.ErrQuotaExceeded},
		{name: "restored meanwhile", found: &model.Organisation{}, want: dao.ErrNotFound},
		{name: "past the window", found: &model.Organisation{DeletedAt: new(deletedAfter.Add(-time.Minute))}, want: dao.ErrNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repo, mocks := newMockedOrganisationRepo(t)
			tx := new(sql.Tx)
			inTx := []*sql.Tx{tx}
			restore := spec.OrganisationRestore{OrgID: uuid.New(), OwnerID: uuid.New(), DeletedAfter: deletedAfter, MaxOwned: 1}
			mocks.users.EXPECT().GetForUpdate(mock.Anything, restore.OwnerID, inTx).Return(&model.User{ID: restore.OwnerID}, nil).Once()
			mocks.organisations.EXPECT().Restore(mock.Anything, restore, inTx).Return(nil, dao.ErrNotFound).Once()
			mocks.organisations.EXPECT().GetByID(mock.Anything, restore.OrgID, inTx).Return(tc.found, nil).Once()

			// when
			_, err := repo.Restore(t.Context(), restore, tx)

			// then
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestOrganisationMembershipCache_ServesTheCacheWithoutTouchingTheDao(t *testing.T) {
	// given a warm cache, and a dao mock with no expectations so any call fails the test
	repo, mocks := newMockedOrganisationRepo(t)
	orgID, userID := uuid.New(), uuid.New()
	captureSettingsCommands(mocks.client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString(`{"OrgID":"` + orgID.String() + `","UserID":"` + userID.String() + `","Role":"admin","JoinedAt":"2026-09-27T12:00:00Z"}`))
	})

	// when
	got, err := repo.GetMember(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.OrgRoleAdmin, got.Role)
	assert.Equal(t, userID, got.UserID)
}

func TestOrganisationWrites_RefuseAnInboundTransaction(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()

	tests := []struct {
		name  string
		write func(ctx context.Context, repo OrganisationRepository, tx *sql.Tx) error
	}{
		{name: "AddMember", write: func(ctx context.Context, repo OrganisationRepository, tx *sql.Tx) error {
			_, err := repo.AddMember(ctx, spec.NewOrganisationMember{OrgID: orgID, UserID: userID, Role: model.OrgRoleMember, MaxMembers: 10}, tx)
			return err
		}},
		{name: "SetMemberRole", write: func(ctx context.Context, repo OrganisationRepository, tx *sql.Tx) error {
			_, err := repo.SetMemberRole(ctx, spec.OrganisationMemberRoleChange{OrgID: orgID, UserID: userID, Role: model.OrgRoleAdmin}, tx)
			return err
		}},
		{name: "RemoveMember", write: func(ctx context.Context, repo OrganisationRepository, tx *sql.Tx) error {
			_, err := repo.RemoveMember(ctx, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}, tx)
			return err
		}},
		{name: "TransferOwnership", write: func(ctx context.Context, repo OrganisationRepository, tx *sql.Tx) error {
			return repo.TransferOwnership(ctx, spec.OwnershipTransfer{OrgID: orgID, FromUserID: userID, ToUserID: uuid.New()}, tx)
		}},
		{name: "AcceptInvite", write: func(ctx context.Context, repo OrganisationRepository, tx *sql.Tx) error {
			_, err := repo.AcceptInvite(ctx, spec.InviteAcceptance{TokenHash: "hash", UserID: userID, Email: "alice@example.com", MaxMembers: 10}, tx)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given dao and cache mocks with no expectations, so any call fails the test
			repo, mocks := newMockedOrganisationRepo(t)
			mocks.client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

			// when
			err := tc.write(t.Context(), repo, new(sql.Tx))

			// then an invalidation can never run inside a transaction it did not open
			require.ErrorIs(t, err, db.ErrInboundTx)
		})
	}
}

func TestOrganisationSetMemberRole_ClearsTheMembersKeyAfterTheWrite(t *testing.T) {
	// given
	repo, mocks := newMockedOrganisationRepo(t)
	change := spec.OrganisationMemberRoleChange{OrgID: uuid.New(), UserID: uuid.New(), Role: model.OrgRoleAdmin}
	changed := &model.OrganisationMember{OrgID: change.OrgID, UserID: change.UserID, Role: model.OrgRoleAdmin}
	mocks.organisations.EXPECT().SetMemberRole(mock.Anything, change).Return(changed, nil).Once()
	commands := captureSettingsCommands(mocks.client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyInt64(1))
	})

	// when
	got, err := repo.SetMemberRole(t.Context(), change)

	// then
	require.NoError(t, err)
	assert.Same(t, changed, got)
	assert.Equal(t, [][]string{{"DEL", cache.OrgMembership.Key(change.OrgID.String(), change.UserID.String())}}, *commands)
}

func TestOrganisationSetMemberRole_AFailedWriteLeavesTheCacheAlone(t *testing.T) {
	// given
	repo, mocks := newMockedOrganisationRepo(t)
	change := spec.OrganisationMemberRoleChange{OrgID: uuid.New(), UserID: uuid.New(), Role: model.OrgRoleAdmin}
	mocks.organisations.EXPECT().SetMemberRole(mock.Anything, change).Return(nil, dao.ErrNotFound).Once()
	mocks.client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

	// when
	_, err := repo.SetMemberRole(t.Context(), change)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestOrganisationSetMemberRole_ReportsAFailedInvalidationWithTheSavedMember(t *testing.T) {
	// given a write that commits while the cache refuses the delete
	repo, mocks := newMockedOrganisationRepo(t)
	change := spec.OrganisationMemberRoleChange{OrgID: uuid.New(), UserID: uuid.New(), Role: model.OrgRoleMember}
	changed := &model.OrganisationMember{OrgID: change.OrgID, UserID: change.UserID, Role: model.OrgRoleMember}
	mocks.organisations.EXPECT().SetMemberRole(mock.Anything, change).Return(changed, nil).Once()
	cacheErr := errors.New("valkey unreachable")
	captureSettingsCommands(mocks.client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.ErrorResult(cacheErr)
	})

	// when
	got, err := repo.SetMemberRole(t.Context(), change)

	// then the caller learns the old role may apply for up to a minute, and still gets the saved row
	require.ErrorIs(t, err, ErrStaleTenantCache)
	require.ErrorIs(t, err, cacheErr)
	assert.Same(t, changed, got)
}
