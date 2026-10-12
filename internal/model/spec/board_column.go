package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	BoardColumnSeed struct {
		ID          uuid.UUID
		Title       string
		Description string
		Colour      model.Colour
		Position    string
	}

	NewBoardColumn struct {
		BoardID uuid.UUID

		BoardColumnSeed
	}

	BoardColumnKey struct {
		BoardID  uuid.UUID
		ColumnID uuid.UUID
	}

	BoardColumnUpdate struct {
		BoardID     uuid.UUID
		ColumnID    uuid.UUID
		Title       string
		Description string
		Colour      model.Colour
	}

	BoardColumnMove struct {
		BoardID  uuid.UUID
		ColumnID uuid.UUID
		Position string
	}

	ColumnCreate struct {
		BoardWrite

		Column BoardColumnSeed
	}

	ColumnEdit struct {
		BoardWrite

		ColumnID    uuid.UUID
		Title       string
		Description string
		Colour      model.Colour
	}

	ColumnPlacement struct {
		BoardWrite

		ColumnID uuid.UUID
		Position string
		Rewrite  []PositionChange
	}

	ColumnRemoval struct {
		BoardWrite

		ColumnID uuid.UUID
	}

	BoardColumnMaxVotes struct {
		BoardID  uuid.UUID
		ColumnID uuid.UUID
		MaxVotes *int
	}

	ColumnVoteLimitWrite struct {
		BoardWrite

		ColumnID uuid.UUID
		MaxVotes *int
	}
)
