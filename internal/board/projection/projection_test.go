package projection_test

import (
	"encoding/json"
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

type (
	viewerCase struct {
		name   string
		viewer projection.Viewer
		owns   []uuid.UUID
	}
)

const (
	secretText        = "SECRET-TEXT-9f2c"
	aliText           = "ALI-TEXT-41d0"
	secretCommentText = "SECRET-COMMENT-3a9d"
	aliCommentText    = "ALI-COMMENT-7e1b"
)

var (
	boardID   = uuid.MustParse("8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b")
	teamID    = uuid.MustParse("1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0")
	orgID     = uuid.MustParse("2c3d4e5f-6071-4283-94a5-b6c7d8e9f0a1")
	columnA   = uuid.MustParse("a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3")
	columnB   = uuid.MustParse("b8c2d3e4-f5a6-4b7c-ad8e-9fa0b1c2d3e4")
	samID     = uuid.MustParse("9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6")
	aliID     = uuid.MustParse("2a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d")
	hostID    = uuid.MustParse("3b9c0d1e-2f3a-4b4c-9d5e-6f7a8b9c0d1e")
	ownerID   = uuid.MustParse("4c0d1e2f-3a4b-4c5d-ae6f-7a8b9c0d1e2f")
	guestID   = uuid.MustParse("5d1e2f3a-4b5c-4d6e-bf7a-8b9c0d1e2f3a")
	removedID = uuid.MustParse("6e2f3a4b-5c6d-4e7f-8a8b-9c0d1e2f3a4b")
	secretID  = uuid.MustParse("c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6")
	aliCardID = uuid.MustParse("d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b")

	secretCommentID = uuid.MustParse("e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c")
	aliCommentID    = uuid.MustParse("f6b9c0d1-e2f3-4a4b-9c5d-6e7f8a9b0c1d")

	participantCapabilities = []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityParticipate}
	facilitatorCapabilities = []model.BoardCapability{
		model.BoardCapabilityView,
		model.BoardCapabilityParticipate,
		model.BoardCapabilityFacilitate,
		model.BoardCapabilityManage,
		model.BoardCapabilityExport,
	}

	cardPositions = map[uuid.UUID]string{secretID: "a0Vq", aliCardID: "a0F"}
	cardBodies    = map[uuid.UUID]string{secretID: secretText, aliCardID: aliText}
	cardAuthors   = map[uuid.UUID]uuid.UUID{secretID: samID, aliCardID: aliID}
	cardColumns   = map[uuid.UUID]uuid.UUID{secretID: columnA, aliCardID: columnA}
	cardComments  = map[uuid.UUID]struct {
		id   uuid.UUID
		body string
	}{
		secretID:  {id: secretCommentID, body: secretCommentText},
		aliCardID: {id: aliCommentID, body: aliCommentText},
	}
)

func at(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}

	return parsed
}

func boardState(anonymity model.Anonymity, hidden bool) model.BoardState {
	amber := model.ColourAmber
	edited := at("2026-09-22T10:03:00.25Z")
	removedAt := at("2026-09-22T10:30:00Z")
	samUser := uuid.MustParse("7f3a4b5c-6d7e-4f8a-9b9c-0d1e2f3a4b5c")
	stage := model.StageVote
	if hidden {
		stage = model.StageWrite
	}

	header := model.BoardHeader{
		ID:                 boardID,
		OrgID:              orgID,
		TeamID:             new(teamID),
		Title:              "Sprint 42",
		ShareSlug:          "k3v9x2",
		LinkAccess:         model.LinkAccessOff,
		Version:            1043,
		CreatedBy:          &samUser,
		CreatedAt:          at("2026-09-22T09:00:00Z"),
		UpdatedAt:          at("2026-09-22T10:03:00Z"),
		Context:            "Payments squad",
		TemplateKey:        "went-well",
		Anonymity:          anonymity,
		Stage:              stage,
		CardsHidden:        hidden,
		ShowPrimeDirective: true,
		SortMode:           model.SortModeManual,
		VotingOpen:         true,
		MaxVotes:           6,
		CommentsEnabled:    true,
		MergingEnabled:     true,
	}

	return model.BoardState{
		Header: header,
		Columns: []model.BoardColumn{
			{ID: columnA, BoardID: boardID, Title: "Went well", Description: "What helped", Colour: model.ColourGreen, Position: "a0"},
			{ID: columnB, BoardID: boardID, Title: "To improve", Colour: model.ColourRed, Position: "a1"},
		},
		Cards: []model.Card{
			{
				ID:                  aliCardID,
				BoardID:             boardID,
				ColumnID:            columnA,
				AuthorParticipantID: &aliID,
				Body:                aliText,
				Position:            cardPositions[aliCardID],
				CreatedAt:           at("2026-09-22T10:02:40.118Z"),
			},
			{
				ID:                  secretID,
				BoardID:             boardID,
				ColumnID:            columnA,
				AuthorParticipantID: &samID,
				Body:                secretText,
				Colour:              &amber,
				Position:            cardPositions[secretID],
				VoteTotal:           3,
				CreatedAt:           at("2026-09-22T10:01:02.512Z"),
				UpdatedAt:           &edited,
			},
		},
		Participants: []model.BoardParticipantProfile{
			{ID: samID, BoardID: boardID, UserID: &samUser, Role: model.BoardRoleParticipant, AccountName: "Sam"},
			{ID: aliID, BoardID: boardID, DisplayName: "Ali", Role: model.BoardRoleParticipant, AccountName: "Alison"},
			{ID: hostID, BoardID: boardID, DisplayName: "Hana", Role: model.BoardRoleFacilitator},
			{ID: ownerID, BoardID: boardID, DisplayName: "Olu", Role: model.BoardRoleFacilitator},
			{ID: guestID, BoardID: boardID, DisplayName: "Gil", Role: model.BoardRoleParticipant},
			{ID: removedID, BoardID: boardID, DisplayName: "Rae", Role: model.BoardRoleParticipant, RemovedAt: &removedAt},
		},
		Votes: []model.CardVote{
			{CardID: secretID, ParticipantID: samID, Count: 2, CardAuthorParticipantID: &samID, CardColumnID: columnA},
			{CardID: secretID, ParticipantID: aliID, Count: 1, CardAuthorParticipantID: &samID, CardColumnID: columnA},
		},
		Comments: []model.CardComment{
			{ID: aliCommentID, BoardID: boardID, CardID: aliCardID, AuthorParticipantID: &aliID, Body: aliCommentText, CreatedAt: at("2026-09-22T10:04:00Z")},
			{ID: secretCommentID, BoardID: boardID, CardID: secretID, AuthorParticipantID: &samID, Body: secretCommentText, CreatedAt: at("2026-09-22T10:05:00Z")},
		},
	}
}

