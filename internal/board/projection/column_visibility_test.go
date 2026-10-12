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

const (
	bSamText     = "B-SAM-TEXT-5c1e"
	bHanaText    = "B-HANA-TEXT-8d2f"
	bCommentText = "B-COMMENT-TEXT-0a7c"
	bActionTitle = "Follow up the hidden column"
	bSamPosition = "b0Zx"
	bHanaSpot    = "b0Yw"
)

var (
	bSamCardID  = uuid.MustParse("0b1c2d3e-4f5a-4b6c-8d7e-8f9a0b1c2d3e")
	bHanaCardID = uuid.MustParse("1c2d3e4f-5a6b-4c7d-9e8f-9a0b1c2d3e4f")
	bCommentID  = uuid.MustParse("2d3e4f5a-6b7c-4d8e-af9a-0b1c2d3e4f5a")
	bActionID   = uuid.MustParse("3e4f5a6b-7c8d-4e9f-b0a1-1c2d3e4f5a6b")
)

func columnVisibilityState(anonymity model.Anonymity) model.BoardState {
	state := boardState(anonymity, true)
	state.Header.RevealedColumnIDs = []uuid.UUID{columnA}
	state.Header.HiddenColumnIDs = []uuid.UUID{columnB}

	state.Cards = append(state.Cards,
		model.Card{
			ID:                  bSamCardID,
			BoardID:             boardID,
			ColumnID:            columnB,
			AuthorParticipantID: &samID,
			Body:                bSamText,
			Position:            bSamPosition,
			VoteTotal:           1,
			CreatedAt:           at("2026-09-22T10:06:00Z"),
		},
		model.Card{
			ID:                  bHanaCardID,
			BoardID:             boardID,
			ColumnID:            columnB,
			AuthorParticipantID: &hostID,
			Body:                bHanaText,
			Position:            bHanaSpot,
			CreatedAt:           at("2026-09-22T10:07:00Z"),
		},
	)

	state.Comments = append(state.Comments, model.CardComment{
		ID:                  bCommentID,
		BoardID:             boardID,
		CardID:              bSamCardID,
		AuthorParticipantID: &aliID,
		Body:                bCommentText,
		CreatedAt:           at("2026-09-22T10:08:00Z"),
	})

	state.Votes = append(state.Votes, model.CardVote{CardID: bSamCardID, ParticipantID: aliID, Count: 1, CardAuthorParticipantID: &samID, CardColumnID: columnB})

	state.ActionItems = []model.ActionItem{{
		ID:           bActionID,
		OrgID:        orgID,
		BoardID:      new(boardID),
		SourceCardID: new(bSamCardID),
		Title:        bActionTitle,
		Status:       model.ActionItemOpen,
	}}

	return state
}

func columnBSecrets() []string {
	return []string{
		bSamText,
		bSamCardID.String(),
		`"` + bSamPosition + `"`,
		bCommentText,
		bCommentID.String(),
	}
}

func hanaSecrets() []string {
	return []string{bHanaText, bHanaCardID.String(), `"` + bHanaSpot + `"`}
}

func ownsHanaCard(viewer projection.Viewer) bool {
	return viewer.ParticipantID != nil && *viewer.ParticipantID == hostID
}

func assertNoTimesOnAnAnonymousBoard(t *testing.T, board model.Board, payload string) {
	t.Helper()

	if !board.Anonymous() {
		return
	}

	var tree any
	require.NoError(t, json.Unmarshal([]byte(payload), &tree))
	walk(tree, func(key string, _ any) {
		assert.NotEqual(t, "created_at", key, "an anonymous board carries no created_at")
		assert.NotEqual(t, "updated_at", key, "an anonymous board carries no updated_at")
	})
}

func assertColumnBKept(t *testing.T, state model.BoardState, viewer projection.Viewer, payload string) {
	t.Helper()

	for _, secret := range columnBSecrets() {
		assert.NotContains(t, payload, secret, "another person's hidden card in column B reached the viewer")
	}

	if !slices.Contains(viewer.Capabilities, model.BoardCapabilityFacilitate) {
		assert.NotContains(t, payload, columnB.String(), "a hidden column's ID reached a viewer who does not facilitate")
	}

	if !ownsHanaCard(viewer) {
		for _, secret := range hanaSecrets() {
			assert.NotContains(t, payload, secret, "Hana's hidden card in column B reached someone else")
		}
	}

	assertNoTimesOnAnAnonymousBoard(t, state.Header.Board, payload)
}

