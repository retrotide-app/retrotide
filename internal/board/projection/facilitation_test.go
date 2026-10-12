package projection_test

import (
	"fmt"
	"testing"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	coHostID = uuid.MustParse("7f3a4b5c-6d7e-4f8a-8b9c-0d1e2f3a4b5d")
	timerNow = at("2026-09-22T10:06:02.512Z")
)

func facilitationViewerCases() []viewerCase {
	cases := append(viewerCases(), viewerCase{
		name:   "co-facilitator",
		viewer: projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &coHostID, Role: model.BoardRoleFacilitator, Capabilities: facilitatorCapabilities},
	})

	return cases
}

func ownedBy(author *uuid.UUID, viewer projection.Viewer) bool {
	if author == nil || viewer.ParticipantID == nil {
		return false
	}

	return *author == *viewer.ParticipantID
}

func TestCardsRevealed_ReachesEveryViewerWithEveryCardAndMarksOnlyTheirOwn(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		t.Run(string(anonymity), func(t *testing.T) {
			// given the state just committed by the reveal
			state := boardState(anonymity, false)
			board := state.Header.Board
			event := projection.CardsRevealed(state, 1044)
			shared := marshal(t, event.Payload(projection.ClassOthers))

			cardOwners := map[string]*uuid.UUID{}
			for _, card := range state.Cards {
				cardOwners[card.ID.String()] = card.AuthorParticipantID
			}

			commentOwners := map[string]*uuid.UUID{}
			for _, comment := range state.Comments {
				commentOwners[comment.ID.String()] = comment.AuthorParticipantID
			}

			for _, tc := range facilitationViewerCases() {
				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then every viewer receives every card, and is_mine marks only the viewer's own
				require.True(t, seen, tc.name)
				assertPrivate(t, board, tc, payload)

				decoded := decode(t, payload)
				assert.Equal(t, projection.EventCardsRevealed, decoded["type"])
				cards := decoded["cards"].([]any)
				require.Len(t, cards, 2)

				ownsAny := false
				for _, raw := range cards {
					card := raw.(map[string]any)
					id := uuid.MustParse(card["id"].(string))
					assert.Equal(t, cardBodies[id], card["body"])
					assert.Equal(t, false, card["hidden"])

					mine := ownedBy(cardOwners[card["id"].(string)], tc.viewer)
					ownsAny = ownsAny || mine
					assert.Equal(t, mine, card["is_mine"] == true, "%s: is_mine on card %s", tc.name, id)

					if board.Anonymous() {
						assert.Nil(t, card["author"], tc.name)
						assert.NotContains(t, card, "created_at")
						continue
					}

					assert.Equal(t, cardAuthors[id].String(), card["author"].(map[string]any)["participant_id"])
				}

				comments := decoded["comments"].([]any)
				assert.Len(t, comments, 2, "the comments on every card arrive with it")
				for _, raw := range comments {
					comment := raw.(map[string]any)
					mine := ownedBy(commentOwners[comment["id"].(string)], tc.viewer)
					ownsAny = ownsAny || mine
					assert.Equal(t, mine, comment["is_mine"] == true, "%s: is_mine on comment %s", tc.name, comment["id"])
				}

				if !ownsAny {
					assert.Equal(t, shared, payload, "%s: a viewer who owns nothing gets the shared payload", tc.name)
				}
			}
		})
	}
}

func TestCardsRevealed_GivesBackAnOwnCommentOnSomeoneElsesCard(t *testing.T) {
	// given an anonymous board where the non-author commented on the author's card
	state := boardState(model.AnonymityAnonymous, false)
	state.Comments = []model.CardComment{{ID: aliCommentID, BoardID: boardID, CardID: secretID, AuthorParticipantID: new(aliID), Body: aliCommentText}}
	state.Cards = state.Cards[1:]
	nonAuthor := viewerCases()[1].viewer
	facilitator := viewerCases()[3].viewer

	// when
	mine, _ := seenPayload(t, projection.CardsRevealed(state, 1044), nonAuthor)
	theirs, _ := seenPayload(t, projection.CardsRevealed(state, 1044), facilitator)

	// then the commenter alone is told the comment is theirs, and nobody learns who wrote it
	comments := decode(t, mine)["comments"].([]any)
	require.Len(t, comments, 1)
	assert.Equal(t, true, comments[0].(map[string]any)["is_mine"])
	assert.NotContains(t, mine, aliID.String())
	assert.NotContains(t, theirs, "is_mine")
	assert.Contains(t, theirs, aliCommentText)
}

