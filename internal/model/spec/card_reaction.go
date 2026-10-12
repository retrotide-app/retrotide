package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	CardReactionKey struct {
		OrgID         uuid.UUID
		BoardID       uuid.UUID
		CardID        uuid.UUID
		ParticipantID uuid.UUID
		Reaction      model.Reaction
	}

	CardReactionWrite struct {
		BoardWrite

		CardID        uuid.UUID
		ParticipantID uuid.UUID
		Reaction      model.Reaction
		Add           bool
	}
)
