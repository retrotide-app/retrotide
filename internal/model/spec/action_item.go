package spec

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewActionItem struct {
		ID             uuid.UUID
		OrgID          uuid.UUID
		TeamID         *uuid.UUID
		BoardID        *uuid.UUID
		SourceCardID   *uuid.UUID
		Title          string
		AssigneeUserID *uuid.UUID
		DueOn          *time.Time
		CreatedBy      *uuid.UUID
	}

	ActionItemKey struct {
		OrgID  uuid.UUID
		ItemID uuid.UUID
	}

	ActionItemRef struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		ItemID  uuid.UUID
	}

	ActionItemEdit struct {
		OrgID          uuid.UUID
		BoardID        uuid.UUID
		ItemID         uuid.UUID
		Title          string
		AssigneeUserID *uuid.UUID
		DueOn          *time.Time
	}

	ActionItemStatusChange struct {
		OrgID  uuid.UUID
		ItemID uuid.UUID
		Status model.ActionItemStatus
	}

	ActionItemMentionKey struct {
		OrgID  uuid.UUID
		ItemID uuid.UUID
		UserID uuid.UUID
	}

	NewActionItemReview struct {
		OrgID   uuid.UUID
		ItemID  uuid.UUID
		BoardID uuid.UUID
		Outcome model.ReviewOutcome
	}

	PreviousActionsQuery struct {
		OrgID   uuid.UUID
		TeamID  uuid.UUID
		BoardID uuid.UUID
		Before  time.Time
	}

	ActionItemFilter struct {
		OrgID              uuid.UUID
		ViewerID           uuid.UUID
		EveryTeam          bool
		Status             *model.ActionItemStatus
		AssigneeUserID     *uuid.UUID
		Unassigned         bool
		TeamID             *uuid.UUID
		OrganisationBoards bool
		Limit              int
		Offset             int
	}

	UserMentionsQuery struct {
		UserID uuid.UUID
		Limit  int
	}

	ActionItemFields struct {
		Title          string
		AssigneeUserID *uuid.UUID
		DueOn          *time.Time
		Mentions       []uuid.UUID
	}

	ActionItemCreate struct {
		BoardWrite
		ActionItemFields

		ItemID       uuid.UUID
		TeamID       *uuid.UUID
		SourceCardID *uuid.UUID
		CreatedBy    *uuid.UUID
	}

	ActionItemEditWrite struct {
		BoardWrite
		ActionItemFields

		ItemID uuid.UUID
	}

	ActionItemStatusWrite struct {
		BoardWrite

		ItemID uuid.UUID
		Status model.ActionItemStatus
	}

	ActionItemRemoval struct {
		BoardWrite

		ItemID uuid.UUID
	}

	ActionItemReviewWrite struct {
		BoardWrite

		ItemID    uuid.UUID
		Outcome   model.ReviewOutcome
		Status    *model.ActionItemStatus
		ItemBoard *BoardWrite
	}
)
