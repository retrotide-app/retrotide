package management

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	wentWell  = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000021")
	toImprove = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000022")
	shipped   = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000023")
	stacked   = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000024")
)

func importCommand(csv string) ImportCommand {
	return ImportCommand{Command: baseCommand(), CSV: csv}
}

func existingColumns() []model.BoardColumn {
	return []model.BoardColumn{
		{ID: wentWell, BoardID: boardID, Title: " Went well ", Colour: model.ColourGreen, Position: "a0"},
		{ID: toImprove, BoardID: boardID, Title: "To improve", Colour: model.ColourRed, Position: "b0"},
	}
}

func existingCards() []model.Card {
	return []model.Card{
		{ID: shipped, BoardID: boardID, ColumnID: wentWell, AuthorParticipantID: new(myID), Body: "Shipped on time", Position: "a5", CreatedAt: stamp},
		{ID: stacked, BoardID: boardID, ColumnID: wentWell, ParentCardID: new(shipped), AuthorParticipantID: new(myID), Body: "Me too", Position: "y", CreatedAt: stamp},
	}
}

func imported(write spec.CardImportWrite) *model.Versioned[model.CardImport] {
	columns := make([]model.BoardColumn, 0, len(write.Columns))
	for _, seed := range write.Columns {
		columns = append(columns, model.BoardColumn{ID: seed.ID, BoardID: boardID, Title: seed.Title, Colour: seed.Colour, Position: seed.Position})
	}

	cards := make([]model.Card, 0, len(write.Cards))
	for _, seed := range write.Cards {
		cards = append(cards, model.Card{ID: seed.CardID, BoardID: boardID, ColumnID: seed.ColumnID, AuthorParticipantID: seed.AuthorParticipantID, Body: seed.Body, Position: seed.Position, CreatedAt: stamp})
	}

	return &model.Versioned[model.CardImport]{Value: model.CardImport{Columns: columns, Cards: cards}, Version: 8}
}

func (m *testMocks) importsInto(written *spec.CardImportWrite) {
	record := func(_ context.Context, write spec.CardImportWrite, _ ...*sql.Tx) (*model.Versioned[model.CardImport], error) {
		*written = write

		return imported(write), nil
	}
	m.boards.EXPECT().ImportCards(mock.Anything, mock.Anything).RunAndReturn(record).Once()
}

func TestParseCSV_ReadsTitlesFromTheFirstRowAndCardsFromTheCellsBelow(t *testing.T) {
	// given a spreadsheet export with a byte order mark, a blank cell and a trailing empty column
	raw := string(rune(0xFEFF)) + "Went well,To improve,\r\nShipped,Flaky tests,\r\n,Slow reviews,\r\n"

	// when
	columns, err := parseCSV(raw)

	// then
	require.NoError(t, err)
	assert.Equal(t, []importColumn{
		{title: "Went well", bodies: []string{"Shipped"}},
		{title: "To improve", bodies: []string{"Flaky tests", "Slow reviews"}},
	}, columns)
}

func TestParseCSV_MergesTitlesThatDifferOnlyInCase(t *testing.T) {
	// when
	columns, err := parseCSV("Kudos,kudos \nThanks Sam,Thanks Ali\n")

	// then
	require.NoError(t, err)
	assert.Equal(t, []importColumn{{title: "Kudos", bodies: []string{"Thanks Sam", "Thanks Ali"}}}, columns)
}

func TestParseCSV_KeepsQuotedCommasAndLineBreaksInACard(t *testing.T) {
	// when
	columns, err := parseCSV("Ideas\n\"Pair more, review less\"\n\"Two\nlines\"\n")

	// then
	require.NoError(t, err)
	assert.Equal(t, []importColumn{{title: "Ideas", bodies: []string{"Pair more, review less", "Two\nlines"}}}, columns)
}

func TestParseCSV_RefusesWhatItCannotImport(t *testing.T) {
	tooMany := "Ideas\n" + strings.Repeat("idea\n", model.CSVImportMaxCards+1)

	tests := []struct {
		name string
		raw  string
		want error
	}{
		{name: "nothing at all", raw: "", want: ErrCSVEmpty},
		{name: "a byte order mark alone", raw: string(rune(0xFEFF)), want: ErrCSVEmpty},
		{name: "titles but no cards", raw: "Went well,To improve\n", want: ErrCSVEmpty},
		{name: "an unterminated quote", raw: "Ideas\n\"never closed\n", want: ErrCSVInvalid},
		{name: "a cell past the last title", raw: "Ideas\nfirst,stray\n", want: ErrCSVInvalid},
		{name: "cells under a blank title", raw: "Ideas,\nfirst,second\n", want: board.ErrColumnTitleRequired},
		{name: "a title too long", raw: strings.Repeat("t", model.BoardColumnTitleMaxRunes+1) + "\nidea\n", want: board.ErrColumnTitleTooLong},
		{name: "a card too long", raw: "Ideas\n" + strings.Repeat("c", model.CardBodyMaxRunes+1) + "\n", want: board.ErrCardBodyTooLong},
		{name: "more than 500 cards", raw: tooMany, want: ErrCSVTooManyCards},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			columns, err := parseCSV(tc.raw)

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, columns)
		})
	}
}

func TestParseCSV_AcceptsExactly500Cards(t *testing.T) {
	// given
	raw := "Ideas\n" + strings.Repeat("idea\n", model.CSVImportMaxCards)

	// when
	columns, err := parseCSV(raw)

	// then
	require.NoError(t, err)
	assert.Len(t, columns[0].bodies, model.CSVImportMaxCards)
}

