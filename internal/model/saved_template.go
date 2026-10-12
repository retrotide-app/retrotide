package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	SavedTemplateScope string

	SavedTemplateColumn struct {
		Title       string
		Description string
		Colour      Colour
	}

	SavedTemplate struct {
		ID          uuid.UUID
		OrgID       uuid.UUID
		TeamID      *uuid.UUID
		OwnerID     *uuid.UUID
		CreatedBy   *uuid.UUID
		Name        string
		Description string
		Columns     []SavedTemplateColumn
		TeamName    string
		CreatedAt   time.Time
	}
)

const (
	SavedTemplatePersonal SavedTemplateScope = "personal"
	SavedTemplateTeam     SavedTemplateScope = "team"

	SavedTemplateNameMaxRunes        = 80
	SavedTemplateDescriptionMaxRunes = 300
)

func (t *SavedTemplate) Scope() SavedTemplateScope {
	if t.TeamID != nil {
		return SavedTemplateTeam
	}

	return SavedTemplatePersonal
}
