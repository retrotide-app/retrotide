package dao_test

import (
	"strings"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	actionItemInsert = `INSERT INTO action_items (id, org_id, team_id, board_id, title, status, completed_at) VALUES ($1, $2, $3::uuid, $4::uuid, $5, $6, $7::timestamptz)`
	reviewInsert     = `INSERT INTO action_item_reviews (org_id, action_item_id, board_id, outcome) VALUES ($1, $2, $3, $4)`
)

func TestActionItem_SchemaConstraintsRefuseBadRows(t *testing.T) {
	tests := []struct {
		name       string
		statement  string
		args       func(f boardFixture) []any
		constraint string
	}{
		{
			name:       "an empty title",
			statement:  actionItemInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.orgID(), nil, nil, "", "open", nil} },
			constraint: "action_items_title_length",
		},
		{
			name:      "a title over 500 characters",
			statement: actionItemInsert,
			args: func(f boardFixture) []any {
				return []any{uuid.New(), f.orgID(), nil, nil, strings.Repeat("a", 501), "open", nil}
			},
			constraint: "action_items_title_length",
		},
		{
			name:       "an unknown status",
			statement:  actionItemInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.orgID(), nil, nil, "Fix it", "finished", nil} },
			constraint: "action_items_status_check",
		},
		{
			name:       "a done item with no completion time",
			statement:  actionItemInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.orgID(), nil, nil, "Fix it", "done", nil} },
			constraint: "action_items_done_has_completed_at",
		},
		{
			name:      "an open item with a completion time",
			statement: actionItemInsert,
			args: func(f boardFixture) []any {
				return []any{uuid.New(), f.orgID(), nil, nil, "Fix it", "open", time.Now()}
			},
			constraint: "action_items_done_has_completed_at",
		},
		{
			name:      "an item on a team its organisation does not have",
			statement: actionItemInsert,
			args: func(f boardFixture) []any {
				return []any{uuid.New(), f.orgID(), uuid.New(), nil, "Fix it", "open", nil}
			},
			constraint: "action_items_team_fkey",
		},
		{
			name:      "an item on a board its organisation does not have",
			statement: actionItemInsert,
			args: func(f boardFixture) []any {
				return []any{uuid.New(), f.orgID(), nil, uuid.New(), "Fix it", "open", nil}
			},
			constraint: "action_items_board_fkey",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newBoardFixture(t)

			// when
			_, err := f.repos.DB().ExecContext(t.Context(), tc.statement, tc.args(f)...)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}

func TestActionItem_AnUnknownReviewOutcomeIsRefused(t *testing.T) {
	// given
	f := newBoardFixture(t)
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Fix it"}, nil)

	// when
	_, err := f.repos.DB().ExecContext(t.Context(), reviewInsert, f.orgID(), item.ID, f.boardID(), "forgotten")

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action_item_reviews_outcome_check")
}

func (f boardFixture) teamMember(t *testing.T, name string) *model.User {
	t.Helper()

	user := daotest.CreateUser(t, f.repos, daotest.WithDisplayName(name))
	daotest.AddOrgMember(t, f.repos, f.orgID(), user.ID, model.OrgRoleMember)
	daotest.AddTeamMember(t, f.repos, &f.workspace.Team, user.ID, model.TeamRoleMember)

	return user
}

func (f boardFixture) addItem(t *testing.T, boardID uuid.UUID, fields spec.ActionItemFields, source *uuid.UUID) model.ActionItem {
	t.Helper()

	header, err := f.repos.Board.GetHeader(t.Context(), boardID)
	require.NoError(t, err)

	added, err := f.repos.ActionItem.AddActionItem(t.Context(), spec.ActionItemCreate{
		OrgID: f.orgID(), BoardID: boardID, Version: header.Version, OpID: uuid.New(),
		ActionItemFields: fields,
		ItemID:           uuid.New(),
		TeamID:           header.TeamID,
		SourceCardID:     source,
		CreatedBy:        new(f.workspace.Owner.UserID),
	})
	require.NoError(t, err)

	return added.Value
}

