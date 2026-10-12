package board

import (
	"fmt"
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

func presenting(id uuid.UUID) accessOption {
	return func(access *model.BoardAccess) {
		access.Header.Spotlight.PresentingCardID = new(id)
	}
}

func highlighting(id uuid.UUID) accessOption {
	return func(access *model.BoardAccess) {
		access.Header.Spotlight.HighlightedCardID = new(id)
	}
}

func presentCommand(id *uuid.UUID) CardSpotlightCommand {
	return CardSpotlightCommand{Command: baseCommand(), CardID: id}
}

func focusCommand(id *uuid.UUID) ColumnFocusCommand {
	return ColumnFocusCommand{Command: baseCommand(), ColumnID: id}
}

func spotlightWrite(spotlight model.BoardSpotlight) spec.BoardFacilitationWrite {
	return spec.BoardFacilitationWrite{BoardWrite: target(), Spotlight: spotlight}
}

func spotlitState(access *model.BoardAccess, spotlight model.BoardSpotlight) *model.Versioned[model.BoardState] {
	board := access.Header.Board
	board.Spotlight = spotlight

	return &model.Versioned[model.BoardState]{Value: model.BoardState{Header: model.BoardHeader{Board: board}}, Version: 8}
}

func spotlightChanged(view dto.SpotlightView) dto.SpotlightChangedEvent {
	return dto.SpotlightChangedEvent{BoardEventHead: dto.BoardEventHead{Type: projection.EventSpotlightChanged, Version: 8}, Spotlight: view}
}

func TestPresent_StartsAPresentationOnTheCardNamedAndTellsEveryone(t *testing.T) {
	// given a board whose cards everyone can see
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageDiscuss))
	m.entered(access)
	m.stored(card(theirID))
	spotlight := model.BoardSpotlight{PresentingCardID: new(cardID)}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, spotlightWrite(spotlight)).Return(spotlitState(access, spotlight), nil)

	// when
	got, err := svc.Present(t.Context(), presentCommand(new(cardID)))

	// then one payload reaches everyone, and the roster learns the new spotlight with it
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	event := m.onlyEvent(t)
	assert.Equal(t, projection.ClassOthers, event.ClassOf(otherViewer))
	assert.Equal(t, projection.ClassOthers, event.ClassOf(hostViewer))
	assert.Equal(t, spotlightChanged(dto.SpotlightView{PresentingCardID: new(cardID)}), event.Payload(projection.ClassOthers))
	assert.Equal(t, new(cardID), m.events.published[0].access.Header.Spotlight.PresentingCardID)
}

func TestPresent_StepsToTheNextCardAndLeavesTheHighlightAsItIs(t *testing.T) {
	// given a presentation on one card while another is highlighted
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageVote), presenting(cardID), highlighting(thirdCardID))
	m.entered(access)
	next := card(theirID)
	next.ID = otherID
	m.stored(next)
	spotlight := model.BoardSpotlight{PresentingCardID: new(otherID), HighlightedCardID: new(thirdCardID)}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, spotlightWrite(spotlight)).Return(spotlitState(access, spotlight), nil)

	// when
	got, err := svc.Present(t.Context(), presentCommand(new(otherID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	assert.Equal(t, spotlightChanged(dto.SpotlightView{PresentingCardID: new(otherID), HighlightedCardID: new(thirdCardID)}), m.onlyEvent(t).Payload(projection.ClassOthers))
}

func TestPresent_NoCardEndsThePresentationForEveryone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageDiscuss), presenting(cardID))
	m.entered(access)
	m.boards.EXPECT().SetFacilitationState(mock.Anything, spotlightWrite(model.BoardSpotlight{})).Return(spotlitState(access, model.BoardSpotlight{}), nil)

	// when
	got, err := svc.Present(t.Context(), presentCommand(nil))

	// then the strict card mock proves no card was read
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	assert.Equal(t, spotlightChanged(dto.SpotlightView{}), m.onlyEvent(t).Payload(projection.ClassOthers))
}

func TestPresent_AMergedCardIsPresentedAsItsStacksTopCard(t *testing.T) {
	// given a card merged under another
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageDiscuss))
	m.entered(access)
	child := card(theirID)
	child.ID = otherID
	child.ParentCardID = new(cardID)
	m.stored(child)
	m.stored(card(theirID))
	spotlight := model.BoardSpotlight{PresentingCardID: new(cardID)}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, spotlightWrite(spotlight)).Return(spotlitState(access, spotlight), nil)

	// when
	got, err := svc.Present(t.Context(), presentCommand(new(otherID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestPresent_PresentingTheCardAlreadyShownWritesNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, atStage(model.StageDiscuss), presenting(cardID)))
	m.stored(card(theirID))

	// when
	got, err := svc.Present(t.Context(), presentCommand(new(cardID)))

	// then the strict board mock proves nothing was written or announced
	require.NoError(t, err)
	assert.Equal(t, versionResponse(7), got)
	assert.Empty(t, m.events.published)
}

