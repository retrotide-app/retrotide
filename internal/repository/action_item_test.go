package repository

import (
	"context"
	"database/sql"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestActionItem_ReviewBumpsBothBoardsAndRecordsTheOpOnlyOnTheReviewingOne(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	items := dao.NewMockActionItemDAO(t)
	repo := NewActionItemRepo(nil, items, boards)
	write, inTx := boardWriteFixture()
	itemID := uuid.New()
	itemBoard := spec.BoardWrite{OrgID: write.OrgID, BoardID: uuid.New(), Version: 12}
	done := model.ActionItemDone
	var order []string
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).RunAndReturn(func(context.Context, spec.BoardWrite, ...*sql.Tx) (int64, error) {
		order = append(order, "bump reviewing board")
		return 42, nil
	}).Once()
	boards.EXPECT().RecordOp(mock.Anything, spec.NewBoardOp{BoardID: write.BoardID, OpID: write.OpID, Version: 42}, inTx).Return(nil).Once()
	boards.EXPECT().BumpVersion(mock.Anything, itemBoard, inTx).RunAndReturn(func(context.Context, spec.BoardWrite, ...*sql.Tx) (int64, error) {
		order = append(order, "bump item board")
		return 13, nil
	}).Once()
	items.EXPECT().SetItemStatus(mock.Anything, spec.ActionItemStatusChange{OrgID: write.OrgID, ItemID: itemID, Status: done}, inTx).RunAndReturn(func(context.Context, spec.ActionItemStatusChange, ...*sql.Tx) error {
		order = append(order, "status")
		return nil
	}).Once()
	items.EXPECT().RecordReview(mock.Anything, spec.NewActionItemReview{OrgID: write.OrgID, ItemID: itemID, BoardID: write.BoardID, Outcome: model.ReviewDone}, inTx).RunAndReturn(func(context.Context, spec.NewActionItemReview, ...*sql.Tx) error {
		order = append(order, "review")
		return nil
	}).Once()
	items.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: write.OrgID, ItemID: itemID}, inTx).Return(&model.ActionItem{ID: itemID, Status: done}, nil).Once()

	// when
	reviewed, err := repo.ReviewActionItem(t.Context(), spec.ActionItemReviewWrite{BoardWrite: write, ItemID: itemID, Outcome: model.ReviewDone, Status: &done, ItemBoard: &itemBoard}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"bump reviewing board", "bump item board", "status", "review"}, order)
	assert.Equal(t, int64(42), reviewed.Version)
	assert.Equal(t, new(int64(13)), reviewed.Value.ItemBoardVersion)
	assert.Equal(t, new(model.ReviewDone), reviewed.Value.Item.Review)
}

func TestActionItem_ACarriedReviewTouchesNeitherTheItemNorItsBoard(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	items := dao.NewMockActionItemDAO(t)
	repo := NewActionItemRepo(nil, items, boards)
	write, inTx := boardWriteFixture()
	itemID := uuid.New()
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	items.EXPECT().RecordReview(mock.Anything, spec.NewActionItemReview{OrgID: write.OrgID, ItemID: itemID, BoardID: write.BoardID, Outcome: model.ReviewCarried}, inTx).Return(nil).Once()
	items.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: write.OrgID, ItemID: itemID}, inTx).Return(&model.ActionItem{ID: itemID, Status: model.ActionItemOpen}, nil).Once()

	// when
	reviewed, err := repo.ReviewActionItem(t.Context(), spec.ActionItemReviewWrite{BoardWrite: write, ItemID: itemID, Outcome: model.ReviewCarried}, inTx...)

	// then the strict mocks refuse any status write or second bump
	require.NoError(t, err)
	assert.Nil(t, reviewed.Value.ItemBoardVersion)
}

func TestActionItem_EditMentionsOnlyThePeopleAddedAndForgetsThoseLeftOut(t *testing.T) {
	// given an item that mentions Ana and Ben, edited to mention Ben and Cal
	boards := dao.NewMockBoardDAO(t)
	items := dao.NewMockActionItemDAO(t)
	repo := NewActionItemRepo(nil, items, boards)
	write, inTx := boardWriteFixture()
	itemID := uuid.New()
	ana, ben, cal := uuid.New(), uuid.New(), uuid.New()
	current := &model.ActionItem{ID: itemID, BoardID: new(write.BoardID), Mentions: []model.ActionItemPerson{{UserID: ana}, {UserID: ben}}}
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	key := spec.ActionItemKey{OrgID: write.OrgID, ItemID: itemID}
	items.EXPECT().GetByID(mock.Anything, key, inTx).Return(current, nil).Twice()
	items.EXPECT().EditItem(mock.Anything, spec.ActionItemEdit{OrgID: write.OrgID, BoardID: write.BoardID, ItemID: itemID, Title: "Fix it"}, inTx).Return(nil).Once()
	items.EXPECT().RemoveMention(mock.Anything, spec.ActionItemMentionKey{OrgID: write.OrgID, ItemID: itemID, UserID: ana}, inTx).Return(nil).Once()
	items.EXPECT().AddMention(mock.Anything, spec.ActionItemMentionKey{OrgID: write.OrgID, ItemID: itemID, UserID: ben}, inTx).Return(nil).Once()
	items.EXPECT().AddMention(mock.Anything, spec.ActionItemMentionKey{OrgID: write.OrgID, ItemID: itemID, UserID: cal}, inTx).Return(nil).Once()

	// when
	_, err := repo.EditActionItem(t.Context(), spec.ActionItemEditWrite{BoardWrite: write, Title: "Fix it", Mentions: []uuid.UUID{ben, cal}, ItemID: itemID}, inTx...)

	// then Ben's mention keeps its place, and only Ana's is removed
	require.NoError(t, err)
}

func TestActionItem_AWriteToAnItemOfAnotherBoardIsNotFound(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	items := dao.NewMockActionItemDAO(t)
	repo := NewActionItemRepo(nil, items, boards)
	write, inTx := boardWriteFixture()
	itemID := uuid.New()
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	items.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: write.OrgID, ItemID: itemID}, inTx).Return(&model.ActionItem{ID: itemID, BoardID: new(uuid.New())}, nil).Once()

	// when
	_, err := repo.SetActionItemStatus(t.Context(), spec.ActionItemStatusWrite{BoardWrite: write, ItemID: itemID, Status: model.ActionItemDone}, inTx...)

	// then the strict mock refuses the status write
	require.ErrorIs(t, err, dao.ErrNotFound)
}