func (f boardFixture) laterBoard(t *testing.T) *model.BoardState {
	t.Helper()

	return daotest.CreateBoard(t, f.repos, f.workspace.Team, f.workspace.Owner.UserID)
}

func itemIDs(items []model.ActionItem) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}

func previousIDs(actions []model.PreviousAction) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(actions))
	for _, action := range actions {
		ids = append(ids, action.ID)
	}

	return ids
}

func TestActionItem_AddStoresTheItemWithItsAssigneeAndMentionsAndBumpsTheBoard(t *testing.T) {
	// given
	f := newBoardFixture(t)
	alice := f.teamMember(t, "Alice")
	ben := f.teamMember(t, "Ben")
	due := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	before := f.version(t)

	// when
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Fix the pipeline with @Ben", AssigneeUserID: new(alice.ID), DueOn: &due, Mentions: []uuid.UUID{ben.ID}}, nil)

	// then
	assert.Equal(t, before+1, f.version(t))
	assert.Equal(t, "Fix the pipeline with @Ben", item.Title)
	assert.Equal(t, new(alice.ID), item.AssigneeUserID)
	assert.Equal(t, "Alice", item.AssigneeName)
	require.NotNil(t, item.DueOn)
	assert.Equal(t, "2026-10-20", item.DueOn.Format(model.DueDateLayout))
	assert.Equal(t, model.ActionItemOpen, item.Status)
	assert.Nil(t, item.CompletedAt)
	assert.Equal(t, []model.ActionItemPerson{{UserID: ben.ID, DisplayName: "Ben"}}, item.Mentions)
	assert.Equal(t, new(f.boardID()), item.BoardID)
	assert.Equal(t, new(f.workspace.Team.ID), item.TeamID)

	listed, err := f.repos.ActionItem.ListForBoard(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: f.boardID()})
	require.NoError(t, err)
	assert.Equal(t, []model.ActionItem{item}, listed)
}

func TestActionItem_ACardCarriesManyItemsAndAnIDIsTakenOnce(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "From the card"}, new(card.ID))

	// when
	second := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Again from the card"}, new(card.ID))
	before := f.version(t)
	_, takenErr := f.repos.ActionItem.AddActionItem(t.Context(), spec.ActionItemCreate{BoardWrite: f.write(t), Title: "Same ID", ItemID: item.ID, TeamID: new(f.workspace.Team.ID), SourceCardID: new(card.ID)})

	// then both items name the card, and the taken ID burns no version
	assert.Equal(t, new(card.ID), item.SourceCardID)
	assert.Equal(t, new(card.ID), second.SourceCardID)
	require.ErrorIs(t, takenErr, dao.ErrConflict)
	assert.Equal(t, before, f.version(t))
}

func TestActionItem_AnAssigneeOrAMentionMustBeAMemberOfTheOrganisation(t *testing.T) {
	// given someone with an account but no place in the organisation
	f := newBoardFixture(t)
	stranger := daotest.CreateUser(t, f.repos)
	before := f.version(t)

	// when
	_, assigneeErr := f.repos.ActionItem.AddActionItem(t.Context(), spec.ActionItemCreate{BoardWrite: f.write(t), Title: "Fix it", AssigneeUserID: new(stranger.ID), ItemID: uuid.New()})
	_, mentionErr := f.repos.ActionItem.AddActionItem(t.Context(), spec.ActionItemCreate{BoardWrite: f.write(t), Title: "Fix it", Mentions: []uuid.UUID{stranger.ID}, ItemID: uuid.New()})

	// then the composite foreign keys refuse both, and the version bump rolls back with them
	require.ErrorIs(t, assigneeErr, dao.ErrNotFound)
	require.ErrorIs(t, mentionErr, dao.ErrNotFound)
	assert.Equal(t, before, f.version(t))
}

