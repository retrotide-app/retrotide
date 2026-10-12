package projection_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func exportPayload(t *testing.T, state model.BoardState, viewer projection.Viewer) string {
	t.Helper()

	return marshal(t, projection.Export(state, viewer, projection.Tally(state.Header.Board, state.Cards)))
}

func assertExportPrivate(t *testing.T, board model.Board, tc viewerCase, payload string) {
	t.Helper()

	visible := visibleTo(board, tc)
	for _, id := range []uuid.UUID{aliCardID, secretID} {
		if slices.Contains(visible, id) {
			assert.Contains(t, payload, cardBodies[id], "a card the requester may see is missing from the export")

			continue
		}

		assert.NotContains(t, payload, cardBodies[id], "hidden text reached the export")
		assert.NotContains(t, payload, cardComments[id].body, "a comment on a hidden card reached the export")
	}

	identifiers := []uuid.UUID{
		samID, aliID, hostID, ownerID, guestID, removedID, samUserID, aliUserID,
		aliCardID, secretID, aliCommentID, secretCommentID, convertedItemID, directItemID,
	}
	for _, id := range identifiers {
		assert.NotContains(t, payload, id.String(), "an export carries no participant, user, card, comment or action item ID")
	}

	for _, key := range []string{`"is_mine"`, `"participant_id"`, `"user_id"`, `"position"`} {
		assert.NotContains(t, payload, key)
	}

	var tree any
	require.NoError(t, json.Unmarshal([]byte(payload), &tree))
	walk(tree, func(key string, _ any) {
		if !board.Anonymous() {
			return
		}

		assert.NotEqual(t, "created_at", key, "an anonymous board's export carries no created_at")
		assert.NotEqual(t, "updated_at", key, "an anonymous board's export carries no updated_at")
		assert.NotEqual(t, "author", key, "an anonymous board's export names no author")
	})
}

func hiddenCountsOf(t *testing.T, payload string) []any {
	t.Helper()

	columns, ok := decode(t, payload)["columns"].([]any)
	require.True(t, ok, "columns is always a list")

	counts := make([]any, 0, len(columns))
	for _, raw := range columns {
		counts = append(counts, raw.(map[string]any)["hidden_count"])
	}

	return counts
}

func TestExport_GoldenNamedBoardAfterReveal(t *testing.T) {
	// given a revealed named board with Ali's card merged into Sam's, an item raised from Sam's card and a legacy item
	state := withActionItems(boardState(model.AnonymityNamed, false))
	state.Cards[0].ParentCardID = new(secretID)
	viewer := viewerCases()[7].viewer

	// when
	payload := exportPayload(t, state, viewer)

	// then
	assert.JSONEq(t, `{
		"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"title": "Sprint 42",
		"context": "Payments squad",
		"template_key": "went-well",
		"anonymity": "named",
		"stage": "vote",
		"cards_hidden": false,
		"closed": false,
		"created_at": "2026-09-22T09:00:00.000Z",
		"columns": [
			{
				"title": "Went well",
				"description": "What helped",
				"hidden_count": 0,
				"cards": [
					{
						"body": "SECRET-TEXT-9f2c",
						"author": "Sam",
						"votes": 3,
						"hidden": false,
						"created_at": "2026-09-22T10:01:02.512Z",
						"updated_at": "2026-09-22T10:03:00.250Z",
						"comments": [
							{"body": "SECRET-COMMENT-3a9d", "author": "Sam", "created_at": "2026-09-22T10:05:00.000Z"}
						],
						"action_items": [
							{"title": "Fix the deploy pipeline with @Sam", "assignee": "Alison", "due_on": "2026-10-20", "status": "open"}
						],
						"merged_cards": [
							{
								"body": "ALI-TEXT-41d0",
								"author": "Ali",
								"votes": 0,
								"hidden": false,
								"created_at": "2026-09-22T10:02:40.118Z",
								"comments": [
									{"body": "ALI-COMMENT-7e1b", "author": "Ali", "created_at": "2026-09-22T10:04:00.000Z"}
								],
								"action_items": [],
								"merged_cards": []
							}
						]
					}
				]
			},
			{"title": "To improve", "description": "", "hidden_count": 0, "cards": []}
		],
		"other_actions": [
			{"title": "Book the next retro", "due_on": null, "status": "done"}
		]
	}`, payload)
	assertExportPrivate(t, state.Header.Board, viewerCases()[7], payload)
}

