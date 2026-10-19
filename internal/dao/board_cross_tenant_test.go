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

type (
	boardTenants struct {
		tenants

		boardA *model.BoardState
		boardB *model.BoardState
		cardA  model.Card
	}
)

func newBoardTenants(t *testing.T) boardTenants {
	t.Helper()

	tt := newTenants(t)
	boardA := daotest.CreateBoard(t, tt.repos, tt.a.Team, tt.a.Owner.UserID)
	boardB := daotest.CreateBoard(t, tt.repos, tt.b.Team, tt.b.Owner.UserID)

	added, err := tt.repos.Card.AddCard(t.Context(), spec.CardCreate{
		OrgID: tt.a.Organisation.ID, BoardID: boardA.Header.ID, Version: boardA.Header.Version, OpID: uuid.New(),
		CardID:              uuid.New(),
		ColumnID:            boardA.Columns[0].ID,
		AuthorParticipantID: new(boardA.Participants[0].ID),
		Body:                "SECRET-TEXT-9f2c",
		Position:            "a0",
	})
	require.NoError(t, err)

	return boardTenants{tenants: tt, boardA: boardA, boardB: boardB, cardA: added.Value}
}

func (bt boardTenants) boardAThroughB() spec.BoardWrite {
	return spec.BoardWrite{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, Version: 1, OpID: uuid.New()}
}

func (bt boardTenants) boardAUnchanged(t *testing.T) {
	t.Helper()

	snapshot, err := bt.repos.Board.GetSnapshot(t.Context(), spec.BoardKey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), snapshot.Header.Version)
	assert.Equal(t, bt.boardA.Header.Title, snapshot.Header.Title)
	assert.False(t, snapshot.Header.Archived())
	assert.False(t, snapshot.Header.Deleted())
	assert.Empty(t, snapshot.Header.Board.Context)
	assert.Equal(t, bt.boardA.Columns, snapshot.Columns)
	assert.Equal(t, []model.Card{bt.cardA}, snapshot.Cards)
	assert.Len(t, snapshot.Participants, 1)
}

func TestBoard_CrossTenantVersionsAndTeamsAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given
	bt := newBoardTenants(t)

	// when
	_, bumpErr := bt.repos.Board.BumpVersion(t.Context(), bt.boardAThroughB())
	_, crossTeamErr := bt.repos.Board.Create(t.Context(), spec.NewBoardRecord{
		OrgID:      bt.b.Organisation.ID,
		TeamID:     new(bt.a.Team.ID),
		Title:      "Planted",
		ShareSlug:  uuid.NewString(),
		LinkAccess: model.LinkAccessOff,
		Anonymity:  model.AnonymityNamed,
	})

	// then a board of organisation B can never sit in a team of organisation A
	require.ErrorIs(t, bumpErr, dao.ErrNotFound)
	require.ErrorIs(t, crossTeamErr, dao.ErrNotFound)
	bt.boardAUnchanged(t)
}

func TestBoard_CrossTenantBoardsAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given
	bt := newBoardTenants(t)

	// when
	_, getErr := bt.repos.Board.GetByID(t.Context(), spec.BoardKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID})
	listed, listErr := bt.repos.Board.ListForTeam(t.Context(), spec.TeamKey{OrgID: bt.b.Organisation.ID, TeamID: bt.a.Team.ID})
	listedB, listBErr := bt.repos.Board.ListForOrganisation(t.Context(), bt.b.Organisation.ID)
	_, snapshotErr := bt.repos.Board.GetSnapshot(t.Context(), spec.BoardKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID})
	_, setErr := bt.repos.Board.SetContext(t.Context(), spec.BoardContextChange{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, Context: "Planted"})
	_, writeErr := bt.repos.Board.SetBoardContext(t.Context(), spec.BoardContextWrite{BoardWrite: bt.boardAThroughB(), Context: "Planted"})

	// then
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, snapshotErr, dao.ErrNotFound)
	require.ErrorIs(t, setErr, dao.ErrNotFound)
	require.ErrorIs(t, writeErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	assert.Empty(t, listed)
	require.NoError(t, listBErr)
	require.Len(t, listedB, 1)
	assert.Equal(t, bt.boardB.Header.ID, listedB[0].ID)
	bt.boardAUnchanged(t)
}

