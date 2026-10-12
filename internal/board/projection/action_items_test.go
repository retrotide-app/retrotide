package projection_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	convertedItemID = uuid.MustParse("a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d")
	directItemID    = uuid.MustParse("b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e")
	previousItemID  = uuid.MustParse("c3d4e5f6-a7b8-4c9d-8e0f-2a3b4c5d6e7f")
	aliUserID       = uuid.MustParse("d4e5f6a7-b8c9-4d0e-9f1a-3b4c5d6e7f80")
	samUserID       = uuid.MustParse("7f3a4b5c-6d7e-4f8a-9b9c-0d1e2f3a4b5c")
	olderBoardID    = uuid.MustParse("e5f6a7b8-c9d0-4e1f-8a2b-4c5d6e7f8091")
	dueDate         = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
)

func convertedItem() model.ActionItem {
	return model.ActionItem{
		ID:             convertedItemID,
		OrgID:          orgID,
		TeamID:         new(teamID),
		BoardID:        new(boardID),
		SourceCardID:   new(secretID),
		Title:          "Fix the deploy pipeline with @Sam",
		AssigneeUserID: new(aliUserID),
		AssigneeName:   "Alison",
		DueOn:          new(dueDate),
		Status:         model.ActionItemOpen,
		CreatedBy:      new(samUserID),
		CreatedAt:      at("2026-09-22T11:00:00Z"),
		UpdatedAt:      at("2026-09-22T11:05:00Z"),
		Mentions:       []model.ActionItemPerson{{UserID: samUserID, DisplayName: "Sam"}},
	}
}

func directItem() model.ActionItem {
	completed := at("2026-09-22T11:30:00Z")

	return model.ActionItem{
		ID:          directItemID,
		OrgID:       orgID,
		TeamID:      new(teamID),
		BoardID:     new(boardID),
		Title:       "Book the next retro",
		Status:      model.ActionItemDone,
		CompletedAt: &completed,
		CreatedAt:   at("2026-09-22T11:10:00Z"),
		UpdatedAt:   at("2026-09-22T11:30:00Z"),
		Mentions:    []model.ActionItemPerson{},
	}
}

func previousAction() model.PreviousAction {
	carried := model.ReviewCarried

	return model.PreviousAction{
		ID:             previousItemID,
		OrgID:          orgID,
		TeamID:         new(teamID),
		BoardID:        new(olderBoardID),
		SourceCardID:   new(aliCardID),
		Title:          "Pair on the flaky test",
		AssigneeUserID: new(samUserID),
		AssigneeName:   "Sam",
		Status:         model.ActionItemOpen,
		CreatedBy:      new(samUserID),
		CreatedAt:      at("2026-09-08T11:00:00Z"),
		UpdatedAt:      at("2026-09-08T11:00:00Z"),
		Mentions:       []model.ActionItemPerson{},
		Review:         &carried,
	}
}

func withActionItems(state model.BoardState) model.BoardState {
	state.ActionItems = []model.ActionItem{convertedItem(), directItem()}
	state.PreviousActions = []model.PreviousAction{previousAction()}

	return state
}

func assertNoActionItemTimes(t *testing.T, items []any) {
	t.Helper()

	for _, raw := range items {
		item := raw.(map[string]any)
		for _, key := range []string{"created_at", "updated_at", "completed_at", "created_by", "org_id", "team_id", "board_id"} {
			assert.NotContains(t, item, key, "an action item on a board carries no %s", key)
		}
	}
}

func TestSnapshot_ActionItemsReachEveryViewerWithoutTimesOrAHiddenSourceCard(t *testing.T) {
	type boardCase struct {
		name  string
		state func(model.Anonymity, bool) model.BoardState
		cases []viewerCase
	}

	boards := []boardCase{
		{name: "team board", state: boardState, cases: viewerCases()},
		{name: "organisation board", state: organisationBoardState, cases: organisationViewerCases()},
	}

	for _, kind := range boards {
		for _, board := range boardMatrix() {
			for _, tc := range kind.cases {
				t.Run(fmt.Sprintf("%s/%s/hidden=%t/%s", kind.name, board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
					// given a board with a converted card, a direct action item and a previous action
					state := withActionItems(kind.state(board.Anonymity, board.CardsHidden))

					// when
					payload := marshal(t, projection.Snapshot(state, tc.viewer))

					// then every viewer sees every action item, and nothing about a card they may not see
					assertPrivate(t, board, tc, payload)

					decoded := decode(t, payload)
					items, ok := decoded["action_items"].([]any)
					require.True(t, ok, "action_items is always a list")
					require.Len(t, items, 2)
					assertNoActionItemTimes(t, items)

					converted := items[0].(map[string]any)
					assert.Equal(t, convertedItemID.String(), converted["id"])
					if slices.Contains(visibleTo(board, tc), secretID) {
						assert.Equal(t, secretID.String(), converted["source_card_id"])
					} else {
						assert.Nil(t, converted["source_card_id"], "a converted card hidden from the viewer is never named")
					}

					previous, ok := decoded["previous_actions"].([]any)
					require.True(t, ok, "previous_actions is always a list")
					require.Len(t, previous, 1)
					assertNoActionItemTimes(t, previous)
					assert.Nil(t, previous[0].(map[string]any)["source_card_id"], "a previous action never names a card of another board")
					assert.Equal(t, "carried", previous[0].(map[string]any)["review"])
				})
			}
		}
	}
}