func viewerCases() []viewerCase {
	return []viewerCase{
		{
			name:   "author",
			viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &samID, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities},
			owns:   []uuid.UUID{secretID},
		},
		{
			name:   "non-author",
			viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &aliID, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities},
			owns:   []uuid.UUID{aliCardID},
		},
		{
			name:   "guest",
			viewer: projection.Viewer{Kind: projection.ViewerGuest, ParticipantID: &guestID, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities},
		},
		{
			name:   "facilitator",
			viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &hostID, Role: model.BoardRoleFacilitator, Capabilities: facilitatorCapabilities},
		},
		{
			name:   "organisation owner",
			viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &ownerID, Role: model.BoardRoleFacilitator, Capabilities: facilitatorCapabilities},
		},
		{
			name:   "site staff",
			viewer: projection.Viewer{Kind: projection.ViewerMember},
		},
		{
			name:   "removed participant",
			viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &removedID, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities},
		},
		{
			name:   "export",
			viewer: projection.Viewer{Kind: projection.ViewerExport, ParticipantID: &hostID, Role: model.BoardRoleFacilitator, Capabilities: facilitatorCapabilities},
		},
	}
}

func boardMatrix() []model.Board {
	var boards []model.Board
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, hidden := range []bool{true, false} {
			boards = append(boards, boardState(anonymity, hidden).Header.Board)
		}
	}

	return boards
}

func marshal(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return string(data)
}

func decode(t *testing.T, payload string) map[string]any {
	t.Helper()

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &decoded))

	return decoded
}

func withoutRoster(t *testing.T, payload string) string {
	t.Helper()

	decoded := decode(t, payload)
	delete(decoded, "participants")
	delete(decoded, "viewer")

	return marshal(t, decoded)
}

func walk(node any, visit func(key string, value any)) {
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			visit(key, value)
			walk(value, visit)
		}

	case []any:
		for _, value := range typed {
			visit("", value)
			walk(value, visit)
		}
	}
}

func visibleTo(board model.Board, tc viewerCase) []uuid.UUID {
	if !board.CardsHidden {
		return []uuid.UUID{aliCardID, secretID}
	}

	var visible []uuid.UUID
	for _, id := range []uuid.UUID{aliCardID, secretID} {
		if slices.Contains(tc.owns, id) {
			visible = append(visible, id)
		}
	}

	return visible
}

func assertCardObject(t *testing.T, board model.Board, tc viewerCase, card map[string]any) {
	t.Helper()

	id := uuid.MustParse(card["id"].(string))
	assert.Equal(t, cardBodies[id], card["body"])
	assert.Equal(t, cardPositions[id], card["position"])
	assert.Equal(t, cardColumns[id].String(), card["column_id"])
	assert.Equal(t, board.CardsHidden, card["hidden"])

	isMine, present := card["is_mine"]
	if slices.Contains(tc.owns, id) {
		assert.Equal(t, true, isMine)
	} else {
		assert.False(t, present, "is_mine must be left out of a card the viewer did not write")
	}

	if board.Anonymous() {
		assert.Nil(t, card["author"])
		assert.NotContains(t, card, "created_at")
		assert.NotContains(t, card, "updated_at")

		return
	}

	author, ok := card["author"].(map[string]any)
	require.True(t, ok, "a named board shows the author")
	assert.Equal(t, cardAuthors[id].String(), author["participant_id"])
	assert.NotEmpty(t, card["created_at"])
}

