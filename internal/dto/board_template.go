package dto

import (
	"retrotide/internal/model"
)

type (
	TemplateColumnResponse struct {
		Title       string       `json:"title"`
		Description string       `json:"description"`
		Colour      model.Colour `json:"colour"`
	}

	TemplateResponse struct {
		Key         string                   `json:"key"`
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		Columns     []TemplateColumnResponse `json:"columns"`
	}

	TemplateListResponse struct {
		Templates []TemplateResponse `json:"templates"`
	}
)
