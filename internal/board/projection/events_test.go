package projection_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	cardEventCase struct {
		name  string
		event projection.Event
		card  bool
	}
)

func seenPayload(t *testing.T, event projection.Event, viewer projection.Viewer) (string, bool) {
	t.Helper()

	payload := event.Payload(event.ClassOf(viewer))
	if payload == nil {
		return "", false
	}

	if personal, ok := event.PayloadFor(viewer); ok {
		return marshal(t, personal), true
	}

	return marshal(t, payload), true
}

func secretCard(state model.BoardState) model.Card {
	return state.Cards[1]
}

func samProfile(state model.BoardState) *model.BoardParticipantProfile {
	return &state.Participants[0]
}

func rawMessages(payloads []string) []json.RawMessage {
	raw := make([]json.RawMessage, 0, len(payloads))
	for _, payload := range payloads {
		raw = append(raw, json.RawMessage(payload))
	}

	return raw
}

func cardEventCases(state model.BoardState) []cardEventCase {
	board := state.Header.Board
	moved := secretCard(state)
	moved.ColumnID = columnB

	return []cardEventCase{
		{name: "card_upserted on add", event: projection.CardAdded(board, secretCard(state), samProfile(state), 1044), card: true},
		{name: "card_upserted on edit", event: projection.CardChanged(board, secretCard(state), samProfile(state), 1044), card: true},
		{name: "card_moved", event: projection.CardMoved(board, moved, columnA, []spec.PositionChange{{ID: secretID, Position: cardPositions[secretID]}}, 1044)},
		{name: "card_deleted", event: projection.CardDeleted(board, secretCard(state), 1044)},
	}
}

func TestCardEvents_PrivacyAcrossEveryViewerAndBoard(t *testing.T) {
	for _, board := range boardMatrix() {
		state := boardState(board.Anonymity, board.CardsHidden)

		for _, eventCase := range cardEventCases(state) {
			for _, tc := range viewerCases() {
				t.Run(fmt.Sprintf("%s/%s/hidden=%t/%s", eventCase.name, board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
					// given
					event := eventCase.event

					// when
					payload, seen := seenPayload(t, event, tc.viewer)

					// then
					if !slices.Contains(visibleTo(board, tc), secretID) {
						assert.False(t, seen, "a viewer who may not see the hidden card gets nothing at all for it")
						return
					}

					require.True(t, seen, "a viewer who may see the card is told about it")
					assert.Equal(t, int64(1044), event.Version())
					assert.NotContains(t, payload, aliText)
					assert.NotContains(t, payload, aliCardID.String())

					decoded := decode(t, payload)
					if !eventCase.card {
						assert.Equal(t, secretID.String(), decoded["card_id"])
						assert.NotContains(t, payload, samID.String(), "a card event without a card carries no author")
						assert.NotContains(t, payload, secretText)
						return
					}

					assertPrivate(t, board, tc, payload)
					cardObject, ok := decoded["card"].(map[string]any)
					require.True(t, ok)
					assertCardObject(t, board, tc, cardObject)
				})
			}
		}
	}
}

func TestCardEvents_OnlyHiddenCardChangesMoveTheTally(t *testing.T) {
	for _, board := range boardMatrix() {
		t.Run(fmt.Sprintf("%s/hidden=%t", board.Anonymity, board.CardsHidden), func(t *testing.T) {
			// given
			state := boardState(board.Anonymity, board.CardsHidden)
			moved := secretCard(state)
			moved.ColumnID = columnB

			// when
			added := projection.CardAdded(board, secretCard(state), nil, 1044)
			edited := projection.CardChanged(board, secretCard(state), nil, 1044)
			relocated := projection.CardMoved(board, moved, columnA, nil, 1044)
			deleted := projection.CardDeleted(board, secretCard(state), 1044)

			// then
			assert.Equal(t, board.CardsHidden, added.MovesHiddenCards())
			assert.False(t, edited.MovesHiddenCards(), "an edit never changes a count")
			assert.Equal(t, board.CardsHidden, relocated.MovesHiddenCards())
			assert.Equal(t, board.CardsHidden, deleted.MovesHiddenCards())
		})
	}
}