func TestExport_GoldenAnonymousBoardAfterReveal(t *testing.T) {
	// given a revealed anonymous board
	state := withActionItems(boardState(model.AnonymityAnonymous, false))
	viewer := viewerCases()[7].viewer

	// when
	payload := exportPayload(t, state, viewer)

	// then no author, no date and no ID of anyone appears
	assert.JSONEq(t, `{
		"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"title": "Sprint 42",
		"context": "Payments squad",
		"template_key": "went-well",
		"anonymity": "anonymous",
		"stage": "vote",
		"cards_hidden": false,
		"closed": false,
		"columns": [
			{
				"title": "Went well",
				"description": "What helped",
				"hidden_count": 0,
				"cards": [
					{
						"body": "ALI-TEXT-41d0",
						"votes": 0,
						"hidden": false,
						"comments": [{"body": "ALI-COMMENT-7e1b"}],
						"action_items": [],
						"merged_cards": []
					},
					{
						"body": "SECRET-TEXT-9f2c",
						"votes": 3,
						"hidden": false,
						"comments": [{"body": "SECRET-COMMENT-3a9d"}],
						"action_items": [
							{"title": "Fix the deploy pipeline with @Sam", "assignee": "Alison", "due_on": "2026-10-20", "status": "open"}
						],
						"merged_cards": []
					}
				]
			},
			{"title": "To improve", "description": "", "hidden_count": 0, "cards": []}
		],
		"other_actions": [
			{"title": "Book the next retro", "due_on": null, "status": "done"}
		]
	}`, payload)
	assertExportPrivate(t, state.Header.Board, viewerCases()[7], payload)
}

func TestExport_GoldenRequesterOnANamedBoardWithCardsHidden(t *testing.T) {
	// given Sam's card is still hidden and Ali wrote her own card
	state := withActionItems(boardState(model.AnonymityNamed, true))
	viewer := viewerCases()[1].viewer

	// when
	payload := exportPayload(t, state, viewer)

	// then Ali gets her own card in full, Sam's only as a count, and Sam's item without its card
	assert.JSONEq(t, `{
		"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"title": "Sprint 42",
		"context": "Payments squad",
		"template_key": "went-well",
		"anonymity": "named",
		"stage": "write",
		"cards_hidden": true,
		"closed": false,
		"created_at": "2026-09-22T09:00:00.000Z",
		"columns": [
			{
				"title": "Went well",
				"description": "What helped",
				"hidden_count": 1,
				"cards": [
					{
						"body": "ALI-TEXT-41d0",
						"author": "Ali",
						"votes": 0,
						"hidden": true,
						"created_at": "2026-09-22T10:02:40.118Z",
						"comments": [
							{"body": "ALI-COMMENT-7e1b", "author": "Ali", "created_at": "2026-09-22T10:04:00.000Z"}
						],
						"action_items": [],
						"merged_cards": []
					}
				]
			},
			{"title": "To improve", "description": "", "hidden_count": 0, "cards": []}
		],
		"other_actions": [
			{"title": "Fix the deploy pipeline with @Sam", "assignee": "Alison", "due_on": "2026-10-20", "status": "open"},
			{"title": "Book the next retro", "due_on": null, "status": "done"}
		]
	}`, payload)
	assertExportPrivate(t, state.Header.Board, viewerCases()[1], payload)
}

func TestExport_PrivacyAcrossEveryViewerAndBoard(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given
				state := withActionItems(boardState(board.Anonymity, board.CardsHidden))

				// when
				payload := exportPayload(t, state, tc.viewer)

				// then hidden cards appear only to their author, and everyone else's only as a count
				assertExportPrivate(t, board, tc, payload)

				hiddenInA := 0
				if board.CardsHidden {
					hiddenInA = 2 - len(tc.owns)
				}
				assert.Equal(t, []any{float64(hiddenInA), float64(0)}, hiddenCountsOf(t, payload))
			})
		}
	}
}

