package dto

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	SaveTemplateRequest struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Scope       string `json:"scope"`
	}

	SavedTemplateResponse struct {
		ID          uuid.UUID                `json:"id"`
		Scope       string                   `json:"scope"`
		TeamID      *uuid.UUID               `json:"team_id,omitempty"`
		TeamName    string                   `json:"team_name,omitempty"`
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		Columns     []TemplateColumnResponse `json:"columns"`
		CreatedAt   string                   `json:"created_at"`
		CanDelete   bool                     `json:"can_delete"`
	}

	SavedTemplateListResponse struct {
		Templates []SavedTemplateResponse `json:"templates"`
	}
)

func NewSavedTemplateResponse(template model.SavedTemplate, canDelete bool) SavedTemplateResponse {
	columns := make([]TemplateColumnResponse, 0, len(template.Columns))
	for _, column := range template.Columns {
		columns = append(columns, TemplateColumnResponse{Title: column.Title, Description: column.Description, Colour: column.Colour})
	}

	return SavedTemplateResponse{
		ID:          template.ID,
		Scope:       string(template.Scope()),
		TeamID:      template.TeamID,
		TeamName:    template.TeamName,
		Name:        template.Name,
		Description: template.Description,
		Columns:     columns,
		CreatedAt:   template.CreatedAt.UTC().Format(time.RFC3339),
		CanDelete:   canDelete,
	}
}
