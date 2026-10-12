package audit

import (
	"time"

	"github.com/google/uuid"
)

type (
	Entry struct {
		ID          int64
		OrgID       *uuid.UUID
		ActorID     *uuid.UUID
		ActorKind   ActorKind
		ActorName   string
		Action      Action
		TargetType  TargetType
		TargetID    string
		SubjectID   *uuid.UUID
		SubjectName string
		Details     map[string]string
		IP          *string
		CreatedAt   time.Time
	}

	NewEntry struct {
		OrgID      uuid.UUID
		ActorID    uuid.UUID
		ActorKind  ActorKind
		Action     Action
		TargetType TargetType
		TargetID   string
		SubjectID  uuid.UUID
		Details    map[string]string
		IP         string
	}
)