func TestCardEvents_TheTallyFollowsAddMoveAndDelete(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, true)
	board := state.Header.Board
	tally := projection.Tally(board, state.Cards)
	moved := secretCard(state)
	moved.ColumnID = columnB
	viewer := viewerCases()[3].viewer

	// when
	afterMove := projection.CardMoved(board, moved, columnA, nil, 1044).MoveHiddenCards(tally)
	afterDelete := projection.CardDeleted(board, moved, 1045).MoveHiddenCards(afterMove)
	afterAdd := projection.CardAdded(board, moved, nil, 1046).MoveHiddenCards(afterDelete)

	// then
	assert.Equal(t, map[uuid.UUID]int{columnA: 2}, tally.For(board, viewer))
	assert.Equal(t, map[uuid.UUID]int{columnA: 1, columnB: 1}, afterMove.For(board, viewer))
	assert.Equal(t, map[uuid.UUID]int{columnA: 1}, afterDelete.For(board, viewer))
	assert.Equal(t, map[uuid.UUID]int{columnA: 1, columnB: 1}, afterAdd.For(board, viewer))
	assert.Equal(t, map[uuid.UUID]int{columnA: 2}, tally.For(board, viewer), "applying an event never changes the tally it was given")
}

func TestHiddenCounts_PrivacyAcrossEveryViewer(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given
				state := boardState(anonymity, true)
				tally := projection.Tally(state.Header.Board, state.Cards)

				// when
				payload := marshal(t, projection.HiddenCounts(tally.For(state.Header.Board, tc.viewer), 1044))

				// then the frame is a handful of integers: no ID, text, position or author of any card
				assertPrivate(t, state.Header.Board, tc, payload)
				for _, id := range []uuid.UUID{secretID, aliCardID, samID, aliID} {
					assert.NotContains(t, payload, id.String())
				}

				want := map[string]any{"type": "hidden_counts", "version": float64(1044), "hidden_counts": map[string]any{}}
				if hiddenInA := 2 - len(tc.owns); hiddenInA > 0 {
					want["hidden_counts"] = map[string]any{columnA.String(): float64(hiddenInA)}
				}
				assert.Equal(t, want, decode(t, payload))
			})
		}
	}
}

func TestHiddenCounts_ACardWithNoAuthorCountsForEveryone(t *testing.T) {
	// given a card whose authorship was severed
	state := boardState(model.AnonymityAnonymous, true)
	state.Cards[1].AuthorParticipantID = nil

	// when
	counts := projection.Tally(state.Header.Board, state.Cards).For(state.Header.Board, viewerCases()[0].viewer)

	// then
	assert.Equal(t, map[uuid.UUID]int{columnA: 2}, counts)
}

func TestPublicEvents_ReachEveryViewerAlikeAndCarryNoCard(t *testing.T) {
	state := boardState(model.AnonymityAnonymous, true)
	board := state.Header.Board
	column := state.Columns[0]
	board.Spotlight = model.BoardSpotlight{FocusedColumnID: new(columnB)}

	events := map[string]projection.Event{
		"board_updated":        projection.BoardUpdated(board, 1044),
		"participant_upserted": projection.ParticipantUpserted(board, state.Participants[1], 1044),
		"column_upserted":      projection.ColumnUpserted(board, column, 1044),
		"column_moved":         projection.ColumnMoved(board, column, []spec.PositionChange{{ID: columnB, Position: "a2"}}, 1044),
		"column_deleted":       projection.ColumnDeleted(board, columnA, 1044),
		"column_cleared":       projection.ColumnCleared(board, columnA, 1044),
		"column_hidden":        projection.ColumnHidden(columnA, 1044),
		"column_votes_reset":   projection.ColumnVotesReset(columnA, 1044),
		"spotlight_changed":    projection.SpotlightChanged(board, 1044),
		"resync":               projection.Resync(1044),
	}

	for name, event := range events {
		t.Run(name, func(t *testing.T) {
			// given
			var payloads []string

			// when
			for _, tc := range viewerCases() {
				payload, seen := seenPayload(t, event, tc.viewer)
				require.True(t, seen)
				payloads = append(payloads, payload)
			}

			// then
			for _, payload := range payloads {
				assert.Equal(t, payloads[0], payload, "a public event is marshalled once for everyone")
				assert.NotContains(t, payload, secretText)
				assert.NotContains(t, payload, aliText)
				assert.NotContains(t, payload, secretID.String())
				assert.NotContains(t, payload, aliCardID.String())
				assert.NotContains(t, payload, "created_at")
				assert.Equal(t, name, decode(t, payload)["type"])
			}
		})
	}
}