func TestBoard_CrossTenantOrganisationBoardsAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given an organisation board in A, which names no team
	bt := newBoardTenants(t)
	organisationBoardA := daotest.CreateOrganisationBoard(t, bt.repos, bt.a.Organisation.ID, bt.a.Owner.UserID)
	throughB := spec.BoardKey{OrgID: bt.b.Organisation.ID, BoardID: organisationBoardA.Header.ID}
	writeThroughB := spec.BoardWrite{OrgID: bt.b.Organisation.ID, BoardID: organisationBoardA.Header.ID, Version: 0, OpID: uuid.New()}

	// when
	_, getErr := bt.repos.Board.GetByID(t.Context(), throughB)
	_, snapshotErr := bt.repos.Board.GetSnapshot(t.Context(), throughB)
	_, setErr := bt.repos.Board.SetContext(t.Context(), spec.BoardContextChange{OrgID: bt.b.Organisation.ID, BoardID: organisationBoardA.Header.ID, Context: "Planted"})
	_, bumpErr := bt.repos.Board.BumpVersion(t.Context(), writeThroughB)
	listedB, listBErr := bt.repos.Board.ListForOrganisation(t.Context(), bt.b.Organisation.ID)
	listedA, listAErr := bt.repos.Board.ListForOrganisation(t.Context(), bt.a.Organisation.ID)

	// then B never reaches it, and A lists it beside its team board
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, snapshotErr, dao.ErrNotFound)
	require.ErrorIs(t, setErr, dao.ErrNotFound)
	require.ErrorIs(t, bumpErr, dao.ErrNotFound)
	require.NoError(t, listBErr)
	require.Len(t, listedB, 1)
	assert.Equal(t, bt.boardB.Header.ID, listedB[0].ID)
	require.NoError(t, listAErr)
	require.Len(t, listedA, 2)
	assert.Equal(t, organisationBoardA.Header.ID, listedA[0].ID)
	assert.Nil(t, listedA[0].TeamID)

	unchanged, err := bt.repos.Board.GetByID(t.Context(), spec.BoardKey{OrgID: bt.a.Organisation.ID, BoardID: organisationBoardA.Header.ID})
	require.NoError(t, err)
	assert.Empty(t, unchanged.Board.Context)
	assert.Equal(t, int64(0), unchanged.Version)
}

func TestBoard_CrossTenantFacilitationStateAndTimerLocksAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given board A with a running timer that locks it when it ends
	bt := newBoardTenants(t)
	endsAt := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	_, err := bt.repos.DB().ExecContext(t.Context(), `UPDATE boards SET timer_auto_lock = TRUE, timer_duration_ms = 60000, timer_ends_at = $2 WHERE id = $1`, bt.boardA.Header.ID, endsAt)
	require.NoError(t, err)
	columnA := bt.boardA.Columns[0].ID

	// when organisation B tries to hide A's column, spotlight A's card and lock A at its timer's end
	_, stateErr := bt.repos.Board.SetFacilitation(t.Context(), spec.BoardFacilitationChange{
		OrgID:           bt.b.Organisation.ID,
		BoardID:         bt.boardA.Header.ID,
		HiddenColumnIDs: []uuid.UUID{columnA},
		Spotlight:       model.BoardSpotlight{PresentingCardID: new(bt.cardA.ID)},
	})
	_, writeErr := bt.repos.Board.SetFacilitationState(t.Context(), spec.BoardFacilitationWrite{BoardWrite: bt.boardAThroughB(), HiddenColumnIDs: []uuid.UUID{columnA}})
	_, lockErr := bt.repos.Board.LockAtTimerEnd(t.Context(), spec.BoardTimerLock{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, EndsAt: endsAt})
	_, lockWriteErr := bt.repos.Board.LockBoardAtTimerEnd(t.Context(), spec.BoardTimerLockWrite{BoardWrite: bt.boardAThroughB(), EndsAt: endsAt})

	// then
	require.ErrorIs(t, stateErr, dao.ErrNotFound)
	require.ErrorIs(t, writeErr, dao.ErrNotFound)
	require.ErrorIs(t, lockErr, dao.ErrTimerMoved)
	require.ErrorIs(t, lockWriteErr, dao.ErrNotFound)
	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Empty(t, header.HiddenColumnIDs)
	assert.Nil(t, header.Spotlight.PresentingCardID)
	assert.False(t, header.Locked())
	bt.boardAUnchanged(t)
}