func assertPrivate(t *testing.T, board model.Board, tc viewerCase, payload string) {
	t.Helper()

	visible := visibleTo(board, tc)
	outsideRoster := withoutRoster(t, payload)

	for _, id := range []uuid.UUID{aliCardID, secretID} {
		if slices.Contains(visible, id) {
			continue
		}
		assert.NotContains(t, payload, cardBodies[id], "hidden text reached the viewer")
		assert.NotContains(t, payload, id.String(), "a hidden card's ID reached the viewer")
		assert.NotContains(t, payload, `"`+cardPositions[id]+`"`, "a hidden card's position reached the viewer")
		assert.NotContains(t, payload, cardComments[id].body, "a comment on a hidden card reached the viewer")
		assert.NotContains(t, payload, cardComments[id].id.String(), "the ID of a comment on a hidden card reached the viewer")
	}

	shownAuthors := map[uuid.UUID]struct{}{}
	if !board.Anonymous() {
		for _, id := range visible {
			shownAuthors[cardAuthors[id]] = struct{}{}
		}
	}

	for _, author := range []uuid.UUID{samID, aliID} {
		if _, shown := shownAuthors[author]; shown {
			continue
		}
		assert.NotContains(t, outsideRoster, author.String(), "an author's participant ID reached the viewer outside the roster")
	}

	var tree any
	require.NoError(t, json.Unmarshal([]byte(payload), &tree))
	walk(tree, func(key string, value any) {
		if board.Anonymous() {
			assert.NotEqual(t, "created_at", key, "an anonymous board carries no created_at")
			assert.NotEqual(t, "updated_at", key, "an anonymous board carries no updated_at")
		}

		text, ok := value.(string)
		if !ok {
			return
		}

		parsed, err := uuid.Parse(text)
		if err != nil {
			return
		}
		assert.Equal(t, uuid.Version(4), parsed.Version(), "every ID is v4, never v7: %s", text)
	})
}

func TestSnapshot_PrivacyAcrossEveryViewerAndBoard(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given
				state := boardState(board.Anonymity, board.CardsHidden)

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then
				assertPrivate(t, board, tc, payload)

				decoded := decode(t, payload)
				cards, ok := decoded["cards"].([]any)
				require.True(t, ok, "cards is always a list")

				var ids []uuid.UUID
				for _, raw := range cards {
					card := raw.(map[string]any)
					ids = append(ids, uuid.MustParse(card["id"].(string)))
					assertCardObject(t, board, tc, card)
				}
				assert.Equal(t, visibleTo(board, tc), ids)

				wantCounts := map[string]any{}
				if board.CardsHidden {
					hiddenInA := 2 - len(tc.owns)
					if hiddenInA > 0 {
						wantCounts[columnA.String()] = float64(hiddenInA)
					}
				}
				assert.Equal(t, wantCounts, decoded["hidden_counts"])
			})
		}
	}
}

func TestCardResponse_PrivacyAcrossEveryViewerAndBoard(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given
				state := boardState(board.Anonymity, board.CardsHidden)
				card := state.Cards[1]
				author := state.Participants[0]

				// when
				payload := marshal(t, projection.CardResponse(board, card, &author, tc.viewer, 1044))

				// then
				assertPrivate(t, board, tc, payload)

				decoded := decode(t, payload)
				if !slices.Contains(visibleTo(board, tc), secretID) {
					assert.JSONEq(t, `{"version":1044}`, payload)

					return
				}

				cardObject, ok := decoded["card"].(map[string]any)
				require.True(t, ok)
				assertCardObject(t, board, tc, cardObject)
			})
		}
	}
}

