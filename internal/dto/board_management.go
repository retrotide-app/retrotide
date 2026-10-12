package dto

import "github.com/google/uuid"

type (
	MoveBoardRequest struct {
		BoardCommandRequest

		TeamID *uuid.UUID `json:"team_id"`
	}

	CSVImportRequest struct {
		BoardCommandRequest

		CSV string `json:"csv"`
	}

	CSVImportResponse struct {
		Version        int64 `json:"version"`
		ColumnsCreated int   `json:"columns_created"`
		CardsCreated   int   `json:"cards_created"`
	}

	BulkBoardRequest struct {
		Action   string      `json:"action"`
		BoardIDs []uuid.UUID `json:"board_ids"`
	}

	BulkBoardResult struct {
		BoardID uuid.UUID `json:"board_id"`
		OK      bool      `json:"ok"`
		Code    string    `json:"code,omitempty"`
		Error   string    `json:"error,omitempty"`
	}

	BulkBoardResponse struct {
		Results []BulkBoardResult `json:"results"`
	}

	DeletedBoardItem struct {
		ID           uuid.UUID  `json:"id"`
		TeamID       *uuid.UUID `json:"team_id,omitempty"`
		TeamName     string     `json:"team_name,omitempty"`
		Title        string     `json:"title"`
		DeletedAt    string     `json:"deleted_at"`
		RestoreUntil string     `json:"restore_until"`
	}

	DeletedBoardListResponse struct {
		Boards []DeletedBoardItem `json:"boards"`
	}
)
