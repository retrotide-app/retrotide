package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	ActionItemDAO interface {
		CreateItem(ctx context.Context, s spec.NewActionItem, tx ...*sql.Tx) error
		GetByID(ctx context.Context, s spec.ActionItemKey, tx ...*sql.Tx) (*model.ActionItem, error)
		ListForBoard(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) ([]model.ActionItem, error)
		ListPrevious(ctx context.Context, s spec.PreviousActionsQuery, tx ...*sql.Tx) ([]model.PreviousAction, error)
		ListForOrganisation(ctx context.Context, s spec.ActionItemFilter, tx ...*sql.Tx) ([]model.ActionItemListing, error)
		CountForOrganisation(ctx context.Context, s spec.ActionItemFilter, tx ...*sql.Tx) (model.ActionItemCounts, error)
		EditItem(ctx context.Context, s spec.ActionItemEdit, tx ...*sql.Tx) error
		SetItemStatus(ctx context.Context, s spec.ActionItemStatusChange, tx ...*sql.Tx) error
		DeleteItem(ctx context.Context, s spec.ActionItemRef, tx ...*sql.Tx) error
		AddMention(ctx context.Context, s spec.ActionItemMentionKey, tx ...*sql.Tx) error
		RemoveMention(ctx context.Context, s spec.ActionItemMentionKey, tx ...*sql.Tx) error
		RecordReview(ctx context.Context, s spec.NewActionItemReview, tx ...*sql.Tx) error
		ListMentionsForUser(ctx context.Context, s spec.UserMentionsQuery, tx ...*sql.Tx) ([]model.Mention, error)
		ListForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.UserActionItem, error)
		SetTeamForBoard(ctx context.Context, s spec.ActionItemTeamChange, tx ...*sql.Tx) (int64, error)
	}

	actionItemDAO struct {
		db *sql.DB
	}

	mentionedPerson struct {
		UserID      uuid.UUID `json:"user_id"`
		DisplayName string    `json:"display_name"`
	}
)

func toActionItem(row sqlcgen.ActionItem, assigneeName string, mentions string) (*model.ActionItem, error) {
	var people []mentionedPerson
	if err := json.Unmarshal([]byte(mentions), &people); err != nil {
		return nil, fmt.Errorf("decode the mentions of action item %s: %w", row.ID, err)
	}

	mentioned := make([]model.ActionItemPerson, 0, len(people))
	for _, person := range people {
		mentioned = append(mentioned, model.ActionItemPerson{UserID: person.UserID, DisplayName: person.DisplayName})
	}

	return &model.ActionItem{
		ID:             row.ID,
		OrgID:          row.OrgID,
		TeamID:         row.TeamID,
		BoardID:        row.BoardID,
		SourceCardID:   row.SourceCardID,
		Title:          row.Title,
		AssigneeUserID: row.AssigneeUserID,
		AssigneeName:   assigneeName,
		DueOn:          nullableTime(row.DueOn),
		Status:         model.ActionItemStatus(row.Status),
		CompletedAt:    nullableTime(row.CompletedAt),
		CreatedBy:      row.CreatedBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Mentions:       mentioned,
	}, nil
}

func touched(affected int64, err error, action string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	if affected == 0 {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}

	return nil
}

