package policy

import (
	"testing"

	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
)

func notMember(a *model.BoardAccess) {
	a.OrgMember = false
}

func TestCanReact(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "a participant on someone else's card", access: access(writer), card: theirs(), want: nil},
		{name: "a participant in any stage", access: access(writer, in(model.StageDiscuss)), card: theirs(), want: nil},
		{name: "a guest who is no organisation member", access: access(writer, notMember), card: theirs(), want: nil},
		{name: "a facilitator on a severed card", access: access(host), card: authorless(), want: nil},
		{name: "a viewer with no seat", access: access(writer, noParticipant), card: theirs(), want: ErrNotParticipant},
		{name: "someone else's hidden card", access: access(writer, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "their own hidden card", access: access(writer, hidden), card: mine(), want: ErrCardNotPublic},
		{name: "a card in a hidden column, even for a facilitator", access: access(host, hiddenColumn), card: inColumn(theirs(), shutColumn), want: ErrCardNotPublic},
		{name: "a card in a hidden column for a participant", access: access(writer, hiddenColumn), card: inColumn(theirs(), shutColumn), want: ErrCardNotFound},
		{name: "a locked board", access: access(writer, locked), card: theirs(), want: ErrReadOnly},
		{name: "an archived board", access: access(writer, archived), card: theirs(), want: ErrReadOnly},
		{name: "a closed board", access: access(writer, closed), card: theirs(), want: ErrReadOnly},
		{name: "a viewer who cannot participate", access: access(writer, noCapabilities), card: theirs(), want: ErrNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanReact(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanPin(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "a facilitator on a card everyone sees", access: access(host), card: theirs(), want: nil},
		{name: "a facilitator on a locked board", access: access(host, locked), card: theirs(), want: nil},
		{name: "a participant", access: access(writer), card: mine(), want: ErrNotAllowed},
		{name: "a facilitator on their own hidden card", access: access(host, hidden), card: mine(), want: ErrCardNotPublic},
		{name: "a facilitator on someone else's hidden card", access: access(host, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "a facilitator on a card in a hidden column", access: access(host, hiddenColumn), card: inColumn(theirs(), shutColumn), want: ErrCardNotPublic},
		{name: "an archived board", access: access(host, archived), card: theirs(), want: ErrReadOnly},
		{name: "a closed board", access: access(host, closed), card: theirs(), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanPin(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanSetCardImage(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "a member on their own card", access: access(writer), card: mine(), want: nil},
		{name: "a member on their own hidden card", access: access(writer, hidden), card: mine(), want: nil},
		{name: "a guest on their own card", access: access(writer, notMember), card: mine(), want: ErrMembersOnly},
		{name: "a member on someone else's card", access: access(writer), card: theirs(), want: ErrNotAllowed},
		{name: "outside the Write stage", access: access(writer, in(model.StageVote)), card: mine(), want: ErrCardsOutOfStage},
		{name: "cards locked", access: access(writer, cardsLocked), card: mine(), want: ErrCardsLocked},
		{name: "a closed board", access: access(writer, closed), card: mine(), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanSetCardImage(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanUploadImage(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a member with a seat", access: access(writer), want: nil},
		{name: "a member in any stage", access: access(writer, in(model.StageActions)), want: nil},
		{name: "a guest", access: access(writer, notMember), want: ErrMembersOnly},
		{name: "a member with no seat", access: access(writer, noParticipant), want: ErrNotParticipant},
		{name: "a locked board", access: access(writer, locked), want: ErrReadOnly},
		{name: "an archived board", access: access(writer, archived), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanUploadImage(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanSetBackground(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator", access: access(host), want: nil},
		{name: "a participant", access: access(writer), want: ErrNotAllowed},
		{name: "an archived board", access: access(host, archived), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanSetBackground(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
