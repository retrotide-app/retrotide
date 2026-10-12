package dao_test

import (
	"strings"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f boardFixture) addComment(t *testing.T, cardID uuid.UUID, body string) model.CardComment {
	t.Helper()

	added, err := f.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{
		BoardWrite:          f.write(t),
		CommentID:           uuid.New(),
		CardID:              cardID,
		AuthorParticipantID: new(f.facilitator()),
		Body:                body,
	})
	require.NoError(t, err)

	return added.Value
}

func TestComment_AddCommentStoresItsAuthorAndReadsBack(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	id := uuid.New()

	// when
	added, err := f.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{
		BoardWrite:          f.write(t),
		CommentID:           id,
		CardID:              card.ID,
		AuthorParticipantID: new(f.facilitator()),
		Body:                "Agreed, the pipeline helped",
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), added.Version)
	assert.Equal(t, id, added.Value.ID)
	assert.Equal(t, card.ID, added.Value.CardID)
	assert.Equal(t, new(f.facilitator()), added.Value.AuthorParticipantID)
	assert.Nil(t, added.Value.UpdatedAt)

	read, err := f.repos.CardComment.GetByID(t.Context(), spec.CardCommentRef{BoardID: f.boardID(), CommentID: id})
	require.NoError(t, err)
	assert.Equal(t, added.Value, *read)
}

func TestComment_ASecondCreateWithTheSameIDConflicts(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	existing := f.addComment(t, card.ID, "First")
	before := f.version(t)

	// when
	_, err := f.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{BoardWrite: f.write(t), CommentID: existing.ID, CardID: card.ID, Body: "Again"})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Equal(t, before, f.version(t))
}

func TestComment_ACommentOnAMissingCardIsNotFound(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	_, err := f.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{BoardWrite: f.write(t), CommentID: uuid.New(), CardID: uuid.New(), Body: "Lost"})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestComment_EditMarksItEditedAndDeleteRemovesIt(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	comment := f.addComment(t, card.ID, "First")

	// when
	edited, editErr := f.repos.CardComment.EditComment(t.Context(), spec.CommentBodyWrite{BoardWrite: f.write(t), CommentID: comment.ID, Body: "Second"})
	deleted, deleteErr := f.repos.CardComment.DeleteComment(t.Context(), spec.CommentRemoval{BoardWrite: f.write(t), CommentID: comment.ID})
	_, goneErr := f.repos.CardComment.GetByID(t.Context(), spec.CardCommentRef{BoardID: f.boardID(), CommentID: comment.ID})
	_, missingErr := f.repos.CardComment.EditComment(t.Context(), spec.CommentBodyWrite{BoardWrite: f.write(t), CommentID: comment.ID, Body: "Third"})

	// then
	require.NoError(t, editErr)
	assert.Equal(t, "Second", edited.Value.Body)
	assert.NotNil(t, edited.Value.UpdatedAt)
	require.NoError(t, deleteErr)
	assert.Equal(t, comment.ID, deleted.Value.ID)
	require.ErrorIs(t, goneErr, dao.ErrNotFound)
	require.ErrorIs(t, missingErr, dao.ErrNotFound)
	assert.Equal(t, int64(4), f.version(t))
}

func TestComment_ListIsOldestFirstAndDeletingTheCardTakesItsComments(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	kept := f.addCard(t, f.column(1), "a0")
	first := f.addComment(t, card.ID, "First")
	second := f.addComment(t, card.ID, "Second")
	other := f.addComment(t, kept.ID, "Elsewhere")

	listed, err := f.repos.CardComment.List(t.Context(), f.boardID())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first.ID, second.ID, other.ID}, commentIDs(listed))

	// when
	_, err = f.repos.Card.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: f.write(t), CardID: card.ID})

	// then
	require.NoError(t, err)
	left, err := f.repos.CardComment.List(t.Context(), f.boardID())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{other.ID}, commentIDs(left))
}