func TestCardsRevealed_GoldenOnAnAnonymousBoard(t *testing.T) {
	// given
	state := boardState(model.AnonymityAnonymous, false)
	state.Comments = nil

	// when
	payload, _ := seenPayload(t, projection.CardsRevealed(state, 1044), viewerCases()[0].viewer)

	// then
	assert.JSONEq(t, `{"type":"cards_revealed","version":1044,"stage":"vote","cards":[
		{"id":"d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b","column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","parent_card_id":null,"position":"a0F","body":"ALI-TEXT-41d0","colour":null,"author":null,"hidden":false,"votes":{"total":0}},
		{"id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","column_id":"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3","parent_card_id":null,"position":"a0Vq","body":"SECRET-TEXT-9f2c","colour":"amber","author":null,"is_mine":true,"hidden":false,"votes":{"total":3}}
	],"comments":[]}`, payload)
}

func TestCardsRevealed_EnteringVoteRevealsExactlyAsRevealDidAndNothingLeaksBefore(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range facilitationViewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given a board in its Write stage with cards hidden, and the same state once it has moved on to Vote
				writing := boardState(anonymity, true)
				voting := boardState(anonymity, false)
				revealedInWrite := voting
				revealedInWrite.Header.Stage = model.StageWrite

				// when
				before := marshal(t, projection.Snapshot(writing, tc.viewer))
				entered, enteredSeen := seenPayload(t, projection.CardsRevealed(voting, 1044), tc.viewer)
				revealed, revealedSeen := seenPayload(t, projection.CardsRevealed(revealedInWrite, 1044), tc.viewer)

				// then nobody holds anyone else's hidden text before the step, and the step reveals what a reveal did, plus the stage
				assertPrivate(t, writing.Header.Board, tc, before)
				require.True(t, enteredSeen)
				require.True(t, revealedSeen)
				for _, body := range cardBodies {
					assert.Contains(t, entered, body, "%s: every card arrives with the step", tc.name)
				}

				enteredFields := decode(t, entered)
				revealedFields := decode(t, revealed)
				assert.Equal(t, "vote", enteredFields["stage"])
				assert.Equal(t, "write", revealedFields["stage"])
				delete(enteredFields, "stage")
				delete(revealedFields, "stage")
				assert.Equal(t, revealedFields, enteredFields)
				assertPrivate(t, voting.Header.Board, tc, entered)
			})
		}
	}
}

func TestStageChanged_ReachesEveryViewerAlikeAndCarriesNoCard(t *testing.T) {
	for _, board := range boardMatrix() {
		board.CardsHidden = false
		board.Stage = model.StageDiscuss
		event := projection.StageChanged(board, 1044)

		for _, tc := range facilitationViewerCases() {
			t.Run(fmt.Sprintf("%s/%s", board.Anonymity, tc.name), func(t *testing.T) {
				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then
				require.True(t, seen)
				assert.JSONEq(t, `{"type":"stage_changed","version":1044,"stage":"discuss"}`, payload)
				assert.NotContains(t, payload, secretText)
				assert.NotContains(t, payload, aliText)
				assert.NotContains(t, payload, secretID.String())
				assert.NotContains(t, payload, samID.String())
			})
		}
	}
}

