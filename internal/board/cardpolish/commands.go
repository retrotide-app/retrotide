package cardpolish

import (
	"retrotide/internal/board"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	ReactCommand struct {
		board.Command

		CardID   uuid.UUID
		Reaction model.Reaction
	}

	PinCommand struct {
		board.Command

		CardID uuid.UUID
		Pinned bool
	}
)
