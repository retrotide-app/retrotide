package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

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
	teamRepoMocks struct {
		teams         *dao.MockTeamDAO
		organisations *dao.MockOrganisationDAO
		client        *valkeymock.Client
	}
)

func newMockedTeamRepo(t *testing.T) (TeamRepository, teamRepoMocks) {
	t.Helper()

	mocks := teamRepoMocks{
		teams:         dao.NewMockTeamDAO(t),
		organisations: dao.NewMockOrganisationDAO(t),
		client:        valkeymock.NewClient(gomock.NewController(t)),
	}

	return NewTeamRepo(nil, mocks.teams, mocks.organisations, cache.NewManager(engines.NewValkeyWithClient(mocks.client))), mocks
}

func TestTeamCreate_LocksTheOrganisationBeforeTheQuotaInsert(t *testing.T) {
	// given
	repo, mocks := newMockedTeamRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	newTeam := spec.NewTeam{OrgID: uuid.New(), Name: "Platform", MaxTeams: 1}
	created := &model.Team{ID: uuid.New(), OrgID: newTeam.OrgID, Name: "Platform"}
	var order []string
	mocks.organisations.EXPECT().Lock(mock.Anything, newTeam.OrgID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) error {
		order = append(order, "lock")
		return nil
	}).Once()
	mocks.teams.EXPECT().Create(mock.Anything, newTeam, inTx).RunAndReturn(func(context.Context, spec.NewTeam, ...*sql.Tx) (*model.Team, error) {
		order = append(order, "insert")
		return created, nil
	}).Once()

	// when
	got, err := repo.Create(t.Context(), newTeam, tx)

	// then the count the insert compares with cannot change under it
	require.NoError(t, err)
	assert.Same(t, created, got)
	assert.Equal(t, []string{"lock", "insert"}, order)
}

