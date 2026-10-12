package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewSavedTemplate struct {
		OrgID        uuid.UUID
		TeamID       *uuid.UUID
		OwnerID      *uuid.UUID
		CreatedBy    uuid.UUID
		Name         string
		Description  string
		Columns      []model.SavedTemplateColumn
		MaxTemplates int
	}

	SavedTemplateKey struct {
		OrgID uuid.UUID
		ID    uuid.UUID
	}

	SavedTemplateViewer struct {
		OrgID  uuid.UUID
		UserID uuid.UUID
	}

	SavedTemplateRemoval struct {
		OrgID  uuid.UUID
		ID     uuid.UUID
		UserID uuid.UUID
	}
)
