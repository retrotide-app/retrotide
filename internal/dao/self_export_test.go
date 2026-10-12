package dao_test

import (
	"testing"

	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	otherOrganisation struct {
		workspace *model.Workspace
		state     *model.BoardState
	}
)

func (f boardFixture) otherOrganisation(t *testing.T, member *model.User) otherOrganisation {
	t.Helper()

	workspace := daotest.CreateOrg(t, f.repos, daotest.CreateUser(t, f.repos, daotest.WithDisplayName("Bea")).ID)
	daotest.AddOrgMember(t, f.repos, workspace.Organisation.ID, member.ID, model.OrgRoleMember)
	state := daotest.CreateOrganisationBoard(t, f.repos, workspace.Organisation.ID, workspace.Owner.UserID)

	return otherOrganisation{workspace: workspace, state: state}
}

func (o otherOrganisation) cardBy(t *testing.T, f boardFixture, author uuid.UUID, body string) model.Card {
	t.Helper()

	header, err := f.repos.Board.GetHeader(t.Context(), o.state.Header.ID)
	require.NoError(t, err)

	added, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
		OrgID: o.workspace.Organisation.ID, BoardID: o.state.Header.ID, Version: header.Version, OpID: uuid.New(),
		CardID:              uuid.New(),
		ColumnID:            o.state.Columns[0].ID,
		AuthorParticipantID: new(author),
		Body:                body,
		Position:            "a" + uuid.NewString()[:4],
	})
	require.NoError(t, err)

	return added.Value
}

func writtenBodies(cards []model.WrittenCard) []string {
	bodies := make([]string, 0, len(cards))
	for _, card := range cards {
		bodies = append(bodies, card.Body)
	}

	return bodies
}

func TestCardListWrittenByUser_ListsTheUsersOwnCardsInEveryOrganisationAndNoOneElses(t *testing.T) {
	// given Sam writes on a board of organisation A and on one of organisation B, beside the owner of A
	f := newBoardFixture(t)
	sam := f.teamMember(t, "Sam")
	samInA := f.seat(t, sam)
	f.mentionCard(t, samInA, "Sam in A")
	f.mentionCard(t, f.facilitator(), "Owner in A")
	other := f.otherOrganisation(t, sam)
	samInB := daotest.CreateParticipant(t, f.repos, other.state.Header.ID, sam.ID).ID
	other.cardBy(t, f, samInB, "Sam in B")

	// when
	cards, err := f.repos.Card.ListWrittenByUser(t.Context(), sam.ID)

	// then
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Sam in A", "Sam in B"}, writtenBodies(cards))
	for _, card := range cards {
		assert.Equal(t, "Went well", card.ColumnTitle)
		assert.False(t, card.CreatedAt.IsZero())

		if card.Body == "Sam in A" {
			assert.Equal(t, f.workspace.Organisation.Name, card.OrgName)
			assert.Equal(t, f.state.Header.Title, card.BoardTitle)

			continue
		}

		assert.Equal(t, other.workspace.Organisation.Name, card.OrgName)
		assert.Equal(t, other.state.Header.Title, card.BoardTitle)
	}

	// and the owner of A, who shares a board with Sam, gets only their own card
	owned, err := f.repos.Card.ListWrittenByUser(t.Context(), f.workspace.Owner.UserID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Owner in A"}, writtenBodies(owned))
}

func TestCardListWrittenByUser_LeavesOutACardWhoseAuthorWasSevered(t *testing.T) {
	// given Sam's card on a board whose authorship was severed at End
	f := newBoardFixture(t)
	sam := f.teamMember(t, "Sam")
	f.mentionCard(t, f.seat(t, sam), "Written anonymously")

	_, err := f.repos.Card.SeverAuthors(t.Context(), f.boardID())
	require.NoError(t, err)

	// when
	cards, err := f.repos.Card.ListWrittenByUser(t.Context(), sam.ID)

	// then nothing links the card back to Sam
	require.NoError(t, err)
	assert.Empty(t, cards)
}

func TestCardCommentListWrittenByUser_ListsTheUsersOwnCommentsAndNoOneElses(t *testing.T) {
	// given Sam and the owner both comment on the owner's card
	f := newBoardFixture(t)
	sam := f.teamMember(t, "Sam")
	samSeat := f.seat(t, sam)
	card := f.mentionCard(t, f.facilitator(), "Deploys were smooth")
	f.mentionComment(t, card.ID, samSeat, "Agreed")
	f.mentionComment(t, card.ID, f.facilitator(), "Thanks")

	// when
	comments, err := f.repos.CardComment.ListWrittenByUser(t.Context(), sam.ID)

	// then the card's own text is not part of Sam's comment
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, "Agreed", comments[0].Body)
	assert.Equal(t, f.workspace.Organisation.Name, comments[0].OrgName)
	assert.Equal(t, f.state.Header.Title, comments[0].BoardTitle)
	assert.Nil(t, comments[0].UpdatedAt)
}

