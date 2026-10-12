package policy

import (
	"testing"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var (
	me    = uuid.MustParse("6f1c4a2e-1d3b-4c5a-9e7f-0a1b2c3d4e5f")
	other = uuid.MustParse("0b9d8c7a-6e5f-4a3b-8c2d-1e0f9a8b7c6d")
	stamp = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

	host   = model.BoardRoleFacilitator
	writer = model.BoardRoleParticipant
)

type (
	option func(*model.BoardAccess)
)

func access(role model.BoardRole, options ...option) model.BoardAccess {
	built := model.BoardAccess{
		Header:        model.BoardHeader{Board: model.Board{Anonymity: model.AnonymityNamed, Stage: model.StageWrite}},
		ParticipantID: new(me),
		Role:          role,
		OrgMember:     true,
		Capabilities:  Capabilities(role, true),
	}

	for _, apply := range options {
		apply(&built)
	}

	return built
}

func hidden(a *model.BoardAccess) {
	a.Header.CardsHidden = true
}

func cardsLocked(a *model.BoardAccess) {
	a.Header.CardsLocked = true
}

func movesLocked(a *model.BoardAccess) {
	a.Header.MovesLocked = true
}

func archived(a *model.BoardAccess) {
	a.Header.ArchivedAt = new(stamp)
}

func closed(a *model.BoardAccess) {
	a.Header.ClosedAt = new(stamp)
}

func locked(a *model.BoardAccess) {
	a.Header.LockedAt = new(stamp)
	a.Capabilities = []model.BoardCapability{}
	for _, capability := range Capabilities(a.Role, a.OrgMember) {
		if capability != model.BoardCapabilityParticipate {
			a.Capabilities = append(a.Capabilities, capability)
		}
	}
}

func in(stage model.Stage) option {
	return func(a *model.BoardAccess) {
		a.Header.Stage = stage
	}
}

func noParticipant(a *model.BoardAccess) {
	a.ParticipantID = nil
}

func noCapabilities(a *model.BoardAccess) {
	a.Capabilities = []model.BoardCapability{}
}

func mine() model.Card {
	return model.Card{ID: uuid.New(), AuthorParticipantID: new(me), Body: "mine"}
}

func theirs() model.Card {
	return model.Card{ID: uuid.New(), AuthorParticipantID: new(other), Body: "theirs"}
}

func authorless() model.Card {
	return model.Card{ID: uuid.New(), Body: "severed"}
}

func TestCapabilities_FollowTheBoardRoleTable(t *testing.T) {
	cases := []struct {
		name      string
		role      model.BoardRole
		orgMember bool
		want      []model.BoardCapability
	}{
		{
			name:      "a facilitator holds every capability",
			role:      model.BoardRoleFacilitator,
			orgMember: true,
			want: []model.BoardCapability{
				model.BoardCapabilityView,
				model.BoardCapabilityParticipate,
				model.BoardCapabilityFacilitate,
				model.BoardCapabilityManage,
				model.BoardCapabilityExport,
			},
		},
		{
			name:      "a facilitator holds every capability whatever the membership says",
			role:      model.BoardRoleFacilitator,
			orgMember: false,
			want: []model.BoardCapability{
				model.BoardCapabilityView,
				model.BoardCapabilityParticipate,
				model.BoardCapabilityFacilitate,
				model.BoardCapabilityManage,
				model.BoardCapabilityExport,
			},
		},
		{
			name:      "a participant who is an organisation member may export",
			role:      model.BoardRoleParticipant,
			orgMember: true,
			want:      []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityParticipate, model.BoardCapabilityExport},
		},
		{
			name:      "a participant through the link alone may not export",
			role:      model.BoardRoleParticipant,
			orgMember: false,
			want:      []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityParticipate},
		},
		{
			name:      "no role holds nothing",
			role:      "",
			orgMember: true,
			want:      []model.BoardCapability{},
		},
		{
			name:      "an unknown role holds nothing",
			role:      "owner",
			orgMember: true,
			want:      []model.BoardCapability{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := Capabilities(tc.role, tc.orgMember)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCapabilities_ReturnsAFreshSliceEachTime(t *testing.T) {
	// given
	first := Capabilities(model.BoardRoleFacilitator, true)

	// when
	first[0] = "changed"

	// then
	assert.Equal(t, model.BoardCapabilityView, Capabilities(model.BoardRoleFacilitator, true)[0])
}

func TestStanding_AsksTheProjectionWhetherTheCardIsVisibleAndOwned(t *testing.T) {
	cases := []struct {
		name        string
		access      model.BoardAccess
		card        model.Card
		wantVisible bool
		wantOwn     bool
	}{
		{name: "every card is visible once cards are shown", access: access(writer), card: theirs(), wantVisible: true},
		{name: "a severed card is visible once cards are shown", access: access(writer), card: authorless(), wantVisible: true},
		{name: "your own hidden card is visible to you", access: access(writer, hidden), card: mine(), wantVisible: true, wantOwn: true},
		{name: "someone else's hidden card is not", access: access(writer, hidden), card: theirs()},
		{name: "a facilitator cannot read someone else's hidden card", access: access(host, hidden), card: theirs()},
		{name: "a hidden card without an author is visible to nobody", access: access(host, hidden), card: authorless()},
		{name: "a viewer with no participant row sees no hidden card", access: access(host, hidden, noParticipant), card: mine()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			visible, own := standing(tc.access, tc.card)

			// then
			viewer := projection.ViewerFor(tc.access)
			assert.Equal(t, tc.wantVisible, visible)
			assert.Equal(t, tc.wantOwn, own)
			assert.Equal(t, projection.Visible(tc.access.Header.Board, tc.card, viewer), visible)
		})
	}
}

func TestCanCreateCard(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a participant writes a card", access: access(writer), want: nil},
		{name: "a participant writes while cards are hidden", access: access(writer, hidden), want: nil},
		{name: "a facilitator writes a card", access: access(host), want: nil},
		{name: "an archived board is read-only", access: access(host, archived), want: ErrReadOnly},
		{name: "a closed board is read-only", access: access(writer, closed), want: ErrReadOnly},
		{name: "a locked board is read-only for a participant", access: access(writer, locked), want: ErrReadOnly},
		{name: "a locked board is read-only for a facilitator's cards too", access: access(host, locked), want: ErrReadOnly},
		{name: "a viewer without board.participate may not", access: access(writer, noCapabilities), want: ErrNotAllowed},
		{name: "a member who has not joined has no author row", access: access(writer, noParticipant), want: ErrNotParticipant},
		{name: "locked cards stop a participant", access: access(writer, cardsLocked), want: ErrCardsLocked},
		{name: "locked cards do not stop a facilitator", access: access(host, cardsLocked), want: nil},
		{name: "no new card once the board is in its Vote stage", access: access(writer, in(model.StageVote)), want: ErrCardsOutOfStage},
		{name: "no new card in the Discuss stage", access: access(writer, in(model.StageDiscuss)), want: ErrCardsOutOfStage},
		{name: "no new card in the Actions stage", access: access(writer, in(model.StageActions)), want: ErrCardsOutOfStage},
		{name: "a facilitator is bound by the stage too", access: access(host, in(model.StageVote)), want: ErrCardsOutOfStage},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanCreateCard(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCanEditCard(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "an author edits their card", access: access(writer), card: mine(), want: nil},
		{name: "an author edits their hidden card", access: access(writer, hidden), card: mine(), want: nil},
		{name: "nobody edits someone else's visible card", access: access(writer), card: theirs(), want: ErrNotAllowed},
		{name: "not even a facilitator", access: access(host), card: theirs(), want: ErrNotAllowed},
		{name: "a severed card has no author to edit it", access: access(host), card: authorless(), want: ErrNotAllowed},
		{name: "someone else's hidden card does not exist for you", access: access(host, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "an archived board is read-only", access: access(writer, archived), card: mine(), want: ErrReadOnly},
		{name: "a closed board is read-only", access: access(writer, closed), card: mine(), want: ErrReadOnly},
		{name: "a locked board is read-only", access: access(host, locked), card: mine(), want: ErrReadOnly},
		{name: "a viewer without board.participate may not", access: access(writer, noCapabilities), card: mine(), want: ErrNotAllowed},
		{name: "locked cards stop an author", access: access(writer, cardsLocked), card: mine(), want: ErrCardsLocked},
		{name: "locked cards do not stop a facilitator author", access: access(host, cardsLocked), card: mine(), want: nil},
		{name: "an author edits no more once the board is in its Vote stage", access: access(writer, in(model.StageVote)), card: mine(), want: ErrCardsOutOfStage},
		{name: "a facilitator author edits no more in the Discuss stage", access: access(host, in(model.StageDiscuss)), card: mine(), want: ErrCardsOutOfStage},
		{name: "someone else's hidden card stays not found whatever the stage", access: access(writer, hidden, in(model.StageVote)), card: theirs(), want: ErrCardNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanEditCard(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCanDeleteCard(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "an author deletes their card", access: access(writer), card: mine(), want: nil},
		{name: "an author deletes their hidden card", access: access(writer, hidden), card: mine(), want: nil},
		{name: "a participant may not delete someone else's card", access: access(writer), card: theirs(), want: ErrNotAllowed},
		{name: "a facilitator removes any card they can see", access: access(host), card: theirs(), want: nil},
		{name: "a facilitator removes a severed card", access: access(host), card: authorless(), want: nil},
		{name: "a facilitator cannot remove a hidden card they cannot see", access: access(host, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "a participant cannot find someone else's hidden card", access: access(writer, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "a facilitator removes cards on a locked board", access: access(host, locked), card: theirs(), want: nil},
		{name: "a facilitator removes cards while cards are locked", access: access(host, cardsLocked), card: theirs(), want: nil},
		{name: "an archived board is read-only even for a facilitator", access: access(host, archived), card: theirs(), want: ErrReadOnly},
		{name: "a closed board is read-only", access: access(writer, closed), card: mine(), want: ErrReadOnly},
		{name: "a locked board is read-only for an author", access: access(writer, locked), card: mine(), want: ErrReadOnly},
		{name: "a viewer without board.participate may not", access: access(writer, noCapabilities), card: mine(), want: ErrNotAllowed},
		{name: "locked cards stop an author", access: access(writer, cardsLocked), card: mine(), want: ErrCardsLocked},
		{name: "an author deletes no more once the board is in its Vote stage", access: access(writer, in(model.StageVote)), card: mine(), want: ErrCardsOutOfStage},
		{name: "an author deletes no more in the Actions stage", access: access(writer, in(model.StageActions)), card: mine(), want: ErrCardsOutOfStage},
		{name: "a facilitator still removes a card in the Discuss stage", access: access(host, in(model.StageDiscuss)), card: theirs(), want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanDeleteCard(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCanMoveCard(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "a participant moves their own card", access: access(writer), card: mine(), want: nil},
		{name: "a participant may not move someone else's visible card", access: access(writer), card: theirs(), want: ErrNotAllowed},
		{name: "a facilitator moves someone else's visible card", access: access(host), card: theirs(), want: nil},
		{name: "nobody moves a card once the board is in its Vote stage", access: access(writer, in(model.StageVote)), card: mine(), want: ErrCardsOutOfStage},
		{name: "a facilitator moves no card in the Discuss stage", access: access(host, in(model.StageDiscuss)), card: theirs(), want: ErrCardsOutOfStage},
		{name: "an author moves their hidden card", access: access(writer, hidden), card: mine(), want: nil},
		{name: "nobody else can move a hidden card", access: access(host, hidden), card: theirs(), want: ErrCardNotFound},
		{name: "locked moves stop a participant", access: access(writer, movesLocked), card: mine(), want: ErrMovesLocked},
		{name: "locked moves do not stop a facilitator", access: access(host, movesLocked), card: theirs(), want: nil},
		{name: "a facilitator moves cards on a locked board", access: access(host, locked), card: theirs(), want: nil},
		{name: "a locked board is read-only for a participant", access: access(writer, locked), card: mine(), want: ErrReadOnly},
		{name: "an archived board is read-only", access: access(host, archived), card: mine(), want: ErrReadOnly},
		{name: "a closed board is read-only", access: access(writer, closed), card: mine(), want: ErrReadOnly},
		{name: "a viewer without board.participate may not", access: access(writer, noCapabilities), card: mine(), want: ErrNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanMoveCard(tc.access, tc.card)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCanManageColumns(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator manages columns", access: access(host), want: nil},
		{name: "a facilitator manages columns on a locked board", access: access(host, locked), want: nil},
		{name: "a participant may not", access: access(writer), want: ErrNotAllowed},
		{name: "an archived board is read-only", access: access(host, archived), want: ErrReadOnly},
		{name: "a closed board is read-only", access: access(host, closed), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanManageColumns(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCanManageBoard(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator manages the board", access: access(host), want: nil},
		{name: "a facilitator manages a locked board", access: access(host, locked), want: nil},
		{name: "a participant may not", access: access(writer), want: ErrNotAllowed},
		{name: "an archived board is read-only", access: access(host, archived), want: ErrReadOnly},
		{name: "a closed board is read-only", access: access(host, closed), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanManageBoard(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				assert.NoError(t, err)
			}
		})
	}
}

var (
	shutColumn     = uuid.MustParse("7a6b5c4d-3e2f-4a1b-9c8d-7e6f5a4b3c2d")
	revealedColumn = uuid.MustParse("8b7c6d5e-4f3a-4b2c-8d9e-0f1a2b3c4d5e")
)

func hiddenColumn(a *model.BoardAccess) {
	a.Header.HiddenColumnIDs = []uuid.UUID{shutColumn}
}

func columnRevealed(a *model.BoardAccess) {
	a.Header.RevealedColumnIDs = []uuid.UUID{revealedColumn}
}

func anyoneCanEdit(a *model.BoardAccess) {
	a.Header.AnyoneCanEdit = true
}

func inColumn(card model.Card, columnID uuid.UUID) model.Card {
	card.ColumnID = columnID

	return card
}

func TestCanShapeColumnsAndCanControlTimer(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator shapes columns", access: access(host), want: nil},
		{name: "a facilitator shapes columns on a locked board", access: access(host, locked), want: nil},
		{name: "a participant may not by default", access: access(writer), want: ErrNotAllowed},
		{name: "a seated participant may when anyone can edit", access: access(writer, anyoneCanEdit), want: nil},
		{name: "an unseated viewer may not even when anyone can edit", access: access(writer, anyoneCanEdit, noParticipant), want: ErrNotAllowed},
		{name: "a viewer who cannot participate may not", access: access(writer, anyoneCanEdit, noCapabilities), want: ErrNotAllowed},
		{name: "the board lock still stops participants", access: access(writer, anyoneCanEdit, locked), want: ErrReadOnly},
		{name: "an archived board is read-only", access: access(host, archived), want: ErrReadOnly},
		{name: "a closed board is read-only even when anyone can edit", access: access(writer, anyoneCanEdit, closed), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			shapeErr := CanShapeColumns(tc.access)
			timerErr := CanControlTimer(tc.access)

			// then
			assert.ErrorIs(t, shapeErr, tc.want)
			assert.ErrorIs(t, timerErr, tc.want)
			if tc.want == nil {
				assert.NoError(t, shapeErr)
				assert.NoError(t, timerErr)
			}
		})
	}
}

func TestCanManageColumns_StaysWithFacilitatorsWhenAnyoneCanEdit(t *testing.T) {
	// when
	err := CanManageColumns(access(writer, anyoneCanEdit))

	// then
	assert.ErrorIs(t, err, ErrNotAllowed)
}

func TestCanUseColumnAndCanMoveTo(t *testing.T) {
	open := uuid.MustParse("9c8d7e6f-5a4b-4c3d-ae1f-2a3b4c5d6e7f")
	cases := []struct {
		name   string
		access model.BoardAccess
		card   model.Card
		to     uuid.UUID
		want   error
	}{
		{name: "a move between two open columns", access: access(writer, hidden), card: inColumn(mine(), open), to: uuid.New(), want: nil},
		{name: "a move into a hidden column", access: access(host, hiddenColumn), card: inColumn(mine(), open), to: shutColumn, want: ErrColumnHidden},
		{name: "a move from a hidden-cards column into a revealed one", access: access(writer, hidden, columnRevealed), card: inColumn(mine(), open), to: revealedColumn, want: ErrMoveAcrossReveal},
		{name: "a move out of a revealed column into a hidden-cards one", access: access(writer, hidden, columnRevealed), card: inColumn(mine(), revealedColumn), to: open, want: ErrMoveAcrossReveal},
		{name: "a revealed column means nothing once every card is shown", access: access(writer, columnRevealed), card: inColumn(mine(), open), to: revealedColumn, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanMoveTo(tc.access, tc.card, tc.to)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.want == nil {
				assert.NoError(t, err)
			}
		})
	}

	assert.ErrorIs(t, CanUseColumn(access(host, hiddenColumn), shutColumn), ErrColumnHidden)
	assert.NoError(t, CanUseColumn(access(host, hiddenColumn), open))
}

func frozen(role model.BoardRole, stage model.Stage) model.BoardAccess {
	built := access(role, hiddenColumn, in(stage))
	built.Header.VotingOpen = true
	built.Header.CommentsEnabled = true
	built.Header.MergingEnabled = true

	return built
}

func TestCardRules_AHiddenColumnIsFrozenForFacilitatorsAndUnseenByEveryoneElse(t *testing.T) {
	// given cards shown, with the column hidden from participants
	theirCard := inColumn(theirs(), shutColumn)
	myCard := inColumn(mine(), shutColumn)
	target := inColumn(theirs(), uuid.New())
	stacked := inColumn(theirs(), shutColumn)
	stacked.ParentCardID = new(uuid.New())
	comment := model.CardComment{ID: uuid.New(), CardID: myCard.ID, AuthorParticipantID: new(me)}
	write := model.StageWrite
	vote := model.StageVote
	discuss := model.StageDiscuss

	// when
	cases := []struct {
		name            string
		facilitator     error
		participant     error
		wantParticipant error
	}{
		{name: "edit", facilitator: CanEditCard(frozen(host, write), myCard), participant: CanEditCard(frozen(writer, write), myCard), wantParticipant: ErrCardNotFound},
		{name: "delete", facilitator: CanDeleteCard(frozen(host, write), theirCard), participant: CanDeleteCard(frozen(writer, write), theirCard), wantParticipant: ErrCardNotFound},
		{name: "move", facilitator: CanMoveCard(frozen(host, write), theirCard), participant: CanMoveCard(frozen(writer, write), theirCard), wantParticipant: ErrCardNotFound},
		{name: "vote", facilitator: CanVote(frozen(host, vote), theirCard), participant: CanVote(frozen(writer, vote), theirCard), wantParticipant: ErrCardNotFound},
		{name: "comment", facilitator: CanComment(frozen(host, discuss), theirCard), participant: CanComment(frozen(writer, discuss), theirCard), wantParticipant: ErrCardNotFound},
		{name: "edit a comment", facilitator: CanEditComment(frozen(host, discuss), myCard, comment), participant: CanEditComment(frozen(writer, discuss), myCard, comment), wantParticipant: ErrCommentNotFound},
		{name: "delete a comment", facilitator: CanDeleteComment(frozen(host, discuss), myCard, comment), participant: CanDeleteComment(frozen(writer, discuss), myCard, comment), wantParticipant: ErrCommentNotFound},
		{name: "merge", facilitator: CanMerge(frozen(host, discuss), theirCard, target), participant: CanMerge(frozen(writer, discuss), theirCard, target), wantParticipant: ErrCardNotFound},
		{name: "merge into", facilitator: CanMerge(frozen(host, discuss), target, theirCard), participant: CanMerge(frozen(writer, discuss), target, theirCard), wantParticipant: ErrCardNotFound},
		{name: "unmerge", facilitator: CanUnmerge(frozen(host, discuss), stacked), participant: CanUnmerge(frozen(writer, discuss), stacked), wantParticipant: ErrCardNotFound},
	}

	// then a facilitator learns why, and a participant is never told the column exists
	for _, tc := range cases {
		assert.ErrorIs(t, tc.facilitator, ErrColumnHidden, tc.name)
		assert.ErrorIs(t, tc.participant, tc.wantParticipant, tc.name)
	}
}

func TestCanMerge_ChecksTheCardsOwnColumnsForHiddenCards(t *testing.T) {
	// given a facilitator in Discuss with cards hidden only in another column
	a := access(host, hidden, columnRevealed, in(model.StageDiscuss))
	a.Header.MergingEnabled = true
	card := inColumn(theirs(), revealedColumn)
	target := inColumn(theirs(), revealedColumn)
	stacked := inColumn(theirs(), revealedColumn)
	stacked.ParentCardID = new(target.ID)

	// when
	mergeErr := CanMerge(a, card, target)
	unmergeErr := CanUnmerge(a, stacked)

	// then
	assert.NoError(t, mergeErr)
	assert.NoError(t, unmergeErr)
}