func TestSnapshot_GoldenAuthorOnANamedBoardWithCardsHidden(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, true)
	viewer := viewerCases()[0].viewer

	// when
	payload := marshal(t, projection.Snapshot(state, viewer))

	// then
	assert.JSONEq(t, `{
		"board_id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"version": 1043,
		"board": {
			"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
			"team_id": "1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0",
			"title": "Sprint 42",
			"context": "Payments squad",
			"template_key": "went-well",
			"anonymity": "named",
			"stage": "write",
			"cards_hidden": true,
			"closed": false,
			"show_prime_directive": true,
			"created_at": "2026-09-22T09:00:00.000Z"
		},
		"settings": {
			"voting_open": true,
			"vote_counts_hidden": false,
			"max_votes": 6,
			"max_votes_per_card": 0,
			"comments_enabled": true,
			"merging_enabled": true,
			"sort_mode": "manual",
			"cards_locked": false,
			"moves_locked": false,
			"board_locked": false,
			"timer_play_sound": false,
			"timer_auto_lock": false,
			"sort_enforced": false,
			"anyone_can_edit": false,
			"show_card_dates": false
		},
		"timer": null,
		"action_items": [],
		"previous_actions": [],
		"spotlight": {"presenting_card_id": null, "highlighted_card_id": null, "focused_column_id": null},
		"surveys": [],
		"viewer": {
			"participant_id": "9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6",
			"role": "participant",
			"capabilities": ["board.view", "board.participate"]
		},
		"me": {
			"votes_remaining": 4,
			"my_votes": {"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6": 2}
		},
		"columns": [
			{"id": "a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3", "title": "Went well", "description": "What helped", "colour": "green", "position": "a0", "max_votes": null, "cards_revealed": false, "hidden": false},
			{"id": "b8c2d3e4-f5a6-4b7c-ad8e-9fa0b1c2d3e4", "title": "To improve", "description": "", "colour": "red", "position": "a1", "max_votes": null, "cards_revealed": false, "hidden": false}
		],
		"cards": [
			{
				"id": "c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
				"column_id": "a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3",
				"parent_card_id": null,
				"position": "a0Vq",
				"body": "SECRET-TEXT-9f2c",
				"colour": "amber",
				"author": {"participant_id": "9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6", "display_name": "Sam"},
				"is_mine": true,
				"hidden": true,
				"votes": {"total": 3},
				"created_at": "2026-09-22T10:01:02.512Z",
				"updated_at": "2026-09-22T10:03:00.250Z"
			}
		],
		"comments": [
			{
				"id": "e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c",
				"card_id": "c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
				"body": "SECRET-COMMENT-3a9d",
				"author": {"participant_id": "9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6", "display_name": "Sam"},
				"is_mine": true,
				"created_at": "2026-09-22T10:05:00.000Z"
			}
		],
		"hidden_counts": {"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3": 1},
		"participants": [
			{"participant_id": "9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6", "display_name": "Sam", "role": "participant"},
			{"participant_id": "2a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d", "display_name": "Ali", "role": "participant"},
			{"participant_id": "3b9c0d1e-2f3a-4b4c-9d5e-6f7a8b9c0d1e", "display_name": "Hana", "role": "facilitator"},
			{"participant_id": "4c0d1e2f-3a4b-4c5d-ae6f-7a8b9c0d1e2f", "display_name": "Olu", "role": "facilitator"},
			{"participant_id": "5d1e2f3a-4b5c-4d6e-bf7a-8b9c0d1e2f3a", "display_name": "Gil", "role": "participant"}
		]
	}`, payload)
}

func TestSnapshot_GoldenFacilitatorOnANamedBoardWithCardsHidden(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, true)
	viewer := viewerCases()[3].viewer

	// when
	payload := marshal(t, projection.Snapshot(state, viewer))

	// then
	decoded := decode(t, payload)
	assert.JSONEq(t, `[]`, marshal(t, decoded["cards"]))
	assert.JSONEq(t, `{"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3": 2}`, marshal(t, decoded["hidden_counts"]))
	assert.JSONEq(t, `{
		"participant_id": "3b9c0d1e-2f3a-4b4c-9d5e-6f7a8b9c0d1e",
		"role": "facilitator",
		"capabilities": ["board.view", "board.participate", "board.facilitate", "board.manage", "board.export"]
	}`, marshal(t, decoded["viewer"]))
}

func TestSnapshot_GoldenNonAuthorOnAnAnonymousBoardAfterReveal(t *testing.T) {
	// given
	state := boardState(model.AnonymityAnonymous, false)
	viewer := viewerCases()[1].viewer

	// when
	payload := marshal(t, projection.Snapshot(state, viewer))

	// then
	decoded := decode(t, payload)
	assert.JSONEq(t, `{
		"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"team_id": "1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0",
		"title": "Sprint 42",
		"context": "Payments squad",
		"template_key": "went-well",
		"anonymity": "anonymous",
		"stage": "vote",
		"cards_hidden": false,
		"closed": false,
		"show_prime_directive": true
	}`, marshal(t, decoded["board"]))
	assert.JSONEq(t, `[
		{
			"id": "d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b",
			"column_id": "a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3",
			"parent_card_id": null,
			"position": "a0F",
			"body": "ALI-TEXT-41d0",
			"colour": null,
			"author": null,
			"is_mine": true,
			"hidden": false,
			"votes": {"total": 0}
		},
		{
			"id": "c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
			"column_id": "a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3",
			"parent_card_id": null,
			"position": "a0Vq",
			"body": "SECRET-TEXT-9f2c",
			"colour": "amber",
			"author": null,
			"hidden": false,
			"votes": {"total": 3}
		}
	]`, marshal(t, decoded["cards"]))
	assert.JSONEq(t, `[
		{
			"id": "f6b9c0d1-e2f3-4a4b-9c5d-6e7f8a9b0c1d",
			"card_id": "d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b",
			"body": "ALI-COMMENT-7e1b",
			"author": null,
			"is_mine": true
		},
		{
			"id": "e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c",
			"card_id": "c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
			"body": "SECRET-COMMENT-3a9d",
			"author": null
		}
	]`, marshal(t, decoded["comments"]))
	assert.JSONEq(t, `{"votes_remaining": 5, "my_votes": {"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6": 1}}`, marshal(t, decoded["me"]))
	assert.JSONEq(t, `{}`, marshal(t, decoded["hidden_counts"]))
}