func TestColumnVisibility_SnapshotKeepsAHiddenColumnFromParticipantsAndHiddenCardsFromEveryone(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given column A revealed while cards are hidden, and column B hidden from participants
				state := columnVisibilityState(anonymity)

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then
				assertColumnBKept(t, state, tc.viewer, payload)
				assert.Contains(t, payload, aliText, "a revealed column shows every card")
				assert.Contains(t, payload, secretText, "a revealed column shows every card")
				assert.Contains(t, payload, bActionTitle, "an action item stays listed even when its source card cannot be shown")

				decoded := decode(t, payload)
				var columnIDs []string
				for _, raw := range decoded["columns"].([]any) {
					column := raw.(map[string]any)
					columnIDs = append(columnIDs, column["id"].(string))
					if column["id"] == columnA.String() {
						assert.Equal(t, true, column["cards_revealed"])
						assert.Equal(t, false, column["hidden"])
					}
					if column["id"] == columnB.String() {
						assert.Equal(t, false, column["cards_revealed"])
						assert.Equal(t, true, column["hidden"])
					}
				}

				for _, raw := range decoded["cards"].([]any) {
					card := raw.(map[string]any)
					if card["column_id"] == columnA.String() {
						assert.Equal(t, false, card["hidden"], "a card in a revealed column is not hidden")
					}
				}

				wantCounts := map[string]any{}
				if slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate) {
					assert.Equal(t, []string{columnA.String(), columnB.String()}, columnIDs)
					hiddenInB := 2
					if ownsHanaCard(tc.viewer) {
						hiddenInB = 1
						assert.Contains(t, payload, bHanaText, "a facilitator still sees their own card in a hidden column")
					}
					wantCounts[columnB.String()] = float64(hiddenInB)
				} else {
					assert.Equal(t, []string{columnA.String()}, columnIDs)
				}
				assert.Equal(t, wantCounts, decoded["hidden_counts"], "counts leave out revealed columns, and hidden columns for participants")

				if tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == aliID {
					assert.JSONEq(t, `{"votes_remaining":4,"my_votes":{"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6":1}}`, marshal(t, decoded["me"]), "a vote in a hidden column stays spent but unnamed")
				}
			})
		}
	}
}

func TestColumnVisibility_ExportKeepsAHiddenColumnFromParticipants(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given
				state := columnVisibilityState(anonymity)
				tally := projection.Tally(state.Header.Board, state.Cards)

				// when
				export := projection.Export(state, tc.viewer, tally)
				payload := marshal(t, export)

				// then
				assertColumnBKept(t, state, tc.viewer, payload)

				var titles []string
				counts := map[string]int{}
				for _, column := range export.Columns {
					titles = append(titles, column.Title)
					counts[column.Title] = column.HiddenCount
				}

				if !slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate) {
					assert.Equal(t, []string{"Went well"}, titles)
					assert.Equal(t, map[string]int{"Went well": 0}, counts)

					return
				}

				hiddenInB := 2
				if ownsHanaCard(tc.viewer) {
					hiddenInB = 1
				}
				assert.Equal(t, []string{"Went well", "To improve"}, titles)
				assert.Equal(t, map[string]int{"Went well": 0, "To improve": hiddenInB}, counts)
			})
		}
	}
}

func TestColumnVisibility_TallyCountsOnlyColumnsWhoseCardsAreHidden(t *testing.T) {
	// given
	state := columnVisibilityState(model.AnonymityAnonymous)
	board := state.Header.Board

	// when
	tally := projection.Tally(board, state.Cards)

	// then
	assert.Equal(t, projection.HiddenTally{columnB: {samID: 1, hostID: 1}}, tally)
	assert.Empty(t, tally.For(board, viewerCases()[1].viewer), "a participant never receives the count of a hidden column")
	assert.Equal(t, map[uuid.UUID]int{columnB: 2}, tally.For(board, viewerCases()[4].viewer))
}

