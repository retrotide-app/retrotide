package actionitems

import (
	"retrotide/internal/board"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	Fields struct {
		Title          string
		AssigneeUserID *uuid.UUID
		DueOn          *string
		Mentions       []uuid.UUID
	}

	AddCommand struct {
		board.Command
		Fields

		ItemID       uuid.UUID
		SourceCardID *uuid.UUID
	}

	EditCommand struct {
		board.Command
		Fields

		ItemID uuid.UUID
	}

	ItemCommand struct {
		board.Command

		ItemID uuid.UUID
	}

	StatusCommand struct {
		board.Command

		ItemID uuid.UUID
		Status model.ActionItemStatus
	}

	ReviewCommand struct {
		board.Command

		ItemID  uuid.UUID
		Outcome model.ReviewOutcome
	}
)
