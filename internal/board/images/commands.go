package images

import (
	"retrotide/internal/board"

	"github.com/google/uuid"
)

type (
	CardImageCommand struct {
		board.Command

		CardID  uuid.UUID
		MediaID *uuid.UUID
	}

	BackgroundCommand struct {
		board.Command

		MediaID *uuid.UUID
	}
)