func TestActionItem_EditKeepsAnUnchangedMentionAndStatusMovesCompletedAt(t *testing.T) {
	// given
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	ben := f.teamMember(t, "Ben")
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "First", Mentions: []uuid.UUID{ana.ID, ben.ID}}, nil)

	// when
	edited, editErr := f.repos.ActionItem.EditActionItem(t.Context(), spec.ActionItemEditWrite{BoardWrite: f.write(t), Title: "Second", Mentions: []uuid.UUID{ben.ID}, ItemID: item.ID})
	done, doneErr := f.repos.ActionItem.SetActionItemStatus(t.Context(), spec.ActionItemStatusWrite{BoardWrite: f.write(t), ItemID: item.ID, Status: model.ActionItemDone})
	reopened, reopenErr := f.repos.ActionItem.SetActionItemStatus(t.Context(), spec.ActionItemStatusWrite{BoardWrite: f.write(t), ItemID: item.ID, Status: model.ActionItemOpen})

	// then
	require.NoError(t, editErr)
	assert.Equal(t, "Second", edited.Value.Title)
	assert.Equal(t, []model.ActionItemPerson{{UserID: ben.ID, DisplayName: "Ben"}}, edited.Value.Mentions)
	require.NoError(t, doneErr)
	assert.Equal(t, model.ActionItemDone, done.Value.Status)
	assert.NotNil(t, done.Value.CompletedAt)
	require.NoError(t, reopenErr)
	assert.Equal(t, model.ActionItemOpen, reopened.Value.Status)
	assert.Nil(t, reopened.Value.CompletedAt)
}

func TestActionItem_DeleteRemovesItAndAnItemOfAnotherBoardIsNotFound(t *testing.T) {
	// given
	f := newBoardFixture(t)
	other := f.laterBoard(t)
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Fix it"}, nil)
	before := f.version(t)

	// when
	_, wrongBoardErr := f.repos.ActionItem.DeleteActionItem(t.Context(), spec.ActionItemRemoval{OrgID: f.orgID(), BoardID: other.Header.ID, Version: other.Header.Version, OpID: uuid.New(), ItemID: item.ID})
	removed, removeErr := f.repos.ActionItem.DeleteActionItem(t.Context(), spec.ActionItemRemoval{BoardWrite: f.write(t), ItemID: item.ID})
	_, goneErr := f.repos.ActionItem.GetByID(t.Context(), spec.ActionItemKey{OrgID: f.orgID(), ItemID: item.ID})

	// then
	require.ErrorIs(t, wrongBoardErr, dao.ErrNotFound)
	require.NoError(t, removeErr)
	assert.Equal(t, item.ID, removed.Value.ID)
	assert.Equal(t, before+1, f.version(t))
	require.ErrorIs(t, goneErr, dao.ErrNotFound)
}

func TestActionItem_DeletingTheSourceCardKeepsTheItem(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "From the card"}, new(card.ID))

	// when
	_, err := f.repos.Card.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: f.write(t), CardID: card.ID})

	// then
	require.NoError(t, err)
	kept, err := f.repos.ActionItem.GetByID(t.Context(), spec.ActionItemKey{OrgID: f.orgID(), ItemID: item.ID})
	require.NoError(t, err)
	assert.Nil(t, kept.SourceCardID)
	assert.Equal(t, new(f.boardID()), kept.BoardID)
}

func TestActionItem_RemovingAMemberClearsTheirAssignmentAndMentions(t *testing.T) {
	// given
	f := newBoardFixture(t)
	alice := f.teamMember(t, "Alice")
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Fix it", AssigneeUserID: new(alice.ID), Mentions: []uuid.UUID{alice.ID}}, nil)

	// when
	_, err := f.repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: f.orgID(), UserID: alice.ID})

	// then
	require.NoError(t, err)
	kept, err := f.repos.ActionItem.GetByID(t.Context(), spec.ActionItemKey{OrgID: f.orgID(), ItemID: item.ID})
	require.NoError(t, err)
	assert.Nil(t, kept.AssigneeUserID)
	assert.Empty(t, kept.AssigneeName)
	assert.Empty(t, kept.Mentions)
}

