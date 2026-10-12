package projection

import (
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

func DeletedBoardList(boards []model.BoardHeader, teamNames map[uuid.UUID]string) dto.DeletedBoardListResponse {
	items := make([]dto.DeletedBoardItem, 0, len(boards))
	for _, header := range boards {
		if header.DeletedAt == nil {
			continue
		}

		item := dto.DeletedBoardItem{
			ID:           header.ID,
			TeamID:       teamOf(header.Board),
			Title:        header.Title,
			DeletedAt:    header.DeletedAt.UTC().Format(timeLayout),
			RestoreUntil: header.DeletedAt.Add(model.BoardRestoreWindow).UTC().Format(timeLayout),
		}

		if header.TeamID != nil {
			item.TeamName = teamNames[*header.TeamID]
		}

		items = append(items, item)
	}

	return dto.DeletedBoardListResponse{Boards: items}
}
