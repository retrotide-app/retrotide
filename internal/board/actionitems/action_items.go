package actionitems

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) Add(ctx context.Context, command AddCommand) (*dto.BoardCommandResponse, error) {
	if err := board.ClientID(command.ItemID); err != nil {
		return nil, err
	}

	if command.SourceCardID == nil || *command.SourceCardID == uuid.Nil {
		return nil, ErrCardRequired
	}

	fields, err := cleanActionItem(command.Fields)
	if err != nil {
		return nil, err
	}

	add := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanWriteActionItems(access); err != nil {
			return nil, err
		}

		if err := s.visibleCard(ctx, access, *command.SourceCardID); err != nil {
			return nil, err
		}

		if err := s.members.CheckNamed(ctx, access, newlyNamed(fields, nil)); err != nil {
			return nil, err
		}

		written, err := s.actionItems.AddActionItem(ctx, spec.ActionItemCreate{
			BoardWrite:       target,
			ActionItemFields: fields,
			ItemID:           command.ItemID,
			TeamID:           access.Header.TeamID,
			SourceCardID:     command.SourceCardID,
			CreatedBy:        creator(access),
		})
		if errors.Is(err, dao.ErrConflict) {
			return nil, fmt.Errorf("%w: action item %s: %w", board.ErrIDConflict, command.ItemID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("add action item %s to board %s: %w", command.ItemID, target.BoardID, err)
		}

		return s.announceActionItem(access, written), nil
	}

	return s.Run(ctx, command.Command, s.actionItemReplay(ctx, command.ItemID), add)
}

func (s *service) Edit(ctx context.Context, command EditCommand) (*dto.BoardCommandResponse, error) {
	fields, err := cleanActionItem(command.Fields)
	if err != nil {
		return nil, err
	}

	edit := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanWriteActionItems(access); err != nil {
			return nil, err
		}

		current, err := s.boardItem(ctx, access, command.ItemID)
		if err != nil {
			return nil, err
		}

		if err := s.members.CheckNamed(ctx, access, newlyNamed(fields, current)); err != nil {
			return nil, err
		}

		written, err := s.actionItems.EditActionItem(ctx, spec.ActionItemEditWrite{BoardWrite: target, ActionItemFields: fields, ItemID: current.ID})
		if err != nil {
			return nil, fmt.Errorf("edit action item %s of board %s: %w", current.ID, target.BoardID, err)
		}

		return s.announceActionItem(access, written), nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, edit)
}

func (s *service) SetStatus(ctx context.Context, command StatusCommand) (*dto.BoardCommandResponse, error) {
	if !slices.Contains(model.ActionItemStatuses, command.Status) {
		return nil, ErrInvalidStatus
	}

	mark := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanMarkActionItem(access); err != nil {
			return nil, err
		}

		current, err := s.boardItem(ctx, access, command.ItemID)
		if err != nil {
			return nil, err
		}

		if current.Status == command.Status {
			response := projection.ActionItemResponse(access.Header.Board, *current, projection.ViewerFor(access), access.Header.Version)

			return &response, nil
		}

		written, err := s.actionItems.SetActionItemStatus(ctx, spec.ActionItemStatusWrite{BoardWrite: target, ItemID: current.ID, Status: command.Status})
		if err != nil {
			return nil, fmt.Errorf("mark action item %s of board %s %s: %w", current.ID, target.BoardID, command.Status, err)
		}

		return s.announceActionItem(access, written), nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, mark)
}

func (s *service) Delete(ctx context.Context, command ItemCommand) (*dto.BoardCommandResponse, error) {
	remove := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		current, err := s.boardItem(ctx, access, command.ItemID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanDeleteActionItem(access, *current); err != nil {
			return nil, err
		}

		written, err := s.actionItems.DeleteActionItem(ctx, spec.ActionItemRemoval{BoardWrite: target, ItemID: current.ID})
		if err != nil {
			return nil, fmt.Errorf("delete action item %s of board %s: %w", current.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ActionItemRemoved(current.ID, written.Version))
		s.announceTeam(access.Header.Board)

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, remove)
}

func (s *service) Review(ctx context.Context, command ReviewCommand) (*dto.BoardCommandResponse, error) {
	if !slices.Contains(model.ReviewOutcomes, command.Outcome) {
		return nil, ErrInvalidReviewOutcome
	}

	review := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanReviewActionItems(access); err != nil {
			return nil, err
		}

		action, err := s.previousAction(ctx, access, command.ItemID)
		if err != nil {
			return nil, err
		}

		write := spec.ActionItemReviewWrite{BoardWrite: target, ItemID: action.ID, Outcome: command.Outcome}

		status := command.Outcome.Status()
		if action.Status == status {
			return s.recordReview(ctx, access, write, nil)
		}

		write.Status = new(status)
		if action.BoardID == nil {
			return s.recordReview(ctx, access, write, nil)
		}

		return s.reviewAcrossBoards(ctx, access, write, *action.BoardID)
	}

	return s.Run(ctx, command.Command, board.VersionReplay, review)
}