func TestActionItemEvents_ReachEveryViewerAlikeAndCarryNoTimes(t *testing.T) {
	type boardCase struct {
		name   string
		teamID *uuid.UUID
		cases  []viewerCase
	}

	boards := []boardCase{
		{name: "team board", teamID: new(teamID), cases: viewerCases()},
		{name: "organisation board", cases: organisationViewerCases()},
	}

	for _, kind := range boards {
		for _, board := range boardMatrix() {
			converted := convertedItem()
			converted.TeamID = kind.teamID
			previous := previousAction()
			previous.TeamID = kind.teamID

			events := map[string]projection.Event{
				"action_item_upserted":     projection.ActionItemUpserted(board, converted, 1050),
				"action_item_removed":      projection.ActionItemRemoved(convertedItemID, 1050),
				"previous_action_upserted": projection.PreviousActionUpserted(previous, 1050),
			}

			for name, event := range events {
				var first string

				for _, tc := range kind.cases {
					t.Run(fmt.Sprintf("%s/%s/%s/hidden=%t/%s", kind.name, name, board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
						// when
						payload, seen := seenPayload(t, event, tc.viewer)

						// then
						require.True(t, seen, "an action item is board content every viewer sees")
						assert.Equal(t, projection.ClassOthers, event.ClassOf(tc.viewer))
						assertPrivate(t, board, tc, payload)
						assert.NotContains(t, payload, "created_at")
						assert.NotContains(t, payload, "completed_at")
						assert.NotContains(t, payload, samID.String(), "an action item never names a participant")

						if board.CardsHidden {
							assert.NotContains(t, payload, secretID.String(), "a hidden card is never named as a source")
						}

						if first == "" {
							first = payload
						}
						assert.Equal(t, first, payload, "one payload for everyone")
					})
				}
			}
		}
	}
}

func TestActionItemUpserted_Golden(t *testing.T) {
	// given
	board := boardState(model.AnonymityNamed, false).Header.Board

	// when
	payload, _ := seenPayload(t, projection.ActionItemUpserted(board, convertedItem(), 1050), projection.Viewer{})

	// then
	assert.JSONEq(t, `{
		"type": "action_item_upserted",
		"version": 1050,
		"action_item": {
			"id": "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
			"title": "Fix the deploy pipeline with @Sam",
			"assignee": {"user_id": "d4e5f6a7-b8c9-4d0e-9f1a-3b4c5d6e7f80", "display_name": "Alison"},
			"mentions": [{"user_id": "7f3a4b5c-6d7e-4f8a-9b9c-0d1e2f3a4b5c", "display_name": "Sam"}],
			"due_on": "2026-10-20",
			"status": "open",
			"source_card_id": "c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6"
		}
	}`, payload)
}

func TestActionItemRemovedAndPreviousActionUpserted_Golden(t *testing.T) {
	// when
	removed, _ := seenPayload(t, projection.ActionItemRemoved(convertedItemID, 1051), projection.Viewer{})
	reviewed, _ := seenPayload(t, projection.PreviousActionUpserted(previousAction(), 1052), projection.Viewer{})

	// then
	assert.JSONEq(t, `{"type":"action_item_removed","version":1051,"action_item_id":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d"}`, removed)
	assert.JSONEq(t, `{
		"type": "previous_action_upserted",
		"version": 1052,
		"previous_action": {
			"id": "c3d4e5f6-a7b8-4c9d-8e0f-2a3b4c5d6e7f",
			"title": "Pair on the flaky test",
			"assignee": {"user_id": "7f3a4b5c-6d7e-4f8a-9b9c-0d1e2f3a4b5c", "display_name": "Sam"},
			"mentions": [],
			"due_on": null,
			"status": "open",
			"source_card_id": null,
			"review": "carried"
		}
	}`, reviewed)
}

func TestActionItemResponses_AnswerTheItemAndAReplayKeepsItsID(t *testing.T) {
	// given
	board := boardState(model.AnonymityAnonymous, true).Header.Board

	// when
	creator := projection.Viewer{Kind: projection.ViewerMember, UserID: new(samUserID), ParticipantID: &samID}
	answered := marshal(t, projection.ActionItemResponse(board, directItem(), creator, 1053))
	replayed := marshal(t, projection.ReplayActionItemResponse(board, convertedItem(), creator, 1054))
	reviewed := marshal(t, projection.PreviousActionResponse(previousAction(), 1055))

	// then a board that hides cards never names the source, and no response carries a time
	assert.JSONEq(t, `{
		"version": 1053,
		"action_item": {"id": "b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e", "title": "Book the next retro", "assignee": null, "mentions": [], "due_on": null, "status": "done", "source_card_id": null}
	}`, answered)
	assert.NotContains(t, replayed, secretID.String())
	assert.NotContains(t, replayed, "is_mine", "an anonymous board never tells even the creator that an item is theirs")
	assert.Contains(t, replayed, `"replayed":true`)
	assert.Contains(t, replayed, convertedItemID.String())
	assert.Contains(t, reviewed, `"previous_action"`)
	assert.NotContains(t, reviewed, "created_at")
}

