package projection_test

import (
	"slices"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	shareSlug = "k3v9x2"
)

var (
	guestDevice = uuid.MustParse("7f8a9b0c-1d2e-4f3a-8b4c-5d6e7f8a9b0c")
)

func guestBoardState(anonymity model.Anonymity, hidden bool, names bool) model.BoardState {
	state := boardState(anonymity, hidden)
	state.Header.LinkAccess = model.LinkAccessGuests
	state.Header.PasswordHash = new("$2a$10$not.a.real.hash")
	state.Header.GuestNamesEnabled = names

	for i := range state.Participants {
		if state.Participants[i].ID == guestID {
			state.Participants[i].GuestDeviceID = new(guestDevice)
			state.Participants[i].JoinedVia = model.JoinPathLink
		}
	}

	state.Cards = append(state.Cards, model.Card{
		ID:                  uuid.MustParse("e5f6a7b8-c9d0-4e1f-8a2b-3c4d5e6f7a8b"),
		BoardID:             boardID,
		ColumnID:            columnB,
		AuthorParticipantID: new(guestID),
		Body:                "GUEST-TEXT-77aa",
		Position:            "a1",
		CreatedAt:           at("2026-09-22T10:04:00Z"),
	})

	return state
}

func TestSharing_ReachesOnlyAViewerWhoManagesTheBoard(t *testing.T) {
	for _, board := range boardMatrix() {
		for _, tc := range viewerCases() {
			t.Run(string(board.Anonymity)+"/"+tc.name, func(t *testing.T) {
				// given
				board.ShareSlug = shareSlug
				board.LinkAccess = model.LinkAccessGuests

				// when
				view := projection.Sharing(board, tc.viewer)
				response := marshal(t, projection.SharingResponse(board, tc.viewer, 1044))

				// then the slug is a capability, so only a manager, who may also rotate it, is ever sent it
				manages := slices.Contains(tc.viewer.Capabilities, model.BoardCapabilityManage)
				if !manages {
					assert.Nil(t, view)
					assert.NotContains(t, response, shareSlug)
					assert.JSONEq(t, `{"version":1044}`, response)
					return
				}

				require.NotNil(t, view)
				assert.Equal(t, dto.SharingView{ShareSlug: shareSlug, LinkAccess: model.LinkAccessGuests}, *view)
			})
		}
	}
}

func TestSnapshot_NeverCarriesTheSharingForAnyViewer(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, hidden := range []bool{true, false} {
			for _, tc := range viewerCases() {
				t.Run(string(anonymity)+"/"+tc.name, func(t *testing.T) {
					// given
					state := guestBoardState(anonymity, hidden, true)
					state.Header.ShareSlug = shareSlug

					// when
					payload := marshal(t, projection.Snapshot(state, tc.viewer))

					// then the snapshot every viewer receives holds neither the slug, the password hash nor the guest's device
					assert.NotContains(t, payload, shareSlug)
					assert.NotContains(t, payload, "$2a$")
					assert.NotContains(t, payload, guestDevice.String())
					assert.NotContains(t, payload, "link_access")
				})
			}
		}
	}
}

func TestSnapshot_AGuestsHiddenCardStaysHiddenFromEveryoneButTheGuest(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(string(anonymity)+"/"+tc.name, func(t *testing.T) {
				// given a guest's card while cards are hidden
				state := guestBoardState(anonymity, true, true)

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then
				if tc.viewer.ParticipantID != nil && *tc.viewer.ParticipantID == guestID {
					assert.Contains(t, payload, "GUEST-TEXT-77aa")
					return
				}

				assert.NotContains(t, payload, "GUEST-TEXT-77aa")
				assert.NotContains(t, payload, "e5f6a7b8-c9d0-4e1f-8a2b-3c4d5e6f7a8b")
			})
		}
	}
}

func TestSnapshot_AGuestNeverSeesAnotherAuthorOnAnAnonymousBoard(t *testing.T) {
	// given
	state := guestBoardState(model.AnonymityAnonymous, false, true)
	guest := viewerCases()[2].viewer

	// when
	snapshot := projection.Snapshot(state, guest)

	// then the cards are shown but no author, participant ID of an author or timestamp rides with them
	payload := marshal(t, snapshot)
	assert.Contains(t, payload, secretText)
	assert.NotContains(t, payload, `"author":{`)
	for _, card := range snapshot.Cards {
		assert.Nil(t, card.Author)
		assert.Empty(t, card.CreatedAt)
	}
}

