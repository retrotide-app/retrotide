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

var (
	polishImageID      = uuid.MustParse("0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d")
	polishBackgroundID = uuid.MustParse("1c2d3e4f-5a6b-4c7d-9e8f-0a1b2c3d4e5f")

	polishImagePath      = "/uploads/" + boardID.String() + "/" + polishImageID.String() + ".webp"
	polishBackgroundPath = "/uploads/" + boardID.String() + "/" + polishBackgroundID.String() + ".webp"

	polishCounts = `[{"reaction":"thumbs_up","count":1},{"reaction":"heart","count":2},{"reaction":"party","count":1}]`
)

func polishedState(anonymity model.Anonymity, hidden bool) model.BoardState {
	state := boardState(anonymity, hidden)
	pinnedAt := at("2026-09-22T10:06:00Z")

	state.Cards[1].PinnedAt = &pinnedAt
	state.Cards[1].ImageMediaID = new(polishImageID)
	state.Reactions = []model.CardReaction{
		{BoardID: boardID, CardID: secretID, ParticipantID: &samID, Reaction: model.ReactionHeart},
		{BoardID: boardID, CardID: secretID, ParticipantID: &aliID, Reaction: model.ReactionThumbsUp},
		{BoardID: boardID, CardID: secretID, ParticipantID: &aliID, Reaction: model.ReactionHeart},
		{BoardID: boardID, CardID: secretID, Reaction: model.ReactionParty},
	}

	return state
}

func ownReactions(tc viewerCase) []any {
	if tc.viewer.ParticipantID == nil {
		return nil
	}

	switch *tc.viewer.ParticipantID {
	case samID:
		return []any{"heart"}
	case aliID:
		return []any{"thumbs_up", "heart"}
	default:
		return nil
	}
}

func cardObjectByID(t *testing.T, payload string, key string, id uuid.UUID) map[string]any {
	t.Helper()

	cards, ok := decode(t, payload)[key].([]any)
	require.True(t, ok, "%s is a list", key)

	for _, raw := range cards {
		card, ok := raw.(map[string]any)
		require.True(t, ok)

		if card["id"] == id.String() {
			return card
		}
	}

	return nil
}

func assertNoReactorIDs(t *testing.T, value any) {
	t.Helper()

	encoded := marshal(t, value)
	for _, participant := range []uuid.UUID{samID, aliID, hostID, guestID} {
		assert.NotContains(t, encoded, participant.String(), "a reaction payload never names who reacted")
	}
}

func TestSnapshot_GoldenARevealedCardCarriesItsPinImageAndReactionCounts(t *testing.T) {
	// given a revealed named board where Ali reacted twice and Sam once
	state := polishedState(model.AnonymityNamed, false)
	viewer := viewerCases()[1].viewer

	// when
	payload := marshal(t, projection.Snapshot(state, viewer))

	// then
	card := cardObjectByID(t, payload, "cards", secretID)
	require.NotNil(t, card)
	assert.Equal(t, true, card["pinned"])
	assert.Equal(t, polishImagePath, card["image_path"])
	assert.JSONEq(t, polishCounts, marshal(t, card["reactions"]))
	assert.JSONEq(t, `["thumbs_up","heart"]`, marshal(t, card["my_reactions"]))

	other := cardObjectByID(t, payload, "cards", aliCardID)
	require.NotNil(t, other)
	assert.NotContains(t, other, "pinned", "an unpinned card leaves the field out")
	assert.NotContains(t, other, "image_path")
	assert.NotContains(t, other, "reactions")
	assert.NotContains(t, other, "my_reactions")
}

func TestSnapshot_ReactionsShowCountsAndOnlyTheViewersOwnKeys(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given
				state := polishedState(anonymity, false)

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then
				card := cardObjectByID(t, payload, "cards", secretID)
				require.NotNil(t, card)
				assert.JSONEq(t, polishCounts, marshal(t, card["reactions"]))
				assertNoReactorIDs(t, card["reactions"])
				assertNoReactorIDs(t, card["my_reactions"])

				mine := ownReactions(tc)
				if mine == nil {
					assert.NotContains(t, card, "my_reactions", "a viewer who reacted with nothing gets no my_reactions")
				} else {
					assert.Equal(t, mine, card["my_reactions"])
				}

				if anonymity == model.AnonymityAnonymous {
					outside := withoutRoster(t, payload)
					assert.NotContains(t, outside, samID.String())
					assert.NotContains(t, outside, aliID.String())
				}
			})
		}
	}
}