func TestTeamCreate_ADeletedOrganisationIsNeverLockedForATeam(t *testing.T) {
	// given a team dao mock with no expectations, so an insert fails the test
	repo, mocks := newMockedTeamRepo(t)
	tx := new(sql.Tx)
	newTeam := spec.NewTeam{OrgID: uuid.New(), Name: "Platform", MaxTeams: 1}
	mocks.organisations.EXPECT().Lock(mock.Anything, newTeam.OrgID, []*sql.Tx{tx}).Return(dao.ErrNotFound).Once()

	// when
	_, err := repo.Create(t.Context(), newTeam, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTeamHeader_ServesTheCacheWithoutTouchingTheDao(t *testing.T) {
	// given a warm cache, and a dao mock with no expectations so any call fails the test
	repo, mocks := newMockedTeamRepo(t)
	teamID, orgID := uuid.New(), uuid.New()
	captureSettingsCommands(mocks.client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.Result(valkeymock.ValkeyString(`{"ID":"` + teamID.String() + `","OrgID":"` + orgID.String() + `","Name":"Platform","CreatedAt":"2026-09-27T12:00:00Z","ArchivedAt":null}`))
	})

	// when
	got, err := repo.GetHeader(t.Context(), teamID)

	// then
	require.NoError(t, err)
	assert.Equal(t, orgID, got.OrgID)
	assert.False(t, got.Archived())
}

func TestTeamMembership_AWarmCacheNeverAnswersForAnotherOrganisation(t *testing.T) {
	// given alice's place on team A, read through its own organisation into a real cache
	teams, organisations := dao.NewMockTeamDAO(t), dao.NewMockOrganisationDAO(t)
	repo := NewTeamRepo(nil, teams, organisations, cache.NewManager(engines.NewInMemory(0)))
	orgA, orgB, teamA, alice := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	own := spec.TeamMemberKey{OrgID: orgA, TeamID: teamA, UserID: alice}
	foreign := spec.TeamMemberKey{OrgID: orgB, TeamID: teamA, UserID: alice}
	teams.EXPECT().GetMember(mock.Anything, own).Return(&model.TeamMember{OrgID: orgA, TeamID: teamA, UserID: alice, Role: model.TeamRoleLead}, nil).Once()
	teams.EXPECT().GetMember(mock.Anything, foreign).Return(nil, dao.ErrNotFound).Once()

	_, err := repo.GetMember(t.Context(), own)
	require.NoError(t, err)

	// when
	got, err := repo.GetMember(t.Context(), foreign)

	// then the other organisation's key misses, and the organisation-scoped read answers
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Nil(t, got)
}

func TestTeamWrites_RefuseAnInboundTransaction(t *testing.T) {
	key := spec.TeamKey{OrgID: uuid.New(), TeamID: uuid.New()}
	memberKey := spec.TeamMemberKey{OrgID: key.OrgID, TeamID: key.TeamID, UserID: uuid.New()}

	tests := []struct {
		name  string
		write func(ctx context.Context, repo TeamRepository, tx *sql.Tx) error
	}{
		{name: "Rename", write: func(ctx context.Context, repo TeamRepository, tx *sql.Tx) error {
			_, err := repo.Rename(ctx, spec.TeamRename{OrgID: key.OrgID, TeamID: key.TeamID, Name: "Renamed"}, tx)
			return err
		}},
		{name: "Archive", write: func(ctx context.Context, repo TeamRepository, tx *sql.Tx) error {
			_, err := repo.Archive(ctx, key, tx)
			return err
		}},
		{name: "Delete", write: func(ctx context.Context, repo TeamRepository, tx *sql.Tx) error {
			_, err := repo.Delete(ctx, key, tx)
			return err
		}},
		{name: "AddMember", write: func(ctx context.Context, repo TeamRepository, tx *sql.Tx) error {
			_, err := repo.AddMember(ctx, spec.NewTeamMember{OrgID: key.OrgID, TeamID: key.TeamID, UserID: memberKey.UserID, Role: model.TeamRoleMember}, tx)
			return err
		}},
		{name: "SetMemberRole", write: func(ctx context.Context, repo TeamRepository, tx *sql.Tx) error {
			_, err := repo.SetMemberRole(ctx, spec.TeamMemberRoleChange{OrgID: key.OrgID, TeamID: key.TeamID, UserID: memberKey.UserID, Role: model.TeamRoleLead}, tx)
			return err
		}},
		{name: "RemoveMember", write: func(ctx context.Context, repo TeamRepository, tx *sql.Tx) error {
			_, err := repo.RemoveMember(ctx, memberKey, tx)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given dao and cache mocks with no expectations, so any call fails the test
			repo, mocks := newMockedTeamRepo(t)
			mocks.client.EXPECT().Do(gomock.Any(), gomock.Any()).Times(0)

			// when
			err := tc.write(t.Context(), repo, new(sql.Tx))

			// then
			require.ErrorIs(t, err, db.ErrInboundTx)
		})
	}
}

func TestTeamWrites_ClearTheKeyTheyChange(t *testing.T) {
	orgID, teamID, userID := uuid.New(), uuid.New(), uuid.New()
	header := cache.TeamHeader.Key(teamID.String())
	membership := cache.TeamMembership.Key(orgID.String(), teamID.String(), userID.String())

	tests := []struct {
		name   string
		expect func(teams *dao.MockTeamDAO)
		write  func(ctx context.Context, repo TeamRepository) error
		key    string
	}{
		{
			name: "Rename",
			expect: func(teams *dao.MockTeamDAO) {
				teams.EXPECT().Rename(mock.Anything, spec.TeamRename{OrgID: orgID, TeamID: teamID, Name: "Renamed"}).Return(&model.Team{ID: teamID}, nil).Once()
			},
			write: func(ctx context.Context, repo TeamRepository) error {
				_, err := repo.Rename(ctx, spec.TeamRename{OrgID: orgID, TeamID: teamID, Name: "Renamed"})
				return err
			},
			key: header,
		},
		{
			name: "Archive",
			expect: func(teams *dao.MockTeamDAO) {
				teams.EXPECT().Archive(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return(&model.Team{ID: teamID}, nil).Once()
			},
			write: func(ctx context.Context, repo TeamRepository) error {
				_, err := repo.Archive(ctx, spec.TeamKey{OrgID: orgID, TeamID: teamID})
				return err
			},
			key: header,
		},
		{
			name: "AddMember",
			expect: func(teams *dao.MockTeamDAO) {
				teams.EXPECT().AddMember(mock.Anything, spec.NewTeamMember{OrgID: orgID, TeamID: teamID, UserID: userID, Role: model.TeamRoleMember}).Return(&model.TeamMember{UserID: userID}, nil).Once()
			},
			write: func(ctx context.Context, repo TeamRepository) error {
				_, err := repo.AddMember(ctx, spec.NewTeamMember{OrgID: orgID, TeamID: teamID, UserID: userID, Role: model.TeamRoleMember})
				return err
			},
			key: membership,
		},
		{
			name: "SetMemberRole",
			expect: func(teams *dao.MockTeamDAO) {
				teams.EXPECT().SetMemberRole(mock.Anything, spec.TeamMemberRoleChange{OrgID: orgID, TeamID: teamID, UserID: userID, Role: model.TeamRoleLead}).Return(&model.TeamMember{UserID: userID}, nil).Once()
			},
			write: func(ctx context.Context, repo TeamRepository) error {
				_, err := repo.SetMemberRole(ctx, spec.TeamMemberRoleChange{OrgID: orgID, TeamID: teamID, UserID: userID, Role: model.TeamRoleLead})
				return err
			},
			key: membership,
		},
		{
			name: "RemoveMember",
			expect: func(teams *dao.MockTeamDAO) {
				teams.EXPECT().RemoveMember(mock.Anything, spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: userID}).Return(&model.TeamMember{UserID: userID}, nil).Once()
			},
			write: func(ctx context.Context, repo TeamRepository) error {
				_, err := repo.RemoveMember(ctx, spec.TeamMemberKey{OrgID: orgID, TeamID: teamID, UserID: userID})
				return err
			},
			key: membership,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repo, mocks := newMockedTeamRepo(t)
			tc.expect(mocks.teams)
			commands := captureSettingsCommands(mocks.client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
				return valkeymock.Result(valkeymock.ValkeyInt64(1))
			})

			// when
			err := tc.write(t.Context(), repo)

			// then
			require.NoError(t, err)
			assert.Equal(t, [][]string{{"DEL", tc.key}}, *commands)
		})
	}
}

func TestTeamRename_ReportsAFailedInvalidationWithTheSavedTeam(t *testing.T) {
	// given
	repo, mocks := newMockedTeamRepo(t)
	rename := spec.TeamRename{OrgID: uuid.New(), TeamID: uuid.New(), Name: "Renamed"}
	renamed := &model.Team{ID: rename.TeamID, OrgID: rename.OrgID, Name: "Renamed"}
	mocks.teams.EXPECT().Rename(mock.Anything, rename).Return(renamed, nil).Once()
	cacheErr := errors.New("valkey unreachable")
	captureSettingsCommands(mocks.client, 1, func(_ valkey.Completed) valkey.ValkeyResult {
		return valkeymock.ErrorResult(cacheErr)
	})

	// when
	got, err := repo.Rename(t.Context(), rename)

	// then
	require.ErrorIs(t, err, ErrStaleTenantCache)
	require.ErrorIs(t, err, cacheErr)
	assert.Same(t, renamed, got)
}
