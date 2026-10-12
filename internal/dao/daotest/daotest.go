package daotest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	appdb "retrotide/internal/db"
	"retrotide/internal/db/dbtest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/store"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type (
	UserOpt func(*spec.NewUser)
)

const (
	templateDBName = "retrotide_template"

	TestPasswordHash = "$2a$04$daotest.password.hash.not.a.real.bcrypt.value"

	UnlimitedQuota = 1000
)

var (
	ensureTemplate = sync.OnceValue(createTemplate)
)

func createTemplate() error {
	if err := dbtest.EnsureRunning(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := dbtest.CreateDatabase(ctx, templateDBName); err != nil {
		return err
	}

	templateDB, err := appdb.Open(dbtest.DSNFor(templateDBName))
	if err != nil {
		return fmt.Errorf("open template db: %w", err)
	}

	if err := appdb.Migrate(templateDB); err != nil {
		migrateErr := fmt.Errorf("migrate template db: %w", err)

		if closeErr := templateDB.Close(); closeErr != nil {
			return errors.Join(migrateErr, fmt.Errorf("close template db: %w", closeErr))
		}

		return migrateErr
	}

	if err := templateDB.Close(); err != nil {
		return fmt.Errorf("close template db: %w", err)
	}

	return dbtest.MarkAsTemplate(ctx, templateDBName)
}

func NewDB(t *testing.T) *sql.DB {
	t.Helper()

	require.NoError(t, ensureTemplate())

	return dbtest.NewDatabaseFromTemplate(t, templateDBName)
}

func NewRepos(t *testing.T) *repository.Repositories {
	t.Helper()

	return store.New(NewDB(t), nil)
}

func WithEmail(email string) UserOpt {
	return func(u *spec.NewUser) {
		u.Email = email
	}
}

func WithDisplayName(displayName string) UserOpt {
	return func(u *spec.NewUser) {
		u.DisplayName = displayName
	}
}

func WithVerifiedEmail() UserOpt {
	return func(u *spec.NewUser) {
		u.EmailVerified = true
	}
}

func WithoutPassword() UserOpt {
	return func(u *spec.NewUser) {
		u.PasswordHash = nil
	}
}

func CreateUser(t *testing.T, repos *repository.Repositories, opts ...UserOpt) *model.User {
	t.Helper()

	newUser := spec.NewUser{
		Email:        "user-" + uuid.NewString()[:8] + "@example.com",
		PasswordHash: new(TestPasswordHash),
		DisplayName:  "Test User",
	}
	for _, opt := range opts {
		opt(&newUser)
	}

	created, err := repos.User.Create(t.Context(), newUser)
	require.NoError(t, err)

	return created
}

func CreateOrg(t *testing.T, repos *repository.Repositories, ownerID uuid.UUID) *model.Workspace {
	t.Helper()

	suffix := uuid.NewString()[:8]

	created, err := repos.Organisation.CreateWorkspace(t.Context(), spec.NewWorkspace{
		OwnerID:  ownerID,
		Slug:     "org-" + suffix,
		Name:     "Organisation " + suffix,
		TeamName: "First team",
		MaxOwned: UnlimitedQuota,
	})
	require.NoError(t, err)

	return created
}

func CreateTeam(t *testing.T, repos *repository.Repositories, orgID uuid.UUID) *model.Team {
	t.Helper()

	created, err := repos.Team.Create(t.Context(), spec.NewTeam{OrgID: orgID, Name: "Team " + uuid.NewString()[:8], MaxTeams: UnlimitedQuota})
	require.NoError(t, err)

	return created
}

func AddOrgMember(t *testing.T, repos *repository.Repositories, orgID uuid.UUID, userID uuid.UUID, role model.OrgRole) *model.OrganisationMember {
	t.Helper()

	added, err := repos.Organisation.AddMember(t.Context(), spec.NewOrganisationMember{OrgID: orgID, UserID: userID, Role: role, MaxMembers: UnlimitedQuota})
	require.NoError(t, err)

	return added
}

func AddTeamMember(t *testing.T, repos *repository.Repositories, team *model.Team, userID uuid.UUID, role model.TeamRole) *model.TeamMember {
	t.Helper()

	added, err := repos.Team.AddMember(t.Context(), spec.NewTeamMember{OrgID: team.OrgID, TeamID: team.ID, UserID: userID, Role: role})
	require.NoError(t, err)

	return added
}

func CreateBoard(t *testing.T, repos *repository.Repositories, team model.Team, creatorID uuid.UUID) *model.BoardState {
	t.Helper()

	created, err := repos.Board.CreateBoard(t.Context(), boardSeed(team.OrgID, new(team.ID), creatorID))
	require.NoError(t, err)

	return created
}

func CreateOrganisationBoard(t *testing.T, repos *repository.Repositories, orgID uuid.UUID, creatorID uuid.UUID) *model.BoardState {
	t.Helper()

	created, err := repos.Board.CreateBoard(t.Context(), boardSeed(orgID, nil, creatorID))
	require.NoError(t, err)

	return created
}

func boardSeed(orgID uuid.UUID, teamID *uuid.UUID, creatorID uuid.UUID) spec.NewBoard {
	return spec.NewBoard{
		OrgID:       orgID,
		TeamID:      teamID,
		Title:       "Retro " + uuid.NewString()[:8],
		ShareSlug:   uuid.NewString(),
		CreatedBy:   creatorID,
		Anonymity:   model.AnonymityNamed,
		CardsHidden: true,
		Columns: []spec.BoardColumnSeed{
			{ID: uuid.New(), Title: "Went well", Colour: model.ColourGreen, Position: "a0"},
			{ID: uuid.New(), Title: "To improve", Colour: model.ColourRed, Position: "a1"},
		},
	}
}

func CreateParticipant(t *testing.T, repos *repository.Repositories, boardID uuid.UUID, userID uuid.UUID) *model.BoardParticipant {
	t.Helper()

	created, err := repos.Board.AddParticipant(t.Context(), spec.NewBoardParticipant{BoardID: boardID, UserID: new(userID), Role: model.BoardRoleParticipant})
	require.NoError(t, err)

	return created
}

func CreateGuest(t *testing.T, repos *repository.Repositories) (*model.GuestDevice, string) {
	t.Helper()

	token := uuid.NewString()
	created, err := repos.GuestDevice.Create(t.Context(), spec.NewGuestDevice{Token: token, ExpiresAt: time.Now().Add(time.Hour)})
	require.NoError(t, err)

	return created, token
}

func CreateGuestParticipant(t *testing.T, repos *repository.Repositories, board model.BoardHeader, deviceID uuid.UUID) *model.BoardParticipant {
	t.Helper()

	created, err := repos.Board.AddParticipant(t.Context(), spec.NewBoardParticipant{
		BoardID:       board.ID,
		GuestDeviceID: new(deviceID),
		DisplayName:   "Gil",
		Role:          model.BoardRoleParticipant,
		AccessEpoch:   board.AccessEpoch,
		JoinedVia:     model.JoinPathLink,
	})
	require.NoError(t, err)

	return created
}

func CreateSession(t *testing.T, repos *repository.Repositories, userID uuid.UUID) string {
	t.Helper()

	token := uuid.NewString()
	require.NoError(t, repos.Session.Create(t.Context(), spec.NewSession{Token: token, UserID: userID, ExpiresAt: time.Now().Add(time.Hour)}))

	return token
}