func (r *actionItemDAO) CreateItem(ctx context.Context, s spec.NewActionItem, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateActionItem(ctx, sqlcgen.CreateActionItemParams{
		ID:             s.ID,
		OrgID:          s.OrgID,
		TeamID:         s.TeamID,
		BoardID:        s.BoardID,
		SourceCardID:   s.SourceCardID,
		Title:          s.Title,
		AssigneeUserID: s.AssigneeUserID,
		DueOn:          toNullTime(s.DueOn),
		CreatedBy:      s.CreatedBy,
	})
	if isUniqueViolation(err) {
		return fmt.Errorf("create action item %s in organisation %s: %w: %w", s.ID, s.OrgID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return fmt.Errorf("create action item %s in organisation %s: %w: %w", s.ID, s.OrgID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("create action item %s in organisation %s: %w", s.ID, s.OrgID, err)
	}

	return nil
}

func (r *actionItemDAO) GetByID(ctx context.Context, s spec.ActionItemKey, tx ...*sql.Tx) (*model.ActionItem, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetActionItem(ctx, sqlcgen.GetActionItemParams{OrgID: s.OrgID, ID: s.ItemID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get action item %s of organisation %s: %w", s.ItemID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get action item %s of organisation %s: %w", s.ItemID, s.OrgID, err)
	}

	return toActionItem(row.ActionItem, row.AssigneeName, row.Mentions)
}

func (r *actionItemDAO) ListForBoard(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) ([]model.ActionItem, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListActionItemsForBoard(ctx, sqlcgen.ListActionItemsForBoardParams{OrgID: s.OrgID, BoardID: s.BoardID})
	if err != nil {
		return nil, fmt.Errorf("list the action items of board %s: %w", s.BoardID, err)
	}

	items := make([]model.ActionItem, 0, len(rows))
	for _, row := range rows {
		item, err := toActionItem(row.ActionItem, row.AssigneeName, row.Mentions)
		if err != nil {
			return nil, err
		}

		items = append(items, *item)
	}

	return items, nil
}

func (r *actionItemDAO) ListPrevious(ctx context.Context, s spec.PreviousActionsQuery, tx ...*sql.Tx) ([]model.PreviousAction, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListPreviousActionItems(ctx, sqlcgen.ListPreviousActionItemsParams{
		BoardID: s.BoardID,
		OrgID:   s.OrgID,
		TeamID:  s.TeamID,
		Before:  s.Before,
	})
	if err != nil {
		return nil, fmt.Errorf("list the previous actions of team %s for board %s: %w", s.TeamID, s.BoardID, err)
	}

	previous := make([]model.PreviousAction, 0, len(rows))
	for _, row := range rows {
		item, err := toActionItem(row.ActionItem, row.AssigneeName, row.Mentions)
		if err != nil {
			return nil, err
		}

		action := model.PreviousAction{ActionItem: *item}
		if row.ReviewOutcome != "" {
			action.Review = new(model.ReviewOutcome(row.ReviewOutcome))
		}

		previous = append(previous, action)
	}

	return previous, nil
}

func (r *actionItemDAO) ListForOrganisation(ctx context.Context, s spec.ActionItemFilter, tx ...*sql.Tx) ([]model.ActionItemListing, error) {
	params := sqlcgen.ListActionItemsForOrganisationParams{
		OrgID:              s.OrgID,
		EveryTeam:          s.EveryTeam,
		ViewerID:           s.ViewerID,
		AssigneeUserID:     s.AssigneeUserID,
		Unassigned:         s.Unassigned,
		TeamID:             s.TeamID,
		OrganisationBoards: s.OrganisationBoards,
		RowLimit:           clampInt32(s.Limit),
		RowOffset:          clampInt32(s.Offset),
	}
	if s.Status != nil {
		params.Status = sql.NullString{String: string(*s.Status), Valid: true}
	}

	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListActionItemsForOrganisation(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list the action items of organisation %s: %w", s.OrgID, err)
	}

	listings := make([]model.ActionItemListing, 0, len(rows))
	for _, row := range rows {
		item, err := toActionItem(row.ActionItem, row.AssigneeName, row.Mentions)
		if err != nil {
			return nil, err
		}

		listings = append(listings, model.ActionItemListing{
			ActionItem:     *item,
			BoardTitle:     row.BoardTitle,
			BoardAnonymity: model.Anonymity(row.BoardAnonymity),
			TeamName:       row.TeamName,
		})
	}

	return listings, nil
}

func (r *actionItemDAO) CountForOrganisation(ctx context.Context, s spec.ActionItemFilter, tx ...*sql.Tx) (model.ActionItemCounts, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountActionItemsForOrganisation(ctx, sqlcgen.CountActionItemsForOrganisationParams{
		OrgID:              s.OrgID,
		EveryTeam:          s.EveryTeam,
		ViewerID:           s.ViewerID,
		AssigneeUserID:     s.AssigneeUserID,
		Unassigned:         s.Unassigned,
		TeamID:             s.TeamID,
		OrganisationBoards: s.OrganisationBoards,
	})
	if err != nil {
		return model.ActionItemCounts{}, fmt.Errorf("count the action items of organisation %s: %w", s.OrgID, err)
	}

	var counts model.ActionItemCounts
	for _, row := range rows {
		switch model.ActionItemStatus(row.Status) {
		case model.ActionItemOpen:
			counts.Open = int(row.ItemCount)
		case model.ActionItemDone:
			counts.Done = int(row.ItemCount)
		case model.ActionItemDropped:
			counts.Dropped = int(row.ItemCount)
		}
	}

	return counts, nil
}

func (r *actionItemDAO) EditItem(ctx context.Context, s spec.ActionItemEdit, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).EditActionItem(ctx, sqlcgen.EditActionItemParams{
		Title:          s.Title,
		AssigneeUserID: s.AssigneeUserID,
		DueOn:          toNullTime(s.DueOn),
		OrgID:          s.OrgID,
		ID:             s.ItemID,
		BoardID:        s.BoardID,
	})
	if isForeignKeyViolation(err) {
		return fmt.Errorf("edit action item %s of board %s: %w: %w", s.ItemID, s.BoardID, ErrNotFound, err)
	}

	return touched(affected, err, fmt.Sprintf("edit action item %s of board %s", s.ItemID, s.BoardID))
}

func (r *actionItemDAO) SetItemStatus(ctx context.Context, s spec.ActionItemStatusChange, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetActionItemStatus(ctx, sqlcgen.SetActionItemStatusParams{Status: string(s.Status), OrgID: s.OrgID, ID: s.ItemID})

	return touched(affected, err, fmt.Sprintf("set action item %s of organisation %s to %s", s.ItemID, s.OrgID, s.Status))
}

func (r *actionItemDAO) DeleteItem(ctx context.Context, s spec.ActionItemRef, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteActionItem(ctx, sqlcgen.DeleteActionItemParams{OrgID: s.OrgID, ID: s.ItemID, BoardID: s.BoardID})

	return touched(affected, err, fmt.Sprintf("delete action item %s of board %s", s.ItemID, s.BoardID))
}

func (r *actionItemDAO) SetTeamForBoard(ctx context.Context, s spec.ActionItemTeamChange, tx ...*sql.Tx) (int64, error) {
	moved, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetActionItemTeamForBoard(ctx, sqlcgen.SetActionItemTeamForBoardParams{TeamID: s.TeamID, OrgID: s.OrgID, BoardID: s.BoardID})
	if isForeignKeyViolation(err) {
		return 0, fmt.Errorf("move the action items of board %s to %s: %w: %w", s.BoardID, boardPlace(s.OrgID, s.TeamID), ErrNotFound, err)
	}
	if err != nil {
		return 0, fmt.Errorf("move the action items of board %s to %s: %w", s.BoardID, boardPlace(s.OrgID, s.TeamID), err)
	}

	return moved, nil
}

func (r *actionItemDAO) AddMention(ctx context.Context, s spec.ActionItemMentionKey, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddActionItemMention(ctx, sqlcgen.AddActionItemMentionParams{OrgID: s.OrgID, ActionItemID: s.ItemID, UserID: s.UserID})
	if isForeignKeyViolation(err) {
		return fmt.Errorf("mention user %s on action item %s: %w: %w", s.UserID, s.ItemID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("mention user %s on action item %s: %w", s.UserID, s.ItemID, err)
	}

	return nil
}

func (r *actionItemDAO) RemoveMention(ctx context.Context, s spec.ActionItemMentionKey, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveActionItemMention(ctx, sqlcgen.RemoveActionItemMentionParams{OrgID: s.OrgID, ActionItemID: s.ItemID, UserID: s.UserID})
	if err != nil {
		return fmt.Errorf("remove the mention of user %s from action item %s: %w", s.UserID, s.ItemID, err)
	}

	return nil
}

func (r *actionItemDAO) RecordReview(ctx context.Context, s spec.NewActionItemReview, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).RecordActionItemReview(ctx, sqlcgen.RecordActionItemReviewParams{
		OrgID:        s.OrgID,
		ActionItemID: s.ItemID,
		BoardID:      s.BoardID,
		Outcome:      string(s.Outcome),
	})
	if isForeignKeyViolation(err) {
		return fmt.Errorf("review action item %s on board %s: %w: %w", s.ItemID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("review action item %s on board %s: %w", s.ItemID, s.BoardID, err)
	}

	return nil
}

func (r *actionItemDAO) ListMentionsForUser(ctx context.Context, s spec.UserMentionsQuery, tx ...*sql.Tx) ([]model.Mention, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListMentionsForUser(ctx, sqlcgen.ListMentionsForUserParams{UserID: s.UserID, RowLimit: clampInt32(s.Limit)})
	if err != nil {
		return nil, fmt.Errorf("list the mentions of user %s: %w", s.UserID, err)
	}

	mentions := make([]model.Mention, 0, len(rows))
	for _, row := range rows {
		teamID := row.TeamID
		if row.BoardID != nil {
			teamID = row.BoardTeamID
		}

		mentions = append(mentions, model.Mention{
			Kind:         model.MentionActionItem,
			ActionItemID: row.ID,
			OrgID:        row.OrgID,
			OrgSlug:      row.OrgSlug,
			OrgName:      row.OrgName,
			TeamID:       teamID,
			BoardID:      row.BoardID,
			BoardTitle:   row.BoardTitle,
			Title:        row.Title,
			Status:       model.ActionItemStatus(row.Status),
			DueOn:        nullableTime(row.DueOn),
			MentionedAt:  row.MentionedAt,
		})
	}

	return mentions, nil
}

func (r *actionItemDAO) ListForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.UserActionItem, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListActionItemsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the action items raised by or assigned to user %s: %w", userID, err)
	}

	items := make([]model.UserActionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.UserActionItem{
			OrgName:        row.OrgName,
			TeamName:       row.TeamName,
			BoardTitle:     row.BoardTitle,
			Title:          row.Title,
			Status:         model.ActionItemStatus(row.Status),
			DueOn:          nullableTime(row.DueOn),
			RaisedByUser:   row.CreatedBy != nil && *row.CreatedBy == userID,
			AssignedToUser: row.AssigneeUserID != nil && *row.AssigneeUserID == userID,
		})
	}

	return items, nil
}