func (s *service) reviewAcrossBoards(ctx context.Context, access model.BoardAccess, write spec.ActionItemReviewWrite, itemBoardID uuid.UUID) (*dto.BoardCommandResponse, error) {
	release, err := s.live.Acquire(ctx, itemBoardID)
	if err != nil {
		return nil, err
	}
	defer release()

	header, err := s.boards.GetHeader(ctx, itemBoardID)
	if errors.Is(err, dao.ErrNotFound) {
		return s.recordReview(ctx, access, write, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("read the header of board %s, where action item %s was raised: %w", itemBoardID, write.ItemID, err)
	}

	if header.Gone() {
		return s.recordReview(ctx, access, write, nil)
	}

	write.ItemBoard = &spec.BoardWrite{OrgID: header.OrgID, BoardID: header.ID, Version: header.Version}

	return s.recordReview(ctx, access, write, header)
}

func (s *service) recordReview(ctx context.Context, access model.BoardAccess, write spec.ActionItemReviewWrite, itemBoard *model.BoardHeader) (*dto.BoardCommandResponse, error) {
	written, err := s.actionItems.ReviewActionItem(ctx, write)
	if err != nil {
		return nil, fmt.Errorf("review action item %s on board %s: %w", write.ItemID, write.BoardID, err)
	}

	reviewed := written.Value

	s.events.Publish(access, projection.PreviousActionUpserted(reviewed.Item, written.Version))

	exclude := []uuid.UUID{access.Header.ID}
	if itemBoard != nil && reviewed.ItemBoardVersion != nil {
		held := model.BoardAccess{Header: *itemBoard}
		s.events.Publish(held, projection.ActionItemUpserted(itemBoard.Board, reviewed.Item.ActionItem, *reviewed.ItemBoardVersion))
		exclude = append(exclude, itemBoard.ID)
	}

	if access.Header.TeamID != nil {
		s.events.AnnounceTeamActionItems(*access.Header.TeamID, exclude)
	}

	response := projection.PreviousActionResponse(reviewed.Item, written.Version)

	return &response, nil
}

func (s *service) announceActionItem(access model.BoardAccess, written *model.Versioned[model.ActionItem]) *dto.BoardCommandResponse {
	s.events.Publish(access, projection.ActionItemUpserted(access.Header.Board, written.Value, written.Version))
	s.announceTeam(access.Header.Board)

	response := projection.ActionItemResponse(access.Header.Board, written.Value, projection.ViewerFor(access), written.Version)

	return &response
}

func (s *service) announceTeam(itemBoard model.Board) {
	if itemBoard.TeamID == nil {
		return
	}

	s.events.AnnounceTeamActionItems(*itemBoard.TeamID, []uuid.UUID{itemBoard.ID})
}

func (s *service) actionItemReplay(ctx context.Context, itemID uuid.UUID) board.Replay {
	return func(access model.BoardAccess, version int64) (*dto.BoardCommandResponse, error) {
		created := projection.ReplayResponse(version, &itemID)

		item, err := s.actionItems.GetByID(ctx, spec.ActionItemKey{OrgID: access.Header.OrgID, ItemID: itemID})
		if errors.Is(err, dao.ErrNotFound) {
			return &created, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read the replayed action item %s of board %s: %w", itemID, access.Header.ID, err)
		}

		if item.BoardID == nil || *item.BoardID != access.Header.ID {
			return &created, nil
		}

		response := projection.ReplayActionItemResponse(access.Header.Board, *item, projection.ViewerFor(access), version)

		return &response, nil
	}
}

func (s *service) boardItem(ctx context.Context, access model.BoardAccess, itemID uuid.UUID) (*model.ActionItem, error) {
	item, err := s.actionItems.GetByID(ctx, spec.ActionItemKey{OrgID: access.Header.OrgID, ItemID: itemID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", policy.ErrActionItemNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read action item %s of board %s: %w", itemID, access.Header.ID, err)
	}

	if item.BoardID == nil || *item.BoardID != access.Header.ID {
		return nil, fmt.Errorf("%w: action item %s is not on board %s", policy.ErrActionItemNotFound, itemID, access.Header.ID)
	}

	return item, nil
}

func (s *service) previousAction(ctx context.Context, access model.BoardAccess, itemID uuid.UUID) (*model.PreviousAction, error) {
	if access.Header.TeamID == nil {
		return nil, fmt.Errorf("%w: board %s belongs to no team, so it reviews no previous actions", policy.ErrActionItemNotFound, access.Header.ID)
	}

	previous, err := s.actionItems.ListPrevious(ctx, spec.PreviousActionsQuery{
		OrgID:   access.Header.OrgID,
		TeamID:  *access.Header.TeamID,
		BoardID: access.Header.ID,
		Before:  access.Header.CreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("list the previous actions of board %s: %w", access.Header.ID, err)
	}

	for _, action := range previous {
		if action.ID == itemID {
			return &action, nil
		}
	}

	return nil, fmt.Errorf("%w: action item %s is not up for review on board %s", policy.ErrActionItemNotFound, itemID, access.Header.ID)
}

func (s *service) visibleCard(ctx context.Context, access model.BoardAccess, cardID uuid.UUID) error {
	card, err := s.Card(ctx, access, cardID)
	if err != nil {
		return err
	}

	if !projection.Visible(access.Header.Board, *card, projection.ViewerFor(access)) {
		return fmt.Errorf("%w: card %s is hidden from the person making an action item from it", policy.ErrCardNotFound, card.ID)
	}

	if slices.Contains(access.Header.HiddenColumnIDs, card.ColumnID) {
		return fmt.Errorf("%w: card %s sits in a hidden column", policy.ErrColumnHidden, card.ID)
	}

	return nil
}

func newlyNamed(fields spec.ActionItemFields, current *model.ActionItem) []uuid.UUID {
	var added []uuid.UUID

	keptAssignee := current != nil && current.AssigneeUserID != nil && fields.AssigneeUserID != nil && *current.AssigneeUserID == *fields.AssigneeUserID
	if fields.AssigneeUserID != nil && !keptAssignee {
		added = append(added, *fields.AssigneeUserID)
	}

	var mentioned []model.ActionItemPerson
	if current != nil {
		mentioned = current.Mentions
	}

	for _, userID := range board.NewMentions(fields.Mentions, mentioned) {
		if !slices.Contains(added, userID) {
			added = append(added, userID)
		}
	}

	return added
}

func creator(access model.BoardAccess) *uuid.UUID {
	if access.Principal.Kind != model.PrincipalUser || access.Header.Anonymous() {
		return nil
	}

	return new(access.Principal.ID)
}

func cleanActionItem(raw Fields) (spec.ActionItemFields, error) {
	title := strings.TrimSpace(raw.Title)
	if title == "" {
		return spec.ActionItemFields{}, ErrTitleRequired
	}

	if utf8.RuneCountInString(title) > model.ActionItemTitleMaxRunes {
		return spec.ActionItemFields{}, ErrTitleTooLong
	}

	due, err := cleanDueOn(raw.DueOn)
	if err != nil {
		return spec.ActionItemFields{}, err
	}

	mentions, err := board.CleanMentions(raw.Mentions)
	if err != nil {
		return spec.ActionItemFields{}, err
	}

	if raw.AssigneeUserID != nil && *raw.AssigneeUserID == uuid.Nil {
		return spec.ActionItemFields{}, fmt.Errorf("%w: an empty assignee", board.ErrNotBoardMember)
	}

	return spec.ActionItemFields{Title: title, AssigneeUserID: raw.AssigneeUserID, DueOn: due, Mentions: mentions}, nil
}

func cleanDueOn(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}

	due, err := time.Parse(model.DueDateLayout, strings.TrimSpace(*raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDueDate, err)
	}

	return &due, nil
}
