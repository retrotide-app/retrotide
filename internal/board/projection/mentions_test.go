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

const (
	benName  = "BEN-NAME-5c1e"
	cleoName = "CLEO-NAME-2b7f"
)

var (
	benUserID  = uuid.MustParse("0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d")
	cleoUserID = uuid.MustParse("1b2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e")
	otherUser  = uuid.MustParse("2c3d4e5f-6a7b-4c8d-ae9f-1a2b3c4d5e6f")
)

func mentionedState(anonymity model.Anonymity, hidden bool) model.BoardState {
	state := boardState(anonymity, hidden)
	state.Cards[1].Mentions = []model.ActionItemPerson{{UserID: benUserID, DisplayName: benName}}
	state.Comments[1].Mentions = []model.ActionItemPerson{{UserID: cleoUserID, DisplayName: cleoName}}

	return state
}

func cardMention(hidden, own bool) model.Mention {
	return model.Mention{
		Kind:        model.MentionCard,
		CardID:      secretID,
		OrgID:       orgID,
		OrgSlug:     "acme",
		OrgName:     "Acme",
		TeamID:      new(teamID),
		BoardID:     new(boardID),
		BoardTitle:  "Sprint 42",
		Body:        secretText,
		CardsHidden: hidden,
		OwnCard:     own,
		MentionedAt: at("2026-09-22T10:01:02.512Z"),
	}
}

func commentMention(hidden, own bool) model.Mention {
	return model.Mention{
		Kind:        model.MentionComment,
		CardID:      secretID,
		CommentID:   secretCommentID,
		OrgID:       orgID,
		OrgSlug:     "acme",
		OrgName:     "Acme",
		TeamID:      new(teamID),
		BoardID:     new(boardID),
		BoardTitle:  "Sprint 42",
		Body:        secretCommentText,
		CardsHidden: hidden,
		OwnCard:     own,
		MentionedAt: at("2026-09-22T10:05:00Z"),
	}
}

func TestSnapshot_CardAndCommentMentionsFollowTheirCardForEveryViewer(t *testing.T) {
	type boardCase struct {
		name  string
		state func(model.Anonymity, bool) model.BoardState
		cases []viewerCase
	}

	boards := []boardCase{
		{name: "team board", state: mentionedState, cases: viewerCases()},
		{
			name: "organisation board",
			state: func(anonymity model.Anonymity, hidden bool) model.BoardState {
				state := mentionedState(anonymity, hidden)
				state.Header.TeamID = nil

				return state
			},
			cases: organisationViewerCases(),
		},
	}

	for _, kind := range boards {
		for _, board := range boardMatrix() {
			for _, tc := range kind.cases {
				t.Run(fmt.Sprintf("%s/%s/hidden=%t/%s", kind.name, board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
					// given Sam's card mentions Ben, and Sam's comment on it mentions Cleo
					state := kind.state(board.Anonymity, board.CardsHidden)

					// when
					payload := marshal(t, projection.Snapshot(state, tc.viewer))

					// then a mention is seen only with its card, and never says who wrote it
					assertPrivate(t, board, tc, payload)
					if slices.Contains(visibleTo(board, tc), secretID) {
						assert.Contains(t, payload, benUserID.String())
						assert.Contains(t, payload, benName)
						assert.Contains(t, payload, cleoUserID.String())
					} else {
						assert.NotContains(t, payload, benUserID.String(), "a mention on a hidden card reached the viewer")
						assert.NotContains(t, payload, benName, "a mention on a hidden card reached the viewer")
						assert.NotContains(t, payload, cleoUserID.String(), "a mention in a comment on a hidden card reached the viewer")
						assert.NotContains(t, payload, cleoName, "a mention in a comment on a hidden card reached the viewer")
					}
				})
			}
		}
	}
}

