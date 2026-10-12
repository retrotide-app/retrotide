package engines

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"retrotide/internal/dto"
	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines/utils"
)

type (
	CSV struct{}
)

const (
	formulaTriggers = "=+-@\t\r"
)

func (CSV) Format() engine.Format {
	return engine.FormatCSV
}

func (CSV) ContentType() string {
	return "text/csv; charset=utf-8"
}

func (CSV) Extension() string {
	return "csv"
}

func (CSV) WriteBoard(w io.Writer, board dto.BoardExport) error {
	width := max(len(board.Columns), 1)

	header := make([]string, width)
	columns := make([][]string, 0, len(board.Columns))
	depth := 0
	for at, column := range board.Columns {
		header[at] = column.Title

		cells := columnCells(column)
		columns = append(columns, cells)
		depth = max(depth, len(cells))
	}

	rows := [][]string{header}
	for line := range depth {
		row := make([]string, width)
		for at, cells := range columns {
			if line < len(cells) {
				row[at] = cells[line]
			}
		}

		rows = append(rows, row)
	}

	if len(board.OtherActions) > 0 {
		rows = append(rows, make([]string, width), leadingCell(width, utils.OtherActionsTitle))
		for _, item := range board.OtherActions {
			rows = append(rows, leadingCell(width, csvAction(item)))
		}
	}

	writer := csv.NewWriter(w)
	for _, row := range rows {
		if err := writer.Write(defused(row)); err != nil {
			return fmt.Errorf("write a row of the CSV export: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush the CSV export: %w", err)
	}

	return nil
}

func columnCells(column dto.ExportColumn) []string {
	var cells []string
	for _, card := range column.Cards {
		cells = cardCells(cells, card, false)
	}

	if column.HiddenCount > 0 {
		cells = append(cells, utils.HiddenText(column.HiddenCount))
	}

	return cells
}

func cardCells(cells []string, card dto.ExportCard, merged bool) []string {
	cells = append(cells, csvCard(card, merged))

	for _, comment := range card.Comments {
		cells = append(cells, csvComment(comment))
	}

	for _, item := range card.ActionItems {
		cells = append(cells, csvAction(item))
	}

	for _, child := range card.MergedCards {
		cells = cardCells(cells, child, true)
	}

	return cells
}

func csvCard(card dto.ExportCard, merged bool) string {
	text := card.Body + utils.VotesText(card.Votes)

	if merged {
		text = utils.MergedPrefix + text
	}

	if card.Author != "" {
		text += " by " + card.Author
	}

	if card.Hidden {
		text += utils.HiddenMark
	}

	return text
}

func csvComment(comment dto.ExportComment) string {
	if comment.Author == "" {
		return "Comment: " + comment.Body
	}

	return "Comment by " + comment.Author + ": " + comment.Body
}

func csvAction(item dto.ExportActionItem) string {
	return utils.ActionPrefix + item.Title + " " + utils.ActionDetails(item, item.Assignee)
}

func leadingCell(width int, text string) []string {
	row := make([]string, width)
	row[0] = text

	return row
}

func defused(row []string) []string {
	safe := make([]string, 0, len(row))
	for _, cell := range row {
		if cell != "" && strings.IndexByte(formulaTriggers, cell[0]) >= 0 {
			cell = "'" + cell
		}

		safe = append(safe, cell)
	}

	return safe
}