func TestActionItem_PreviousActionsAreTheTeamsOpenItemsFromEarlierBoardsAndTheirReview(t *testing.T) {
	// given retro 1's open and done actions, then retro 2 with its own action, then retro 3
	f := newBoardFixture(t)
	open := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Still open"}, nil)
	done := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Done already"}, nil)
	_, err := f.repos.ActionItem.SetActionItemStatus(t.Context(), spec.ActionItemStatusWrite{BoardWrite: f.write(t), ItemID: done.ID, Status: model.ActionItemDone})
	require.NoError(t, err)
	second := f.laterBoard(t)
	own := f.addItem(t, second.Header.ID, spec.ActionItemFields{Title: "Raised on retro 2"}, nil)
	third := f.laterBoard(t)
	query := spec.PreviousActionsQuery{OrgID: f.orgID(), TeamID: f.workspace.Team.ID, BoardID: second.Header.ID, Before: second.Header.CreatedAt}

	// when
	previous, err := f.repos.ActionItem.ListPrevious(t.Context(), query)

	// then retro 2 reviews retro 1's open action only, never its own, and never a later board's
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{open.ID}, previousIDs(previous))
	assert.Nil(t, previous[0].Review)

	thirdPrevious, err := f.repos.ActionItem.ListPrevious(t.Context(), spec.PreviousActionsQuery{OrgID: f.orgID(), TeamID: f.workspace.Team.ID, BoardID: third.Header.ID, Before: third.Header.CreatedAt})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{open.ID, own.ID}, previousIDs(thirdPrevious))
}

func TestActionItem_AReviewMarksTheItemAndStaysVisibleOnTheReviewingBoard(t *testing.T) {
	// given retro 1's open action, reviewed on retro 2
	f := newBoardFixture(t)
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Still open"}, nil)
	second := f.laterBoard(t)
	firstVersion := f.version(t)
	done := model.ActionItemDone

	// when
	reviewed, err := f.repos.ActionItem.ReviewActionItem(t.Context(), spec.ActionItemReviewWrite{
		OrgID: f.orgID(), BoardID: second.Header.ID, Version: second.Header.Version, OpID: uuid.New(),
		ItemID:    item.ID,
		Outcome:   model.ReviewDone,
		Status:    &done,
		ItemBoard: &spec.BoardWrite{OrgID: f.orgID(), BoardID: f.boardID(), Version: firstVersion},
	})

	// then both boards move on, and the done item still shows on retro 2 with its review
	require.NoError(t, err)
	assert.Equal(t, second.Header.Version+1, reviewed.Version)
	assert.Equal(t, new(firstVersion+1), reviewed.Value.ItemBoardVersion)
	assert.Equal(t, model.ActionItemDone, reviewed.Value.Item.Status)
	assert.Equal(t, firstVersion+1, f.version(t))

	previous, err := f.repos.ActionItem.ListPrevious(t.Context(), spec.PreviousActionsQuery{OrgID: f.orgID(), TeamID: f.workspace.Team.ID, BoardID: second.Header.ID, Before: second.Header.CreatedAt})
	require.NoError(t, err)
	require.Len(t, previous, 1)
	assert.Equal(t, new(model.ReviewDone), previous[0].Review)
	assert.Equal(t, model.ActionItemDone, previous[0].Status)
}

