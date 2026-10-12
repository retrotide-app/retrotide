package board

import (
	"context"
	"database/sql"
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func mergingOn(access *model.BoardAccess) {
	access.Header.Stage = model.StageDiscuss
	access.Header.MergingEnabled = true
}

func mergeCommand() MergeCardCommand {
	return MergeCardCommand{Command: baseCommand(), CardID: otherID, TargetCardID: cardID}
}

func dragged() model.Card {
	moving := card(myID)
	moving.ID = otherID
	moving.ColumnID = uuid.MustParse("1a2b3c4d-0000-4000-8000-00000000000c")

	return moving
}

func TestMergeCard_StacksTheDraggedCardAtTheEndOfItsTargetsColumn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, mergingOn))
	m.stored(dragged())
	m.stored(card(theirID))
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(columnCards("V", "k"), nil)
	stacked := dragged()
	stacked.ParentCardID = new(cardID)
	stacked.ColumnID = columnID
	stacked.Position = "l"
	m.cards.EXPECT().MergeCard(mock.Anything, spec.CardMergeWrite{BoardWrite: target(), CardID: otherID, ParentCardID: cardID, ColumnID: columnID, Position: "l"}).
		Return(&model.Versioned[model.CardMerge]{Value: model.CardMerge{Card: stacked, Restacked: []uuid.UUID{}}, Version: 8}, nil)
	m.participants()

	// when
	got, err := svc.MergeCard(t.Context(), mergeCommand())

	// then
	require.NoError(t, err)
	require.NotNil(t, got.Card)
	assert.Equal(t, new(cardID), got.Card.ParentCardID)
	require.Len(t, m.events.published, 1)
	payload := m.events.published[0].event.Payload(projection.ClassOthers)
	assert.Equal(t, dto.CardsMergedEvent{
		Type: projection.EventCardsMerged, Version: 8,
		CardID:       otherID,
		ParentCardID: cardID,
		ColumnID:     columnID,
		Position:     "l",
		Restacked:    []uuid.UUID{},
	}, payload)
}

func TestMergeCard_ACardDroppedOnAStackJoinsItsTopCard(t *testing.T) {
	// given a target that is itself merged under another card
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, mergingOn))
	m.stored(dragged())
	child := card(theirID)
	child.ID = firstCardID
	child.ParentCardID = new(cardID)
	m.stored(child)
	m.stored(card(theirID))
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return(nil, nil)
	m.cards.EXPECT().MergeCard(mock.Anything, mock.MatchedBy(func(s spec.CardMergeWrite) bool {
		return s.ParentCardID == cardID
	})).Return(&model.Versioned[model.CardMerge]{Value: model.CardMerge{Card: dragged()}, Version: 8}, nil)
	m.participants()

	// when
	_, err := svc.MergeCard(t.Context(), MergeCardCommand{Command: baseCommand(), CardID: otherID, TargetCardID: firstCardID})

	// then
	require.NoError(t, err)
}

func TestMergeCard_RefusesWhatThePolicyRefuses(t *testing.T) {
	tests := []struct {
		name    string
		options []accessOption
		want    error
	}{
		{name: "merging is switched off", options: []accessOption{atStage(model.StageDiscuss)}, want: policy.ErrMergingDisabled},
		{name: "the board is still in its Write stage", options: []accessOption{mergingOn, atStage(model.StageWrite)}, want: policy.ErrMergesOutOfStage},
		{name: "the board is in its Vote stage", options: []accessOption{mergingOn, atStage(model.StageVote)}, want: policy.ErrMergesOutOfStage},
		{name: "the board has moved on to Actions", options: []accessOption{mergingOn, atStage(model.StageActions)}, want: policy.ErrMergesOutOfStage},
		{name: "cards are hidden", options: []accessOption{mergingOn, hiddenCards}, want: policy.ErrCardNotFound},
		{name: "moves are locked", options: []accessOption{mergingOn, movesLocked}, want: policy.ErrMovesLocked},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, tc.options...))
			m.stored(dragged())
			m.stored(card(theirID))

			// when
			got, err := svc.MergeCard(t.Context(), mergeCommand())

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestUnmergeCard_PlacesTheCardJustAfterItsStack(t *testing.T) {
	// given a stacked card whose top card sits between two others
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, atStage(model.StageDiscuss)))
	stackedCard := dragged()
	stackedCard.ParentCardID = new(secondCardID)
	stackedCard.ColumnID = columnID
	m.stored(stackedCard)
	cards := columnCards("V", "k", "s")
	m.stored(cards[1])
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(append(cards, stackedCard), nil)
	var placed spec.CardPlacement
	m.cards.EXPECT().MoveCard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.CardPlacement, _ ...*sql.Tx) (*model.Versioned[model.Card], error) {
		placed = s
		moved := stackedCard
		moved.ParentCardID = nil
		moved.Position = s.Position

		return &model.Versioned[model.Card]{Value: moved, Version: 8}, nil
	})
	m.participants()

	// when
	got, err := svc.UnmergeCard(t.Context(), CardCommand{Command: baseCommand(), CardID: otherID})

	// then
	require.NoError(t, err)
	assert.Equal(t, columnID, placed.ColumnID)
	assert.Greater(t, placed.Position, "k")
	assert.Less(t, placed.Position, "s")
	require.NotNil(t, got.Card)
	assert.Nil(t, got.Card.ParentCardID)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, projection.EventCardUnmerged, m.events.published[0].event.Payload(projection.ClassOthers).(dto.CardUnmergedEvent).Type)
}

func TestUnmergeCard_IsRefusedOutsideTheDiscussStageEvenToAFacilitator(t *testing.T) {
	for _, stage := range []model.Stage{model.StageWrite, model.StageVote, model.StageActions} {
		t.Run(string(stage), func(t *testing.T) {
			// given a stacked card
			svc, m := newTestService(t)
			m.entered(boardAccess(host, atStage(stage)))
			stackedCard := dragged()
			stackedCard.ParentCardID = new(cardID)
			m.stored(stackedCard)

			// when
			got, err := svc.UnmergeCard(t.Context(), CardCommand{Command: baseCommand(), CardID: otherID})

			// then the strict card mock proves nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, policy.ErrMergesOutOfStage)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestUnmergeCard_ACardThatIsNotStackedIsRefused(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, atStage(model.StageDiscuss)))
	m.stored(card(theirID))

	// when
	got, err := svc.UnmergeCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotMerged)
}