func TestBoard_CrossTenantParticipantsAndOpsAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given an op recorded on board A
	bt := newBoardTenants(t)
	ops := countWhere(t, bt.repos, `SELECT COUNT(*) FROM board_ops WHERE board_id = $1`, bt.boardA.Header.ID)
	require.Equal(t, 1, ops)
	var opID uuid.UUID
	require.NoError(t, bt.repos.DB().QueryRowContext(t.Context(), `SELECT op_id FROM board_ops WHERE board_id = $1`, bt.boardA.Header.ID).Scan(&opID))

	// when
	_, userErr := bt.repos.Board.GetParticipantForUser(t.Context(), spec.BoardParticipantUser{BoardID: bt.boardB.Header.ID, UserID: bt.a.Owner.UserID})
	listedB, listErr := bt.repos.Board.ListParticipants(t.Context(), bt.boardB.Header.ID)
	_, opErr := bt.repos.Board.GetOp(t.Context(), spec.BoardOpKey{BoardID: bt.boardB.Header.ID, OpID: opID})
	_, joinErr := bt.repos.Board.JoinParticipant(t.Context(), spec.BoardParticipantJoin{BoardWrite: bt.boardAThroughB(), UserID: bt.alice.ID, Role: model.BoardRoleFacilitator})

	// then
	require.ErrorIs(t, userErr, dao.ErrNotFound)
	require.ErrorIs(t, opErr, dao.ErrNotFound)
	require.ErrorIs(t, joinErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	require.Len(t, listedB, 1)
	assert.Equal(t, bt.boardB.Participants[0].ID, listedB[0].ID)
	bt.boardAUnchanged(t)
}

