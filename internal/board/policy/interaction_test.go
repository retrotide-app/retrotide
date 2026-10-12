package policy

import (
	"testing"

	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func votingOpen(a *model.BoardAccess) {
	a.Header.Stage = model.StageVote
	a.Header.VotingOpen = true
}

func commentsOn(a *model.BoardAccess) {
	a.Header.Stage = model.StageDiscuss
	a.Header.CommentsEnabled = true
}

func mergingOn(a *model.BoardAccess) {
	a.Header.Stage = model.StageDiscuss
	a.Header.MergingEnabled = true
}

func commentBy(author *uuid.UUID) model.CardComment {
	return model.CardComment{ID: uuid.New(), AuthorParticipantID: author, Body: "a comment"}
}

func stackedOn(parent model.Card) model.Card {
	card := theirs()
	card.ParentCardID = new(parent.ID)

	return card
}

func TestCanFacilitate(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator", access: access(host), want: nil},
		{name: "a facilitator on a locked board", access: access(host, locked), want: nil},
		{name: "a facilitator in any stage before the end", access: access(host, in(model.StageActions)), want: nil},
		{name: "a participant", access: access(writer), want: ErrNotAllowed},
		{name: "an archived board", access: access(host, archived), want: ErrReadOnly},
		{name: "a closed board", access: access(host, closed), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanFacilitate(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanHideCards(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator in the Write stage", access: access(host), want: nil},
		{name: "a facilitator once the board is in its Vote stage", access: access(host, in(model.StageVote)), want: ErrHideOutOfStage},
		{name: "a facilitator in the Discuss stage", access: access(host, in(model.StageDiscuss)), want: ErrHideOutOfStage},
		{name: "a participant", access: access(writer), want: ErrNotAllowed},
		{name: "a closed board", access: access(host, closed, in(model.StageEnd)), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanHideCards(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanVote(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "a participant on a shown card", access: access(writer, votingOpen), card: theirs(), want: nil},
		{name: "a facilitator on their own card", access: access(host, votingOpen), card: mine(), want: nil},
		{name: "the author on their own hidden card", access: access(writer, votingOpen, hidden), card: mine(), want: nil},
		{name: "anyone else on a hidden card", access: access(host, votingOpen, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "a hidden card while voting is closed still says nothing about it", access: access(writer, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "voting closed", access: access(writer, in(model.StageVote)), card: theirs(), want: ErrVotingClosed},
		{name: "not yet, while the board is in its Write stage", access: access(writer, votingOpen, in(model.StageWrite)), card: theirs(), want: ErrVotesOutOfStage},
		{name: "no more, once the board is in its Discuss stage", access: access(writer, votingOpen, in(model.StageDiscuss)), card: theirs(), want: ErrVotesOutOfStage},
		{name: "a facilitator is bound by the stage too", access: access(host, votingOpen, in(model.StageActions)), card: theirs(), want: ErrVotesOutOfStage},
		{name: "not joined", access: access(writer, votingOpen, noParticipant), card: theirs(), want: ErrNotParticipant},
		{name: "a locked board", access: access(host, votingOpen, locked), card: theirs(), want: ErrReadOnly},
		{name: "an archived board", access: access(writer, votingOpen, archived), card: theirs(), want: ErrReadOnly},
		{name: "a closed board", access: access(writer, votingOpen, closed), card: theirs(), want: ErrReadOnly},
		{name: "no capability", access: access(writer, votingOpen, noCapabilities), card: theirs(), want: ErrNotAllowed},
		{name: "a card with no author", access: access(writer, votingOpen), card: authorless(), want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanVote(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanComment(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "a participant on a shown card", access: access(writer, commentsOn), card: theirs(), want: nil},
		{name: "the author on their own hidden card", access: access(writer, commentsOn, hidden), card: mine(), want: nil},
		{name: "a facilitator on someone else's hidden card", access: access(host, commentsOn, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "comments switched off", access: access(writer, in(model.StageDiscuss)), card: theirs(), want: ErrCommentsDisabled},
		{name: "not yet, while the board is in its Write stage", access: access(writer, commentsOn, in(model.StageWrite)), card: theirs(), want: ErrCommentsOutOfStage},
		{name: "not while people vote", access: access(writer, commentsOn, in(model.StageVote)), card: theirs(), want: ErrCommentsOutOfStage},
		{name: "a facilitator is bound by the stage too", access: access(host, commentsOn, in(model.StageActions)), card: theirs(), want: ErrCommentsOutOfStage},
		{name: "not joined", access: access(writer, commentsOn, noParticipant), card: theirs(), want: ErrNotParticipant},
		{name: "a locked board", access: access(writer, commentsOn, locked), card: theirs(), want: ErrReadOnly},
		{name: "an archived board", access: access(writer, commentsOn, archived), card: theirs(), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanComment(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanEditComment(t *testing.T) {
	cases := []struct {
		name    string
		access  model.BoardAccess
		card    model.Card
		comment model.CardComment
		want    error
	}{
		{name: "the writer", access: access(writer, commentsOn), card: theirs(), comment: commentBy(new(me)), want: nil},
		{name: "a facilitator on someone else's comment", access: access(host, commentsOn), card: theirs(), comment: commentBy(new(other)), want: ErrNotAllowed},
		{name: "a severed comment", access: access(writer, commentsOn), card: theirs(), comment: commentBy(nil), want: ErrNotAllowed},
		{name: "the writer once the card is hidden from them", access: access(writer, commentsOn, hidden), card: theirs(), comment: commentBy(new(me)), want: ErrCommentNotFound},
		{name: "the writer once comments are switched off", access: access(writer, in(model.StageDiscuss)), card: theirs(), comment: commentBy(new(me)), want: ErrCommentsDisabled},
		{name: "the writer once the board has moved on to Actions", access: access(writer, commentsOn, in(model.StageActions)), card: theirs(), comment: commentBy(new(me)), want: ErrCommentsOutOfStage},
		{name: "a locked board", access: access(writer, commentsOn, locked), card: theirs(), comment: commentBy(new(me)), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanEditComment(tc.access, tc.card, tc.comment)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanDeleteComment(t *testing.T) {
	cases := []struct {
		name    string
		access  model.BoardAccess
		card    model.Card
		comment model.CardComment
		want    error
	}{
		{name: "the writer", access: access(writer, in(model.StageDiscuss)), card: theirs(), comment: commentBy(new(me)), want: nil},
		{name: "the writer once the board has moved on to Actions", access: access(writer, in(model.StageActions)), card: theirs(), comment: commentBy(new(me)), want: ErrCommentsOutOfStage},
		{name: "a facilitator removing anyone's comment", access: access(host), card: theirs(), comment: commentBy(new(other)), want: nil},
		{name: "a facilitator removing a comment in any stage", access: access(host, in(model.StageActions)), card: theirs(), comment: commentBy(new(other)), want: nil},
		{name: "a facilitator on a locked board", access: access(host, locked), card: theirs(), comment: commentBy(new(other)), want: nil},
		{name: "a facilitator on a hidden card they cannot see", access: access(host, hidden), card: theirs(), comment: commentBy(new(other)), want: ErrCommentNotFound},
		{name: "a participant on someone else's comment", access: access(writer, in(model.StageDiscuss)), card: theirs(), comment: commentBy(new(other)), want: ErrNotAllowed},
		{name: "a participant on a locked board", access: access(writer, locked), card: theirs(), comment: commentBy(new(me)), want: ErrReadOnly},
		{name: "an archived board", access: access(host, archived), card: theirs(), comment: commentBy(new(me)), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanDeleteComment(tc.access, tc.card, tc.comment)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanMerge(t *testing.T) {
	target := theirs()

	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		target model.Card
		want   error
	}{
		{name: "a participant", access: access(writer, mergingOn), card: mine(), target: target, want: nil},
		{name: "a participant merging someone else's card", access: access(writer, mergingOn), card: theirs(), target: target, want: nil},
		{name: "a facilitator while moves are locked", access: access(host, mergingOn, movesLocked), card: theirs(), target: target, want: nil},
		{name: "a participant while moves are locked", access: access(writer, mergingOn, movesLocked), card: mine(), target: target, want: ErrMovesLocked},
		{name: "merging switched off", access: access(host, in(model.StageDiscuss)), card: mine(), target: target, want: ErrMergingDisabled},
		{name: "not while people write", access: access(writer, mergingOn, in(model.StageWrite)), card: mine(), target: target, want: ErrMergesOutOfStage},
		{name: "not while people vote", access: access(writer, mergingOn, in(model.StageVote)), card: mine(), target: target, want: ErrMergesOutOfStage},
		{name: "a facilitator is bound by the stage too", access: access(host, mergingOn, in(model.StageActions)), card: mine(), target: target, want: ErrMergesOutOfStage},
		{name: "cards hidden, with both cards the merger's own", access: access(writer, mergingOn, hidden), card: mine(), target: mine(), want: ErrCardsHidden},
		{name: "cards hidden, onto someone else's card", access: access(host, mergingOn, hidden), card: mine(), target: target, want: ErrCardNotFound},
		{name: "a card onto itself", access: access(writer, mergingOn), card: target, target: target, want: ErrMergeIntoSelf},
		{name: "a locked board", access: access(writer, mergingOn, locked), card: mine(), target: target, want: ErrReadOnly},
		{name: "an archived board", access: access(host, mergingOn, archived), card: mine(), target: target, want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanMerge(tc.access, tc.card, tc.target)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanUnmerge(t *testing.T) {
	parent := theirs()
	discussing := in(model.StageDiscuss)

	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "a participant", access: access(writer, discussing), card: stackedOn(parent), want: nil},
		{name: "a participant even with merging switched off", access: access(writer, discussing), card: stackedOn(parent), want: nil},
		{name: "a card that is not stacked", access: access(writer, discussing), card: parent, want: ErrNotMerged},
		{name: "not while people vote", access: access(writer, in(model.StageVote)), card: stackedOn(parent), want: ErrMergesOutOfStage},
		{name: "a facilitator is bound by the stage too", access: access(host, in(model.StageActions)), card: stackedOn(parent), want: ErrMergesOutOfStage},
		{name: "cards hidden", access: access(host, hidden), card: stackedOn(parent), want: ErrCardNotFound},
		{name: "a participant while moves are locked", access: access(writer, discussing, movesLocked), card: stackedOn(parent), want: ErrMovesLocked},
		{name: "a facilitator while moves are locked", access: access(host, discussing, movesLocked), card: stackedOn(parent), want: nil},
		{name: "a closed board", access: access(host, closed), card: stackedOn(parent), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanUnmerge(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