func TestSnapshot_KeepsTheStoredOrderRatherThanCreationOrder(t *testing.T) {
	// given
	state := boardState(model.AnonymityAnonymous, false)
	state.Cards[0].CreatedAt, state.Cards[1].CreatedAt = state.Cards[1].CreatedAt, state.Cards[0].CreatedAt
	slices.Reverse(state.Cards)

	// when
	snapshot := projection.Snapshot(state, viewerCases()[3].viewer)

	// then
	require.Len(t, snapshot.Cards, 2)
	assert.Equal(t, secretID, snapshot.Cards[0].ID)
	assert.Equal(t, aliCardID, snapshot.Cards[1].ID)
}

func TestSnapshot_ARemovedAuthorStaysNamedOnTheirCardsButLeavesTheRoster(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)
	state.Cards[0].AuthorParticipantID = &removedID

	// when
	snapshot := projection.Snapshot(state, viewerCases()[3].viewer)

	// then
	require.Len(t, snapshot.Cards, 2)
	require.NotNil(t, snapshot.Cards[0].Author)
	assert.Equal(t, removedID, snapshot.Cards[0].Author.ParticipantID)
	assert.Equal(t, "Rae", snapshot.Cards[0].Author.DisplayName)

	var roster []uuid.UUID
	for _, participant := range snapshot.Participants {
		roster = append(roster, participant.ParticipantID)
	}
	assert.Equal(t, []uuid.UUID{samID, aliID, hostID, ownerID, guestID}, roster)
}

func TestSnapshot_ACardWithoutAnAuthorCountsForEveryoneAndShowsNoAuthor(t *testing.T) {
	// given
	hiddenState := boardState(model.AnonymityNamed, true)
	hiddenState.Cards[1].AuthorParticipantID = nil
	shownState := boardState(model.AnonymityNamed, false)
	shownState.Cards[1].AuthorParticipantID = nil

	// when
	hidden := projection.Snapshot(hiddenState, viewerCases()[0].viewer)
	shown := projection.Snapshot(shownState, viewerCases()[0].viewer)

	// then
	assert.Empty(t, hidden.Cards)
	assert.Equal(t, map[uuid.UUID]int{columnA: 2}, hidden.HiddenCounts)
	require.Len(t, shown.Cards, 2)
	assert.Nil(t, shown.Cards[1].Author)
	assert.False(t, shown.Cards[1].IsMine)
}

func TestSnapshot_NoSettingLetsAnyRoleReadHiddenCards(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, true)
	state.Header.Board.CardsLocked = true
	state.Header.Board.MovesLocked = true
	state.Header.Board.VotingOpen = true
	state.Header.Board.CommentsEnabled = true
	state.Header.Board.MergingEnabled = true
	state.Header.Board.GuestNamesEnabled = true
	state.Header.LockedAt = &state.Header.UpdatedAt
	state.Header.ClosedAt = &state.Header.UpdatedAt

	for _, tc := range viewerCases()[1:] {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload := marshal(t, projection.Snapshot(state, tc.viewer))

			// then
			assert.NotContains(t, payload, secretText)
			assert.NotContains(t, payload, secretID.String())
		})
	}
}

func TestReplayCardResponse_OnlyTheAuthorGetsTheirCardBack(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given
				state := boardState(board.Anonymity, board.CardsHidden)
				card := state.Cards[1]
				author := state.Participants[0]

				// when
				payload := marshal(t, projection.ReplayCardResponse(board, card, &author, tc.viewer, 1044))

				// then
				if !slices.Contains(tc.owns, secretID) {
					assert.JSONEq(t, `{"version":1044,"replayed":true,"id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6"}`, payload)
					assert.NotContains(t, payload, secretText)

					return
				}

				assertPrivate(t, board, tc, payload)
				decoded := decode(t, payload)
				assert.Equal(t, true, decoded["replayed"])
				assert.Equal(t, secretID.String(), decoded["id"])
				cardObject, ok := decoded["card"].(map[string]any)
				require.True(t, ok)
				assertCardObject(t, board, tc, cardObject)
			})
		}
	}
}

func TestBoardList_GoldenNamedAndAnonymousBoards(t *testing.T) {
	// given a named board, then an archived anonymous one
	named := boardState(model.AnonymityNamed, true).Header
	anonymousBoard := boardState(model.AnonymityAnonymous, true).Header
	anonymousBoard.ID = aliCardID
	anonymousBoard.Title = "Sprint 41"
	anonymousBoard.ArchivedAt = new(at("2026-09-30T08:00:00Z"))

	// when
	payload := marshal(t, projection.BoardList([]model.BoardHeader{named, anonymousBoard}))

	// then the anonymous board carries no creation time
	assert.JSONEq(t, `{"boards":[
		{"id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","title":"Sprint 42","created_at":"2026-09-22T09:00:00.000Z"},
		{"id":"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","title":"Sprint 41","archived_at":"2026-09-30T08:00:00.000Z"}
	]}`, payload)
}

