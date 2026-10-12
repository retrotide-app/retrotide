package dto

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	BoardExport struct {
		ID           uuid.UUID          `json:"id"`
		Title        string             `json:"title"`
		Context      string             `json:"context"`
		TemplateKey  string             `json:"template_key"`
		Anonymity    model.Anonymity    `json:"anonymity"`
		Stage        model.Stage        `json:"stage"`
		CardsHidden  bool               `json:"cards_hidden"`
		Closed       bool               `json:"closed"`
		CreatedAt    string             `json:"created_at,omitempty"`
		Columns      []ExportColumn     `json:"columns"`
		OtherActions []ExportActionItem `json:"other_actions"`
	}

	ExportColumn struct {
		Title       string       `json:"title"`
		Description string       `json:"description"`
		Cards       []ExportCard `json:"cards"`
		HiddenCount int          `json:"hidden_count"`
	}

	ExportCard struct {
		Body        string             `json:"body"`
		Author      string             `json:"author,omitempty"`
		Votes       *int               `json:"votes"`
		Hidden      bool               `json:"hidden"`
		CreatedAt   string             `json:"created_at,omitempty"`
		UpdatedAt   string             `json:"updated_at,omitempty"`
		Comments    []ExportComment    `json:"comments"`
		ActionItems []ExportActionItem `json:"action_items"`
		MergedCards []ExportCard       `json:"merged_cards"`
	}

	ExportComment struct {
		Body      string `json:"body"`
		Author    string `json:"author,omitempty"`
		CreatedAt string `json:"created_at,omitempty"`
		UpdatedAt string `json:"updated_at,omitempty"`
	}

	ExportActionItem struct {
		Title    string                 `json:"title"`
		Assignee string                 `json:"assignee,omitempty"`
		DueOn    *string                `json:"due_on"`
		Status   model.ActionItemStatus `json:"status"`
	}
)
