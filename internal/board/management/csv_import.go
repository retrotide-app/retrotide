package management

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"slices"
	"strings"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	importColumn struct {
		title  string
		bodies []string
	}

	importPlan struct {
		columns []spec.BoardColumnSeed
		cards   []spec.CardSeed
	}
)

const (
	byteOrderMark = string(rune(0xFEFF))
)

func (s *service) ImportCSV(ctx context.Context, command ImportCommand) (*dto.CSVImportResponse, error) {
	parsed, err := parseCSV(command.CSV)
	if err != nil {
		return nil, err
	}

	var imported *dto.CSVImportResponse

	load := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanImportCards(access); err != nil {
			return nil, err
		}

		plan, err := s.planImport(ctx, access, parsed)
		if err != nil {
			return nil, err
		}

		written, err := s.boards.ImportCards(ctx, spec.CardImportWrite{BoardWrite: target, Columns: plan.columns, Cards: plan.cards})
		if errors.Is(err, dao.ErrConflict) {
			return nil, fmt.Errorf("%w: an imported column or card: %w", board.ErrIDConflict, err)
		}
		if err != nil {
			return nil, fmt.Errorf("import %d cards into board %s: %w", len(plan.cards), target.BoardID, err)
		}

		if err := s.announceImport(ctx, access, written); err != nil {
			return nil, err
		}

		imported = &dto.CSVImportResponse{
			Version:        written.Version,
			ColumnsCreated: len(written.Value.Columns),
			CardsCreated:   len(written.Value.Cards),
		}

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	response, err := s.Run(ctx, command.Command, board.VersionReplay, load)
	if err != nil {
		return nil, err
	}

	if imported == nil {
		return &dto.CSVImportResponse{Version: response.Version}, nil
	}

	return imported, nil
}

func parseCSV(raw string) ([]importColumn, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(raw, byteOrderMark)))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCSVInvalid, err)
	}

	if len(records) == 0 {
		return nil, ErrCSVEmpty
	}

	if err := checkRowWidths(records); err != nil {
		return nil, err
	}

	var columns []importColumn
	cards := 0

	for at, title := range records[0] {
		bodies, err := cellBodies(records[1:], at)
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(title) == "" && len(bodies) == 0 {
			continue
		}

		fields, err := board.CleanColumn(board.ColumnFields{Title: title})
		if err != nil {
			return nil, fmt.Errorf("column %d of the CSV: %w", at+1, err)
		}

		cards += len(bodies)
		if cards > model.CSVImportMaxCards {
			return nil, fmt.Errorf("%w: the CSV holds more than %d", ErrCSVTooManyCards, model.CSVImportMaxCards)
		}

		columns = mergeColumn(columns, fields.Title, bodies)
	}

	if cards == 0 {
		return nil, ErrCSVEmpty
	}

	return columns, nil
}

func checkRowWidths(records [][]string) error {
	width := len(records[0])

	for row, record := range records[1:] {
		if len(record) <= width {
			continue
		}

		for _, cell := range record[width:] {
			if strings.TrimSpace(cell) != "" {
				return fmt.Errorf("%w: row %d has a cell past the last column title", ErrCSVInvalid, row+2)
			}
		}
	}

	return nil
}

func cellBodies(rows [][]string, at int) ([]string, error) {
	bodies := make([]string, 0, len(rows))

	for row, record := range rows {
		if at >= len(record) || strings.TrimSpace(record[at]) == "" {
			continue
		}

		body, err := board.CleanCardBody(record[at])
		if err != nil {
			return nil, fmt.Errorf("row %d, column %d of the CSV: %w", row+2, at+1, err)
		}

		bodies = append(bodies, body)
	}

	return bodies, nil
}

func mergeColumn(columns []importColumn, title string, bodies []string) []importColumn {
	at := slices.IndexFunc(columns, func(column importColumn) bool {
		return strings.EqualFold(column.title, title)
	})
	if at < 0 {
		return append(columns, importColumn{title: title, bodies: bodies})
	}

	columns[at].bodies = append(columns[at].bodies, bodies...)

	return columns
}