func TestParticipantName_AGuestShowsAsAnonymousWhileGuestNamesAreOff(t *testing.T) {
	tests := []struct {
		name  string
		names bool
		want  string
	}{
		{name: "names on", names: true, want: "Gil"},
		{name: "names off", names: false, want: "Anonymous"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			state := guestBoardState(model.AnonymityNamed, false, tc.names)
			viewer := viewerCases()[0].viewer

			// when
			snapshot := projection.Snapshot(state, viewer)

			// then the participant list and the card's author agree, and an account holder keeps their name
			var listed []string
			for _, participant := range snapshot.Participants {
				if participant.ParticipantID == guestID {
					listed = append(listed, participant.DisplayName)
				}
			}
			assert.Equal(t, []string{tc.want}, listed)

			for _, card := range snapshot.Cards {
				if card.Author != nil && card.Author.ParticipantID == guestID {
					assert.Equal(t, tc.want, card.Author.DisplayName)
				}
			}

			assert.NotContains(t, marshal(t, snapshot), `"Anonymous","role":"facilitator"`)
		})
	}
}

func TestParticipantUpserted_NamesAGuestByTheBoardsRule(t *testing.T) {
	// given
	state := guestBoardState(model.AnonymityNamed, true, false)
	guestProfile := state.Participants[4]
	require.Equal(t, guestID, guestProfile.ID)

	// when
	payload, seen := seenPayload(t, projection.ParticipantUpserted(state.Header.Board, guestProfile, 1044), viewerCases()[0].viewer)

	// then
	require.True(t, seen)
	assert.JSONEq(t, `{"type":"participant_upserted","version":1044,"participant":{"participant_id":"5d1e2f3a-4b5c-4d6e-bf7a-8b9c0d1e2f3a","display_name":"Anonymous","role":"participant"}}`, payload)
}

func TestJoinPreview_CarriesOnlyTheThreeAnswers(t *testing.T) {
	// when
	payload := marshal(t, projection.JoinPreview(projection.LinkVerdict{RequiresPassword: true, CanJoin: true}))

	// then the title and the organisation are withheld until access is granted
	assert.JSONEq(t, `{"requires_password":true,"requires_account":false,"can_join":true}`, payload)
}

func TestGuestClaimOffer_OffersNamedBoardsOnly(t *testing.T) {
	// given a device seated on a named board, an anonymous board and a board it was removed from
	named := boardState(model.AnonymityNamed, true).Header.Board
	anonymousBoard := boardState(model.AnonymityAnonymous, true).Header.Board
	anonymousBoard.ID = uuid.MustParse("9a0b1c2d-3e4f-4a5b-8c6d-7e8f9a0b1c2d")
	anonymousBoard.Title = "ANON-TITLE-31cc"
	removedFrom := boardState(model.AnonymityNamed, true).Header.Board
	removedFrom.ID = uuid.MustParse("0b1c2d3e-4f5a-4b6c-9d7e-8f9a0b1c2d3e")
	removedFrom.Title = "REMOVED-TITLE-52dd"
	removedAt := at("2026-09-22T11:00:00Z")

	joined := []model.GuestBoard{
		{Board: named, Participant: model.BoardParticipant{ID: guestID, BoardID: named.ID, GuestDeviceID: new(guestDevice), DisplayName: ""}},
		{Board: anonymousBoard, Participant: model.BoardParticipant{ID: uuid.New(), BoardID: anonymousBoard.ID, GuestDeviceID: new(guestDevice), DisplayName: "Gil"}},
		{Board: removedFrom, Participant: model.BoardParticipant{ID: uuid.New(), BoardID: removedFrom.ID, GuestDeviceID: new(guestDevice), DisplayName: "Gil", RemovedAt: &removedAt}},
	}

	// when
	offer := projection.GuestClaimOffer(joined)
	payload := marshal(t, offer)

	// then claiming an anonymous board would link anonymous cards to an account, so it is never even named
	assert.Equal(t, dto.GuestClaimOffer{Boards: []dto.GuestClaimBoard{{BoardID: named.ID, Title: "Sprint 42", DisplayName: "Anonymous"}}}, offer)
	assert.NotContains(t, payload, "ANON-TITLE-31cc")
	assert.NotContains(t, payload, "REMOVED-TITLE-52dd")
	assert.NotContains(t, payload, guestID.String())
	assert.NotContains(t, payload, guestDevice.String())
}

func TestGuestClaimOffer_IsAnEmptyListWhenNothingIsOffered(t *testing.T) {
	// when
	payload := marshal(t, projection.GuestClaimOffer(nil))

	// then
	assert.JSONEq(t, `{"boards":[]}`, payload)
}

func TestLinkJoined_NamesTheOrganisationOnlyToAMember(t *testing.T) {
	orgID := uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")
	header := model.BoardHeader{ID: boardID, OrgID: orgID}

	tests := []struct {
		name   string
		member bool
		want   dto.LinkJoinResponse
	}{
		{name: "a member", member: true, want: dto.LinkJoinResponse{BoardID: boardID, OrgID: &orgID}},
		{name: "a guest or an outside account", member: false, want: dto.LinkJoinResponse{BoardID: boardID}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			response := projection.LinkJoined(model.BoardAccess{Header: header, OrgMember: tc.member})

			// then only a member learns which organisation holds the board, so it can open it there
			assert.Equal(t, tc.want, response)
		})
	}
}
