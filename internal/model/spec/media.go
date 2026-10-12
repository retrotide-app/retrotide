package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewMedia struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		Kind    model.MediaKind
		Width   int
		Height  int
		Data    []byte
	}

	MediaKey struct {
		BoardID uuid.UUID
		MediaID uuid.UUID
	}
)