func TestCardAndCommentEvents_CarryTheirMentionsOnlyToWhoeverMaySeeTheCard(t *testing.T) {
	for _, board := range boardMatrix() {
		state := mentionedState(board.Anonymity, board.CardsHidden)
		card := state.Cards[1]
		comment := state.Comments[1]

		events := map[string]projection.Event{
			"card_upserted":    projection.CardChanged(board, card, nil, 1050),
			"comment_upserted": projection.CommentUpserted(board, card, comment, nil, 1050),
		}

		for name, event := range events {
			for _, tc := range viewerCases() {
				t.Run(fmt.Sprintf("%s/%s/hidden=%t/%s", name, board.Anonymity, board.CardsHidden, tc.name), func(t *testing.T) {
					// when
					payload, seen := seenPayload(t, event, tc.viewer)

					// then
					if !slices.Contains(visibleTo(board, tc), secretID) {
						assert.False(t, seen, "an event about a hidden card reached the viewer")
						assert.NotContains(t, payload, benUserID.String())
						assert.NotContains(t, payload, cleoUserID.String())

						return
					}

					require.True(t, seen)
					assertPrivate(t, board, tc, payload)
					assert.NotContains(t, payload, "created_by")
					if name == "card_upserted" {
						assert.Contains(t, payload, `"mentions":[{"user_id":"`+benUserID.String()+`","display_name":"`+benName+`"}]`)
					} else {
						assert.Contains(t, payload, `"mentions":[{"user_id":"`+cleoUserID.String()+`","display_name":"`+cleoName+`"}]`)
					}
				})
			}
		}
	}
}

func TestCardsRevealed_CarriesEveryCardsMentionsAtTheReveal(t *testing.T) {
	// given an anonymous board revealing cards and comments that mention people
	state := mentionedState(model.AnonymityAnonymous, false)

	// when
	payload, seen := seenPayload(t, projection.CardsRevealed(state, 1060), projection.Viewer{Kind: projection.ViewerMember, ParticipantID: &guestID})

	// then everyone receives the mentions with the cards, and still no author
	require.True(t, seen)
	assert.Contains(t, payload, benUserID.String())
	assert.Contains(t, payload, cleoUserID.String())
	assert.NotContains(t, payload, samID.String(), "a reveal on an anonymous board names no author")
	assert.NotContains(t, payload, `"author":{`)
}

func TestCardView_LeavesOutMentionsWhenThereAreNone(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)

	// when
	payload := marshal(t, projection.Snapshot(state, projection.Viewer{Kind: projection.ViewerMember}))

	// then
	assert.NotContains(t, payload, `"mentions":null`)
	assert.NotContains(t, payload, `"mentions":[]`)
}

func TestMentions_ACardOrCommentHiddenFromTheMentionedPersonNeverAppears(t *testing.T) {
	tests := []struct {
		name    string
		mention model.Mention
		shown   bool
	}{
		{name: "a card still hidden from them", mention: cardMention(true, false), shown: false},
		{name: "their own hidden card", mention: cardMention(true, true), shown: true},
		{name: "a revealed card", mention: cardMention(false, false), shown: true},
		{name: "a comment on a card hidden from them", mention: commentMention(true, false), shown: false},
		{name: "a comment on their own hidden card", mention: commentMention(true, true), shown: true},
		{name: "a comment on a revealed card", mention: commentMention(false, false), shown: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload := marshal(t, projection.Mentions([]model.Mention{tc.mention}))

			// then
			assert.Equal(t, tc.shown, projection.MentionShown(tc.mention))
			if tc.shown {
				assert.Contains(t, payload, tc.mention.Body)

				return
			}

			assert.JSONEq(t, `{"mentions":[]}`, payload)
			assert.NotContains(t, payload, tc.mention.Body, "the text of a hidden card or comment leaked")
			assert.NotContains(t, payload, secretID.String(), "the hidden card's existence leaked")
			assert.NotContains(t, payload, boardID.String(), "the board of a hidden mention leaked")
		})
	}
}

func TestMentions_GoldenForACardAndACommentWithNoAuthorOrTimes(t *testing.T) {
	// given the mentions of a revealed anonymous board, after End severed its authors
	mentions := []model.Mention{commentMention(false, false), cardMention(false, false)}

	// when
	payload := marshal(t, projection.Mentions(mentions))

	// then
	assert.JSONEq(t, `{"mentions":[
		{"kind":"comment","card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","comment_id":"e5a8b9c0-d1e2-4f3a-8b4c-5d6e7f8a9b0c","body":"SECRET-COMMENT-3a9d","org_id":"2c3d4e5f-6071-4283-94a5-b6c7d8e9f0a1","org_slug":"acme","org_name":"Acme","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","board_id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","board_title":"Sprint 42"},
		{"kind":"card","card_id":"c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6","body":"SECRET-TEXT-9f2c","org_id":"2c3d4e5f-6071-4283-94a5-b6c7d8e9f0a1","org_slug":"acme","org_name":"Acme","team_id":"1b2c3d4e-5f60-4172-8394-a5b6c7d8e9f0","board_id":"8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b","board_title":"Sprint 42"}
	]}`, payload)
}