func TestColumn_CrossTenantColumnsAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given
	bt := newBoardTenants(t)
	columnA := bt.boardA.Columns[0]
	throughB := spec.BoardColumnKey{BoardID: bt.boardB.Header.ID, ColumnID: columnA.ID}

	// when
	_, getErr := bt.repos.BoardColumn.GetByID(t.Context(), throughB)
	_, updateErr := bt.repos.BoardColumn.Update(t.Context(), spec.BoardColumnUpdate{BoardID: bt.boardB.Header.ID, ColumnID: columnA.ID, Title: "Taken", Colour: model.ColourRed})
	_, moveErr := bt.repos.BoardColumn.Move(t.Context(), spec.BoardColumnMove{BoardID: bt.boardB.Header.ID, ColumnID: columnA.ID, Position: "z"})
	_, deleteErr := bt.repos.BoardColumn.Delete(t.Context(), throughB)
	_, editErr := bt.repos.BoardColumn.EditColumn(t.Context(), spec.ColumnEdit{BoardWrite: bt.boardAThroughB(), ColumnID: columnA.ID, Title: "Taken", Colour: model.ColourRed})
	listedB, listErr := bt.repos.BoardColumn.List(t.Context(), bt.boardB.Header.ID)

	// then
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, updateErr, dao.ErrNotFound)
	require.ErrorIs(t, moveErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	require.ErrorIs(t, editErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	assert.Equal(t, columnIDs(bt.boardB.Columns), columnIDs(listedB))
	bt.boardAUnchanged(t)
}

func TestCard_CrossTenantCardsAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given
	bt := newBoardTenants(t)
	throughB := spec.CardRef{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID}

	// when
	_, getErr := bt.repos.Card.GetByID(t.Context(), throughB)
	_, editErr := bt.repos.Card.Edit(t.Context(), spec.CardEdit{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, Body: "Rewritten"})
	_, colourErr := bt.repos.Card.SetColour(t.Context(), spec.CardColourChange{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, Colour: new(model.ColourRed)})
	_, moveErr := bt.repos.Card.Move(t.Context(), spec.CardMove{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, ColumnID: bt.boardB.Columns[0].ID, Position: "a0"})
	_, deleteErr := bt.repos.Card.Delete(t.Context(), throughB)
	cleared, clearErr := bt.repos.Card.ClearColumn(t.Context(), spec.BoardColumnKey{BoardID: bt.boardB.Header.ID, ColumnID: bt.cardA.ColumnID})
	listedB, listErr := bt.repos.Card.List(t.Context(), bt.boardB.Header.ID)
	inColumn, columnErr := bt.repos.Card.ListInColumn(t.Context(), spec.BoardColumnKey{BoardID: bt.boardB.Header.ID, ColumnID: bt.cardA.ColumnID})
	_, plantErr := bt.repos.Card.Create(t.Context(), spec.NewCard{ID: uuid.New(), BoardID: bt.boardB.Header.ID, ColumnID: bt.cardA.ColumnID, Body: "Planted", Position: "a0"})
	_, moveInErr := bt.repos.Card.Move(t.Context(), spec.CardMove{BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, ColumnID: bt.boardB.Columns[0].ID, Position: "a0"})

	// then a card of board B can never sit in a column of board A, nor move into one
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, editErr, dao.ErrNotFound)
	require.ErrorIs(t, colourErr, dao.ErrNotFound)
	require.ErrorIs(t, moveErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	require.ErrorIs(t, plantErr, dao.ErrNotFound)
	require.ErrorIs(t, moveInErr, dao.ErrNotFound)
	require.NoError(t, clearErr)
	assert.Zero(t, cleared)
	require.NoError(t, listErr)
	assert.Empty(t, listedB)
	require.NoError(t, columnErr)
	assert.Empty(t, inColumn)
	bt.boardAUnchanged(t)
}

func (bt boardTenants) mediaOn(t *testing.T, orgID uuid.UUID, board *model.BoardState, kind model.MediaKind) *model.Media {
	t.Helper()

	created, err := bt.repos.Media.Create(t.Context(), spec.NewMedia{OrgID: orgID, BoardID: board.Header.ID, Kind: kind, Width: 4, Height: 3, Data: []byte("RIFF-webp-bytes"), MaxStoredBytes: testMediaStorage})
	require.NoError(t, err)

	return created
}

func TestBoard_CrossTenantReactionsAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given a reaction on board A's card
	bt := newBoardTenants(t)
	seatA := bt.boardA.Participants[0].ID
	reacted := spec.CardReactionKey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, ParticipantID: seatA, Reaction: model.ReactionFire}
	_, err := bt.repos.CardReaction.Add(t.Context(), reacted)
	require.NoError(t, err)

	// when
	_, plantOrgErr := bt.repos.CardReaction.Add(t.Context(), spec.CardReactionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, ParticipantID: seatA, Reaction: model.ReactionStar})
	_, plantBoardErr := bt.repos.CardReaction.Add(t.Context(), spec.CardReactionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, ParticipantID: bt.boardB.Participants[0].ID, Reaction: model.ReactionStar})
	removed, removeErr := bt.repos.CardReaction.Remove(t.Context(), spec.CardReactionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, ParticipantID: seatA, Reaction: model.ReactionFire})
	forCard, forCardErr := bt.repos.CardReaction.ListForCard(t.Context(), spec.CardRef{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID})
	listedB, listErr := bt.repos.CardReaction.List(t.Context(), bt.boardB.Header.ID)
	handed, handErr := bt.repos.CardReaction.HandOver(t.Context(), spec.ParticipantHandover{BoardID: bt.boardB.Header.ID, FromParticipantID: seatA, ToParticipantID: bt.boardB.Participants[0].ID})
	severed, severErr := bt.repos.CardReaction.Sever(t.Context(), bt.boardB.Header.ID)

	// then a reaction never lands on another tenant's board or card, and B's reads and writes never touch A's
	require.ErrorIs(t, plantOrgErr, dao.ErrNotFound)
	require.ErrorIs(t, plantBoardErr, dao.ErrNotFound)
	require.NoError(t, removeErr)
	assert.False(t, removed)
	require.NoError(t, forCardErr)
	assert.Empty(t, forCard)
	require.NoError(t, listErr)
	assert.Empty(t, listedB)
	require.NoError(t, handErr)
	assert.Zero(t, handed)
	require.NoError(t, severErr)
	assert.Zero(t, severed)

	listedA, err := bt.repos.CardReaction.List(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, []model.CardReaction{{BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, ParticipantID: new(seatA), Reaction: model.ReactionFire}}, listedA)
	bt.boardAUnchanged(t)
}