func TestCardCommentListWrittenByUser_LeavesOutACommentWhoseAuthorWasSevered(t *testing.T) {
	// given
	f := newBoardFixture(t)
	sam := f.teamMember(t, "Sam")
	card := f.mentionCard(t, f.facilitator(), "Deploys were smooth")
	f.mentionComment(t, card.ID, f.seat(t, sam), "Agreed")

	_, err := f.repos.CardComment.SeverAuthors(t.Context(), f.boardID())
	require.NoError(t, err)

	// when
	comments, err := f.repos.CardComment.ListWrittenByUser(t.Context(), sam.ID)

	// then
	require.NoError(t, err)
	assert.Empty(t, comments)
}

func TestActionItemListForUser_ListsWhatTheUserRaisedOrIsAssignedAndNothingElse(t *testing.T) {
	// given the owner raises one item assigned to Sam and one assigned to nobody, and organisation B has an item of its own
	f := newBoardFixture(t)
	sam := f.teamMember(t, "Sam")
	f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Thank @Hana", AssigneeUserID: new(sam.ID)}, nil)
	f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Book the next retro"}, nil)
	other := f.otherOrganisation(t, sam)
	header, err := f.repos.Board.GetHeader(t.Context(), other.state.Header.ID)
	require.NoError(t, err)
	_, err = f.repos.ActionItem.AddActionItem(t.Context(), spec.ActionItemCreate{
		OrgID: other.workspace.Organisation.ID, BoardID: other.state.Header.ID, Version: header.Version, OpID: uuid.New(),
		Title:     "Someone else's work",
		ItemID:    uuid.New(),
		CreatedBy: new(other.workspace.Owner.UserID),
	})
	require.NoError(t, err)

	// when
	assigned, err := f.repos.ActionItem.ListForUser(t.Context(), sam.ID)
	require.NoError(t, err)
	raised, err := f.repos.ActionItem.ListForUser(t.Context(), f.workspace.Owner.UserID)
	require.NoError(t, err)

	// then
	require.Len(t, assigned, 1)
	assert.Equal(t, model.UserActionItem{
		OrgName:        f.workspace.Organisation.Name,
		TeamName:       f.workspace.Team.Name,
		BoardTitle:     f.state.Header.Title,
		Title:          "Thank @Hana",
		Status:         model.ActionItemOpen,
		AssignedToUser: true,
	}, assigned[0])

	require.Len(t, raised, 2)
	for _, item := range raised {
		assert.True(t, item.RaisedByUser)
		assert.False(t, item.AssignedToUser)
		assert.Equal(t, f.workspace.Organisation.Name, item.OrgName)
	}
	assert.ElementsMatch(t, []string{"Thank @Hana", "Book the next retro"}, []string{raised[0].Title, raised[1].Title})
}
