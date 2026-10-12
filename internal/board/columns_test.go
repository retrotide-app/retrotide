package board

import (
	"strings"
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	firstColumnID  = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000001")
	secondColumnID = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000002")
	thirdColumnID  = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000003")
)

func boardColumns(positions ...string) []model.BoardColumn {
	ids := []uuid.UUID{firstColumnID, secondColumnID, thirdColumnID}
	columns := make([]model.BoardColumn, 0, len(positions))
	for i, position := range positions {
		columns = append(columns, model.BoardColumn{ID: ids[i], BoardID: boardID, Title: "Column", Colour: model.ColourGrey, Position: position})
	}

	return columns
}

func addColumnCommand() AddColumnCommand {
	return AddColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: "  Ideas  ", Description: " What should we try? "}
}

func TestAddColumn_AddsAColumnAfterTheLast(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(boardColumns("V", "k"), nil)
	seed := spec.BoardColumnSeed{ID: columnID, Title: "Ideas", Description: "What should we try?", Colour: model.ColourGrey, Position: "l"}
	added := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Ideas", Description: "What should we try?", Colour: model.ColourGrey, Position: "l"}
	m.columns.EXPECT().AddColumn(mock.Anything, spec.ColumnCreate{BoardWrite: target(), Column: seed}).Return(&model.Versioned[model.BoardColumn]{Value: added, Version: 8}, nil)

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Column)
	assert.Equal(t, dto.ColumnView{ID: columnID, Title: "Ideas", Description: "What should we try?", Colour: model.ColourGrey, Position: "l"}, *got.Column)
}

func TestAddColumn_TheFirstColumnOfAnEmptyBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return([]model.BoardColumn{}, nil)
	command := addColumnCommand()
	command.Colour = model.ColourTeal
	m.columns.EXPECT().AddColumn(mock.Anything, mock.MatchedBy(func(s spec.ColumnCreate) bool {
		return s.Column.Position == "V" && s.Column.Colour == model.ColourTeal
	})).Return(&model.Versioned[model.BoardColumn]{Value: model.BoardColumn{ID: columnID, Position: "V", Colour: model.ColourTeal}, Version: 8}, nil)

	// when
	got, err := svc.AddColumn(t.Context(), command)

	// then
	require.NoError(t, err)
	assert.Equal(t, "V", got.Column.Position)
}

func TestAddColumn_RefusesAColumnIDThatIsNotV4(t *testing.T) {
	for _, id := range []uuid.UUID{uuid.Nil, v7ID} {
		// given
		svc, _ := newTestService(t)
		command := addColumnCommand()
		command.ColumnID = id

		// when
		got, err := svc.AddColumn(t.Context(), command)

		// then
		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrInvalidID)
	}
}

func TestColumnFields_AreCheckedBeforeTheGate(t *testing.T) {
	cases := []struct {
		name   string
		fields ColumnFields
		want   error
	}{
		{name: "a blank title", fields: ColumnFields{Title: "  "}, want: ErrColumnTitleRequired},
		{name: "a title over 80 characters", fields: ColumnFields{Title: strings.Repeat("é", 81)}, want: ErrColumnTitleTooLong},
		{name: "a description over 200 characters", fields: ColumnFields{Title: "Ideas", Description: strings.Repeat("a", 201)}, want: ErrColumnDescriptionTooLong},
		{name: "a colour outside the palette", fields: ColumnFields{Title: "Ideas", Colour: "orange"}, want: ErrInvalidColour},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			added, addErr := svc.AddColumn(t.Context(), AddColumnCommand{Command: baseCommand(), ColumnID: columnID, ColumnFields: tc.fields})
			edited, editErr := svc.EditColumn(t.Context(), EditColumnCommand{Command: baseCommand(), ColumnID: columnID, ColumnFields: tc.fields})

			// then
			assert.Nil(t, added)
			assert.ErrorIs(t, addErr, tc.want)
			assert.Nil(t, edited)
			assert.ErrorIs(t, editErr, tc.want)
		})
	}
}

func TestAddColumn_AParticipantMayNot(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestAddColumn_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(nil, errDatabase)

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestAddColumn_AStoredKeyItCannotOrderIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(boardColumns("not-a-key"), nil)

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, fracindex.ErrInvalidKey)
}

func TestAddColumn_AReusedIDIsAConflict(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return([]model.BoardColumn{}, nil)
	m.columns.EXPECT().AddColumn(mock.Anything, mock.Anything).Return(nil, dao.ErrConflict)

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrIDConflict)
}

func TestAddColumn_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return([]model.BoardColumn{}, nil)
	m.columns.EXPECT().AddColumn(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, ErrIDConflict)
}

func TestAddColumn_AReplayAnswersOnlyTheIDAndVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 5)

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(5, &columnID), got)
}

