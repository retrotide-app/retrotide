package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	CardComment struct {
		ID                  uuid.UUID
		BoardID             uuid.UUID
		CardID              uuid.UUID
		AuthorParticipantID *uuid.UUID
		Body                string
		CreatedAt           time.Time
		UpdatedAt           *time.Time
		Mentions            []ActionItemPerson
	}
)

const (
	CardCommentBodyMaxRunes = 1000
)