func TestBoardList_NamesWhoCreatedEachBoardEvenWhenItIsAnonymous(t *testing.T) {
	// given anonymity hides who wrote each card, not who set the board up
	named := boardState(model.AnonymityNamed, true).Header
	named.CreatorName = "Hana"
	anonymousState := boardState(model.AnonymityAnonymous, true)
	anonymousState.Header.CreatorName = "Ali"

	// when
	list := projection.BoardList([]model.BoardHeader{named, anonymousState.Header})
	view := projection.Snapshot(anonymousState, projection.Viewer{Kind: projection.ViewerMember, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities})

	// then
	assert.Equal(t, "Hana", list.Boards[0].CreatorName)
	assert.Equal(t, "Ali", list.Boards[1].CreatorName)
	assert.Equal(t, "Ali", view.Board.CreatorName)
}

func TestOrgBoardList_GoldenNamedAndAnonymousBoardsWithTheirTeamAndTemplate(t *testing.T) {
	// given a named board, then an archived anonymous one, with secret card text in the state they came from
	named := boardState(model.AnonymityNamed, true).Header
	anonymousBoard := boardState(model.AnonymityAnonymous, true).Header
	anonymousBoard.ID = aliCardID
	anonymousBoard.Title = "Sprint 41"
	anonymousBoard.TemplateKey = "blank"
	anonymousBoard.ArchivedAt = new(at("2026-09-30T08:00:00Z"))

	// when
	payload := marshal(t, projection.OrgBoardList([]model.BoardHeader{named, anonymousBoard}, map[uuid.UUID]string{teamID: "Payments"}, nil))

	// then the anonymous board carries no creation time, and nothing beyond the listing leaves
	assert.JSONEq(t, `{"boards":[
		{"id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","team_name":"Payments","title":"Sprint 42","template_key":"went-well","created_at":"2026-09-22T09:00:00.000Z"},
		{"id":"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","team_name":"Payments","title":"Sprint 41","template_key":"blank","archived_at":"2026-09-30T08:00:00.000Z"}
	]}`, payload)
	assert.NotContains(t, payload, secretText)
}

func organisationBoardState(anonymity model.Anonymity, hidden bool) model.BoardState {
	state := boardState(anonymity, hidden)
	state.Header.TeamID = nil

	return state
}

func organisationViewerCases() []viewerCase {
	cases := append(viewerCases(),
		viewerCase{
			name:   "organisation member who has not joined",
			viewer: projection.Viewer{Kind: projection.ViewerMember, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities},
		},
		viewerCase{
			name:   "creator hosting an organisation board",
			viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &hostID, Role: model.BoardRoleFacilitator, Capabilities: facilitatorCapabilities},
		},
		viewerCase{
			name:   "organisation admin on an organisation board",
			viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &ownerID, Role: model.BoardRoleFacilitator, Capabilities: facilitatorCapabilities},
		},
	)

	return cases
}

func TestSnapshot_PrivacyOnAnOrganisationBoardAcrossEveryViewer(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range organisationViewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given a board with no team
				state := organisationBoardState(board.Anonymity, board.CardsHidden)

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then the same privacy rules hold, and the board names no team
				assertPrivate(t, board, tc, payload)

				decoded := decode(t, payload)
				boardObject, ok := decoded["board"].(map[string]any)
				require.True(t, ok)
				assert.NotContains(t, boardObject, "team_id")

				cards, ok := decoded["cards"].([]any)
				require.True(t, ok, "cards is always a list")

				var ids []uuid.UUID
				for _, raw := range cards {
					card := raw.(map[string]any)
					ids = append(ids, uuid.MustParse(card["id"].(string)))
					assertCardObject(t, board, tc, card)
				}
				assert.Equal(t, visibleTo(board, tc), ids)
			})
		}
	}
}

func TestSnapshot_GoldenOrganisationMemberOnAnOrganisationBoardWithCardsHidden(t *testing.T) {
	// given a plain organisation member who has not joined yet
	state := organisationBoardState(model.AnonymityNamed, true)
	viewer := projection.Viewer{Kind: projection.ViewerMember, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities}

	// when
	payload := marshal(t, projection.Snapshot(state, viewer))

	// then every card is a count, and the board carries no team
	decoded := decode(t, payload)
	assert.JSONEq(t, `{
		"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"title": "Sprint 42",
		"context": "Payments squad",
		"template_key": "went-well",
		"anonymity": "named",
		"stage": "write",
		"cards_hidden": true,
		"closed": false,
		"show_prime_directive": true,
		"created_at": "2026-09-22T09:00:00.000Z"
	}`, marshal(t, decoded["board"]))
	assert.JSONEq(t, `[]`, marshal(t, decoded["cards"]))
	assert.JSONEq(t, `{"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3": 2}`, marshal(t, decoded["hidden_counts"]))
	assert.JSONEq(t, `{
		"participant_id": null,
		"role": "participant",
		"capabilities": ["board.view", "board.participate"]
	}`, marshal(t, decoded["viewer"]))
	assert.NotContains(t, payload, secretText)
	assert.NotContains(t, payload, aliText)
}