func (s *service) planImport(ctx context.Context, access model.BoardAccess, parsed []importColumn) (*importPlan, error) {
	existing, err := s.columns.List(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the columns of board %s: %w", access.Header.ID, err)
	}

	lastCards, err := s.lastVisibleCards(ctx, access)
	if err != nil {
		return nil, err
	}

	lastColumn := ""
	for _, column := range existing {
		lastColumn = max(lastColumn, column.Position)
	}

	plan := &importPlan{}

	for _, column := range parsed {
		columnID, matched := matchColumn(existing, column.title)

		if matched {
			if err := policy.CanUseColumn(access, columnID); err != nil {
				return nil, err
			}
		} else {
			count := len(existing) + len(plan.columns)
			if count >= model.BoardMaxColumns {
				return nil, fmt.Errorf("%w: board %s would need more than %d", ErrCSVTooManyColumns, access.Header.ID, model.BoardMaxColumns)
			}

			position, err := nextKey(lastColumn)
			if err != nil {
				return nil, fmt.Errorf("place a new column on board %s: %w", access.Header.ID, err)
			}

			lastColumn = position
			columnID = uuid.New()

			plan.columns = append(plan.columns, spec.BoardColumnSeed{
				ID:       columnID,
				Title:    column.title,
				Colour:   model.Colours[count%len(model.Colours)],
				Position: position,
			})
		}

		for _, body := range column.bodies {
			position, err := nextKey(lastCards[columnID])
			if err != nil {
				return nil, fmt.Errorf("place an imported card in column %s of board %s: %w", columnID, access.Header.ID, err)
			}

			lastCards[columnID] = position

			plan.cards = append(plan.cards, spec.CardSeed{
				CardID:              uuid.New(),
				ColumnID:            columnID,
				AuthorParticipantID: access.ParticipantID,
				Body:                body,
				Position:            position,
			})
		}
	}

	return plan, nil
}

func (s *service) lastVisibleCards(ctx context.Context, access model.BoardAccess) (map[uuid.UUID]string, error) {
	cards, err := s.cards.List(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the cards of board %s: %w", access.Header.ID, err)
	}

	viewer := projection.ViewerFor(access)

	last := make(map[uuid.UUID]string)
	for _, card := range cards {
		if card.ParentCardID != nil || !projection.Visible(access.Header.Board, card, viewer) {
			continue
		}

		last[card.ColumnID] = max(last[card.ColumnID], card.Position)
	}

	return last, nil
}

func matchColumn(existing []model.BoardColumn, title string) (uuid.UUID, bool) {
	for _, column := range existing {
		if strings.EqualFold(strings.TrimSpace(column.Title), title) {
			return column.ID, true
		}
	}

	return uuid.Nil, false
}

func nextKey(lower string) (string, error) {
	key, err := fracindex.Between(lower, "")
	if err != nil {
		return "", err
	}

	if len(key) > fracindex.MaxKeyLength {
		return "", fmt.Errorf("%w: the key after %q is longer than %d characters", fracindex.ErrInvalidKey, lower, fracindex.MaxKeyLength)
	}

	return key, nil
}

func (s *service) announceImport(ctx context.Context, access model.BoardAccess, written *model.Versioned[model.CardImport]) error {
	author, err := s.importer(ctx, access)
	if err != nil {
		s.events.Publish(access, projection.Resync(written.Version))

		return err
	}

	events := make([]projection.Event, 0, len(written.Value.Columns)+len(written.Value.Cards))
	for _, column := range written.Value.Columns {
		events = append(events, projection.ColumnUpserted(access.Header.Board, column, written.Version))
	}

	for _, card := range written.Value.Cards {
		events = append(events, projection.CardAdded(access.Header.Board, card, author, written.Version))
	}

	s.events.Publish(access, events...)

	return nil
}

func (s *service) importer(ctx context.Context, access model.BoardAccess) (*model.BoardParticipantProfile, error) {
	if access.Header.Anonymous() || access.ParticipantID == nil {
		return nil, nil
	}

	participants, err := s.boards.ListParticipants(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the participants of board %s: %w", access.Header.ID, err)
	}

	for _, participant := range participants {
		if participant.ID == *access.ParticipantID {
			return &participant, nil
		}
	}

	return nil, nil
}