func TestStageChanged_GoldenForEveryStageItAnnounces(t *testing.T) {
	for _, stage := range []model.Stage{model.StageVote, model.StageDiscuss, model.StageActions} {
		t.Run(string(stage), func(t *testing.T) {
			// given
			board := boardState(model.AnonymityNamed, false).Header.Board
			board.Stage = stage

			// when
			payload, _ := seenPayload(t, projection.StageChanged(board, 1044), viewerCases()[0].viewer)

			// then
			assert.JSONEq(t, `{"type":"stage_changed","version":1044,"stage":"`+string(stage)+`"}`, payload)
		})
	}
}

func TestSnapshot_TotalsHiddenWhileVotingAreShownFromTheDiscussStage(t *testing.T) {
	tests := []struct {
		stage model.Stage
		shown bool
	}{
		{stage: model.StageVote, shown: false},
		{stage: model.StageDiscuss, shown: true},
		{stage: model.StageActions, shown: true},
		{stage: model.StageEnd, shown: true},
	}

	for _, tc := range tests {
		t.Run(string(tc.stage), func(t *testing.T) {
			// given a board that hides vote totals
			state := boardState(model.AnonymityNamed, false)
			state.Header.VoteCountsHidden = true
			state.Header.Stage = tc.stage

			// when
			decoded := decode(t, marshal(t, projection.Snapshot(state, viewerCases()[1].viewer)))

			// then
			assert.Equal(t, string(tc.stage), decoded["board"].(map[string]any)["stage"])
			assert.Equal(t, true, decoded["settings"].(map[string]any)["vote_counts_hidden"], "the setting is reported as stored")
			for _, raw := range decoded["cards"].([]any) {
				total := raw.(map[string]any)["votes"].(map[string]any)["total"]
				assert.Equal(t, tc.shown, total != nil)
			}
		})
	}
}

func TestCardsHidden_CountsForEveryViewerAndCarriesNoCard(t *testing.T) {
	for _, board := range boardMatrix() {
		if !board.CardsHidden {
			continue
		}

		for _, tc := range facilitationViewerCases() {
			t.Run(fmt.Sprintf("%s/%s", board.Anonymity, tc.name), func(t *testing.T) {
				// given
				state := boardState(board.Anonymity, true)
				event := projection.CardsHidden(state.Header.Board, state.Cards, 1044)

				// when
				tally, hides := event.Hides()
				payload := marshal(t, projection.CardsHiddenCounts(tally.For(state.Header.Board, tc.viewer), 1044))

				// then the event itself carries nothing, and each viewer's counts leave out their own cards
				require.True(t, hides)
				assert.Nil(t, event.Payload(projection.ClassOthers))
				assert.Nil(t, event.Payload(projection.ClassAuthor))
				assertPrivate(t, board, tc, payload)

				want := map[string]any{}
				if hiddenInA := 2 - len(tc.owns); hiddenInA > 0 {
					want[columnA.String()] = float64(hiddenInA)
				}
				decoded := decode(t, payload)
				assert.Equal(t, projection.EventCardsHidden, decoded["type"])
				assert.Equal(t, want, decoded["hidden_counts"])
			})
		}
	}
}

func TestCardsHidden_Golden(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, true)
	tally, _ := projection.CardsHidden(state.Header.Board, state.Cards, 1044).Hides()

	// when
	payload := marshal(t, projection.CardsHiddenCounts(tally.For(state.Header.Board, viewerCases()[0].viewer), 1044))

	// then
	assert.JSONEq(t, `{"type":"cards_hidden","version":1044,"hidden_counts":{"a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3":1}}`, payload)
}

func TestParticipantRemoved_ReachesEveryViewerAlikeAndCarriesNoCard(t *testing.T) {
	for _, board := range boardMatrix() {
		t.Run(fmt.Sprintf("%s/hidden=%t", board.Anonymity, board.CardsHidden), func(t *testing.T) {
			// given
			event := projection.ParticipantRemoved(removedID, 1044)

			var payloads []string
			for _, tc := range facilitationViewerCases() {
				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then
				require.True(t, seen)
				assertPrivate(t, board, tc, payload)
				payloads = append(payloads, payload)
			}

			assert.JSONEq(t, `{"type":"participant_removed","version":1044,"participant_id":"6e2f3a4b-5c6d-4e7f-8a8b-9c0d1e2f3a4b"}`, payloads[0])
			for _, payload := range payloads {
				assert.Equal(t, payloads[0], payload)
			}
		})
	}
}