func TestBoard_CrossTenantMediaIsOnlyReachedThroughItsBoard(t *testing.T) {
	// given an image on board A and another on board B
	bt := newBoardTenants(t)
	mediaA := bt.mediaOn(t, bt.a.Organisation.ID, bt.boardA, model.MediaKindCard)
	mediaB := bt.mediaOn(t, bt.b.Organisation.ID, bt.boardB, model.MediaKindBackground)

	// when
	_, plantErr := bt.repos.Media.Create(t.Context(), spec.NewMedia{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, Kind: model.MediaKindCard, Width: 1, Height: 1, Data: []byte("x"), MaxStoredBytes: testMediaStorage})
	_, getErr := bt.repos.Media.Get(t.Context(), spec.MediaKey{BoardID: bt.boardB.Header.ID, MediaID: mediaA.ID})
	_, openErr := bt.repos.Media.Open(t.Context(), spec.MediaKey{BoardID: bt.boardB.Header.ID, MediaID: mediaA.ID})
	_, attachErr := bt.repos.Card.SetImage(t.Context(), spec.CardImageChange{BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, MediaID: new(mediaB.ID)})
	_, imageThroughBErr := bt.repos.Card.SetImage(t.Context(), spec.CardImageChange{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, MediaID: new(mediaB.ID)})
	_, backgroundErr := bt.repos.Board.SetBackground(t.Context(), spec.BoardBackgroundChange{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, MediaID: new(mediaB.ID)})
	_, backgroundThroughBErr := bt.repos.Board.SetBackground(t.Context(), spec.BoardBackgroundChange{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, MediaID: new(mediaA.ID)})
	_, pinErr := bt.repos.Card.SetPinned(t.Context(), spec.CardPinChange{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, Pinned: true})

	// then an image of one board can never be shown on another
	require.ErrorIs(t, plantErr, dao.ErrNotFound)
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, openErr, dao.ErrNotFound)
	require.ErrorIs(t, attachErr, dao.ErrNotFound)
	require.ErrorIs(t, imageThroughBErr, dao.ErrNotFound)
	require.ErrorIs(t, backgroundErr, dao.ErrNotFound)
	require.ErrorIs(t, backgroundThroughBErr, dao.ErrNotFound)
	require.ErrorIs(t, pinErr, dao.ErrNotFound)

	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Nil(t, header.BackgroundMediaID)
	bt.boardAUnchanged(t)
}

