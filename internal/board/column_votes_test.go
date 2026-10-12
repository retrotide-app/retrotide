package board

import (
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func voteLimitCommand(maxVotes *int) ColumnVoteLimitCommand {
	return ColumnVoteLimitCommand{Command: baseCommand(), ColumnID: columnID, MaxVotes: maxVotes}
}

func (m *testMocks) limitedColumn(maxVotes *int) {
	key := spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}

	m.columns.EXPECT().GetByID(mock.Anything, key).Return(&model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Went well", Colour: model.ColourGreen, Position: "V", MaxVotes: maxVotes}, nil).Once()
}

func TestSetColumnVoteLimit_RefusesALimitOutOfRangeBeforeTakingTheGate(t *testing.T) {
	for _, limit := range []int{-1, 0, 100} {
		// given
		svc, _ := newTestService(t)

		// when
		got, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(new(limit)))

		// then the strict live mock proves the gate was never taken
		assert.Nil(t, got)
		require.ErrorIs(t, err, ErrInvalidColumnVoteLimit)
	}
}

func TestSetColumnVoteLimit_SetsTheLimitAndAnnouncesTheColumn(t *testing.T) {
	// given a column with no limit of its own
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.limitedColumn(nil)
	limited := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Went well", Colour: model.ColourGreen, Position: "V", MaxVotes: new(3)}
	write := spec.ColumnVoteLimitWrite{BoardWrite: target(), ColumnID: columnID, MaxVotes: new(3)}
	m.columns.EXPECT().SetColumnVoteLimit(mock.Anything, write).Return(&model.Versioned[model.BoardColumn]{Value: limited, Version: 8}, nil)

	// when
	got, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(new(3)))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Column)
	assert.Equal(t, new(3), got.Column.MaxVotes)
	event := m.onlyEvent(t)
	upserted, ok := event.Payload(projection.ClassOthers).(dto.ColumnUpsertedEvent)
	require.True(t, ok)
	assert.Equal(t, int64(8), upserted.Version)
	assert.Equal(t, new(3), upserted.Column.MaxVotes)
}

func TestSetColumnVoteLimit_ClearsTheLimit(t *testing.T) {
	// given a column limited to two votes
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.limitedColumn(new(2))
	cleared := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Went well", Colour: model.ColourGreen, Position: "V"}
	write := spec.ColumnVoteLimitWrite{BoardWrite: target(), ColumnID: columnID}
	m.columns.EXPECT().SetColumnVoteLimit(mock.Anything, write).Return(&model.Versioned[model.BoardColumn]{Value: cleared, Version: 8}, nil)

	// when
	got, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(nil))

	// then
	require.NoError(t, err)
	assert.Nil(t, got.Column.MaxVotes)
	require.Len(t, m.events.published, 1)
}

func TestSetColumnVoteLimit_TheLimitAlreadySetIsLeftAsItIs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.limitedColumn(new(3))

	// when
	got, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(new(3)))

	// then the strict mocks prove nothing was written or announced
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.Version)
	assert.Equal(t, new(3), got.Column.MaxVotes)
	assert.Empty(t, m.events.published)
}

func TestSetColumnVoteLimit_ARetriedLimitReplaysItsVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 6)

	// when
	got, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(new(3)))

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(6, nil), got)
}

func TestSetColumnVoteLimit_IsTheFacilitatorsEvenWhenAnyoneCanEdit(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anyoneCanEdit))

	// when
	got, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(new(3)))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestSetColumnVoteLimit_AHiddenColumnsLimitReachesOnlyFacilitators(t *testing.T) {
	// given a column hidden from participants
	svc, m := newTestService(t)
	m.entered(boardAccess(host, columnHidden))
	m.limitedColumn(nil)
	limited := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Went well", MaxVotes: new(2)}
	m.columns.EXPECT().SetColumnVoteLimit(mock.Anything, mock.Anything).Return(&model.Versioned[model.BoardColumn]{Value: limited, Version: 8}, nil)

	// when
	_, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(new(2)))

	// then
	require.NoError(t, err)
	event := m.onlyEvent(t)
	assert.Equal(t, projection.ClassNone, event.ClassOf(otherViewer))
	assert.NotEqual(t, projection.ClassNone, event.ClassOf(projection.Viewer{Capabilities: policy.Capabilities(host, true)}))
}

func TestSetColumnVoteLimit_AnUnknownColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(nil, errNotFound).Once()

	// when
	got, err := svc.SetColumnVoteLimit(t.Context(), voteLimitCommand(new(3)))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrColumnNotFound)
}

func TestResetColumnVotes_RefundsTheColumnsVotesAndAnnouncesIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, openVoting))
	m.column()
	m.votes.EXPECT().ResetColumnVotes(mock.Anything, spec.ColumnVotesResetWrite{BoardWrite: target(), ColumnID: columnID}).Return(&model.Versioned[int64]{Value: 4, Version: 8}, nil)

	// when
	got, err := svc.ResetColumnVotes(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	event := m.onlyEvent(t)
	assert.Equal(t, dto.ColumnVotesResetEvent{BoardEventHead: dto.BoardEventHead{Type: projection.EventColumnVotesReset, Version: 8}, ColumnID: columnID}, event.Payload(projection.ClassOthers))
}

func TestResetColumnVotes_AHiddenColumnIsFrozen(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, columnHidden))
	m.column()

	// when
	got, err := svc.ResetColumnVotes(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then the strict vote mock proves nothing was reset
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrColumnHidden)
	assert.Empty(t, m.events.published)
}

func TestResetColumnVotes_IsRefusedToAParticipant(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.ResetColumnVotes(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}
