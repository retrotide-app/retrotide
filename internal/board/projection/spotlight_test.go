package projection_test

import (
	"slices"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshot_SpotlightNamesOnlyWhatTheViewerCanSee(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given cards shown, column B hidden, and a spotlight that names a card and a column inside it
			state := columnVisibilityState(model.AnonymityNamed)
			state.Header.CardsHidden = false
			state.Header.RevealedColumnIDs = nil
			state.Header.Spotlight = model.BoardSpotlight{
				PresentingCardID:  new(secretID),
				HighlightedCardID: new(bHanaCardID),
				FocusedColumnID:   new(columnB),
			}

			// when
			snapshot := projection.Snapshot(state, tc.viewer)

			// then the presented card is shown to everyone, and the hidden column's references only to facilitators
			require.NotNil(t, snapshot.Spotlight.PresentingCardID)
			assert.Equal(t, secretID, *snapshot.Spotlight.PresentingCardID)
			if !slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate) {
				assert.Nil(t, snapshot.Spotlight.HighlightedCardID)
				assert.Nil(t, snapshot.Spotlight.FocusedColumnID)
				assert.NotContains(t, marshal(t, snapshot), bHanaCardID.String())

				return
			}

			require.NotNil(t, snapshot.Spotlight.HighlightedCardID)
			assert.Equal(t, bHanaCardID, *snapshot.Spotlight.HighlightedCardID)
			require.NotNil(t, snapshot.Spotlight.FocusedColumnID)
			assert.Equal(t, columnB, *snapshot.Spotlight.FocusedColumnID)
		})
	}
}

func TestSnapshot_SpotlightNeverNamesAHiddenCard(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given cards hidden and a stale spotlight on Sam's card
			state := boardState(model.AnonymityNamed, true)
			state.Header.Spotlight = model.BoardSpotlight{HighlightedCardID: new(secretID), FocusedColumnID: new(columnA)}

			// when
			snapshot := projection.Snapshot(state, tc.viewer)

			// then only the author, who can see the card anyway, gets its ID back
			assert.Equal(t, columnA, *snapshot.Spotlight.FocusedColumnID)
			if tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == samID {
				assert.Equal(t, secretID, *snapshot.Spotlight.HighlightedCardID)

				return
			}

			assert.Nil(t, snapshot.Spotlight.HighlightedCardID)
			assert.NotContains(t, marshal(t, snapshot), secretID.String())
		})
	}
}

func TestSpotlightChanged_OnePayloadForEveryone(t *testing.T) {
	// given
	board := boardState(model.AnonymityAnonymous, false).Header.Board
	board.Spotlight = model.BoardSpotlight{PresentingCardID: new(secretID), FocusedColumnID: new(columnA)}
	event := projection.SpotlightChanged(board, 1044)

	var payloads []string
	for _, tc := range viewerCases() {
		// when
		payload, seen := seenPayload(t, event, tc.viewer)

		// then
		require.True(t, seen, tc.name)
		assert.Equal(t, projection.ClassOthers, event.ClassOf(tc.viewer), tc.name)
		payloads = append(payloads, payload)
	}

	for _, payload := range payloads {
		assert.Equal(t, payloads[0], payload, "the spotlight is marshalled once for everyone")
	}
	assert.JSONEq(t, `{"type":"spotlight_changed","version":1044,"spotlight":{
		"presenting_card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
		"highlighted_card_id":null,
		"focused_column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3"}}`, payloads[0])
}