func TestColumnRevealed_EachOwnerGetsTheirOwnCopyAndNothingOfAnotherColumn(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given column A just revealed, column B still hidden
				state := columnVisibilityState(anonymity)
				event := projection.ColumnRevealed(state, columnA, 1044)

				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then
				require.True(t, seen, "every viewer is told a column was revealed")
				assert.Equal(t, int64(1044), event.Version())
				assert.True(t, event.ClearsColumns(), "the revealed column leaves the hidden tally")
				assertColumnBKept(t, state, tc.viewer, payload)
				assert.NotContains(t, payload, columnB.String())
				for _, secret := range hanaSecrets() {
					assert.NotContains(t, payload, secret)
				}

				decoded := decode(t, payload)
				assert.Equal(t, projection.EventColumnRevealed, decoded["type"])
				assert.Equal(t, columnA.String(), decoded["column_id"])

				var ids []uuid.UUID
				for _, raw := range decoded["cards"].([]any) {
					card := raw.(map[string]any)
					id := uuid.MustParse(card["id"].(string))
					ids = append(ids, id)
					assert.Equal(t, false, card["hidden"])

					isMine, present := card["is_mine"]
					if slices.Contains(tc.owns, id) {
						assert.Equal(t, true, isMine)
					} else {
						assert.False(t, present, "is_mine is left out of a card the viewer did not write")
					}

					if anonymity == model.AnonymityAnonymous {
						assert.Nil(t, card["author"])
					}
				}
				assert.Equal(t, []uuid.UUID{aliCardID, secretID}, ids)

				var commentIDs []uuid.UUID
				for _, raw := range decoded["comments"].([]any) {
					commentIDs = append(commentIDs, uuid.MustParse(raw.(map[string]any)["id"].(string)))
				}
				assert.Equal(t, []uuid.UUID{aliCommentID, secretCommentID}, commentIDs)
			})
		}
	}
}

func TestCardAdded_IntoARevealedColumnReachesEveryoneAndIntoAHiddenOneOnlyItsAuthor(t *testing.T) {
	// given a board hiding cards with column A revealed and column B not
	state := boardState(model.AnonymityNamed, true)
	state.Header.RevealedColumnIDs = []uuid.UUID{columnA}
	board := state.Header.Board
	inA := secretCard(state)
	inB := secretCard(state)
	inB.ColumnID = columnB

	// when
	revealed := projection.CardAdded(board, inA, samProfile(state), 1044)
	hidden := projection.CardAdded(board, inB, samProfile(state), 1045)

	// then
	assert.False(t, revealed.MovesHiddenCards(), "a card in a revealed column is not counted")
	assert.True(t, hidden.MovesHiddenCards())
	for _, tc := range viewerCases() {
		revealedPayload, revealedSeen := seenPayload(t, revealed, tc.viewer)
		require.True(t, revealedSeen, tc.name)
		assert.Contains(t, revealedPayload, `"hidden":false`, tc.name)

		_, hiddenSeen := seenPayload(t, hidden, tc.viewer)
		assert.Equal(t, slices.Contains(tc.owns, secretID), hiddenSeen, tc.name)
	}
}

func TestCardAdded_IntoAHiddenColumnReachesOnlyFacilitators(t *testing.T) {
	// given a board showing cards, with column B hidden from participants
	state := boardState(model.AnonymityNamed, false)
	state.Header.HiddenColumnIDs = []uuid.UUID{columnB}
	card := secretCard(state)
	card.ColumnID = columnB

	// when
	event := projection.CardAdded(state.Header.Board, card, samProfile(state), 1044)

	// then even the author does not hear of it without board.facilitate
	for _, tc := range viewerCases() {
		_, seen := seenPayload(t, event, tc.viewer)
		assert.Equal(t, slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate), seen, tc.name)
	}
}

