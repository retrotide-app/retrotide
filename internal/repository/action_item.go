package repository

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	ActionItemRepository interface {
		dao.ActionItemDAO

		AddActionItem(ctx context.Context, s spec.ActionItemCreate, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error)
		EditActionItem(ctx context.Context, s spec.ActionItemEditWrite, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error)
		SetActionItemStatus(ctx context.Context, s spec.ActionItemStatusWrite, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error)
		DeleteActionItem(ctx context.Context, s spec.ActionItemRemoval, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error)
		ReviewActionItem(ctx context.Context, s spec.ActionItemReviewWrite, tx ...*sql.Tx) (*model.Versioned[model.ActionItemReviewed], error)
		ListPage(ctx context.Context, s spec.ActionItemFilter, tx ...*sql.Tx) (*model.ActionItemPage, error)
	}

	actionItemRepository struct {
		dao.ActionItemDAO

		db     *sql.DB
		boards dao.BoardDAO
	}
)

func NewActionItemRepo(database *sql.DB, itemDAO dao.ActionItemDAO, boards dao.BoardDAO) ActionItemRepository {
	return &actionItemRepository{ActionItemDAO: itemDAO, db: database, boards: boards}
}

func (r *actionItemRepository) AddActionItem(ctx context.Context, s spec.ActionItemCreate, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error) {
	add := func(tx *sql.Tx) (*model.ActionItem, error) {
		err := r.ActionItemDAO.CreateItem(ctx, spec.NewActionItem{
			ID:             s.ItemID,
			OrgID:          s.OrgID,
			TeamID:         s.TeamID,
			BoardID:        new(s.BoardID),
			SourceCardID:   s.SourceCardID,
			Title:          s.Title,
			AssigneeUserID: s.AssigneeUserID,
			DueOn:          s.DueOn,
			CreatedBy:      s.CreatedBy,
		}, tx)
		if err != nil {
			return nil, err
		}

		for _, userID := range s.Mentions {
			if err := r.ActionItemDAO.AddMention(ctx, spec.ActionItemMentionKey{OrgID: s.OrgID, ItemID: s.ItemID, UserID: userID}, tx); err != nil {
				return nil, err
			}
		}

		return r.ActionItemDAO.GetByID(ctx, spec.ActionItemKey{OrgID: s.OrgID, ItemID: s.ItemID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, add)
}

func (r *actionItemRepository) EditActionItem(ctx context.Context, s spec.ActionItemEditWrite, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error) {
	edit := func(tx *sql.Tx) (*model.ActionItem, error) {
		current, err := r.onBoard(ctx, s.BoardWrite, s.ItemID, tx)
		if err != nil {
			return nil, err
		}

		err = r.ActionItemDAO.EditItem(ctx, spec.ActionItemEdit{
			OrgID:          s.OrgID,
			BoardID:        s.BoardID,
			ItemID:         s.ItemID,
			Title:          s.Title,
			AssigneeUserID: s.AssigneeUserID,
			DueOn:          s.DueOn,
		}, tx)
		if err != nil {
			return nil, err
		}

		for _, person := range current.Mentions {
			if slices.Contains(s.Mentions, person.UserID) {
				continue
			}

			if err := r.ActionItemDAO.RemoveMention(ctx, spec.ActionItemMentionKey{OrgID: s.OrgID, ItemID: s.ItemID, UserID: person.UserID}, tx); err != nil {
				return nil, err
			}
		}

		for _, userID := range s.Mentions {
			if err := r.ActionItemDAO.AddMention(ctx, spec.ActionItemMentionKey{OrgID: s.OrgID, ItemID: s.ItemID, UserID: userID}, tx); err != nil {
				return nil, err
			}
		}

		return r.ActionItemDAO.GetByID(ctx, spec.ActionItemKey{OrgID: s.OrgID, ItemID: s.ItemID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, edit)
}

func (r *actionItemRepository) SetActionItemStatus(ctx context.Context, s spec.ActionItemStatusWrite, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error) {
	mark := func(tx *sql.Tx) (*model.ActionItem, error) {
		if _, err := r.onBoard(ctx, s.BoardWrite, s.ItemID, tx); err != nil {
			return nil, err
		}

		if err := r.ActionItemDAO.SetItemStatus(ctx, spec.ActionItemStatusChange{OrgID: s.OrgID, ItemID: s.ItemID, Status: s.Status}, tx); err != nil {
			return nil, err
		}

		return r.ActionItemDAO.GetByID(ctx, spec.ActionItemKey{OrgID: s.OrgID, ItemID: s.ItemID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, mark)
}

func (r *actionItemRepository) DeleteActionItem(ctx context.Context, s spec.ActionItemRemoval, tx ...*sql.Tx) (*model.Versioned[model.ActionItem], error) {
	remove := func(tx *sql.Tx) (*model.ActionItem, error) {
		current, err := r.onBoard(ctx, s.BoardWrite, s.ItemID, tx)
		if err != nil {
			return nil, err
		}

		if err := r.ActionItemDAO.DeleteItem(ctx, spec.ActionItemRef{OrgID: s.OrgID, BoardID: s.BoardID, ItemID: s.ItemID}, tx); err != nil {
			return nil, err
		}

		return current, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, remove)
}

func (r *actionItemRepository) ReviewActionItem(ctx context.Context, s spec.ActionItemReviewWrite, tx ...*sql.Tx) (*model.Versioned[model.ActionItemReviewed], error) {
	var reviewed *model.Versioned[model.ActionItemReviewed]

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		version, err := beginBoardWrite(ctx, r.boards, s.BoardWrite, tx)
		if err != nil {
			return err
		}

		result := model.ActionItemReviewed{}

		if s.Status != nil && s.ItemBoard != nil {
			itemVersion, err := beginBoardWrite(ctx, r.boards, *s.ItemBoard, tx)
			if err != nil {
				return err
			}

			result.ItemBoardVersion = new(itemVersion)
		}

		if s.Status != nil {
			if err := r.ActionItemDAO.SetItemStatus(ctx, spec.ActionItemStatusChange{OrgID: s.OrgID, ItemID: s.ItemID, Status: *s.Status}, tx); err != nil {
				return err
			}
		}

		if err := r.ActionItemDAO.RecordReview(ctx, spec.NewActionItemReview{OrgID: s.OrgID, ItemID: s.ItemID, BoardID: s.BoardID, Outcome: s.Outcome}, tx); err != nil {
			return err
		}

		item, err := r.ActionItemDAO.GetByID(ctx, spec.ActionItemKey{OrgID: s.OrgID, ItemID: s.ItemID}, tx)
		if err != nil {
			return err
		}

		result.Item = model.PreviousAction{ActionItem: *item, Review: new(s.Outcome)}
		reviewed = &model.Versioned[model.ActionItemReviewed]{Value: result, Version: version}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return reviewed, nil
}

func (r *actionItemRepository) ListPage(ctx context.Context, s spec.ActionItemFilter, tx ...*sql.Tx) (*model.ActionItemPage, error) {
	var page *model.ActionItemPage

	err := db.WithReadTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		items, err := r.ActionItemDAO.ListForOrganisation(ctx, s, tx)
		if err != nil {
			return err
		}

		counted := s
		counted.Status = nil

		counts, err := r.ActionItemDAO.CountForOrganisation(ctx, counted, tx)
		if err != nil {
			return err
		}

		page = &model.ActionItemPage{Items: items, Counts: counts}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return page, nil
}

func (r *actionItemRepository) onBoard(ctx context.Context, s spec.BoardWrite, itemID uuid.UUID, tx *sql.Tx) (*model.ActionItem, error) {
	item, err := r.ActionItemDAO.GetByID(ctx, spec.ActionItemKey{OrgID: s.OrgID, ItemID: itemID}, tx)
	if err != nil {
		return nil, err
	}

	if item.BoardID == nil || *item.BoardID != s.BoardID {
		return nil, fmt.Errorf("action item %s is not on board %s: %w", itemID, s.BoardID, dao.ErrNotFound)
	}

	return item, nil
}