func TestSnapshot_AHiddenCardsImagePinAndReactionsNeverReachAnotherViewer(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given Sam's hidden card is pinned, has an image and carries reactions
				state := polishedState(anonymity, true)
				board := state.Header.Board

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then
				card := cardObjectByID(t, payload, "cards", secretID)
				if !slices.Contains(visibleTo(board, tc), secretID) {
					assert.Nil(t, card)
					assert.NotContains(t, payload, polishImageID.String(), "a hidden card's image reached another viewer")
					assert.NotContains(t, payload, `"pinned"`, "a hidden card's pin reached another viewer")
					assert.NotContains(t, payload, `"reactions"`, "a hidden card's reactions reached another viewer")
					assert.NotContains(t, payload, `"my_reactions"`)

					return
				}

				require.NotNil(t, card)
				assert.Equal(t, true, card["pinned"])
				assert.Equal(t, polishImagePath, card["image_path"])
				assert.NotContains(t, card, "reactions", "reactions stay off a card while it is hidden, even for its author")
				assert.NotContains(t, card, "my_reactions")
			})
		}
	}
}

func TestSnapshot_GoldenBoardCarriesItsBackgroundPath(t *testing.T) {
	// given
	state := boardState(model.AnonymityAnonymous, true)
	state.Header.BackgroundMediaID = new(polishBackgroundID)

	// when
	payload := marshal(t, projection.Snapshot(state, viewerCases()[2].viewer))

	// then
	board, ok := decode(t, payload)["board"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, polishBackgroundPath, board["background_path"])
}

func TestSnapshot_ABoardWithoutABackgroundLeavesThePathOut(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)

	// when
	payload := marshal(t, projection.Snapshot(state, viewerCases()[0].viewer))

	// then
	board, ok := decode(t, payload)["board"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, board, "background_path")
}

func TestCardReactionsChanged_GoldenOthersGetCountsAndAnEmptyListOfTheirOwn(t *testing.T) {
	// given
	state := polishedState(model.AnonymityNamed, false)
	event := projection.CardReactionsChanged(state.Header.Board, secretCard(state), state.Reactions, 1044)

	// when
	payload, seen := seenPayload(t, event, viewerCases()[2].viewer)

	// then
	require.True(t, seen)
	assert.JSONEq(t, `{"type":"card_reactions_changed","version":1044,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
		"reactions":`+polishCounts+`,"my_reactions":[]}`, payload)
}

func TestCardReactionsChanged_MyReactionsGoOnlyToTheirOwner(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given
				state := polishedState(anonymity, false)
				event := projection.CardReactionsChanged(state.Header.Board, secretCard(state), state.Reactions, 1044)

				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then
				require.True(t, seen, "everyone sees the reactions on a card everyone can see")
				decoded := decode(t, payload)
				assert.JSONEq(t, polishCounts, marshal(t, decoded["reactions"]))
				assertNoReactorIDs(t, payload)

				mine := ownReactions(tc)
				if mine == nil {
					mine = []any{}
				}
				assert.Equal(t, mine, decoded["my_reactions"])
			})
		}
	}
}

func TestCardReactionsChanged_AHiddenCardTellsNobody(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given
			state := polishedState(model.AnonymityNamed, true)
			event := projection.CardReactionsChanged(state.Header.Board, secretCard(state), state.Reactions, 1044)

			// when
			_, seen := seenPayload(t, event, tc.viewer)

			// then
			assert.False(t, seen)
		})
	}
}

func TestCardPolishEvents_AHiddenCardsPinAndImageReachOnlyItsAuthor(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		state := polishedState(anonymity, true)
		board := state.Header.Board

		events := map[string]projection.Event{
			"card_pinned":        projection.CardPinned(board, secretCard(state), 1044),
			"card_image_changed": projection.CardImageChanged(board, secretCard(state), 1044),
		}

		for name, event := range events {
			for _, tc := range viewerCases() {
				t.Run(fmt.Sprintf("%s/%s/%s", name, anonymity, tc.name), func(t *testing.T) {
					// when
					payload, seen := seenPayload(t, event, tc.viewer)

					// then
					if !slices.Contains(tc.owns, secretID) {
						assert.False(t, seen, "only the author hears about their hidden card")

						return
					}

					require.True(t, seen)
					assert.Equal(t, secretID.String(), decode(t, payload)["card_id"])
					assert.NotContains(t, payload, secretText)
					assert.NotContains(t, payload, samID.String())
				})
			}
		}
	}
}

func TestCardPolishEvents_AHiddenColumnConcealsThemFromNonFacilitators(t *testing.T) {
	// given column A is hidden from participants on a revealed board
	state := polishedState(model.AnonymityNamed, false)
	state.Header.HiddenColumnIDs = []uuid.UUID{columnA}
	board := state.Header.Board

	events := map[string]projection.Event{
		"card_reactions_changed": projection.CardReactionsChanged(board, secretCard(state), state.Reactions, 1044),
		"card_pinned":            projection.CardPinned(board, secretCard(state), 1044),
		"card_image_changed":     projection.CardImageChanged(board, secretCard(state), 1044),
	}

	for name, event := range events {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", name, tc.name), func(t *testing.T) {
				// when
				_, seen := seenPayload(t, event, tc.viewer)

				// then
				assert.Equal(t, slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityFacilitate), seen)
			})
		}
	}
}

