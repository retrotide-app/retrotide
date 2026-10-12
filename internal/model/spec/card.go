package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewCard struct {
		ID                  uuid.UUID
		BoardID             uuid.UUID
		ColumnID            uuid.UUID
		AuthorParticipantID *uuid.UUID
		Body                string
		Colour              *model.Colour
		Position            string
	}

	CardRef struct {
		BoardID uuid.UUID
		CardID  uuid.UUID
	}

	CardAuthorReassignment struct {
		BoardID           uuid.UUID
		FromParticipantID uuid.UUID
		ToParticipantID   uuid.UUID
	}

	CardEdit struct {
		BoardID uuid.UUID
		CardID  uuid.UUID
		Body    string
	}

	CardColourChange struct {
		BoardID uuid.UUID
		CardID  uuid.UUID
		Colour  *model.Colour
	}

	CardMove struct {
		BoardID  uuid.UUID
		CardID   uuid.UUID
		ColumnID uuid.UUID
		Position string
	}

	CardMentionKey struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		CardID  uuid.UUID
		UserID  uuid.UUID
	}

	CardCreate struct {
		BoardWrite

		CardID              uuid.UUID
		ColumnID            uuid.UUID
		AuthorParticipantID *uuid.UUID
		Body                string
		Colour              *model.Colour
		Position            string
		Mentions            []uuid.UUID
	}

	CardBodyWrite struct {
		BoardWrite

		CardID   uuid.UUID
		Body     string
		Mentions []uuid.UUID
	}

	CardColourWrite struct {
		BoardWrite

		CardID uuid.UUID
		Colour *model.Colour
	}

	CardPlacement struct {
		BoardWrite

		CardID   uuid.UUID
		ColumnID uuid.UUID
		Position string
		Rewrite  []PositionChange
	}

	CardRemoval struct {
		BoardWrite

		CardID uuid.UUID
	}

	CardStacking struct {
		BoardID      uuid.UUID
		CardID       uuid.UUID
		ParentCardID uuid.UUID
		ColumnID     uuid.UUID
		Position     string
	}

	CardRestack struct {
		BoardID          uuid.UUID
		FromParentCardID uuid.UUID
		ToParentCardID   uuid.UUID
		ColumnID         uuid.UUID
	}

	CardMergeWrite struct {
		BoardWrite

		CardID       uuid.UUID
		ParentCardID uuid.UUID
		ColumnID     uuid.UUID
		Position     string
	}

	CardPositionChange struct {
		BoardID  uuid.UUID
		CardID   uuid.UUID
		Position string
	}

	CardPinChange struct {
		BoardID uuid.UUID
		CardID  uuid.UUID
		Pinned  bool
	}

	CardPinWrite struct {
		BoardWrite

		CardID uuid.UUID
		Pinned bool
	}

	CardImageChange struct {
		BoardID uuid.UUID
		CardID  uuid.UUID
		MediaID *uuid.UUID
	}

	CardImageWrite struct {
		BoardWrite

		CardID  uuid.UUID
		MediaID *uuid.UUID
	}

	CardSeed struct {
		CardID              uuid.UUID
		ColumnID            uuid.UUID
		AuthorParticipantID *uuid.UUID
		Body                string
		Position            string
	}

	CardImportWrite struct {
		BoardWrite

		Columns []BoardColumnSeed
		Cards   []CardSeed
	}
)
