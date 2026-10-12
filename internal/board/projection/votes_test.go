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

func votingMatrix() []model.Board {
	var boards []model.Board
	for _, board := range boardMatrix() {
		for _, totalsHidden := range []bool{false, true} {
			board.VoteCountsHidden = totalsHidden
			boards = append(boards, board)
		}
	}

	return boards
}

func ownVotes(tc viewerCase) map[uuid.UUID]int {
	if tc.viewer.ParticipantID == nil {
		return map[uuid.UUID]int{}
	}

	switch *tc.viewer.ParticipantID {
	case samID:
		return map[uuid.UUID]int{secretID: 2}
	case aliID:
		return map[uuid.UUID]int{secretID: 1}
	default:
		return map[uuid.UUID]int{}
	}
}

func TestVotesChanged_PrivacyAcrossEveryViewerVoterAndBoard(t *testing.T) {
	for _, board := range votingMatrix() {
		state := boardState(board.Anonymity, board.CardsHidden)
		card := secretCard(state)

		for _, voter := range []uuid.UUID{samID, aliID} {
			event := projection.VotesChanged(board, card, voter, 1044)

			for _, tc := range viewerCases() {
				t.Run(fmt.Sprintf("%s/hidden=%t/totals-hidden=%t/voter=%s/%s", board.Anonymity, board.CardsHidden, board.VoteCountsHidden, voter, tc.name), func(t *testing.T) {
					// when
					payload, seen := seenPayload(t, event, tc.viewer)

					// then
					isVoter := tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == voter
					voterSees := !board.CardsHidden || voter == samID
					want := !board.CardsHidden && !board.VoteCountsHidden
					if isVoter {
						want = voterSees
					}

					assert.Equal(t, want, seen)
					if !seen {
						return
					}

					assertPrivate(t, board, tc, payload)
					assert.NotContains(t, payload, voter.String(), "a vote never names who cast it")

					decoded := decode(t, payload)
					assert.Equal(t, "votes_changed", decoded["type"])
					assert.Equal(t, secretID.String(), decoded["card_id"])
					if board.VoteCountsHidden {
						assert.Nil(t, decoded["total"], "hidden totals reach nobody, the voter included")
						return
					}
					assert.Equal(t, float64(3), decoded["total"])
				})
			}
		}
	}
}