func TestBoard_CrossTenantSurveysAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given an open survey on board A with one answer
	bt := newBoardTenants(t)
	seatA := bt.boardA.Participants[0].ID
	surveyID := uuid.New()
	questionID := uuid.New()
	_, err := bt.repos.Survey.Create(t.Context(), spec.NewSurvey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, ID: surveyID, Title: "Pulse"})
	require.NoError(t, err)
	_, err = bt.repos.Survey.AddQuestion(t.Context(), spec.NewSurveyQuestion{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, SurveyID: surveyID, ID: questionID, Kind: model.QuestionScale, Prompt: "Mood?", ScaleMin: new(1), ScaleMax: new(5)})
	require.NoError(t, err)
	require.NoError(t, bt.repos.Survey.AddResponse(t.Context(), spec.NewSurveyResponse{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, SurveyID: surveyID, QuestionID: questionID, ParticipantID: seatA, Answer: []int{4}}))
	throughB := spec.SurveyKey{BoardID: bt.boardB.Header.ID, SurveyID: surveyID}

	// when
	_, plantErr := bt.repos.Survey.Create(t.Context(), spec.NewSurvey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, ID: uuid.New(), Title: "Planted"})
	_, questionErr := bt.repos.Survey.AddQuestion(t.Context(), spec.NewSurveyQuestion{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, SurveyID: surveyID, ID: uuid.New(), Position: 1, Kind: model.QuestionScale, Prompt: "Planted?", ScaleMin: new(0), ScaleMax: new(1)})
	_, getErr := bt.repos.Survey.Get(t.Context(), throughB)
	_, closeErr := bt.repos.Survey.Close(t.Context(), throughB)
	deleteErr := bt.repos.Survey.Delete(t.Context(), throughB)
	answerErr := bt.repos.Survey.AddResponse(t.Context(), spec.NewSurveyResponse{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, SurveyID: surveyID, QuestionID: questionID, ParticipantID: bt.boardB.Participants[0].ID, Answer: []int{1}})
	cleared, clearErr := bt.repos.Survey.DeleteResponses(t.Context(), spec.SurveyParticipantKey{BoardID: bt.boardB.Header.ID, SurveyID: surveyID, ParticipantID: seatA})
	listedB, listErr := bt.repos.Survey.List(t.Context(), bt.boardB.Header.ID)
	answersB, answersErr := bt.repos.Survey.ListResponses(t.Context(), bt.boardB.Header.ID)
	forSurvey, forSurveyErr := bt.repos.Survey.ListResponsesForSurvey(t.Context(), throughB)
	counted, countErr := bt.repos.Survey.Count(t.Context(), bt.boardB.Header.ID)
	closedB, closeOpenErr := bt.repos.Survey.CloseOpen(t.Context(), bt.boardB.Header.ID)
	severed, severErr := bt.repos.Survey.SeverResponses(t.Context(), bt.boardB.Header.ID)
	handed, handErr := bt.repos.Survey.HandOverResponses(t.Context(), spec.ParticipantHandover{BoardID: bt.boardB.Header.ID, FromParticipantID: seatA, ToParticipantID: bt.boardB.Participants[0].ID})

	// then
	require.ErrorIs(t, plantErr, dao.ErrNotFound)
	require.ErrorIs(t, questionErr, dao.ErrNotFound)
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, closeErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	require.ErrorIs(t, answerErr, dao.ErrNotFound)
	require.NoError(t, clearErr)
	assert.Zero(t, cleared)
	require.NoError(t, listErr)
	assert.Empty(t, listedB)
	require.NoError(t, answersErr)
	assert.Empty(t, answersB)
	require.NoError(t, forSurveyErr)
	assert.Empty(t, forSurvey)
	require.NoError(t, countErr)
	assert.Zero(t, counted)
	require.NoError(t, closeOpenErr)
	assert.Zero(t, closedB)
	require.NoError(t, severErr)
	assert.Zero(t, severed)
	require.NoError(t, handErr)
	assert.Zero(t, handed)

	surveyA, err := bt.repos.Survey.Get(t.Context(), spec.SurveyKey{BoardID: bt.boardA.Header.ID, SurveyID: surveyID})
	require.NoError(t, err)
	assert.Equal(t, model.SurveyOpen, surveyA.Status)
	require.Len(t, surveyA.Questions, 1)
	answersA, err := bt.repos.Survey.ListResponses(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	require.Len(t, answersA, 1)
	assert.Equal(t, new(seatA), answersA[0].ParticipantID)
	bt.boardAUnchanged(t)
}