func TestComment_RemovingTheAuthorKeepsTheCommentWithoutOne(t *testing.T) {
	// given a comment handed to a second participant
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	comment := f.addComment(t, card.ID, "First")
	other := f.secondParticipant(t)
	_, err := f.repos.CardComment.ReassignAuthor(t.Context(), spec.CardAuthorReassignment{BoardID: f.boardID(), FromParticipantID: f.facilitator(), ToParticipantID: other})
	require.NoError(t, err)
	moved, err := f.repos.CardComment.GetByID(t.Context(), spec.CardCommentRef{BoardID: f.boardID(), CommentID: comment.ID})
	require.NoError(t, err)
	assert.Equal(t, new(other), moved.AuthorParticipantID)

	// when
	_, err = f.repos.DB().ExecContext(t.Context(), `DELETE FROM board_participants WHERE board_id = $1 AND id = $2`, f.boardID(), other)

	// then
	require.NoError(t, err)
	kept, getErr := f.repos.CardComment.GetByID(t.Context(), spec.CardCommentRef{BoardID: f.boardID(), CommentID: comment.ID})
	require.NoError(t, getErr)
	assert.Nil(t, kept.AuthorParticipantID)
	assert.Equal(t, "First", kept.Body)
}

func TestComment_CrossTenantCommentsAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given a comment on board A's card
	bt := newBoardTenants(t)
	authorA := bt.boardA.Participants[0].ID
	added, err := bt.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{
		OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, Version: 1, OpID: uuid.New(),
		CommentID:           uuid.New(),
		CardID:              bt.cardA.ID,
		AuthorParticipantID: new(authorA),
		Body:                "SECRET-COMMENT-3a9d",
	})
	require.NoError(t, err)
	commentA := added.Value
	boardB := bt.boardB.Header.ID
	throughB := spec.CardCommentRef{BoardID: boardB, CommentID: commentA.ID}

	// when
	_, getErr := bt.repos.CardComment.GetByID(t.Context(), throughB)
	_, editErr := bt.repos.CardComment.Edit(t.Context(), spec.CardCommentEdit{BoardID: boardB, CommentID: commentA.ID, Body: "Rewritten"})
	_, deleteErr := bt.repos.CardComment.Delete(t.Context(), throughB)
	listed, listErr := bt.repos.CardComment.List(t.Context(), boardB)
	moved, moveErr := bt.repos.CardComment.ReassignAuthor(t.Context(), spec.CardAuthorReassignment{BoardID: boardB, FromParticipantID: authorA, ToParticipantID: bt.boardB.Participants[0].ID})
	_, plantErr := bt.repos.CardComment.Create(t.Context(), spec.NewCardComment{ID: uuid.New(), BoardID: boardB, CardID: bt.cardA.ID, Body: "Planted"})
	_, foreignAuthorErr := bt.repos.CardComment.Create(t.Context(), spec.NewCardComment{ID: uuid.New(), BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, AuthorParticipantID: new(bt.boardB.Participants[0].ID), Body: "Planted"})
	_, writeErr := bt.repos.CardComment.EditComment(t.Context(), spec.CommentBodyWrite{BoardWrite: bt.boardAThroughB(), CommentID: commentA.ID, Body: "Rewritten"})

	// then
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, editErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	require.ErrorIs(t, plantErr, dao.ErrNotFound)
	require.ErrorIs(t, foreignAuthorErr, dao.ErrNotFound)
	require.ErrorIs(t, writeErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	assert.Empty(t, listed)
	require.NoError(t, moveErr)
	assert.Zero(t, moved)

	kept, err := bt.repos.CardComment.GetByID(t.Context(), spec.CardCommentRef{BoardID: bt.boardA.Header.ID, CommentID: commentA.ID})
	require.NoError(t, err)
	assert.Equal(t, commentA, *kept)
}

func TestComment_SchemaConstraintsRefuseBadRows(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		constraint string
	}{
		{name: "an empty comment", body: "", constraint: "card_comments_body_length"},
		{name: "a comment over 1000 characters", body: strings.Repeat("c", 1001), constraint: "card_comments_body_length"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newBoardFixture(t)
			card := f.addCard(t, f.column(0), "a0")

			// when
			_, err := f.repos.DB().ExecContext(t.Context(), `INSERT INTO card_comments (id, board_id, card_id, body) VALUES ($1, $2, $3, $4)`, uuid.New(), f.boardID(), card.ID, tc.body)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}

func commentIDs(comments []model.CardComment) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(comments))
	for _, comment := range comments {
		ids = append(ids, comment.ID)
	}

	return ids
}
