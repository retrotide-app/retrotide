package dao_test

import (
	"sync"
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
	boardFixture struct {
		repos     *repository.Repositories
		workspace *model.Workspace
		state     *model.BoardState
	}
)

const (
	concurrentCommands = 12
)

func newBoardFixture(t *testing.T) boardFixture {
	t.Helper()

	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	state := daotest.CreateBoard(t, repos, workspace.Team, workspace.Owner.UserID)

	return boardFixture{repos: repos, workspace: workspace, state: state}
}

func (f boardFixture) orgID() uuid.UUID {
	return f.workspace.Organisation.ID
}

func (f boardFixture) boardID() uuid.UUID {
	return f.state.Header.ID
}

func (f boardFixture) column(index int) uuid.UUID {
	return f.state.Columns[index].ID
}

func (f boardFixture) facilitator() uuid.UUID {
	return f.state.Participants[0].ID
}

func (f boardFixture) version(t *testing.T) int64 {
	t.Helper()

	header, err := f.repos.Board.GetHeader(t.Context(), f.boardID())
	require.NoError(t, err)

	return header.Version
}

func (f boardFixture) deleteBoard(t *testing.T, boardID uuid.UUID) {
	t.Helper()

	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET deleted_at = NOW() WHERE id = $1`, boardID)
	require.NoError(t, err)
}

func (f boardFixture) write(t *testing.T) spec.BoardWrite {
	t.Helper()

	return spec.BoardWrite{OrgID: f.orgID(), BoardID: f.boardID(), Version: f.version(t), OpID: uuid.New()}
}

func (f boardFixture) addCard(t *testing.T, columnID uuid.UUID, position string) model.Card {
	t.Helper()

	added, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
		BoardWrite:          f.write(t),
		CardID:              uuid.New(),
		ColumnID:            columnID,
		AuthorParticipantID: new(f.facilitator()),
		Body:                "Card at " + position,
		Position:            position,
	})
	require.NoError(t, err)

	return added.Value
}

func TestBoard_CreateBoardMakesTheBoardItsColumnsAndFacilitator(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)

	// when
	state := daotest.CreateBoard(t, repos, workspace.Team, workspace.Owner.UserID)

	// then
	header, err := repos.Board.GetHeader(t.Context(), state.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, workspace.Organisation.ID, header.OrgID)
	assert.Equal(t, new(workspace.Team.ID), header.TeamID)
	assert.Equal(t, model.LinkAccessOff, header.LinkAccess)
	assert.Equal(t, int64(0), header.Version)
	assert.Equal(t, new(workspace.Owner.UserID), header.CreatedBy)
	assert.Nil(t, header.OrgDeletedAt)
	assert.True(t, header.CardsHidden)
	assert.Equal(t, model.AnonymityNamed, header.Anonymity)
	assert.Equal(t, 3, header.MaxVotes)
	assert.Equal(t, model.SortModeManual, header.SortMode)

	require.Len(t, state.Columns, 2)
	assert.Equal(t, "Went well", state.Columns[0].Title)
	assert.Equal(t, "To improve", state.Columns[1].Title)
	assert.Empty(t, state.Cards)

	require.Len(t, state.Participants, 1)
	assert.Equal(t, new(workspace.Owner.UserID), state.Participants[0].UserID)
	assert.Equal(t, model.BoardRoleFacilitator, state.Participants[0].Role)
	assert.Equal(t, "Test User", state.Participants[0].AccountName)
}

func TestBoard_CreateBoardWritesNothingWhenAColumnIsRefused(t *testing.T) {
	// given a column whose title breaks its length check
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)

	// when
	_, err := repos.Board.CreateBoard(t.Context(), spec.NewBoard{
		OrgID:     workspace.Organisation.ID,
		TeamID:    new(workspace.Team.ID),
		Title:     "Retro",
		ShareSlug: uuid.NewString(),
		CreatedBy: workspace.Owner.UserID,
		Anonymity: model.AnonymityNamed,
		Columns:   []spec.BoardColumnSeed{{ID: uuid.New(), Title: "", Colour: model.ColourGrey, Position: "a0"}},
	})

	// then
	require.Error(t, err)
	assert.Zero(t, countRows(t, repos, "boards"))
	assert.Zero(t, countRows(t, repos, "board_participants"))
}

func TestBoard_ShareSlugsAreUnique(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	_, err := f.repos.Board.Create(t.Context(), spec.NewBoardRecord{
		OrgID:      f.orgID(),
		TeamID:     new(f.workspace.Team.ID),
		Title:      "Copy",
		ShareSlug:  f.state.Header.ShareSlug,
		LinkAccess: model.LinkAccessOff,
		Anonymity:  model.AnonymityNamed,
	})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
}

func TestBoard_GetHeaderCarriesTheDeletionOfItsOrganisation(t *testing.T) {
	// given
	f := newBoardFixture(t)
	_, err := f.repos.Organisation.SoftDelete(t.Context(), f.orgID())
	require.NoError(t, err)

	// when
	header, headerErr := f.repos.Board.GetHeader(t.Context(), f.boardID())
	_, missingErr := f.repos.Board.GetHeader(t.Context(), uuid.New())

	// then
	require.NoError(t, headerErr)
	require.ErrorIs(t, missingErr, dao.ErrNotFound)
	assert.NotNil(t, header.OrgDeletedAt)
	assert.True(t, header.Gone())
	assert.Equal(t, f.boardID(), header.ID)
}

func TestBoard_ListForTeamShowsLiveBoardsNewestFirstWithTheirSettings(t *testing.T) {
	// given three boards, the oldest deleted and the newest anonymous
	f := newBoardFixture(t)
	second := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	third := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET anonymity = 'anonymous' WHERE id = $1`, third.Header.ID)
	require.NoError(t, err)
	f.deleteBoard(t, f.boardID())
	otherTeam := daotest.CreateTeam(t, f.repos, f.orgID())
	daotest.CreateBoard(t, f.repos, *otherTeam, f.workspace.Owner.UserID)

	// when
	listed, err := f.repos.Board.ListForTeam(t.Context(), spec.TeamKey{OrgID: f.orgID(), TeamID: f.workspace.Team.ID})

	// then
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, third.Header.ID, listed[0].ID)
	assert.True(t, listed[0].Anonymous())
	assert.Equal(t, second.Header.ID, listed[1].ID)
	assert.False(t, listed[1].Anonymous())
	assert.Equal(t, second.Header.Title, listed[1].Title)
}

