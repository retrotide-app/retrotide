package policy

import (
	"testing"

	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
)

func asUser(a *model.BoardAccess) {
	a.Principal = model.Principal{Kind: model.PrincipalUser, ID: me}
}

func asGuest(a *model.BoardAccess) {
	a.Principal = model.Principal{Kind: model.PrincipalGuest, ID: me}
	a.OrgMember = false
}

func anonymousBoard(a *model.BoardAccess) {
	a.Header.Anonymity = model.AnonymityAnonymous
}

func ended(a *model.BoardAccess) {
	a.Header.ClosedAt = new(stamp)
	a.Header.Stage = model.StageEnd
	a.Capabilities = []model.BoardCapability{model.BoardCapabilityView}
}

func TestCanWriteActionItems(t *testing.T) {
	tests := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a participant in Actions", access: access(writer, asUser, in(model.StageActions))},
		{name: "a guest in Actions", access: access(writer, asGuest, in(model.StageActions))},
		{name: "a facilitator in Discuss", access: access(host, asUser, in(model.StageDiscuss)), want: ErrActionItemsOutOfStage},
		{name: "a participant who has not joined", access: access(writer, asUser, in(model.StageActions), noParticipant), want: ErrNotParticipant},
		{name: "a locked board", access: access(writer, asUser, in(model.StageActions), locked), want: ErrReadOnly},
		{name: "an ended retro", access: access(writer, asUser, ended), want: ErrReadOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanWriteActionItems(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanMarkActionItem(t *testing.T) {
	tests := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a participant in Actions", access: access(writer, asUser, in(model.StageActions))},
		{name: "a guest in Actions", access: access(writer, asGuest, in(model.StageActions))},
		{name: "a member after the end", access: access(writer, asUser, ended, noParticipant)},
		{name: "a guest after the end", access: access(writer, asGuest, ended), want: ErrReadOnly},
		{name: "an archived board", access: access(host, asUser, in(model.StageActions), archived), want: ErrReadOnly},
		{name: "a participant in Vote", access: access(writer, asUser, in(model.StageVote)), want: ErrActionItemsOutOfStage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanMarkActionItem(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanDeleteActionItem(t *testing.T) {
	mine := model.ActionItem{CreatedBy: new(me)}
	theirs := model.ActionItem{CreatedBy: new(other)}

	tests := []struct {
		name   string
		access model.BoardAccess
		item   model.ActionItem
		want   error
	}{
		{name: "a facilitator in any stage before the end", access: access(host, asUser, in(model.StageWrite)), item: theirs},
		{name: "the creator in Actions", access: access(writer, asUser, in(model.StageActions)), item: mine},
		{name: "a creator stored before the board turned anonymous", access: access(writer, asUser, in(model.StageActions), anonymousBoard), item: mine, want: ErrNotAllowed},
		{name: "a facilitator on an anonymous board", access: access(host, asUser, in(model.StageActions), anonymousBoard), item: theirs},
		{name: "the creator outside Actions", access: access(writer, asUser, in(model.StageDiscuss)), item: mine, want: ErrActionItemsOutOfStage},
		{name: "someone else's item", access: access(writer, asUser, in(model.StageActions)), item: theirs, want: ErrNotAllowed},
		{name: "a guest", access: access(writer, asGuest, in(model.StageActions)), item: mine, want: ErrNotAllowed},
		{name: "an ended retro", access: access(host, asUser, closed), item: mine, want: ErrReadOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanDeleteActionItem(tc.access, tc.item)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanReviewActionItemsAndCanNameMembers(t *testing.T) {
	tests := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a member in any stage", access: access(writer, asUser, in(model.StageWrite))},
		{name: "a guest", access: access(writer, asGuest), want: ErrMembersOnly},
		{name: "an account in through the link alone", access: access(writer, asUser, func(a *model.BoardAccess) { a.OrgMember = false }), want: ErrMembersOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			reviewErr := CanReviewActionItems(tc.access)
			nameErr := CanNameMembers(tc.access)

			// then
			assert.ErrorIs(t, reviewErr, tc.want)
			assert.ErrorIs(t, nameErr, tc.want)
		})
	}
}