func TestPeople_NamesEachPersonByUserAndNameOnly(t *testing.T) {
	// given
	people := []model.ActionItemPerson{{UserID: aliUserID, DisplayName: "Alison"}, {UserID: samUserID, DisplayName: "Sam"}}

	// when
	payload := marshal(t, projection.People(people))

	// then
	assert.JSONEq(t, `{"people":[
		{"user_id":"d4e5f6a7-b8c9-4d0e-9f1a-3b4c5d6e7f80","display_name":"Alison"},
		{"user_id":"7f3a4b5c-6d7e-4f8a-9b9c-0d1e2f3a4b5c","display_name":"Sam"}
	]}`, payload)
}

func TestActionItemList_GoldenWithCountsAndTheCompletionRate(t *testing.T) {
	// given an item of a team board and one of an anonymous organisation board
	orgItem := directItem()
	orgItem.TeamID = nil
	page := model.ActionItemPage{
		Items: []model.ActionItemListing{
			{ActionItem: convertedItem(), BoardTitle: "Sprint 42", BoardAnonymity: model.AnonymityNamed, TeamName: "Payments"},
			{ActionItem: orgItem, BoardTitle: "All hands", BoardAnonymity: model.AnonymityAnonymous},
		},
		Counts: model.ActionItemCounts{Open: 1, Done: 3, Dropped: 2},
	}

	// when
	payload := marshal(t, projection.ActionItemList(page, 50, 0, 6))

	// then dropped items count for nothing in the rate, and no listing carries a time or a source card
	assert.JSONEq(t, `{
		"action_items": [
			{
				"id": "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
				"title": "Fix the deploy pipeline with @Sam",
				"assignee": {"user_id": "d4e5f6a7-b8c9-4d0e-9f1a-3b4c5d6e7f80", "display_name": "Alison"},
				"mentions": [{"user_id": "7f3a4b5c-6d7e-4f8a-9b9c-0d1e2f3a4b5c", "display_name": "Sam"}],
				"due_on": "2026-10-20",
				"status": "open",
				"source_card_id": null,
				"board": {"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b", "title": "Sprint 42"},
				"team_id": "1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0",
				"team_name": "Payments"
			},
			{
				"id": "b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e",
				"title": "Book the next retro",
				"assignee": null,
				"mentions": [],
				"due_on": null,
				"status": "done",
				"source_card_id": null,
				"board": {"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b", "title": "All hands"}
			}
		],
		"total": 6,
		"limit": 50,
		"offset": 0,
		"counts": {"open": 1, "done": 3, "dropped": 2},
		"completion_rate": 0.75
	}`, payload)
}

func TestActionItemList_HasNoRateWithNothingOpenOrDone(t *testing.T) {
	// given
	page := model.ActionItemPage{Counts: model.ActionItemCounts{Dropped: 2}}

	// when
	payload := marshal(t, projection.ActionItemList(page, 50, 0, 2))

	// then
	decoded := decode(t, payload)
	assert.Nil(t, decoded["completion_rate"])
	assert.Equal(t, []any{}, decoded["action_items"])
}

func TestMentions_GoldenWithAndWithoutABoard(t *testing.T) {
	// given
	mentions := []model.Mention{
		{Kind: model.MentionActionItem, ActionItemID: convertedItemID, OrgID: orgID, OrgSlug: "acme", OrgName: "Acme", TeamID: new(teamID), BoardID: new(boardID), BoardTitle: "Sprint 42", Title: "Fix the deploy pipeline with @Sam", Status: model.ActionItemOpen, DueOn: new(dueDate)},
		{Kind: model.MentionActionItem, ActionItemID: previousItemID, OrgID: orgID, OrgSlug: "acme", OrgName: "Acme", TeamID: new(teamID), Title: "Pair on the flaky test", Status: model.ActionItemDone},
	}

	// when
	payload := marshal(t, projection.Mentions(mentions))

	// then
	assert.JSONEq(t, `{"mentions":[
		{"kind":"action_item","action_item_id":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","title":"Fix the deploy pipeline with @Sam","status":"open","due_on":"2026-10-20","org_id":"2c3d4e5f-6071-4283-94a5-b6c7d8e9f0a1","org_slug":"acme","org_name":"Acme","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","board_id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","board_title":"Sprint 42"},
		{"kind":"action_item","action_item_id":"c3d4e5f6-a7b8-4c9d-8e0f-2a3b4c5d6e7f","title":"Pair on the flaky test","status":"done","due_on":null,"org_id":"2c3d4e5f-6071-4283-94a5-b6c7d8e9f0a1","org_slug":"acme","org_name":"Acme","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0"}
	]}`, payload)
}