func TestActionItem_ThePageFiltersAndCountsWhatTheViewerMaySee(t *testing.T) {
	// given an item on the team board, assigned to Alice, one done, and one on an organisation board
	f := newBoardFixture(t)
	alice := f.teamMember(t, "Alice")
	mine := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Alice's", AssigneeUserID: new(alice.ID)}, nil)
	finished := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Finished"}, nil)
	_, err := f.repos.ActionItem.SetActionItemStatus(t.Context(), spec.ActionItemStatusWrite{BoardWrite: f.write(t), ItemID: finished.ID, Status: model.ActionItemDone})
	require.NoError(t, err)
	orgBoard := daotest.CreateOrganisationBoard(t, f.repos, f.orgID(), f.workspace.Owner.UserID)
	orgItem := f.addItem(t, orgBoard.Header.ID, spec.ActionItemFields{Title: "Everyone's"}, nil)
	outsider := daotest.CreateUser(t, f.repos)
	daotest.AddOrgMember(t, f.repos, f.orgID(), outsider.ID, model.OrgRoleMember)
	open := model.ActionItemOpen

	tests := []struct {
		name   string
		filter spec.ActionItemFilter
		want   []uuid.UUID
		counts model.ActionItemCounts
	}{
		{name: "an owner sees every team", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: f.workspace.Owner.UserID, EveryTeam: true, Limit: 50}, want: []uuid.UUID{orgItem.ID, finished.ID, mine.ID}, counts: model.ActionItemCounts{Open: 2, Done: 1}},
		{name: "a member off the team sees only organisation boards", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: outsider.ID, Limit: 50}, want: []uuid.UUID{orgItem.ID}, counts: model.ActionItemCounts{Open: 1}},
		{name: "a member on the team sees it", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: alice.ID, Limit: 50}, want: []uuid.UUID{orgItem.ID, finished.ID, mine.ID}, counts: model.ActionItemCounts{Open: 2, Done: 1}},
		{name: "open items only, counted across every status", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: alice.ID, Status: &open, Limit: 50}, want: []uuid.UUID{orgItem.ID, mine.ID}, counts: model.ActionItemCounts{Open: 2, Done: 1}},
		{name: "assigned to Alice", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: alice.ID, AssigneeUserID: new(alice.ID), Limit: 50}, want: []uuid.UUID{mine.ID}, counts: model.ActionItemCounts{Open: 1}},
		{name: "assigned to nobody", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: alice.ID, Unassigned: true, Limit: 50}, want: []uuid.UUID{orgItem.ID, finished.ID}, counts: model.ActionItemCounts{Open: 1, Done: 1}},
		{name: "one team", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: alice.ID, TeamID: new(f.workspace.Team.ID), Limit: 50}, want: []uuid.UUID{finished.ID, mine.ID}, counts: model.ActionItemCounts{Open: 1, Done: 1}},
		{name: "organisation boards", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: alice.ID, OrganisationBoards: true, Limit: 50}, want: []uuid.UUID{orgItem.ID}, counts: model.ActionItemCounts{Open: 1}},
		{name: "a page", filter: spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: alice.ID, Limit: 1, Offset: 1}, want: []uuid.UUID{finished.ID}, counts: model.ActionItemCounts{Open: 2, Done: 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			page, err := f.repos.ActionItem.ListPage(t.Context(), tc.filter)

			// then newest first
			require.NoError(t, err)
			ids := make([]uuid.UUID, 0, len(page.Items))
			for _, listing := range page.Items {
				ids = append(ids, listing.ID)
			}
			assert.Equal(t, tc.want, ids)
			assert.Equal(t, tc.counts, page.Counts)
		})
	}
}

func TestActionItem_ThePageNamesTheBoardAndTheTeamAndLeavesOutDeletedBoards(t *testing.T) {
	// given
	f := newBoardFixture(t)
	item := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Fix it"}, nil)
	gone := f.laterBoard(t)
	f.addItem(t, gone.Header.ID, spec.ActionItemFields{Title: "On a deleted board"}, nil)
	f.deleteBoard(t, gone.Header.ID)

	// when
	page, err := f.repos.ActionItem.ListPage(t.Context(), spec.ActionItemFilter{OrgID: f.orgID(), ViewerID: f.workspace.Owner.UserID, EveryTeam: true, Limit: 50})

	// then
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, item.ID, page.Items[0].ID)
	assert.Equal(t, f.state.Header.Title, page.Items[0].BoardTitle)
	assert.Equal(t, model.AnonymityNamed, page.Items[0].BoardAnonymity)
	assert.Equal(t, f.workspace.Team.Name, page.Items[0].TeamName)
	assert.Equal(t, model.ActionItemCounts{Open: 1}, page.Counts)
}

