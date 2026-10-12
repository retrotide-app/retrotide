package management

import (
	"retrotide/internal/board"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	MoveCommand struct {
		board.Command

		TeamID *uuid.UUID
	}

	ImportCommand struct {
		board.Command

		CSV string
	}

	RestoreCommand struct {
		Principal model.Principal
		OrgID     uuid.UUID
		BoardID   uuid.UUID
	}

	PinCommand struct {
		Principal model.Principal
		BoardID   uuid.UUID
		Pinned    bool
	}

	BulkCommand struct {
		Principal model.Principal
		OrgID     uuid.UUID
		Action    string
		BoardIDs  []uuid.UUID
	}

	BulkOutcome struct {
		BoardID uuid.UUID
		Err     error
	}
)

const (
	BulkArchive   = "archive"
	BulkUnarchive = "unarchive"
	BulkDelete    = "delete"

	MaxBulkBoards = 100
)