func TestPresent_RefusesACardNotEveryoneCanSee(t *testing.T) {
	tests := []struct {
		name    string
		options []accessOption
		author  uuid.UUID
		want    error
	}{
		{name: "someone else's hidden card is not shown to exist", options: []accessOption{hiddenCards}, author: theirID, want: policy.ErrCardNotFound},
		{name: "the facilitator's own hidden card", options: []accessOption{hiddenCards}, author: myID, want: policy.ErrCardNotShown},
		{name: "a card in a hidden column", options: []accessOption{columnHidden}, author: theirID, want: policy.ErrCardNotShown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(host, tc.options...))
			m.stored(card(tc.author))

			// when
			got, err := svc.Present(t.Context(), presentCommand(new(cardID)))

			// then the strict board mock proves nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestPresent_AnUnknownCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, atStage(model.StageDiscuss)))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errNotFound)

	// when
	got, err := svc.Present(t.Context(), presentCommand(new(cardID)))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestSpotlight_IsTheFacilitatorsAlone(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error)
	}{
		{name: "present", run: func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error) {
			return svc.Present(t.Context(), presentCommand(new(cardID)))
		}},
		{name: "highlight", run: func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error) {
			return svc.Highlight(t.Context(), presentCommand(new(cardID)))
		}},
		{name: "focus", run: func(t *testing.T, svc *service) (*dto.BoardCommandResponse, error) {
			return svc.Focus(t.Context(), focusCommand(new(columnID)))
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a participant, even on a board anyone can edit
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, atStage(model.StageDiscuss), anyoneCanEdit))

			// when
			got, err := tc.run(t, svc)

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, policy.ErrNotAllowed)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestSpotlight_IsRefusedOnAnEndedRetro(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageEnd))
	access.Header.ClosedAt = new(stamp)
	m.entered(access)

	// when
	got, err := svc.Present(t.Context(), presentCommand(new(cardID)))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrReadOnly)
}

func TestSpotlight_ALockedBoardDoesNotBindTheFacilitator(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageDiscuss))
	access.Header.LockedAt = new(stamp)
	m.entered(access)
	m.stored(card(theirID))
	spotlight := model.BoardSpotlight{HighlightedCardID: new(cardID)}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, spotlightWrite(spotlight)).Return(spotlitState(access, spotlight), nil)

	// when
	got, err := svc.Highlight(t.Context(), presentCommand(new(cardID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestSpotlight_ARetriedCommandReplaysItsVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 6)

	// when
	got, err := svc.Present(t.Context(), presentCommand(new(cardID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(6, nil), got)
}

func TestHighlight_HighlightsACardAndKeepsTheBoardsColumnStateAsItIs(t *testing.T) {
	// given a board with one revealed and one hidden column
	svc, m := newTestService(t)
	access := boardAccess(host, hiddenCards)
	access.Header.RevealedColumnIDs = []uuid.UUID{columnID}
	access.Header.HiddenColumnIDs = []uuid.UUID{secondColumnID}
	m.entered(access)
	m.stored(card(theirID))
	spotlight := model.BoardSpotlight{HighlightedCardID: new(cardID)}
	write := spec.BoardFacilitationWrite{BoardWrite: target(), RevealedColumnIDs: []uuid.UUID{columnID}, HiddenColumnIDs: []uuid.UUID{secondColumnID}, Spotlight: spotlight}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, write).Return(spotlitState(access, spotlight), nil)

	// when
	got, err := svc.Highlight(t.Context(), presentCommand(new(cardID)))

	// then a card in a revealed column may be highlighted while the rest stay hidden
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	assert.Equal(t, spotlightChanged(dto.SpotlightView{HighlightedCardID: new(cardID)}), m.onlyEvent(t).Payload(projection.ClassOthers))
}

func TestHighlight_NoCardRemovesTheHighlight(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, highlighting(cardID), presenting(otherID))
	m.entered(access)
	spotlight := model.BoardSpotlight{PresentingCardID: new(otherID)}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, spotlightWrite(spotlight)).Return(spotlitState(access, spotlight), nil)

	// when
	got, err := svc.Highlight(t.Context(), presentCommand(nil))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	assert.Equal(t, spotlightChanged(dto.SpotlightView{PresentingCardID: new(otherID)}), m.onlyEvent(t).Payload(projection.ClassOthers))
}

func TestFocus_FocusesAColumnForEveryone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, atStage(model.StageDiscuss))
	m.entered(access)
	m.column()
	spotlight := model.BoardSpotlight{FocusedColumnID: new(columnID)}
	m.boards.EXPECT().SetFacilitationState(mock.Anything, spotlightWrite(spotlight)).Return(spotlitState(access, spotlight), nil)

	// when
	got, err := svc.Focus(t.Context(), focusCommand(new(columnID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	assert.Equal(t, spotlightChanged(dto.SpotlightView{FocusedColumnID: new(columnID)}), m.onlyEvent(t).Payload(projection.ClassOthers))
}

func TestFocus_AHiddenColumnCannotBeFocused(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, columnHidden))
	m.column()

	// when
	got, err := svc.Focus(t.Context(), focusCommand(new(columnID)))

	// then the strict board mock proves nothing was written
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrColumnHidden)
	assert.Empty(t, m.events.published)
}

func TestFocus_AnUnknownColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(nil, errNotFound)

	// when
	got, err := svc.Focus(t.Context(), focusCommand(new(columnID)))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrColumnNotFound)
}

func TestFocus_ClearingAFocusThatIsNotSetWritesNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))

	// when
	got, err := svc.Focus(t.Context(), focusCommand(nil))

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(7), got)
	assert.Empty(t, m.events.published)
}

func TestSpotlight_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.column()
	m.boards.EXPECT().SetFacilitationState(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("write: %w", errDatabase))

	// when
	got, err := svc.Focus(t.Context(), focusCommand(new(columnID)))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, errDatabase)
	assert.Empty(t, m.events.published)
}