func TestImportCSV_AddsCardsToMatchingColumnsAndNewColumnsAfterTheLast(t *testing.T) {
	// given a named board in the Write stage
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(existingColumns(), nil)
	m.cards.EXPECT().List(mock.Anything, boardID).Return(existingCards(), nil)
	var written spec.CardImportWrite
	m.importsInto(&written)
	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return([]model.BoardParticipantProfile{
		{BoardParticipant: model.BoardParticipant{ID: myID, BoardID: boardID, DisplayName: "Sam", Role: host}},
	}, nil)

	// when
	got, err := svc.ImportCSV(t.Context(), importCommand("went well,Kudos\nShip it,Thanks Sam\n,Thanks Ali\n"))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.CSVImportResponse{Version: 8, ColumnsCreated: 1, CardsCreated: 3}, got)

	assert.Equal(t, target(), written.BoardWrite)
	require.Len(t, written.Columns, 1)
	kudos := written.Columns[0]
	assert.Equal(t, "Kudos", kudos.Title)
	assert.Equal(t, model.Colours[2], kudos.Colour)
	assert.Equal(t, "c", kudos.Position)
	assert.Equal(t, uuid.Version(4), kudos.ID.Version())

	require.Len(t, written.Cards, 3)
	assert.Equal(t, spec.CardSeed{CardID: written.Cards[0].CardID, ColumnID: wentWell, AuthorParticipantID: new(myID), Body: "Ship it", Position: "b"}, written.Cards[0])
	assert.Equal(t, spec.CardSeed{CardID: written.Cards[1].CardID, ColumnID: kudos.ID, AuthorParticipantID: new(myID), Body: "Thanks Sam", Position: "V"}, written.Cards[1])
	assert.Equal(t, spec.CardSeed{CardID: written.Cards[2].CardID, ColumnID: kudos.ID, AuthorParticipantID: new(myID), Body: "Thanks Ali", Position: "W"}, written.Cards[2])

	require.Len(t, m.events.published, 4)
	for _, published := range m.events.published {
		assert.Equal(t, int64(8), published.event.Version(), "every event carries the one version of the import")
	}

	column, ok := m.events.published[0].event.Payload(projection.ClassOthers).(dto.ColumnUpsertedEvent)
	require.True(t, ok)
	assert.Equal(t, kudos.ID, column.Column.ID)

	card, ok := m.events.published[1].event.Payload(projection.ClassOthers).(dto.CardUpsertedEvent)
	require.True(t, ok)
	assert.Equal(t, "Ship it", card.Card.Body)
	require.NotNil(t, card.Card.Author)
}

func TestImportCSV_NamesNoAuthorOnAnAnonymousBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, anonymous))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(existingColumns(), nil)
	m.cards.EXPECT().List(mock.Anything, boardID).Return(nil, nil)
	var written spec.CardImportWrite
	m.importsInto(&written)

	// when
	got, err := svc.ImportCSV(t.Context(), importCommand("To improve\nFlaky tests\n"))

	// then the participant list is never read, so no name can leak
	require.NoError(t, err)
	assert.Equal(t, &dto.CSVImportResponse{Version: 8, CardsCreated: 1}, got)
	require.Len(t, m.events.published, 1)
	card, ok := m.events.published[0].event.Payload(projection.ClassOthers).(dto.CardUpsertedEvent)
	require.True(t, ok)
	assert.Nil(t, card.Card.Author)
	assert.Equal(t, toImprove, written.Cards[0].ColumnID)
}

func TestImportCSV_RefusesMoreColumnsThanABoardHolds(t *testing.T) {
	// given a board one column short of the limit
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	columns := make([]model.BoardColumn, 0, model.BoardMaxColumns-1)
	for range model.BoardMaxColumns - 1 {
		columns = append(columns, model.BoardColumn{ID: uuid.New(), BoardID: boardID, Title: "Existing", Position: "a"})
	}
	m.columns.EXPECT().List(mock.Anything, boardID).Return(columns, nil)
	m.cards.EXPECT().List(mock.Anything, boardID).Return(nil, nil)

	// when
	_, err := svc.ImportCSV(t.Context(), importCommand("New one,New two\nfirst,second\n"))

	// then the strict mocks prove nothing was written
	require.ErrorIs(t, err, ErrCSVTooManyColumns)
}

func TestImportCSV_RefusesAColumnHiddenFromParticipants(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	access.Header.HiddenColumnIDs = []uuid.UUID{toImprove}
	m.entered(access)
	m.columns.EXPECT().List(mock.Anything, boardID).Return(existingColumns(), nil)
	m.cards.EXPECT().List(mock.Anything, boardID).Return(nil, nil)

	// when
	_, err := svc.ImportCSV(t.Context(), importCommand("To improve\nFlaky tests\n"))

	// then
	require.ErrorIs(t, err, policy.ErrColumnHidden)
}

func TestImportCSV_IsRefusedOutsideTheWriteStageAndToAParticipant(t *testing.T) {
	tests := []struct {
		name   string
		access *model.BoardAccess
		want   error
	}{
		{name: "the Vote stage", access: boardAccess(host, atStage(model.StageVote)), want: policy.ErrCardsOutOfStage},
		{name: "a participant", access: boardAccess(writer), want: policy.ErrNotAllowed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)

			// when
			got, err := svc.ImportCSV(t.Context(), importCommand("Ideas\nidea\n"))

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestImportCSV_RefusesABadFileBeforeTakingTheBoard(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	_, err := svc.ImportCSV(t.Context(), importCommand("Ideas\n"))

	// then the strict mocks prove the board was never resolved
	require.ErrorIs(t, err, ErrCSVEmpty)
}

func TestImportCSV_ReplaysAnImportItAlreadyApplied(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 5)

	// when
	got, err := svc.ImportCSV(t.Context(), importCommand("Ideas\nidea\n"))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.CSVImportResponse{Version: 5}, got)
}