func TestBoard_CrossTenantPinsArchivesDeletesAndMovesAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given
	bt := newBoardTenants(t)
	ownerB := bt.b.Owner.UserID

	// when
	pinOrgErr := bt.repos.Board.Pin(t.Context(), spec.BoardPinKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, UserID: ownerB})
	pinMemberErr := bt.repos.Board.Pin(t.Context(), spec.BoardPinKey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, UserID: ownerB})
	unpinErr := bt.repos.Board.Unpin(t.Context(), spec.BoardPinKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, UserID: bt.a.Owner.UserID})
	pinnedB, pinnedErr := bt.repos.Board.ListPinned(t.Context(), spec.OrganisationMemberKey{OrgID: bt.b.Organisation.ID, UserID: ownerB})
	_, archiveErr := bt.repos.Board.SetArchived(t.Context(), spec.BoardArchiveChange{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, Archived: true})
	_, deleteErr := bt.repos.Board.SoftDelete(t.Context(), spec.BoardKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID})
	_, moveErr := bt.repos.Board.SetTeam(t.Context(), spec.BoardTeamChange{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, TeamID: new(bt.b.Team.ID)})
	_, crossTeamErr := bt.repos.Board.SetTeam(t.Context(), spec.BoardTeamChange{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, TeamID: new(bt.b.Team.ID)})
	_, archiveWriteErr := bt.repos.Board.ArchiveBoard(t.Context(), spec.BoardArchiveWrite{BoardWrite: bt.boardAThroughB(), Archived: true})
	_, deleteWriteErr := bt.repos.Board.DeleteBoard(t.Context(), bt.boardAThroughB())
	_, moveWriteErr := bt.repos.Board.MoveBoard(t.Context(), spec.BoardTeamWrite{BoardWrite: bt.boardAThroughB(), TeamID: nil})
	movedItems, itemsErr := bt.repos.ActionItem.SetTeamForBoard(t.Context(), spec.ActionItemTeamChange{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, TeamID: new(bt.b.Team.ID)})

	// then a board never joins a team of another organisation, and B never archives, deletes or pins A's board
	require.ErrorIs(t, pinOrgErr, dao.ErrNotFound)
	require.ErrorIs(t, pinMemberErr, dao.ErrNotFound)
	require.NoError(t, unpinErr)
	require.NoError(t, pinnedErr)
	assert.Empty(t, pinnedB)
	require.ErrorIs(t, archiveErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	require.ErrorIs(t, moveErr, dao.ErrNotFound)
	require.ErrorIs(t, crossTeamErr, dao.ErrNotFound)
	require.ErrorIs(t, archiveWriteErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteWriteErr, dao.ErrNotFound)
	require.ErrorIs(t, moveWriteErr, dao.ErrNotFound)
	require.NoError(t, itemsErr)
	assert.Zero(t, movedItems)
	assert.Zero(t, countWhere(t, bt.repos, `SELECT COUNT(*) FROM board_pins`))

	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, new(bt.a.Team.ID), header.TeamID)
	bt.boardAUnchanged(t)
}

func TestBoard_CrossTenantDeletedBoardsAreOnlyListedAndRestoredThroughTheirOrganisation(t *testing.T) {
	// given board A deleted through its own organisation
	bt := newBoardTenants(t)
	_, err := bt.repos.Board.SoftDelete(t.Context(), spec.BoardKey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID})
	require.NoError(t, err)
	since := time.Now().Add(-model.BoardRestoreWindow)

	// when
	listedB, listErr := bt.repos.Board.ListDeletedForOrganisation(t.Context(), spec.DeletedBoardsQuery{OrgID: bt.b.Organisation.ID, DeletedSince: since})
	_, restoreErr := bt.repos.Board.Restore(t.Context(), spec.BoardRestore{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, DeletedSince: since})

	// then
	require.NoError(t, listErr)
	assert.Empty(t, listedB)
	require.ErrorIs(t, restoreErr, dao.ErrNotFound)

	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.True(t, header.Deleted())
}