func TestEditColumn_RenamesDescribesAndRecolours(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	edited := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Kudos", Description: "Who helped?", Colour: model.ColourPink, Position: "V"}
	m.columns.EXPECT().EditColumn(mock.Anything, spec.ColumnEdit{BoardWrite: target(), ColumnID: columnID, Title: "Kudos", Description: "Who helped?", Colour: model.ColourPink}).Return(&model.Versioned[model.BoardColumn]{Value: edited, Version: 8}, nil)

	// when
	got, err := svc.EditColumn(t.Context(), EditColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: " Kudos ", Description: "Who helped?", Colour: model.ColourPink})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	assert.Equal(t, dto.ColumnView{ID: columnID, Title: "Kudos", Description: "Who helped?", Colour: model.ColourPink, Position: "V"}, *got.Column)
}

func TestEditColumn_AParticipantMayNot(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.EditColumn(t.Context(), EditColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: "Kudos"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestEditColumn_AnUnknownColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().EditColumn(mock.Anything, mock.Anything).Return(nil, errNotFound)

	// when
	got, err := svc.EditColumn(t.Context(), EditColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: "Kudos"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrColumnNotFound)
}

func TestEditColumn_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().EditColumn(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.EditColumn(t.Context(), EditColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: "Kudos"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, ErrColumnNotFound)
}

func TestMoveColumn_PlacesTheColumnAfterItsAnchor(t *testing.T) {
	cases := []struct {
		name     string
		moving   uuid.UUID
		after    *uuid.UUID
		position string
	}{
		{name: "to the front", moving: thirdColumnID, after: nil, position: "U"},
		{name: "between two columns", moving: thirdColumnID, after: new(firstColumnID), position: "c"},
		{name: "to the end", moving: firstColumnID, after: new(thirdColumnID), position: "u"},
		{name: "to the end when the anchor is gone", moving: firstColumnID, after: new(otherID), position: "u"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(host))
			m.columns.EXPECT().List(mock.Anything, boardID).Return(boardColumns("V", "k", "t"), nil)
			moved := model.BoardColumn{ID: tc.moving, BoardID: boardID, Title: "Column", Colour: model.ColourGrey, Position: tc.position}
			m.columns.EXPECT().MoveColumn(mock.Anything, spec.ColumnPlacement{BoardWrite: target(), ColumnID: tc.moving, Position: tc.position}).Return(&model.Versioned[model.BoardColumn]{Value: moved, Version: 8}, nil)

			// when
			got, err := svc.MoveColumn(t.Context(), MoveColumnCommand{Command: baseCommand(), ColumnID: tc.moving, AfterColumnID: tc.after})

			// then
			require.NoError(t, err)
			assert.Equal(t, int64(8), got.Version)
			assert.Equal(t, tc.position, got.Column.Position)
		})
	}
}

func TestMoveColumn_RewritesTheOrderWhenNoKeyFits(t *testing.T) {
	// given two columns that tie on one key, so nothing fits between them
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(boardColumns("V", "V", "k"), nil)
	placing := spec.ColumnPlacement{
		BoardWrite: target(),
		ColumnID:   thirdColumnID,
		Position:   "V",
		Rewrite:    []spec.PositionChange{{ID: firstColumnID, Position: "F"}, {ID: secondColumnID, Position: "k"}},
	}
	m.columns.EXPECT().MoveColumn(mock.Anything, placing).Return(&model.Versioned[model.BoardColumn]{Value: model.BoardColumn{ID: thirdColumnID, Position: "V"}, Version: 8}, nil)

	// when
	got, err := svc.MoveColumn(t.Context(), MoveColumnCommand{Command: baseCommand(), ColumnID: thirdColumnID, AfterColumnID: new(firstColumnID)})

	// then
	require.NoError(t, err)
	assert.Equal(t, "V", got.Column.Position)
}

func TestMoveColumn_AnUnknownColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(boardColumns("V", "k"), nil)

	// when
	got, err := svc.MoveColumn(t.Context(), MoveColumnCommand{Command: baseCommand(), ColumnID: otherID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrColumnNotFound)
}

