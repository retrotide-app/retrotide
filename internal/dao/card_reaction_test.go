package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f boardFixture) reactionKey(cardID, participantID uuid.UUID, reaction model.Reaction) spec.CardReactionKey {
	return spec.CardReactionKey{OrgID: f.orgID(), BoardID: f.boardID(), CardID: cardID, ParticipantID: participantID, Reaction: reaction}
}

func (f boardFixture) react(t *testing.T, cardID, participantID uuid.UUID, reaction model.Reaction) {
	t.Helper()

	_, err := f.repos.CardReaction.Add(t.Context(), f.reactionKey(cardID, participantID, reaction))
	require.NoError(t, err)
}

func TestCardReaction_AddIsIdempotentPerParticipantAndKey(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	key := f.reactionKey(card.ID, f.facilitator(), model.ReactionRocket)

	// when
	first, firstErr := f.repos.CardReaction.Add(t.Context(), key)
	second, secondErr := f.repos.CardReaction.Add(t.Context(), key)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.True(t, first)
	assert.False(t, second, "the same reaction twice is one reaction")
	listed, err := f.repos.CardReaction.ListForCard(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: card.ID})
	require.NoError(t, err)
	assert.Equal(t, []model.CardReaction{{BoardID: f.boardID(), CardID: card.ID, ParticipantID: new(f.facilitator()), Reaction: model.ReactionRocket}}, listed)
}

func TestCardReaction_RemoveTakesBackOnlyTheParticipantsOwnKey(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	other := f.secondParticipant(t)
	f.react(t, card.ID, f.facilitator(), model.ReactionHeart)
	f.react(t, card.ID, other, model.ReactionHeart)

	// when
	removed, removeErr := f.repos.CardReaction.Remove(t.Context(), f.reactionKey(card.ID, f.facilitator(), model.ReactionHeart))
	again, againErr := f.repos.CardReaction.Remove(t.Context(), f.reactionKey(card.ID, f.facilitator(), model.ReactionHeart))

	// then
	require.NoError(t, removeErr)
	assert.True(t, removed)
	require.NoError(t, againErr)
	assert.False(t, again)
	listed, err := f.repos.CardReaction.ListForCard(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: card.ID})
	require.NoError(t, err)
	assert.Equal(t, []model.CardReaction{{BoardID: f.boardID(), CardID: card.ID, ParticipantID: new(other), Reaction: model.ReactionHeart}}, listed)
}

func TestCardReaction_AnUnknownKeyIsRefusedByTheSchema(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")

	// when
	_, err := f.repos.CardReaction.Add(t.Context(), f.reactionKey(card.ID, f.facilitator(), "skull"))

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "card_reactions_reaction_check")
}

func TestCardReaction_ListReadsEveryCardOfTheBoardAndDeletingACardDropsItsReactions(t *testing.T) {
	// given
	f := newBoardFixture(t)
	first := f.addCard(t, f.column(0), "a0")
	second := f.addCard(t, f.column(1), "a0")
	f.react(t, first.ID, f.facilitator(), model.ReactionClap)
	f.react(t, second.ID, f.facilitator(), model.ReactionEyes)

	// when
	before, beforeErr := f.repos.CardReaction.List(t.Context(), f.boardID())
	_, deleteErr := f.repos.Card.Delete(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: first.ID})
	after, afterErr := f.repos.CardReaction.List(t.Context(), f.boardID())

	// then
	require.NoError(t, beforeErr)
	assert.Len(t, before, 2)
	require.NoError(t, deleteErr)
	require.NoError(t, afterErr)
	assert.Equal(t, []model.CardReaction{{BoardID: f.boardID(), CardID: second.ID, ParticipantID: new(f.facilitator()), Reaction: model.ReactionEyes}}, after)
}

func TestCardReaction_HandOverMovesTheGuestsReactionsAndTheAccountsOwnWin(t *testing.T) {
	// given a guest and an account seat that both reacted heart, and only the guest rocket
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	device, _ := daotest.CreateGuest(t, f.repos)
	header, err := f.repos.Board.GetHeader(t.Context(), f.boardID())
	require.NoError(t, err)
	guest := daotest.CreateGuestParticipant(t, f.repos, *header, device.ID)
	seat := f.secondParticipant(t)
	f.react(t, card.ID, guest.ID, model.ReactionHeart)
	f.react(t, card.ID, guest.ID, model.ReactionRocket)
	f.react(t, card.ID, seat, model.ReactionHeart)

	// when
	moved, err := f.repos.CardReaction.HandOver(t.Context(), spec.ParticipantHandover{BoardID: f.boardID(), FromParticipantID: guest.ID, ToParticipantID: seat})

	// then the guest keeps nothing, and the heart is not counted twice
	require.NoError(t, err)
	assert.Equal(t, int64(1), moved)
	listed, err := f.repos.CardReaction.ListForCard(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: card.ID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []model.CardReaction{
		{BoardID: f.boardID(), CardID: card.ID, ParticipantID: new(seat), Reaction: model.ReactionHeart},
		{BoardID: f.boardID(), CardID: card.ID, ParticipantID: new(seat), Reaction: model.ReactionRocket},
	}, listed)
}

func TestCardReaction_SeverKeepsTheCountsAndDropsWhoReacted(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	other := f.secondParticipant(t)
	f.react(t, card.ID, f.facilitator(), model.ReactionStar)
	f.react(t, card.ID, other, model.ReactionStar)

	// when
	severed, err := f.repos.CardReaction.Sever(t.Context(), f.boardID())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), severed)
	listed, err := f.repos.CardReaction.List(t.Context(), f.boardID())
	require.NoError(t, err)
	require.Len(t, listed, 2)
	for _, reaction := range listed {
		assert.Nil(t, reaction.ParticipantID)
		assert.Equal(t, model.ReactionStar, reaction.Reaction)
	}
}

func TestCardReaction_ARemovedSeatsReactionsSurviveWithoutTheirOwner(t *testing.T) {
	// given a participant whose seat is deleted after reacting
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	other := f.secondParticipant(t)
	f.react(t, card.ID, other, model.ReactionHundred)

	// when
	err := f.repos.Board.DeleteParticipant(t.Context(), spec.BoardParticipantKey{BoardID: f.boardID(), ParticipantID: other})

	// then
	require.NoError(t, err)
	listed, err := f.repos.CardReaction.List(t.Context(), f.boardID())
	require.NoError(t, err)
	assert.Equal(t, []model.CardReaction{{BoardID: f.boardID(), CardID: card.ID, Reaction: model.ReactionHundred}}, listed)
}

func TestCardReaction_AReactionOfAnotherBoardsSeatIsNotFound(t *testing.T) {
	// given a seat on another board of the same organisation
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	elsewhere := daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)

	// when
	_, err := f.repos.CardReaction.Add(t.Context(), f.reactionKey(card.ID, elsewhere.Participants[0].ID, model.ReactionWow))

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}