func TestMentions_NeverCarryAuthorshipOrWhenAnythingWasWritten(t *testing.T) {
	// given every kind of mention, including on the mentioned person's own hidden card
	mentions := []model.Mention{
		cardMention(true, true),
		commentMention(true, true),
		{Kind: model.MentionActionItem, ActionItemID: convertedItemID, OrgID: orgID, OrgSlug: "acme", OrgName: "Acme", BoardID: new(boardID), Title: "Fix it", Status: model.ActionItemOpen, MentionedAt: at("2026-09-22T11:00:00Z")},
	}

	// when
	payload := marshal(t, projection.Mentions(mentions))

	// then nothing says who wrote it, that it is the reader's own, or when
	for _, key := range []string{"author", "participant_id", "is_mine", "own_card", "cards_hidden", "created_at", "mentioned_at", "updated_at", "created_by"} {
		assert.NotContains(t, payload, `"`+key+`"`)
	}
	assert.NotContains(t, payload, samID.String())
	assert.NotContains(t, payload, "2026-09-22")
}

func TestSnapshot_IsMineMarksOnlyTheCreatorsActionItemsOnANamedBoard(t *testing.T) {
	creator := projection.Viewer{Kind: projection.ViewerMember, UserID: new(samUserID), ParticipantID: &samID, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities}
	other := projection.Viewer{Kind: projection.ViewerMember, UserID: new(otherUser), ParticipantID: &aliID, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities}
	guest := projection.Viewer{Kind: projection.ViewerGuest, ParticipantID: &guestID, Role: model.BoardRoleParticipant, Capabilities: participantCapabilities}

	for _, board := range boardMatrix() {
		for name, viewer := range map[string]projection.Viewer{"creator": creator, "another member": other, "guest": guest} {
			t.Run(fmt.Sprintf("%s/hidden=%t/%s", board.Anonymity, board.CardsHidden, name), func(t *testing.T) {
				// given Sam created the converted item, and nobody is stored as the direct item's creator
				state := withActionItems(boardState(board.Anonymity, board.CardsHidden))

				// when
				decoded := decode(t, marshal(t, projection.Snapshot(state, viewer)))

				// then
				items := decoded["action_items"].([]any)
				converted := items[0].(map[string]any)
				direct := items[1].(map[string]any)
				assert.NotContains(t, direct, "is_mine")
				if name == "creator" && !board.Anonymous() {
					assert.Equal(t, true, converted["is_mine"])
				} else {
					assert.NotContains(t, converted, "is_mine", "is_mine reached someone other than the creator of a named board's item")
				}

				for _, previous := range decoded["previous_actions"].([]any) {
					assert.NotContains(t, previous.(map[string]any), "is_mine")
				}
			})
		}
	}
}

func TestActionItemUpserted_TellsOnlyTheCreatorOfANamedBoardsItemThatItIsTheirs(t *testing.T) {
	creator := projection.Viewer{Kind: projection.ViewerMember, UserID: new(samUserID), ParticipantID: &samID}
	other := projection.Viewer{Kind: projection.ViewerMember, UserID: new(otherUser), ParticipantID: &aliID}

	for _, board := range boardMatrix() {
		t.Run(fmt.Sprintf("%s/hidden=%t", board.Anonymity, board.CardsHidden), func(t *testing.T) {
			// given
			event := projection.ActionItemUpserted(board, convertedItem(), 1050)

			// when
			mine, creatorSeen := seenPayload(t, event, creator)
			theirs, otherSeen := seenPayload(t, event, other)
			_, personalForOther := event.PayloadFor(other)

			// then
			require.True(t, creatorSeen)
			require.True(t, otherSeen)
			assert.False(t, personalForOther, "everyone but the creator shares one payload")
			assert.NotContains(t, theirs, "is_mine")
			assert.NotContains(t, marshal(t, event.Payload(projection.ClassOthers)), "is_mine")

			if board.Anonymous() {
				_, personal := event.PayloadFor(creator)
				assert.False(t, personal, "an anonymous board never tells the creator an item is theirs")
				assert.NotContains(t, mine, "is_mine")

				return
			}

			assert.Contains(t, mine, `"is_mine":true`)
			assert.Equal(t, decode(t, theirs)["action_item"].(map[string]any)["title"], decode(t, mine)["action_item"].(map[string]any)["title"])
		})
	}
}