func TestMoveColumn_AParticipantMayNot(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.MoveColumn(t.Context(), MoveColumnCommand{Command: baseCommand(), ColumnID: firstColumnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestMoveColumn_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(nil, errDatabase)

	// when
	got, err := svc.MoveColumn(t.Context(), MoveColumnCommand{Command: baseCommand(), ColumnID: firstColumnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestMoveColumn_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(boardColumns("V", "k"), nil)
	m.columns.EXPECT().MoveColumn(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.MoveColumn(t.Context(), MoveColumnCommand{Command: baseCommand(), ColumnID: firstColumnID, AfterColumnID: new(secondColumnID)})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestDeleteColumn_DeletesItAndItsCards(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().DeleteColumn(mock.Anything, spec.ColumnRemoval{BoardWrite: target(), ColumnID: columnID}).Return(&model.Versioned[model.BoardColumn]{Value: model.BoardColumn{ID: columnID}, Version: 8}, nil)

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestDeleteColumn_AParticipantMayNot(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestDeleteColumn_AnUnknownColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().DeleteColumn(mock.Anything, mock.Anything).Return(nil, errNotFound)

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrColumnNotFound)
}

func TestDeleteColumn_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().DeleteColumn(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, ErrColumnNotFound)
}

func TestClearColumn_DeletesEveryCardInTheColumn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(&model.BoardColumn{ID: columnID}, nil)
	m.cards.EXPECT().ClearCards(mock.Anything, spec.ColumnRemoval{BoardWrite: target(), ColumnID: columnID}).Return(&model.Versioned[int64]{Value: 4, Version: 8}, nil)

	// when
	got, err := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then the answer says nothing about how many hidden cards went
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestClearColumn_AParticipantMayNot(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestClearColumn_AnUnknownColumnIsNotFoundAndBurnsNoVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(nil, errNotFound)

	// when
	got, err := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then the strict mocks prove nothing was cleared
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrColumnNotFound)
}

func TestClearColumn_AFailedColumnReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(nil, errDatabase)

	// when
	got, err := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, ErrColumnNotFound)
}

func TestClearColumn_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(&model.BoardColumn{ID: columnID}, nil)
	m.cards.EXPECT().ClearCards(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func anyoneCanEdit(access *model.BoardAccess) {
	access.Header.AnyoneCanEdit = true
}

func columnHidden(access *model.BoardAccess) {
	access.Header.HiddenColumnIDs = []uuid.UUID{columnID}
}

func TestAddColumn_ASeatedParticipantMayWhenAnyoneCanEdit(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anyoneCanEdit))
	m.columns.EXPECT().List(mock.Anything, boardID).Return(boardColumns("V"), nil)
	added := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Ideas", Colour: model.ColourGrey, Position: "W"}
	m.columns.EXPECT().AddColumn(mock.Anything, mock.Anything).Return(&model.Versioned[model.BoardColumn]{Value: added, Version: 8}, nil)

	// when
	got, err := svc.AddColumn(t.Context(), addColumnCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.Len(t, m.events.published, 1)
}

func TestColumnRemovals_StayWithFacilitatorsWhenAnyoneCanEdit(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anyoneCanEdit))
	m.entered(boardAccess(writer, anyoneCanEdit))

	// when
	deleted, deleteErr := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})
	cleared, clearErr := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, deleted)
	assert.Nil(t, cleared)
	assert.ErrorIs(t, deleteErr, policy.ErrNotAllowed)
	assert.ErrorIs(t, clearErr, policy.ErrNotAllowed)
}

func TestEditColumn_AParticipantCannotFindAHiddenColumnEvenWhenAnyoneCanEdit(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anyoneCanEdit, columnHidden))
	m.entered(boardAccess(writer, anyoneCanEdit, columnHidden))

	// when
	edited, editErr := svc.EditColumn(t.Context(), EditColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: "Kudos"})
	moved, moveErr := svc.MoveColumn(t.Context(), MoveColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then the strict mocks prove nothing was written, and the column's existence is never confirmed
	assert.Nil(t, edited)
	assert.Nil(t, moved)
	assert.ErrorIs(t, editErr, ErrColumnNotFound)
	assert.ErrorIs(t, moveErr, ErrColumnNotFound)
}

func TestEditColumn_AFacilitatorEditsAHiddenColumnAndOnlyFacilitatorsHear(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, columnHidden))
	edited := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Kudos", Colour: model.ColourPink, Position: "V"}
	m.columns.EXPECT().EditColumn(mock.Anything, mock.Anything).Return(&model.Versioned[model.BoardColumn]{Value: edited, Version: 8}, nil)

	// when
	got, err := svc.EditColumn(t.Context(), EditColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: "Kudos", Colour: model.ColourPink})

	// then
	require.NoError(t, err)
	assert.True(t, got.Column.Hidden)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	assert.Equal(t, projection.ClassNone, event.ClassOf(projection.Viewer{ParticipantID: new(theirID), Capabilities: policy.Capabilities(writer, true)}))
	assert.Equal(t, projection.ClassOthers, event.ClassOf(projection.Viewer{ParticipantID: new(theirID), Capabilities: policy.Capabilities(host, true)}))
}

func TestClearColumn_AFacilitatorClearsAHiddenColumnAndOnlyFacilitatorsHear(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, columnHidden))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(&model.BoardColumn{ID: columnID}, nil)
	m.cards.EXPECT().ClearCards(mock.Anything, spec.ColumnRemoval{BoardWrite: target(), ColumnID: columnID}).Return(&model.Versioned[int64]{Value: 2, Version: 8}, nil)

	// when a facilitator clears the hidden column
	got, err := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, projection.ClassNone, m.events.published[0].event.ClassOf(projection.Viewer{ParticipantID: new(theirID), Capabilities: policy.Capabilities(writer, true)}))
}