func TestSnapshot_GoldenOrganisationAdminOnAnAnonymousOrganisationBoardWithCardsHidden(t *testing.T) {
	// given an organisation admin, who facilitates every organisation board
	state := organisationBoardState(model.AnonymityAnonymous, true)
	viewer := projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &ownerID, Role: model.BoardRoleFacilitator, Capabilities: facilitatorCapabilities}

	// when
	payload := marshal(t, projection.Snapshot(state, viewer))

	// then the admin sees no more than anyone else while cards are hidden
	decoded := decode(t, payload)
	assert.JSONEq(t, `{
		"id": "8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b",
		"title": "Sprint 42",
		"context": "Payments squad",
		"template_key": "went-well",
		"anonymity": "anonymous",
		"stage": "write",
		"cards_hidden": true,
		"closed": false,
		"show_prime_directive": true
	}`, marshal(t, decoded["board"]))
	assert.JSONEq(t, `[]`, marshal(t, decoded["cards"]))
	assert.JSONEq(t, `{"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3": 2}`, marshal(t, decoded["hidden_counts"]))
	assert.NotContains(t, payload, secretText)
	assert.NotContains(t, payload, aliText)
	assert.NotContains(t, withoutRoster(t, payload), samID.String())
	assert.NotContains(t, withoutRoster(t, payload), aliID.String())
}

func TestOrgBoardList_GoldenOrganisationBoardsCarryNoTeam(t *testing.T) {
	// given an organisation board, an anonymous organisation board, and a team board
	organisationBoard := organisationBoardState(model.AnonymityNamed, true).Header
	organisationBoard.Title = "All hands"
	anonymousBoard := organisationBoardState(model.AnonymityAnonymous, true).Header
	anonymousBoard.ID = aliCardID
	anonymousBoard.Title = "Quarter review"
	anonymousBoard.TemplateKey = "blank"
	teamBoard := boardState(model.AnonymityNamed, true).Header
	teamBoard.ID = secretID

	// when
	payload := marshal(t, projection.OrgBoardList([]model.BoardHeader{organisationBoard, anonymousBoard, teamBoard}, map[uuid.UUID]string{teamID: "Payments"}, nil))

	// then the organisation boards leave out team_id and team_name, and nothing beyond the listing leaves
	assert.JSONEq(t, `{"boards":[
		{"id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","title":"All hands","template_key":"went-well","created_at":"2026-09-22T09:00:00.000Z"},
		{"id":"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b","title":"Quarter review","template_key":"blank"},
		{"id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","team_name":"Payments","title":"Sprint 42","template_key":"went-well","created_at":"2026-09-22T09:00:00.000Z"}
	]}`, payload)
	assert.NotContains(t, payload, secretText)
	assert.NotContains(t, payload, aliText)
}

func TestOrgBoardList_IsNeverNull(t *testing.T) {
	// when
	payload := marshal(t, projection.OrgBoardList(nil, nil, nil))

	// then
	assert.JSONEq(t, `{"boards":[]}`, payload)
}

func TestOrgBoardList_GoldenMarksOnlyThePinnedBoards(t *testing.T) {
	// given two organisation boards, the second pinned by the viewer
	organisationBoard := organisationBoardState(model.AnonymityNamed, true).Header
	organisationBoard.Title = "All hands"
	pinnedBoard := organisationBoardState(model.AnonymityAnonymous, true).Header
	pinnedBoard.ID = aliCardID
	pinnedBoard.Title = "Quarter review"
	pinnedBoard.TemplateKey = "blank"

	// when
	payload := marshal(t, projection.OrgBoardList([]model.BoardHeader{organisationBoard, pinnedBoard}, nil, map[uuid.UUID]struct{}{aliCardID: {}}))

	// then the unpinned board leaves the field out
	assert.JSONEq(t, `{"boards":[
		{"id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","title":"All hands","template_key":"went-well","created_at":"2026-09-22T09:00:00.000Z"},
		{"id":"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b","title":"Quarter review","template_key":"blank","pinned":true}
	]}`, payload)
}

func TestBoardList_IsNeverNull(t *testing.T) {
	// when
	payload := marshal(t, projection.BoardList(nil))

	// then
	assert.JSONEq(t, `{"boards":[]}`, payload)
}