func TestParticipantRoleChanged_ThePromotedParticipantRefetchesAndEveryoneElseSeesTheRole(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range facilitationViewerCases() {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
				// given Ali becomes a co-facilitator
				state := boardState(board.Anonymity, board.CardsHidden)
				promoted := state.Participants[1]
				promoted.Role = model.BoardRoleFacilitator
				event := projection.ParticipantRoleChanged(board, promoted, 1044)

				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then the roster entry carries no card of anyone's
				require.True(t, seen)
				for _, hidden := range []string{secretText, aliText, secretID.String(), aliCardID.String(), samID.String()} {
					assert.NotContains(t, payload, hidden)
				}

				if tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == aliID {
					assert.JSONEq(t, `{"type":"resync","version":1044}`, payload, "the promoted participant refetches for their new capabilities")
					return
				}

				assert.JSONEq(t, `{"type":"participant_upserted","version":1044,"participant":{"participant_id":"2a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d","display_name":"Ali","role":"facilitator"}}`, payload)
			})
		}
	}
}

func TestTimerChanged_GoldenForEveryState(t *testing.T) {
	ends := at("2026-09-22T10:11:02.512Z")

	tests := []struct {
		name  string
		timer model.BoardTimer
		want  string
	}{
		{
			name:  "running",
			timer: model.BoardTimer{DurationMS: new(300000), EndsAt: &ends},
			want:  `{"type":"timer_changed","version":1047,"timer":{"duration_ms":300000,"ends_at":"2026-09-22T10:11:02.512Z","paused_remaining_ms":null},"server_now":"2026-09-22T10:06:02.512Z"}`,
		},
		{
			name:  "paused",
			timer: model.BoardTimer{DurationMS: new(300000), PausedRemainingMS: new(120000)},
			want:  `{"type":"timer_changed","version":1047,"timer":{"duration_ms":300000,"ends_at":null,"paused_remaining_ms":120000},"server_now":"2026-09-22T10:06:02.512Z"}`,
		},
		{
			name: "stopped",
			want: `{"type":"timer_changed","version":1047,"timer":null,"server_now":"2026-09-22T10:06:02.512Z"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, board := range boardMatrix() {
				board.TimerDurationMS = tc.timer.DurationMS
				board.TimerEndsAt = tc.timer.EndsAt
				board.TimerPausedRemainingMS = tc.timer.PausedRemainingMS
				event := projection.TimerChanged(board, timerNow, 1047)

				for _, viewer := range facilitationViewerCases() {
					// when
					payload, seen := seenPayload(t, event, viewer.viewer)

					// then the timer is everyone's, and says nothing about any card
					require.True(t, seen)
					assert.JSONEq(t, tc.want, payload)
				}
			}
		})
	}
}

func TestSnapshot_CarriesTheTimerTheLocksAndTheEndOfTheRetro(t *testing.T) {
	// given a closed, locked board whose timer was left paused
	state := boardState(model.AnonymityNamed, false)
	state.Header.TimerDurationMS = new(300000)
	state.Header.TimerPausedRemainingMS = new(45000)
	state.Header.TimerPlaySound = true
	state.Header.CardsLocked = true
	state.Header.MovesLocked = true
	state.Header.LockedAt = new(at("2026-09-22T11:00:00Z"))
	state.Header.ClosedAt = new(at("2026-09-22T11:30:00Z"))

	// when
	snapshot := projection.WithServerNow(projection.Snapshot(state, viewerCases()[1].viewer), timerNow)
	decoded := decode(t, marshal(t, snapshot))

	// then
	assert.JSONEq(t, `{"duration_ms":300000,"ends_at":null,"paused_remaining_ms":45000}`, marshal(t, decoded["timer"]))
	assert.Equal(t, "2026-09-22T10:06:02.512Z", decoded["server_now"])
	assert.Equal(t, true, decoded["board"].(map[string]any)["closed"])
	settings := decoded["settings"].(map[string]any)
	assert.Equal(t, true, settings["cards_locked"])
	assert.Equal(t, true, settings["moves_locked"])
	assert.Equal(t, true, settings["board_locked"])
	assert.Equal(t, true, settings["timer_play_sound"])
	assert.NotContains(t, decoded["board"], "closed_at", "the moment a retro ended is not sent")
}

func TestSnapshot_WithoutAServerClockLeavesItOut(t *testing.T) {
	// when
	decoded := decode(t, marshal(t, projection.Snapshot(boardState(model.AnonymityNamed, false), viewerCases()[1].viewer)))

	// then
	assert.NotContains(t, decoded, "server_now")
	assert.Nil(t, decoded["timer"])
}

func TestSnapshot_AHiddenCardNeverNamesItsParent(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range facilitationViewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given Ali's card stays stacked under Sam's after the cards were hidden again
				state := boardState(anonymity, true)
				state.Cards[0].ParentCardID = new(secretID)

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then nobody but Sam learns the ID of Sam's hidden card, Ali included
				assertPrivate(t, state.Header.Board, tc, payload)
				for _, raw := range decode(t, payload)["cards"].([]any) {
					assert.Nil(t, raw.(map[string]any)["parent_card_id"])
				}
			})
		}
	}
}

func TestSnapshot_AShownStackNamesItsParent(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)
	state.Cards[0].ParentCardID = new(secretID)

	// when
	snapshot := projection.Snapshot(state, viewerCases()[1].viewer)

	// then
	require.Len(t, snapshot.Cards, 2)
	assert.Equal(t, new(secretID), snapshot.Cards[0].ParentCardID)
}

func TestStackedCardMoved_ReachesItsAuthorAtOnceOnlyOnANamedBoard(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range facilitationViewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given Sam moved a stack whose child is Ali's card
				state := boardState(anonymity, true)
				child := state.Cards[0]
				child.ParentCardID = new(secretID)
				child.ColumnID = columnB
				event := projection.StackedCardMoved(state.Header.Board, child, &state.Participants[1], columnA, 1044)

				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then only Ali hears where Ali's card went, at once on a named board and on the tick on an anonymous one, and nothing names Sam's card
				deferredID, deferred := event.DeferredCard()
				assert.Equal(t, anonymity == model.AnonymityAnonymous, deferred)
				if deferred {
					assert.Equal(t, child.ID, deferredID)
				}

				owner := !deferred && tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == aliID
				assert.Equal(t, owner, seen)
				if !seen {
					return
				}

				assert.NotContains(t, payload, secretID.String())
				assert.NotContains(t, payload, secretText)
				decoded := decode(t, payload)
				assert.Equal(t, projection.EventCardUpserted, decoded["type"])
				assert.Equal(t, columnB.String(), decoded["card"].(map[string]any)["column_id"])
			})
		}
	}
}

func TestStackedCardMoved_MovesTheChildInTheTally(t *testing.T) {
	// given
	state := boardState(model.AnonymityAnonymous, true)
	child := state.Cards[0]
	child.ColumnID = columnB
	tally := projection.Tally(state.Header.Board, state.Cards)

	// when
	moved := projection.StackedCardMoved(state.Header.Board, child, nil, columnA, 1044).MoveHiddenCards(tally)

	// then
	assert.Equal(t, projection.HiddenTally{columnA: {samID: 1}, columnB: {aliID: 1}}, moved)
}

func TestTimer_IsLeftOutWithNoTimerSet(t *testing.T) {
	// given
	board := boardState(model.AnonymityNamed, false).Header.Board
	board.TimerEndsAt = new(time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC))

	// when
	view := projection.Timer(board)

	// then a timer exists only once it has a duration
	assert.Nil(t, view)
}