func TestCardPinned_GoldenEveryoneSeesTheFlagOnARevealedCard(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given
			state := polishedState(model.AnonymityAnonymous, false)

			// when
			payload, seen := seenPayload(t, projection.CardPinned(state.Header.Board, secretCard(state), 1044), tc.viewer)

			// then
			require.True(t, seen)
			assert.JSONEq(t, `{"type":"card_pinned","version":1044,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","pinned":true}`, payload)
		})
	}
}

func TestCardImageChanged_GoldenCarriesTheServerBuiltPathOrNull(t *testing.T) {
	// given
	state := polishedState(model.AnonymityNamed, false)
	removed := secretCard(state)
	removed.ImageMediaID = nil

	// when
	attached, _ := seenPayload(t, projection.CardImageChanged(state.Header.Board, secretCard(state), 1044), viewerCases()[2].viewer)
	detached, _ := seenPayload(t, projection.CardImageChanged(state.Header.Board, removed, 1045), viewerCases()[2].viewer)

	// then
	assert.JSONEq(t, `{"type":"card_image_changed","version":1044,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","image_path":"`+polishImagePath+`"}`, attached)
	assert.JSONEq(t, `{"type":"card_image_changed","version":1045,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","image_path":null}`, detached)
}

func TestBoardBackgroundChanged_GoldenReachesEveryone(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given
			board := boardState(model.AnonymityNamed, true).Header.Board
			board.BackgroundMediaID = new(polishBackgroundID)
			cleared := board
			cleared.BackgroundMediaID = nil

			// when
			set, seenSet := seenPayload(t, projection.BoardBackgroundChanged(board, 1044), tc.viewer)
			unset, seenUnset := seenPayload(t, projection.BoardBackgroundChanged(cleared, 1045), tc.viewer)

			// then
			require.True(t, seenSet)
			require.True(t, seenUnset)
			assert.JSONEq(t, `{"type":"board_background_changed","version":1044,"background_path":"`+polishBackgroundPath+`"}`, set)
			assert.JSONEq(t, `{"type":"board_background_changed","version":1045,"background_path":null}`, unset)
		})
	}
}

func TestReactionsResponse_GoldenCarriesTheViewersOwnReactions(t *testing.T) {
	// given
	state := polishedState(model.AnonymityAnonymous, false)

	// when
	payload := marshal(t, projection.ReactionsResponse(state.Header.Board, secretCard(state), state.Reactions, viewerCases()[1].viewer, 1044))

	// then
	assert.JSONEq(t, `{"version":1044,"reactions":{"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6",
		"reactions":`+polishCounts+`,"my_reactions":["thumbs_up","heart"]}}`, payload)
}

func TestReactionsResponse_AHiddenCardAnswersOnlyTheVersion(t *testing.T) {
	// given
	state := polishedState(model.AnonymityNamed, true)

	// when
	payload := marshal(t, projection.ReactionsResponse(state.Header.Board, secretCard(state), state.Reactions, viewerCases()[0].viewer, 1044))

	// then
	assert.JSONEq(t, `{"version":1044}`, payload)
}

func TestCardsRevealed_RevealedCardsCarryCountsAndOnlyTheOwnersKeys(t *testing.T) {
	for _, tc := range facilitationViewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given the state the reveal committed
			state := polishedState(model.AnonymityAnonymous, false)
			event := projection.CardsRevealed(state, 1044)

			// when
			payload, seen := seenPayload(t, event, tc.viewer)

			// then
			require.True(t, seen)
			card := cardObjectByID(t, payload, "cards", secretID)
			require.NotNil(t, card)
			assert.JSONEq(t, polishCounts, marshal(t, card["reactions"]))
			assertNoReactorIDs(t, card["reactions"])

			if tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == aliID {
				assert.Equal(t, []any{"thumbs_up", "heart"}, card["my_reactions"])

				return
			}

			if tc.viewer.ParticipantID == nil || *tc.viewer.ParticipantID != samID {
				assert.NotContains(t, card, "my_reactions")
			}
		})
	}
}

func TestColumnRevealed_RevealedCardsCarryTheirReactions(t *testing.T) {
	// given the state the column reveal committed
	state := polishedState(model.AnonymityNamed, true)
	state.Header.RevealedColumnIDs = []uuid.UUID{columnA}
	event := projection.ColumnRevealed(state, columnA, 1044)

	// when
	payload, seen := seenPayload(t, event, viewerCases()[2].viewer)

	// then
	require.True(t, seen)
	card := cardObjectByID(t, payload, "cards", secretID)
	require.NotNil(t, card)
	assert.JSONEq(t, polishCounts, marshal(t, card["reactions"]))
	assert.NotContains(t, card, "my_reactions")
}
