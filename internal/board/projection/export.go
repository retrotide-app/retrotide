package projection

import (
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	exportBuilder struct {
		comments map[uuid.UUID][]dto.ExportComment
		actions  map[uuid.UUID][]dto.ExportActionItem
		stacked  map[uuid.UUID][]dto.CardView
	}
)

func Export(state model.BoardState, viewer Viewer, tally HiddenTally) dto.BoardExport {
	snapshot := Snapshot(state, viewer)

	hiddenCounts := map[uuid.UUID]int{}
	if state.Header.CardsHidden {
		hiddenCounts = tally.For(state.Header.Board, viewer)
	}

	builder := exportBuilder{
		comments: map[uuid.UUID][]dto.ExportComment{},
		actions:  map[uuid.UUID][]dto.ExportActionItem{},
		stacked:  map[uuid.UUID][]dto.CardView{},
	}

	tops := make(map[uuid.UUID]struct{}, len(snapshot.Cards))
	for _, card := range snapshot.Cards {
		if card.ParentCardID == nil {
			tops[card.ID] = struct{}{}
		}
	}

	byColumn := map[uuid.UUID][]dto.CardView{}
	for _, card := range snapshot.Cards {
		if card.ParentCardID != nil {
			if _, ok := tops[*card.ParentCardID]; ok {
				builder.stacked[*card.ParentCardID] = append(builder.stacked[*card.ParentCardID], card)
				continue
			}
		}

		byColumn[card.ColumnID] = append(byColumn[card.ColumnID], card)
	}

	for _, comment := range snapshot.Comments {
		builder.comments[comment.CardID] = append(builder.comments[comment.CardID], exportComment(comment))
	}

	others := []dto.ExportActionItem{}
	for _, item := range snapshot.ActionItems {
		if item.SourceCardID == nil {
			others = append(others, exportActionItem(item))
			continue
		}

		builder.actions[*item.SourceCardID] = append(builder.actions[*item.SourceCardID], exportActionItem(item))
	}

	columns := make([]dto.ExportColumn, 0, len(snapshot.Columns))
	for _, column := range snapshot.Columns {
		columns = append(columns, dto.ExportColumn{
			Title:       column.Title,
			Description: column.Description,
			Cards:       builder.cards(byColumn[column.ID]),
			HiddenCount: hiddenCounts[column.ID],
		})
	}

	return dto.BoardExport{
		ID:           snapshot.Board.ID,
		Title:        snapshot.Board.Title,
		Context:      snapshot.Board.Context,
		TemplateKey:  snapshot.Board.TemplateKey,
		Anonymity:    snapshot.Board.Anonymity,
		Stage:        snapshot.Board.Stage,
		CardsHidden:  snapshot.Board.CardsHidden,
		Closed:       snapshot.Board.Closed,
		CreatedAt:    snapshot.Board.CreatedAt,
		Columns:      columns,
		OtherActions: others,
	}
}

func (b exportBuilder) cards(views []dto.CardView) []dto.ExportCard {
	cards := make([]dto.ExportCard, 0, len(views))
	for _, view := range views {
		cards = append(cards, dto.ExportCard{
			Body:        view.Body,
			Author:      exportAuthor(view.Author),
			Votes:       view.Votes.Total,
			Hidden:      view.Hidden,
			CreatedAt:   view.CreatedAt,
			UpdatedAt:   view.UpdatedAt,
			Comments:    orEmpty(b.comments[view.ID]),
			ActionItems: orEmpty(b.actions[view.ID]),
			MergedCards: b.cards(b.stacked[view.ID]),
		})
	}

	return cards
}

func exportComment(view dto.CommentView) dto.ExportComment {
	return dto.ExportComment{
		Body:      view.Body,
		Author:    exportAuthor(view.Author),
		CreatedAt: view.CreatedAt,
		UpdatedAt: view.UpdatedAt,
	}
}

func exportActionItem(view dto.ActionItemView) dto.ExportActionItem {
	item := dto.ExportActionItem{
		Title:  view.Title,
		DueOn:  view.DueOn,
		Status: view.Status,
	}

	if view.Assignee != nil {
		item.Assignee = view.Assignee.DisplayName
	}

	return item
}

func exportAuthor(author *dto.AuthorView) string {
	if author == nil {
		return ""
	}

	return author.DisplayName
}

func orEmpty[T any](items []T) []T {
	if items == nil {
		return []T{}
	}

	return items
}