func TestConcealedEvents_ReachOnlyViewersWhoFacilitate(t *testing.T) {
	// given a board showing cards, with column B hidden
	state := boardState(model.AnonymityNamed, false)
	state.Header.HiddenColumnIDs = []uuid.UUID{columnB}
	board := state.Header.Board
	card := secretCard(state)
	card.ColumnID = columnB
	comment := state.Comments[1]

	events := map[string]projection.Event{
		"column_upserted":  projection.ColumnUpserted(board, state.Columns[1], 1044),
		"column_moved":     projection.ColumnMoved(board, state.Columns[1], nil, 1044),
		"column_deleted":   projection.ColumnDeleted(board, columnB, 1044),
		"column_cleared":   projection.ColumnCleared(board, columnB, 1044),
		"card_upserted":    projection.CardChanged(board, card, samProfile(state), 1044),
		"card_deleted":     projection.CardDeleted(board, card, 1044),
		"votes_changed":    projection.VotesChanged(board, card, aliID, 1044),
		"comment_upserted": projection.CommentUpserted(board, card, comment, samProfile(state), 1044),
		"comment_deleted":  projection.CommentDeleted(board, card, comment, 1044),
	}

	for name, event := range events {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", name, tc.name), func(t *testing.T) {
				// when
				class := event.ClassOf(tc.viewer)

				// then
				if !slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate) {
					assert.Equal(t, projection.ClassNone, class)
					_, personal := event.PayloadFor(tc.viewer)
					assert.False(t, personal)

					return
				}

				assert.NotEqual(t, projection.ClassNone, class)
			})
		}
	}
}

func TestColumnMoved_OthersGetTheRewriteWithoutHiddenColumns(t *testing.T) {
	// given column B hidden and a move of column A that rewrites both keys
	state := boardState(model.AnonymityNamed, false)
	state.Header.HiddenColumnIDs = []uuid.UUID{columnB}
	rewritten := []spec.PositionChange{{ID: columnA, Position: "a2"}, {ID: columnB, Position: "a3"}}
	event := projection.ColumnMoved(state.Header.Board, state.Columns[0], rewritten, 1044)

	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload, seen := seenPayload(t, event, tc.viewer)

			// then
			require.True(t, seen)
			if slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate) {
				assert.Equal(t, projection.ClassStaff, event.ClassOf(tc.viewer))
				assert.JSONEq(t, `{"type":"column_moved","version":1044,"column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","position":"a0","rewritten":[
					{"id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","position":"a2"},{"id":"b8c2d3e4-f5a6-4b7c-ad8e-9fa0b1c2d3e4","position":"a3"}]}`, payload)

				return
			}

			assert.Equal(t, projection.ClassOthers, event.ClassOf(tc.viewer))
			assert.JSONEq(t, `{"type":"column_moved","version":1044,"column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","position":"a0","rewritten":[
				{"id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","position":"a2"}]}`, payload)
		})
	}
}

func TestCardsRevealed_WhileAColumnIsHiddenOnlyFacilitatorsGetItsCards(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given every card just revealed, with column B still hidden from participants
				state := columnVisibilityState(anonymity)
				state.Header.CardsHidden = false
				state.Header.RevealedColumnIDs = nil
				state.Header.Stage = model.StageVote
				event := projection.CardsRevealed(state, 1044)

				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then
				require.True(t, seen)
				assert.Contains(t, payload, aliText)
				assert.Contains(t, payload, secretText)
				assertNoTimesOnAnAnonymousBoard(t, state.Header.Board, payload)

				if !slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate) {
					assert.NotContains(t, payload, columnB.String())
					for _, secret := range append(columnBSecrets(), hanaSecrets()...) {
						assert.NotContains(t, payload, secret)
					}

					return
				}

				assert.Contains(t, payload, bSamText, "once cards are shown a facilitator reads the hidden column")
				assert.Contains(t, payload, bHanaText)
				assert.Contains(t, payload, bCommentText)
				if ownsHanaCard(tc.viewer) {
					assert.Contains(t, payload, `"is_mine":true`)
				} else {
					assert.Equal(t, projection.ClassStaff, event.ClassOf(tc.viewer))
					assert.NotContains(t, payload, "is_mine")
				}
			})
		}
	}
}