func TestActionItem_MentionsForAUserAreNewestFirstAndLeaveOutDeletedBoardsAndOrganisations(t *testing.T) {
	// given Alice mentioned on two items, one of them on a board since deleted
	f := newBoardFixture(t)
	alice := f.teamMember(t, "Alice")
	first := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "First @Alice", Mentions: []uuid.UUID{alice.ID}}, nil)
	second := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Second @Alice", Mentions: []uuid.UUID{alice.ID}}, nil)
	gone := f.laterBoard(t)
	f.addItem(t, gone.Header.ID, spec.ActionItemFields{Title: "Gone @Alice", Mentions: []uuid.UUID{alice.ID}}, nil)
	f.deleteBoard(t, gone.Header.ID)
	f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Not Alice"}, nil)

	// when
	mentions, err := f.repos.ActionItem.ListMentionsForUser(t.Context(), spec.UserMentionsQuery{UserID: alice.ID, Limit: 10})

	// then
	require.NoError(t, err)
	require.Len(t, mentions, 2)
	assert.Equal(t, second.ID, mentions[0].ActionItemID)
	assert.Equal(t, first.ID, mentions[1].ActionItemID)
	assert.Equal(t, f.workspace.Organisation.Slug, mentions[0].OrgSlug)
	assert.Equal(t, new(f.boardID()), mentions[0].BoardID)
	assert.Equal(t, f.state.Header.Title, mentions[0].BoardTitle)
	assert.Equal(t, new(f.workspace.Team.ID), mentions[0].TeamID)

	_, err = f.repos.Organisation.SoftDelete(t.Context(), f.orgID())
	require.NoError(t, err)
	afterDelete, err := f.repos.ActionItem.ListMentionsForUser(t.Context(), spec.UserMentionsQuery{UserID: alice.ID, Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, afterDelete)
}

func TestActionItem_CrossTenantItemsAreOnlyReachedThroughTheirOrganisation(t *testing.T) {
	// given an action item on board A of organisation A
	bt := newBoardTenants(t)
	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	added, err := bt.repos.ActionItem.AddActionItem(t.Context(), spec.ActionItemCreate{
		OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, Version: header.Version, OpID: uuid.New(),
		Title:  "SECRET-TEXT-9f2c",
		ItemID: uuid.New(),
		TeamID: new(bt.a.Team.ID),
	})
	require.NoError(t, err)
	item := added.Value
	throughB := spec.BoardWrite{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, Version: bt.boardB.Header.Version, OpID: uuid.New()}
	done := model.ActionItemDone

	// when organisation B reaches for it every way it can
	_, getErr := bt.repos.ActionItem.GetByID(t.Context(), spec.ActionItemKey{OrgID: bt.b.Organisation.ID, ItemID: item.ID})
	listedB, listErr := bt.repos.ActionItem.ListForBoard(t.Context(), spec.BoardKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID})
	_, editErr := bt.repos.ActionItem.EditActionItem(t.Context(), spec.ActionItemEditWrite{BoardWrite: throughB, Title: "Planted", ItemID: item.ID})
	_, statusErr := bt.repos.ActionItem.SetActionItemStatus(t.Context(), spec.ActionItemStatusWrite{BoardWrite: throughB, ItemID: item.ID, Status: done})
	_, deleteErr := bt.repos.ActionItem.DeleteActionItem(t.Context(), spec.ActionItemRemoval{BoardWrite: throughB, ItemID: item.ID})
	_, reviewErr := bt.repos.ActionItem.ReviewActionItem(t.Context(), spec.ActionItemReviewWrite{BoardWrite: throughB, ItemID: item.ID, Outcome: model.ReviewDone, Status: &done})
	previousB, previousErr := bt.repos.ActionItem.ListPrevious(t.Context(), spec.PreviousActionsQuery{OrgID: bt.b.Organisation.ID, TeamID: bt.a.Team.ID, BoardID: bt.boardB.Header.ID, Before: time.Now().Add(time.Hour)})
	pageB, pageErr := bt.repos.ActionItem.ListPage(t.Context(), spec.ActionItemFilter{OrgID: bt.b.Organisation.ID, ViewerID: bt.b.Owner.UserID, EveryTeam: true, Limit: 50})
	_, plantErr := bt.repos.ActionItem.AddActionItem(t.Context(), spec.ActionItemCreate{BoardWrite: throughB, Title: "Planted", ItemID: uuid.New(), TeamID: new(bt.a.Team.ID)})

	// then
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	assert.Empty(t, listedB)
	require.ErrorIs(t, editErr, dao.ErrNotFound)
	require.ErrorIs(t, statusErr, dao.ErrNotFound)
	require.ErrorIs(t, deleteErr, dao.ErrNotFound)
	require.ErrorIs(t, reviewErr, dao.ErrNotFound)
	require.NoError(t, previousErr)
	assert.Empty(t, previousB)
	require.NoError(t, pageErr)
	assert.Empty(t, pageB.Items)
	assert.Equal(t, model.ActionItemCounts{}, pageB.Counts)
	require.ErrorIs(t, plantErr, dao.ErrNotFound, "an item of organisation B can never sit in a team of organisation A")

	unchanged, err := bt.repos.ActionItem.GetByID(t.Context(), spec.ActionItemKey{OrgID: bt.a.Organisation.ID, ItemID: item.ID})
	require.NoError(t, err)
	assert.Equal(t, item, *unchanged)
	bt.boardAUnchangedSinceItsAction(t, header.Version+1)
}