func TestExport_AnActionItemFromAHiddenCardNeverRevealsTheCard(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given an item raised from Sam's card, a stack merged before the cards were hidden again
				state := withActionItems(boardState(anonymity, true))
				state.Cards[0].ParentCardID = new(secretID)

				// when
				export := projection.Export(state, tc.viewer, projection.Tally(state.Header.Board, state.Cards))
				payload := marshal(t, export)

				// then only Sam sees the item under his card; everyone else finds it among the other actions
				assertExportPrivate(t, state.Header.Board, tc, payload)
				require.Len(t, export.Columns, 2)

				var titles []string
				for _, item := range export.OtherActions {
					titles = append(titles, item.Title)
				}

				for _, card := range export.Columns[0].Cards {
					assert.Empty(t, card.MergedCards, "no stack is shown while cards are hidden")
				}

				if slices.Contains(tc.owns, secretID) {
					assert.Equal(t, []string{"Book the next retro"}, titles)
					require.Len(t, export.Columns[0].Cards, 1)
					assert.Equal(t, secretText, export.Columns[0].Cards[0].Body)
					require.Len(t, export.Columns[0].Cards[0].ActionItems, 1)
					assert.Equal(t, "Fix the deploy pipeline with @Sam", export.Columns[0].Cards[0].ActionItems[0].Title)

					return
				}

				assert.Equal(t, []string{"Fix the deploy pipeline with @Sam", "Book the next retro"}, titles)
				assert.NotContains(t, payload, secretText)
			})
		}
	}
}

func TestExport_AnAnonymousBoardCarriesNoParticipantIDsOrDates(t *testing.T) {
	for _, hidden := range []bool{true, false} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("hidden=%t/%s", hidden, tc.name), func(t *testing.T) {
				// given every row on the board was written on 22 September
				state := withActionItems(boardState(model.AnonymityAnonymous, hidden))

				// when
				payload := exportPayload(t, state, tc.viewer)

				// then no timestamp and no participant or user ID leaves, only an item's own due date
				assertExportPrivate(t, state.Header.Board, tc, payload)
				assert.NotContains(t, payload, "2026-09-22")
				assert.Contains(t, payload, `"due_on":"2026-10-20"`)
			})
		}
	}
}

func TestExport_CountsFromThePublishedTallyNotTheRows(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given rows holding two hidden cards and a tally published before Ali's card was saved
			state := boardState(model.AnonymityAnonymous, true)
			published := projection.Tally(state.Header.Board, state.Cards[1:])

			// when
			payload := marshal(t, projection.Export(state, tc.viewer, published))

			// then
			assertExportPrivate(t, state.Header.Board, tc, payload)

			want := float64(1)
			if slices.Contains(tc.owns, secretID) {
				want = 0
			}
			assert.Equal(t, []any{want, float64(0)}, hiddenCountsOf(t, payload))
		})
	}
}

func TestExport_IgnoresTheTallyOnceCardsAreShown(t *testing.T) {
	// given a stale tally on a board whose cards are shown
	state := boardState(model.AnonymityNamed, false)

	// when
	stale := projection.Tally(boardState(model.AnonymityNamed, true).Header.Board, state.Cards)
	export := projection.Export(state, viewerCases()[3].viewer, stale)

	// then
	require.Len(t, export.Columns, 2)
	assert.Zero(t, export.Columns[0].HiddenCount)
	assert.Len(t, export.Columns[0].Cards, 2)
}

func TestExport_ListsAreNeverNull(t *testing.T) {
	// given
	state := model.BoardState{Header: boardState(model.AnonymityNamed, true).Header}

	// when
	payload := marshal(t, projection.Export(state, projection.Viewer{Kind: projection.ViewerExport}, nil))

	// then
	decoded := decode(t, payload)
	assert.Equal(t, []any{}, decoded["columns"])
	assert.Equal(t, []any{}, decoded["other_actions"])
}