func TestCardResponse_GoldenAuthorWithCardsHidden(t *testing.T) {
	tests := []struct {
		anonymity model.Anonymity
		want      string
	}{
		{
			anonymity: model.AnonymityNamed,
			want: `{
				"version": 1044,
				"card": {
					"id": "c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
					"column_id": "a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3",
					"parent_card_id": null,
					"position": "a0Vq",
					"body": "SECRET-TEXT-9f2c",
					"colour": "amber",
					"author": {"participant_id": "9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6", "display_name": "Sam"},
					"is_mine": true,
					"hidden": true,
					"votes": {"total": 3},
					"created_at": "2026-09-22T10:01:02.512Z",
					"updated_at": "2026-09-22T10:03:00.250Z"
				}
			}`,
		},
		{
			anonymity: model.AnonymityAnonymous,
			want: `{
				"version": 1044,
				"card": {
					"id": "c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
					"column_id": "a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3",
					"parent_card_id": null,
					"position": "a0Vq",
					"body": "SECRET-TEXT-9f2c",
					"colour": "amber",
					"author": null,
					"is_mine": true,
					"hidden": true,
					"votes": {"total": 3}
				}
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.anonymity), func(t *testing.T) {
			// given
			state := boardState(tc.anonymity, true)
			author := state.Participants[0]

			// when
			payload := marshal(t, projection.CardResponse(state.Header.Board, state.Cards[1], &author, viewerCases()[0].viewer, 1044))

			// then
			assert.JSONEq(t, tc.want, payload)
		})
	}
}

func TestCardResponse_AnAuthorWithoutAProfileKeepsOnlyTheirID(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)

	// when
	response := projection.CardResponse(state.Header.Board, state.Cards[1], nil, viewerCases()[3].viewer, 1044)

	// then
	require.NotNil(t, response.Card)
	assert.Equal(t, `{"participant_id":"9d3e4f5a-6b7c-4d8e-9fa0-b1c2d3e4f5a6","display_name":""}`, marshal(t, response.Card.Author))
}

func TestCommandResponses_Golden(t *testing.T) {
	cardID := secretID
	column := boardState(model.AnonymityNamed, true).Columns[0]

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "version", value: projection.VersionResponse(1045), want: `{"version":1045}`},
		{name: "replay with an ID", value: projection.ReplayResponse(1045, &cardID), want: `{"version":1045,"replayed":true,"id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6"}`},
		{name: "replay without an ID", value: projection.ReplayResponse(1045, nil), want: `{"version":1045,"replayed":true}`},
		{
			name:  "column",
			value: projection.ColumnResponse(boardState(model.AnonymityNamed, true).Header.Board, column, 1046),
			want:  `{"version":1046,"column":{"id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","title":"Went well","description":"What helped","colour":"green","position":"a0","max_votes":null,"cards_revealed":false,"hidden":false}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload := marshal(t, tc.value)

			// then
			assert.JSONEq(t, tc.want, payload)
			assert.NotContains(t, payload, secretText)
		})
	}
}

func TestViewerFor_MapsTheResolvedAccess(t *testing.T) {
	tests := []struct {
		name string
		kind model.PrincipalKind
		want projection.ViewerKind
	}{
		{name: "a user is a member", kind: model.PrincipalUser, want: projection.ViewerMember},
		{name: "any other principal is a guest", kind: "guest", want: projection.ViewerGuest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			capabilities := slices.Clone(facilitatorCapabilities)
			principalID := uuid.New()
			access := model.BoardAccess{
				Principal:     model.Principal{Kind: tc.kind, ID: principalID},
				ParticipantID: &hostID,
				Role:          model.BoardRoleFacilitator,
				Capabilities:  capabilities,
			}

			// when
			viewer := projection.ViewerFor(access)
			capabilities[0] = model.BoardCapabilityExport

			// then
			assert.Equal(t, tc.want, viewer.Kind)
			if tc.want == projection.ViewerMember {
				assert.Equal(t, &principalID, viewer.UserID)
			} else {
				assert.Nil(t, viewer.UserID, "a guest is never matched to an account's action items")
			}
			assert.Equal(t, &hostID, viewer.ParticipantID)
			assert.Equal(t, model.BoardRoleFacilitator, viewer.Role)
			assert.Equal(t, facilitatorCapabilities, viewer.Capabilities)
		})
	}
}

func TestSnapshot_ListsAreNeverNull(t *testing.T) {
	// given
	state := model.BoardState{Header: boardState(model.AnonymityNamed, true).Header}

	// when
	payload := marshal(t, projection.Snapshot(state, projection.Viewer{Kind: projection.ViewerMember}))

	// then
	decoded := decode(t, payload)
	for _, key := range []string{"columns", "cards", "comments", "participants", "hidden_counts", "spotlight", "surveys"} {
		assert.NotNil(t, decoded[key], key)
	}
	assert.Equal(t, []any{}, decoded["viewer"].(map[string]any)["capabilities"])
	assert.Equal(t, map[string]any{}, decoded["me"].(map[string]any)["my_votes"])
}

func TestVisible_OnlyTheAuthorSeesAHiddenCard(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given
				card := boardState(board.Anonymity, board.CardsHidden).Cards[1]

				// when
				visible := projection.Visible(board, card, tc.viewer)

				// then
				assert.Equal(t, !board.CardsHidden || slices.Contains(tc.owns, secretID), visible)
			})
		}
	}
}