func (bt boardTenants) boardAUnchangedSinceItsAction(t *testing.T, version int64) {
	t.Helper()

	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, version, header.Version)
}

func TestActionItem_ListsForABoardAreItsOwnOldestFirst(t *testing.T) {
	// given
	f := newBoardFixture(t)
	first := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "First"}, nil)
	other := f.laterBoard(t)
	f.addItem(t, other.Header.ID, spec.ActionItemFields{Title: "Elsewhere"}, nil)
	second := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "Second"}, nil)

	// when
	listed, err := f.repos.ActionItem.ListForBoard(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: f.boardID()})

	// then
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first.ID, second.ID}, itemIDs(listed))
}

func TestActionItem_TheSnapshotCarriesTheBoardsItemsAndItsPreviousActions(t *testing.T) {
	// given
	f := newBoardFixture(t)
	earlier := f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "From retro 1"}, nil)
	second := f.laterBoard(t)
	own := f.addItem(t, second.Header.ID, spec.ActionItemFields{Title: "From retro 2"}, nil)

	// when
	snapshot, err := f.repos.Board.GetSnapshot(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: second.Header.ID})

	// then
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{own.ID}, itemIDs(snapshot.ActionItems))
	assert.Equal(t, []uuid.UUID{earlier.ID}, previousIDs(snapshot.PreviousActions))
}

func TestActionItem_AnOrganisationBoardsSnapshotHasNoPreviousActions(t *testing.T) {
	// given
	f := newBoardFixture(t)
	f.addItem(t, f.boardID(), spec.ActionItemFields{Title: "A team action"}, nil)
	orgBoard := daotest.CreateOrganisationBoard(t, f.repos, f.orgID(), f.workspace.Owner.UserID)

	// when
	snapshot, err := f.repos.Board.GetSnapshot(t.Context(), spec.BoardKey{OrgID: f.orgID(), BoardID: orgBoard.Header.ID})

	// then
	require.NoError(t, err)
	assert.Empty(t, snapshot.ActionItems)
	assert.NotNil(t, snapshot.PreviousActions)
	assert.Empty(t, snapshot.PreviousActions)
}