func TestPublicEvents_GoldenFrames(t *testing.T) {
	state := boardState(model.AnonymityNamed, false)

	tests := []struct {
		name  string
		event projection.Event
		want  string
	}{
		{
			name:  "board_updated",
			event: projection.BoardUpdated(state.Header.Board, 1044),
			want:  `{"type":"board_updated","version":1044,"title":"Sprint 42","context":"Payments squad"}`,
		},
		{
			name:  "participant_upserted",
			event: projection.ParticipantUpserted(state.Header.Board, state.Participants[0], 1044),
			want:  `{"type":"participant_upserted","version":1044,"participant":{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":"Sam","role":"participant"}}`,
		},
		{
			name:  "column_moved",
			event: projection.ColumnMoved(state.Header.Board, state.Columns[1], []spec.PositionChange{{ID: columnA, Position: "a2"}}, 1044),
			want:  `{"type":"column_moved","version":1044,"column_id":"b8c2d3e4-f5a6-4b7c-ad8e-9fa0b1c2d3e4","position":"a1","rewritten":[{"id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","position":"a2"}]}`,
		},
		{
			name:  "column_cleared",
			event: projection.ColumnCleared(state.Header.Board, columnA, 1044),
			want:  `{"type":"column_cleared","version":1044,"column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3"}`,
		},
		{
			name:  "column_upserted",
			event: projection.ColumnUpserted(state.Header.Board, model.BoardColumn{ID: columnB, Title: "To improve", Colour: model.ColourRed, Position: "a1", MaxVotes: new(2)}, 1044),
			want:  `{"type":"column_upserted","version":1044,"column":{"id":"b8c2d3e4-f5a6-4b7c-ad8e-9fa0b1c2d3e4","title":"To improve","description":"","colour":"red","position":"a1","max_votes":2,"cards_revealed":false,"hidden":false}}`,
		},
		{
			name:  "column_hidden",
			event: projection.ColumnHidden(columnA, 1044),
			want:  `{"type":"column_hidden","version":1044,"column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3"}`,
		},
		{
			name:  "column_votes_reset",
			event: projection.ColumnVotesReset(columnA, 1044),
			want:  `{"type":"column_votes_reset","version":1044,"column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3"}`,
		},
		{
			name:  "resync",
			event: projection.Resync(1044),
			want:  `{"type":"resync","version":1044}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload, seen := seenPayload(t, tc.event, viewerCases()[2].viewer)

			// then
			require.True(t, seen)
			assert.JSONEq(t, tc.want, payload)
		})
	}
}

func TestCardUpserted_GoldenForTheAuthorAndForEveryoneElse(t *testing.T) {
	// given a named board with cards shown
	state := boardState(model.AnonymityNamed, false)
	event := projection.CardChanged(state.Header.Board, secretCard(state), samProfile(state), 1044)

	// when
	author, _ := seenPayload(t, event, viewerCases()[0].viewer)
	other, _ := seenPayload(t, event, viewerCases()[1].viewer)

	// then
	assert.JSONEq(t, `{"type":"card_upserted","version":1044,"card":{
		"id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","parent_card_id":null,
		"position":"a0Vq","body":"SECRET-TEXT-9f2c","colour":"amber",
		"author":{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":"Sam"},
		"is_mine":true,"hidden":false,"votes":{"total":3},"created_at":"2026-09-22T10:01:02.512Z","updated_at":"2026-09-22T10:03:00.250Z"}}`, author)
	assert.JSONEq(t, `{"type":"card_upserted","version":1044,"card":{
		"id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","parent_card_id":null,
		"position":"a0Vq","body":"SECRET-TEXT-9f2c","colour":"amber",
		"author":{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":"Sam"},
		"hidden":false,"votes":{"total":3},"created_at":"2026-09-22T10:01:02.512Z","updated_at":"2026-09-22T10:03:00.250Z"}}`, other)
}

func TestCardUpserted_GoldenOnAnAnonymousBoardWithCardsHidden(t *testing.T) {
	// given
	state := boardState(model.AnonymityAnonymous, true)
	event := projection.CardAdded(state.Header.Board, secretCard(state), nil, 1044)

	// when
	author, authorSeen := seenPayload(t, event, viewerCases()[0].viewer)
	_, facilitatorSeen := seenPayload(t, event, viewerCases()[3].viewer)

	// then
	require.True(t, authorSeen)
	assert.False(t, facilitatorSeen)
	assert.JSONEq(t, `{"type":"card_upserted","version":1044,"card":{
		"id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","parent_card_id":null,
		"position":"a0Vq","body":"SECRET-TEXT-9f2c","colour":"amber","author":null,"is_mine":true,"hidden":true,"votes":{"total":3}}}`, author)
}

func TestPresence_LeavesOutActivityWhileAnAnonymousBoardHidesCards(t *testing.T) {
	viewers := []projection.PresenceViewer{
		{ParticipantID: samID, DisplayName: "Sam", State: "active"},
		{ParticipantID: aliID, DisplayName: "Ali", State: "idle"},
	}

	for _, board := range boardMatrix() {
		t.Run(fmt.Sprintf("%s/hidden=%t", board.Anonymity, board.CardsHidden), func(t *testing.T) {
			// when
			payload := marshal(t, projection.Presence(boardID, board, viewers))

			// then
			if board.Anonymous() && board.CardsHidden {
				assert.JSONEq(t, `{"board_id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","viewers":[
					{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":"Sam"},
					{"participant_id":"2a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d","display_name":"Ali"}]}`, payload)
				return
			}

			assert.JSONEq(t, `{"board_id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","viewers":[
				{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":"Sam","state":"active"},
				{"participant_id":"2a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d","display_name":"Ali","state":"idle"}]}`, payload)
		})
	}
}

func TestSnapshotFromTally_CountsFromThePublishedTallyNotTheRows(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given rows holding two hidden cards and a tally published before Ali's card was saved
			state := boardState(model.AnonymityAnonymous, true)
			published := projection.Tally(state.Header.Board, state.Cards[1:])

			// when
			payload := marshal(t, projection.SnapshotFromTally(state, tc.viewer, published))

			// then
			assertPrivate(t, state.Header.Board, tc, payload)

			want := map[string]any{}
			if !slices.Contains(tc.owns, secretID) {
				want[columnA.String()] = float64(1)
			}
			assert.Equal(t, want, decode(t, payload)["hidden_counts"])
		})
	}
}

func TestEventsFrame_Golden(t *testing.T) {
	// given
	events := []projection.Event{projection.ColumnCleared(boardState(model.AnonymityNamed, true).Header.Board, columnA, 1044), projection.Resync(1045)}
	var raw []string
	for _, event := range events {
		payload, _ := seenPayload(t, event, viewerCases()[0].viewer)
		raw = append(raw, payload)
	}

	// when
	payload := marshal(t, projection.EventsFrame(boardID, 1043, 1045, rawMessages(raw)))

	// then
	assert.JSONEq(t, `{"board_id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","from":1043,"to":1045,"events":[
		{"type":"column_cleared","version":1044,"column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3"},
		{"type":"resync","version":1045}]}`, payload)
}