func TestBoard_ListForOrganisationShowsLiveBoardsOfEveryTeamNewestFirst(t *testing.T) {
	// given boards on two teams, the oldest deleted, one on a team since archived, and the newest archived and anonymous
	f := newBoardFixture(t)
	otherTeam := daotest.CreateTeam(t, f.repos, f.orgID())
	onOtherTeam := daotest.CreateBoard(t, f.repos, *otherTeam, f.workspace.Owner.UserID)
	_, err := f.repos.Team.Archive(t.Context(), spec.TeamKey{OrgID: f.orgID(), TeamID: otherTeam.ID})
	require.NoError(t, err)
	newest := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
	_, err = f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET anonymity = 'anonymous', template_key = 'blank', archived_at = NOW() WHERE id = $1`, newest.Header.ID)
	require.NoError(t, err)
	f.deleteBoard(t, f.boardID())

	// when
	listed, err := f.repos.Board.ListForOrganisation(t.Context(), f.orgID())

	// then
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, newest.Header.ID, listed[0].ID)
	assert.Equal(t, new(f.workspace.Team.ID), listed[0].TeamID)
	assert.True(t, listed[0].Anonymous())
	assert.Equal(t, "blank", listed[0].TemplateKey)
	assert.True(t, listed[0].Archived())
	assert.Equal(t, onOtherTeam.Header.ID, listed[1].ID)
	assert.Equal(t, new(otherTeam.ID), listed[1].TeamID)
	assert.Equal(t, onOtherTeam.Header.Title, listed[1].Title)
	assert.False(t, listed[1].Archived())

	var ownerName string
	require.NoError(t, f.repos.DB().QueryRowContext(t.Context(), `SELECT display_name FROM users WHERE id = $1`, f.workspace.Owner.UserID).Scan(&ownerName))
	assert.Equal(t, ownerName, listed[0].CreatorName)
	assert.Equal(t, ownerName, listed[1].CreatorName)
}

func TestBoard_CreateBoardWithNoTeamBelongsToTheOrganisation(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	workspace := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)

	// when
	state := daotest.CreateOrganisationBoard(t, repos, workspace.Organisation.ID, workspace.Owner.UserID)

	// then
	header, err := repos.Board.GetHeader(t.Context(), state.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, workspace.Organisation.ID, header.OrgID)
	assert.Nil(t, header.TeamID)
	assert.Nil(t, state.Header.TeamID)
	require.Len(t, state.Participants, 1)
	assert.Equal(t, model.BoardRoleFacilitator, state.Participants[0].Role)
}

func TestBoard_ListsTreatOrganisationBoardsAsTheOrganisationsOwn(t *testing.T) {
	// given a team board and an organisation board
	f := newBoardFixture(t)
	organisationBoard := daotest.CreateOrganisationBoard(t, f.repos, f.orgID(), f.workspace.Owner.UserID)

	// when
	byOrganisation, orgErr := f.repos.Board.ListForOrganisation(t.Context(), f.orgID())
	byTeam, teamErr := f.repos.Board.ListForTeam(t.Context(), spec.TeamKey{OrgID: f.orgID(), TeamID: f.workspace.Team.ID})

	// then the organisation's list holds both, newest first, and the team's list only its own
	require.NoError(t, orgErr)
	require.Len(t, byOrganisation, 2)
	assert.Equal(t, organisationBoard.Header.ID, byOrganisation[0].ID)
	assert.Nil(t, byOrganisation[0].TeamID)
	assert.Equal(t, f.boardID(), byOrganisation[1].ID)
	assert.Equal(t, new(f.workspace.Team.ID), byOrganisation[1].TeamID)

	require.NoError(t, teamErr)
	require.Len(t, byTeam, 1)
	assert.Equal(t, f.boardID(), byTeam[0].ID)
}

func TestBoard_BumpVersionExpectsTheVersionItWasRead(t *testing.T) {
	// given
	f := newBoardFixture(t)
	read := spec.BoardWrite{OrgID: f.orgID(), BoardID: f.boardID(), Version: 0}

	// when
	bumped, err := f.repos.Board.BumpVersion(t.Context(), read)
	_, staleErr := f.repos.Board.BumpVersion(t.Context(), read)
	_, missingErr := f.repos.Board.BumpVersion(t.Context(), spec.BoardWrite{OrgID: f.orgID(), BoardID: uuid.New(), Version: 0})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), bumped)
	require.ErrorIs(t, staleErr, dao.ErrVersionConflict)
	require.ErrorIs(t, missingErr, dao.ErrNotFound)
	assert.Equal(t, int64(1), f.version(t))
}

func TestBoard_BumpVersionRefusesADeletedBoard(t *testing.T) {
	// given
	f := newBoardFixture(t)
	f.deleteBoard(t, f.boardID())

	// when
	_, bumpErr := f.repos.Board.BumpVersion(t.Context(), spec.BoardWrite{OrgID: f.orgID(), BoardID: f.boardID(), Version: f.version(t)})

	// then
	require.ErrorIs(t, bumpErr, dao.ErrNotFound)
}

func TestBoard_ConcurrentCommandsFromOneVersionCommitOnce(t *testing.T) {
	// given every command read the board at the same version
	f := newBoardFixture(t)
	read := f.write(t)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for range concurrentCommands {
		wg.Go(func() {
			_, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
				OrgID: read.OrgID, BoardID: read.BoardID, Version: read.Version, OpID: uuid.New(),
				CardID:   uuid.New(),
				ColumnID: f.column(0),
				Body:     "Racing",
				Position: "a0",
			})

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

	// then the conditional version bump lets exactly one through
	assert.Equal(t, 1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrVersionConflict)
	}
	assert.Equal(t, read.Version+1, f.version(t))
	assert.Equal(t, 1, countWhere(t, f.repos, `SELECT COUNT(*) FROM cards WHERE board_id = $1`, f.boardID()))
	assert.Equal(t, 1, countWhere(t, f.repos, `SELECT COUNT(*) FROM board_ops WHERE board_id = $1`, f.boardID()))
}

func TestBoard_ADuplicateOpRollsTheWholeCommandBack(t *testing.T) {
	// given a command that committed
	f := newBoardFixture(t)
	first := f.write(t)
	created, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{BoardWrite: first, CardID: uuid.New(), ColumnID: f.column(0), Body: "First", Position: "a0"})
	require.NoError(t, err)

	// when it is retried at the version that followed it
	retry := first
	retry.Version = created.Version
	_, dupErr := f.repos.Card.AddCard(t.Context(), spec.CardCreate{BoardWrite: retry, CardID: uuid.New(), ColumnID: f.column(0), Body: "Retry", Position: "a1"})
	op, opErr := f.repos.Board.GetOp(t.Context(), spec.BoardOpKey{BoardID: f.boardID(), OpID: first.OpID})

	// then nothing of the retry stays, the version bump included
	require.ErrorIs(t, dupErr, dao.ErrDuplicateOp)
	require.NoError(t, opErr)
	assert.Equal(t, created.Version, op.Version)
	assert.Equal(t, created.Version, f.version(t))
	assert.Equal(t, 1, countWhere(t, f.repos, `SELECT COUNT(*) FROM cards WHERE board_id = $1`, f.boardID()))
}

func TestBoard_AFailedCommandLeavesTheVersionUnchanged(t *testing.T) {
	// given
	f := newBoardFixture(t)
	read := f.write(t)

	// when the card names a column the board does not have
	_, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{BoardWrite: read, CardID: uuid.New(), ColumnID: uuid.New(), Body: "Lost", Position: "a0"})
	_, opErr := f.repos.Board.GetOp(t.Context(), spec.BoardOpKey{BoardID: f.boardID(), OpID: read.OpID})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	require.ErrorIs(t, opErr, dao.ErrNotFound)
	assert.Equal(t, read.Version, f.version(t))
}

func TestBoard_JoinParticipantAddsOneRowPerUser(t *testing.T) {
	// given
	f := newBoardFixture(t)
	member := daotest.CreateUser(t, f.repos, daotest.WithDisplayName("Sam"))
	daotest.AddOrgMember(t, f.repos, f.orgID(), member.ID, model.OrgRoleMember)

	// when
	joined, joinErr := f.repos.Board.JoinParticipant(t.Context(), spec.BoardParticipantJoin{BoardWrite: f.write(t), UserID: member.ID, Role: model.BoardRoleParticipant})
	_, againErr := f.repos.Board.JoinParticipant(t.Context(), spec.BoardParticipantJoin{BoardWrite: f.write(t), UserID: member.ID, Role: model.BoardRoleParticipant})
	byUser, byUserErr := f.repos.Board.GetParticipantForUser(t.Context(), spec.BoardParticipantUser{BoardID: f.boardID(), UserID: member.ID})
	_, strangerErr := f.repos.Board.GetParticipantForUser(t.Context(), spec.BoardParticipantUser{BoardID: f.boardID(), UserID: uuid.New()})
	listed, listErr := f.repos.Board.ListParticipants(t.Context(), f.boardID())

	// then
	require.NoError(t, joinErr)
	assert.Equal(t, int64(1), joined.Version)
	assert.Equal(t, model.BoardRoleParticipant, joined.Value.Role)
	assert.Equal(t, model.JoinPathMembership, joined.Value.JoinedVia)
	assert.False(t, joined.Value.JoinedThroughLink())
	require.ErrorIs(t, againErr, dao.ErrConflict)
	assert.Equal(t, int64(1), f.version(t))

	require.NoError(t, byUserErr)
	assert.Equal(t, joined.Value, *byUser)
	require.ErrorIs(t, strangerErr, dao.ErrNotFound)

	require.NoError(t, listErr)
	require.Len(t, listed, 2)
	assert.Equal(t, f.facilitator(), listed[0].ID)
	assert.Equal(t, joined.Value.ID, listed[1].ID)
	assert.Equal(t, "Sam", listed[1].AccountName)
}

func TestBoard_AParticipantWhoseAccountIsDeletedStaysWithoutIt(t *testing.T) {
	// given
	f := newBoardFixture(t)
	member := daotest.CreateUser(t, f.repos)
	participant := daotest.CreateParticipant(t, f.repos, f.boardID(), member.ID)

	// when
	_, err := f.repos.DB().ExecContext(t.Context(), `DELETE FROM users WHERE id = $1`, member.ID)

	// then
	require.NoError(t, err)
	listed, listErr := f.repos.Board.ListParticipants(t.Context(), f.boardID())
	require.NoError(t, listErr)
	require.Len(t, listed, 2)
	assert.Equal(t, participant.ID, listed[1].ID)
	assert.Nil(t, listed[1].UserID)
}