func TestVotesChanged_GoldenFrames(t *testing.T) {
	state := boardState(model.AnonymityNamed, false)
	hiddenTotals := state.Header.Board
	hiddenTotals.VoteCountsHidden = true

	tests := []struct {
		name  string
		board model.Board
		want  string
	}{
		{name: "totals shown", board: state.Header.Board, want: `{"type":"votes_changed","version":1044,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","total":3}`},
		{name: "totals hidden", board: hiddenTotals, want: `{"type":"votes_changed","version":1044,"card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","total":null}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload, seen := seenPayload(t, projection.VotesChanged(tc.board, secretCard(state), aliID, 1044), viewerCases()[1].viewer)

			// then
			require.True(t, seen)
			assert.JSONEq(t, tc.want, payload)
		})
	}
}

func TestSnapshot_VotesPrivacyAcrossEveryViewerAndBoard(t *testing.T) {
	for _, board := range votingMatrix() {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/totals-hidden=%t/%s", board.Anonymity, board.CardsHidden, board.VoteCountsHidden, tc.name), func(t *testing.T) {
				// given
				state := boardState(board.Anonymity, board.CardsHidden)
				state.Header.VoteCountsHidden = board.VoteCountsHidden

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then
				assertPrivate(t, board, tc, payload)

				decoded := decode(t, payload)
				for _, raw := range decoded["cards"].([]any) {
					votes := raw.(map[string]any)["votes"].(map[string]any)
					if board.VoteCountsHidden {
						assert.Nil(t, votes["total"], "hidden totals reach nobody, facilitators included")
					} else {
						assert.NotNil(t, votes["total"])
					}
				}

				me := decode(t, marshal(t, decoded["me"]))
				wantVotes := map[string]any{}
				used := 0
				for card, count := range ownVotes(tc) {
					used += count
					if slices.Contains(visibleTo(board, tc), card) {
						wantVotes[card.String()] = float64(count)
					}
				}
				assert.Equal(t, wantVotes, me["my_votes"], "a viewer sees only their own votes, and only on cards they can see")
				assert.Equal(t, float64(6-used), me["votes_remaining"])
			})
		}
	}
}

func TestVoteResponse_GoldenForTheVoter(t *testing.T) {
	// given Ali, who voted once for her own card and once for Sam's hidden one before it was hidden again
	state := boardState(model.AnonymityNamed, true)
	result := model.CardVoteResult{
		Card: state.Cards[0],
		Mine: []model.CardVote{
			{CardID: aliCardID, ParticipantID: aliID, Count: 1, CardAuthorParticipantID: &aliID},
			{CardID: secretID, ParticipantID: aliID, Count: 1, CardAuthorParticipantID: &samID},
		},
		Count: 1,
	}

	// when
	response := projection.VoteResponse(state.Header.Board, result, viewerCases()[1].viewer, 1045)
	frame := projection.MeFrame(boardID, 1045, *response.Me)

	// then the hidden card's vote still counts against the budget, and its ID never leaves
	assert.JSONEq(t, `{"version":1045,"me":{"votes_remaining":4,"my_votes":{"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b":1}}}`, marshal(t, response))
	assert.JSONEq(t, `{"board_id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","version":1045,"votes_remaining":4,"my_votes":{"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b":1}}`, marshal(t, frame))
	assert.NotContains(t, marshal(t, frame), secretID.String())
}

func TestMe_LoweringTheBudgetBelowVotesSpentLeavesNoneRemaining(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)
	state.Header.MaxVotes = 1

	// when
	me := projection.Me(state.Header.Board, state.Votes, viewerCases()[0].viewer)

	// then
	assert.Zero(t, me.VotesRemaining)
	assert.Equal(t, map[uuid.UUID]int{secretID: 2}, me.MyVotes)
}

func TestVoteAndSettingsEvents_ReachEveryViewerAlike(t *testing.T) {
	state := boardState(model.AnonymityAnonymous, true)

	events := map[string]projection.Event{
		"votes_reset":      projection.VotesReset(1044),
		"settings_changed": projection.SettingsChanged(state.Header.Board, 1044),
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
				assert.Equal(t, payloads[0], payload)
				assert.NotContains(t, payload, secretText)
				assert.NotContains(t, payload, secretID.String())
				assert.NotContains(t, payload, samID.String())
				assert.Equal(t, name, decode(t, payload)["type"])
			}
		})
	}
}

func TestSettingsChanged_Golden(t *testing.T) {
	// given
	board := boardState(model.AnonymityNamed, false).Header.Board
	board.VoteCountsHidden = true
	board.MaxVotesPerCard = 1
	board.SortMode = model.SortModeVotes
	board.TimerAutoLock = true
	board.SortEnforced = true
	board.AnyoneCanEdit = true
	board.ShowCardDates = true

	// when
	payload, _ := seenPayload(t, projection.SettingsChanged(board, 1044), viewerCases()[2].viewer)

	// then
	assert.JSONEq(t, `{"type":"settings_changed","version":1044,"settings":{
		"voting_open":true,"vote_counts_hidden":true,"max_votes":6,"max_votes_per_card":1,
		"comments_enabled":true,"merging_enabled":true,"sort_mode":"votes",
		"cards_locked":false,"moves_locked":false,"board_locked":false,"timer_play_sound":false,
		"timer_auto_lock":true,"sort_enforced":true,"anyone_can_edit":true,"show_card_dates":true}}`, payload)
}

func TestVotesReset_Golden(t *testing.T) {
	// when
	payload, _ := seenPayload(t, projection.VotesReset(1044), viewerCases()[0].viewer)

	// then
	assert.JSONEq(t, `{"type":"votes_reset","version":1044}`, payload)
}
