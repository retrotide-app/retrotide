package spec

import (
	"retrotide/internal/audit"

	"github.com/google/uuid"
)

type (
	AuditLogListing struct {
		Action audit.Action
		Limit  int
		Offset int
	}

	AuditLogSubjectListing struct {
		SubjectID uuid.UUID
		Limit     int
		Offset    int
	}
)
