package projection_test

import (
	"fmt"
	"slices"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func secretComment(state model.BoardState) model.CardComment {
	return state.Comments[1]
}

func commentOwner(tc viewerCase) bool {
	return tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == samID
}

func assertCommentObject(t *testing.T, board model.Board, tc viewerCase, comment map[string]any) {
	t.Helper()

	assert.Equal(t, secretCommentID.String(), comment["id"])
	assert.Equal(t, secretID.String(), comment["card_id"])
	assert.Equal(t, secretCommentText, comment["body"])

	isMine, present := comment["is_mine"]
	if commentOwner(tc) {
		assert.Equal(t, true, isMine)
	} else {
		assert.False(t, present, "is_mine must be left out of a comment the viewer did not write")
	}

	if board.Anonymous() {
		assert.Nil(t, comment["author"])
		assert.NotContains(t, comment, "created_at")
		assert.NotContains(t, comment, "updated_at")

		return
	}

	author, ok := comment["author"].(map[string]any)
	require.True(t, ok, "a named board shows who commented")
	assert.Equal(t, samID.String(), author["participant_id"])
	assert.Equal(t, "Sam", author["display_name"])
	assert.NotEmpty(t, comment["created_at"])
}

func TestCommentEvents_PrivacyAcrossEveryViewerAndBoard(t *testing.T) {
	for _, board := range boardMatrix() {
		state := boardState(board.Anonymity, board.CardsHidden)
		card := secretCard(state)
		comment := secretComment(state)

		events := map[string]projection.Event{
			"comment_upserted": projection.CommentUpserted(board, card, comment, samProfile(state), 1044),
			"comment_deleted":  projection.CommentDeleted(board, card, comment, 1044),
		}

		for name, event := range events {
			for _, tc := range viewerCases() {
				t.Run(fmt.Sprintf("%s/%s/hidden=%t/%s", name, board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
					// when
					payload, seen := seenPayload(t, event, tc.viewer)

					// then a comment on a hidden card is hidden with it
					if !slices.Contains(visibleTo(board, tc), secretID) {
						assert.False(t, seen, "a viewer who may not see the card gets nothing about its comments")
						return
					}

					require.True(t, seen)
					assertPrivate(t, board, tc, payload)
					assert.NotContains(t, payload, aliCommentText)

					decoded := decode(t, payload)
					assert.Equal(t, name, decoded["type"])
					if name == "comment_deleted" {
						assert.Equal(t, secretCommentID.String(), decoded["comment_id"])
						assert.Equal(t, secretID.String(), decoded["card_id"])
						assert.NotContains(t, payload, secretCommentText)
						return
					}

					commentObject, ok := decoded["comment"].(map[string]any)
					require.True(t, ok)
					assertCommentObject(t, board, tc, commentObject)
				})
			}
		}
	}
}

func TestCommentUpserted_AnAnonymousCommentNeverNamesItsWriter(t *testing.T) {
	// given Ali comments on Sam's card on an anonymous board after the reveal
	state := boardState(model.AnonymityAnonymous, false)
	comment := model.CardComment{ID: aliCommentID, BoardID: boardID, CardID: secretID, AuthorParticipantID: &aliID, Body: aliCommentText, CreatedAt: at("2026-09-22T10:06:00Z")}
	event := projection.CommentUpserted(state.Header.Board, secretCard(state), comment, &state.Participants[1], 1044)

	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload, seen := seenPayload(t, event, tc.viewer)

			// then
			require.True(t, seen)
			assert.NotContains(t, withoutRoster(t, payload), aliID.String())
			assert.NotContains(t, payload, "Ali")
			assert.NotContains(t, payload, "created_at")
		})
	}
}

func TestCommentResponses_PrivacyAcrossEveryViewerAndBoard(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given
				state := boardState(board.Anonymity, board.CardsHidden)
				card := secretCard(state)
				comment := secretComment(state)

				// when
				response := marshal(t, projection.CommentResponse(board, card, comment, samProfile(state), tc.viewer, 1044))
				replayed := marshal(t, projection.ReplayCommentResponse(board, card, comment, samProfile(state), tc.viewer, 1044))

				// then
				assertPrivate(t, board, tc, response)
				if !slices.Contains(visibleTo(board, tc), secretID) {
					assert.JSONEq(t, `{"version":1044}`, response)
				} else {
					commentObject, ok := decode(t, response)["comment"].(map[string]any)
					require.True(t, ok)
					assertCommentObject(t, board, tc, commentObject)
				}

				if !commentOwner(tc) {
					assert.JSONEq(t, `{"version":1044,"replayed":true,"id":"e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c"}`, replayed)
					return
				}

				assertPrivate(t, board, tc, replayed)
				commentObject, ok := decode(t, replayed)["comment"].(map[string]any)
				require.True(t, ok)
				assertCommentObject(t, board, tc, commentObject)
			})
		}
	}
}

func TestCommentUpserted_GoldenForTheWriterAndForEveryoneElse(t *testing.T) {
	// given a named board with cards shown
	state := boardState(model.AnonymityNamed, false)
	event := projection.CommentUpserted(state.Header.Board, secretCard(state), secretComment(state), samProfile(state), 1044)

	// when
	writer, _ := seenPayload(t, event, viewerCases()[0].viewer)
	other, _ := seenPayload(t, event, viewerCases()[1].viewer)

	// then
	assert.JSONEq(t, `{"type":"comment_upserted","version":1044,"comment":{
		"id":"e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c","card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","body":"SECRET-COMMENT-3a9d",
		"author":{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":"Sam"},
		"is_mine":true,"created_at":"2026-09-22T10:05:00.000Z"}}`, writer)
	assert.JSONEq(t, `{"type":"comment_upserted","version":1044,"comment":{
		"id":"e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c","card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","body":"SECRET-COMMENT-3a9d",
		"author":{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":"Sam"},
		"created_at":"2026-09-22T10:05:00.000Z"}}`, other)
}

func TestCommentUpserted_AWriterWhoCannotSeeTheCardIsToldNothing(t *testing.T) {
	// given a comment Ali wrote on Sam's card, which has since been hidden again
	state := boardState(model.AnonymityNamed, true)
	comment := model.CardComment{ID: aliCommentID, BoardID: boardID, CardID: secretID, AuthorParticipantID: &aliID, Body: aliCommentText}

	// when
	event := projection.CommentUpserted(state.Header.Board, secretCard(state), comment, &state.Participants[1], 1044)

	// then
	for _, tc := range viewerCases() {
		_, seen := seenPayload(t, event, tc.viewer)
		assert.False(t, seen, tc.name)
	}
}

func TestCardsMergedAndUnmerged_ReachOnlyThoseWhoSeeTheCard(t *testing.T) {
	for _, board := range boardMatrix() {
		state := boardState(board.Anonymity, board.CardsHidden)
		stacked := secretCard(state)
		stacked.ParentCardID = new(aliCardID)

		events := map[string]projection.Event{
			"cards_merged":  projection.CardsMerged(board, model.CardMerge{Card: stacked, Restacked: []uuid.UUID{}}, 1044),
			"card_unmerged": projection.CardUnmerged(board, secretCard(state), nil, 1044),
		}

		for name, event := range events {
			for _, tc := range viewerCases() {
				t.Run(fmt.Sprintf("%s/%s/hidden=%t/%s", name, board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
					// when
					payload, seen := seenPayload(t, event, tc.viewer)

					// then
					assert.Equal(t, slices.Contains(visibleTo(board, tc), secretID), seen)
					if !seen {
						return
					}

					assert.NotContains(t, payload, secretText)
					assert.NotContains(t, payload, samID.String(), "a merge never names who wrote the card")
					assert.Equal(t, name, decode(t, payload)["type"])
				})
			}
		}
	}
}

func TestCardsMerged_Golden(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)
	stacked := secretCard(state)
	stacked.ParentCardID = new(aliCardID)
	stacked.Position = "a0Vz"

	// when
	payload, _ := seenPayload(t, projection.CardsMerged(state.Header.Board, model.CardMerge{Card: stacked, Restacked: []uuid.UUID{secretCommentID}}, 1044), viewerCases()[2].viewer)

	// then
	assert.JSONEq(t, `{"type":"cards_merged","version":1044,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
		"parent_card_id":"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b","column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3",
		"position":"a0Vz","restacked":["e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c"]}`, payload)
}

func TestCardUnmerged_Golden(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)

	// when
	payload, _ := seenPayload(t, projection.CardUnmerged(state.Header.Board, secretCard(state), nil, 1044), viewerCases()[2].viewer)

	// then
	assert.JSONEq(t, `{"type":"card_unmerged","version":1044,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
		"column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","position":"a0Vq","rewritten":[]}`, payload)
}
