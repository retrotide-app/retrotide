package board

import (
	"encoding/json"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	authorViewer = projection.Viewer{Kind: projection.ViewerMember, ParticipantID: new(myID), Role: writer}
	otherViewer  = projection.Viewer{Kind: projection.ViewerMember, ParticipantID: new(theirID), Role: writer}
	hostViewer   = projection.Viewer{Kind: projection.ViewerMember, ParticipantID: new(otherID), Role: host}
)

func seenBy(t *testing.T, event projection.Event, viewer projection.Viewer) string {
	t.Helper()

	payload := event.Payload(event.ClassOf(viewer))
	if payload == nil {
		return ""
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	return string(data)
}

func (m *testMocks) onlyEvent(t *testing.T) projection.Event {
	t.Helper()

	require.Len(t, m.events.published, 1, "a committed command publishes exactly one event")

	return m.events.published[0].event
}

func TestAddCard_PublishesTheCardToItsAuthorOnlyWhileCardsAreHidden(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hiddenCards))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	added := card(myID)
	m.cards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: added, Version: 8}, nil)
	m.participants()

	// when
	_, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	require.NoError(t, err)
	event := m.onlyEvent(t)
	assert.Equal(t, int64(8), event.Version())
	assert.True(t, event.MovesHiddenCards())
	assert.Contains(t, seenBy(t, event, authorViewer), secret)
	assert.Contains(t, seenBy(t, event, authorViewer), `"is_mine":true`)
	assert.Empty(t, seenBy(t, event, otherViewer), "nobody else may see a hidden card")
	assert.Empty(t, seenBy(t, event, hostViewer), "the facilitator may not see a hidden card either")
}

func TestAddCard_PublishesTheCardToEveryoneOnceCardsAreShown(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	m.cards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: card(myID), Version: 8}, nil)
	m.participants()

	// when
	_, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	require.NoError(t, err)
	event := m.onlyEvent(t)
	assert.False(t, event.MovesHiddenCards())
	others := seenBy(t, event, otherViewer)
	assert.Contains(t, others, secret)
	assert.NotContains(t, others, "is_mine")
	assert.Contains(t, others, `"display_name":"Sam"`)
}

func TestAddCard_AnAnonymousCardCarriesNoAuthorToAnyone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anonymous))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	m.cards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: card(myID), Version: 8}, nil)

	// when
	_, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	require.NoError(t, err)
	event := m.onlyEvent(t)
	for _, viewer := range []projection.Viewer{authorViewer, otherViewer, hostViewer} {
		seen := seenBy(t, event, viewer)
		assert.NotContains(t, seen, myID.String(), "an anonymous card never names its author")
		assert.NotContains(t, seen, "created_at")
	}
}

func TestDeleteCard_PublishesTheDeletionAtTheWrittenVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.stored(card(theirID))
	m.cards.EXPECT().DeleteCard(mock.Anything, spec.CardRemoval{BoardWrite: target(), CardID: cardID}).Return(&model.Versioned[model.Card]{Value: card(theirID), Version: 8}, nil)

	// when
	_, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	require.NoError(t, err)
	event := m.onlyEvent(t)
	assert.JSONEq(t, `{"type":"card_deleted","version":8,"card_id":"`+cardID.String()+`"}`, seenBy(t, event, authorViewer))
}

func TestMoveCard_PublishesTheNewPlaceAndTheRewrittenKeys(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	moved := card(myID)
	moved.Position = "k"
	m.cards.EXPECT().MoveCard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: moved, Version: 8}, nil)
	m.participants()

	// when
	_, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	require.NoError(t, err)
	event := m.onlyEvent(t)
	assert.JSONEq(t, `{"type":"card_moved","version":8,"card_id":"`+cardID.String()+`","column_id":"`+columnID.String()+`","position":"k","rewritten":[]}`, seenBy(t, event, otherViewer))
}

func TestColumnCommands_PublishTheirEventToEveryone(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks)
		when  func(svc *service) error
		want  string
	}{
		{
			name: "delete",
			given: func(m *testMocks) {
				m.columns.EXPECT().DeleteColumn(mock.Anything, mock.Anything).Return(&model.Versioned[model.BoardColumn]{Value: model.BoardColumn{ID: columnID}, Version: 8}, nil)
			},
			when: func(svc *service) error {
				_, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})
				return err
			},
			want: `{"type":"column_deleted","version":8,"column_id":"` + columnID.String() + `"}`,
		},
		{
			name: "clear",
			given: func(m *testMocks) {
				m.columns.EXPECT().GetByID(mock.Anything, mock.Anything).Return(&model.BoardColumn{ID: columnID}, nil)
				m.cards.EXPECT().ClearCards(mock.Anything, mock.Anything).Return(&model.Versioned[int64]{Value: 3, Version: 8}, nil)
			},
			when: func(svc *service) error {
				_, err := svc.ClearColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})
				return err
			},
			want: `{"type":"column_cleared","version":8,"column_id":"` + columnID.String() + `"}`,
		},
		{
			name: "add",
			given: func(m *testMocks) {
				m.columns.EXPECT().List(mock.Anything, boardID).Return([]model.BoardColumn{}, nil)
				added := model.BoardColumn{ID: columnID, BoardID: boardID, Title: "Ideas", Colour: model.ColourGrey, Position: "V"}
				m.columns.EXPECT().AddColumn(mock.Anything, mock.Anything).Return(&model.Versioned[model.BoardColumn]{Value: added, Version: 8}, nil)
			},
			when: func(svc *service) error {
				_, err := svc.AddColumn(t.Context(), AddColumnCommand{Command: baseCommand(), ColumnID: columnID, Title: "Ideas", Colour: model.ColourGrey})
				return err
			},
			want: `{"type":"column_upserted","version":8,"column":{"id":"` + columnID.String() + `","title":"Ideas","description":"","colour":"grey","position":"V","max_votes":null,"cards_revealed":false,"hidden":false}}`,
		},
		{
			name: "describe",
			given: func(m *testMocks) {
				m.boards.EXPECT().SetBoardContext(mock.Anything, mock.Anything).Return(&model.Versioned[model.Board]{Value: model.Board{ID: boardID, Title: "Sprint 12", Context: "Payments squad"}, Version: 8}, nil)
			},
			when: func(svc *service) error {
				_, err := svc.Describe(t.Context(), DescribeCommand{Command: baseCommand(), Context: "Payments squad"})
				return err
			},
			want: `{"type":"board_updated","version":8,"title":"Sprint 12","context":"Payments squad"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(host, hiddenCards))
			tc.given(m)

			// when
			err := tc.when(svc)

			// then
			require.NoError(t, err)
			event := m.onlyEvent(t)
			for _, viewer := range []projection.Viewer{authorViewer, otherViewer, hostViewer} {
				assert.JSONEq(t, tc.want, seenBy(t, event, viewer))
			}
		})
	}
}

func TestCommands_PublishNothingWhenReplayedOrRefused(t *testing.T) {
	t.Run("replayed", func(t *testing.T) {
		// given
		svc, m := newTestService(t)
		m.applied(boardAccess(host), 5)

		// when
		_, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

		// then
		require.NoError(t, err)
		assert.Empty(t, m.events.published)
	})

	t.Run("refused", func(t *testing.T) {
		// given
		svc, m := newTestService(t)
		m.entered(boardAccess(writer))

		// when
		_, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

		// then
		require.Error(t, err)
		assert.Empty(t, m.events.published)
	})
}

func TestGetSnapshot_AnAnonymousBoardWithCardsHiddenCountsFromThePublishedTally(t *testing.T) {
	// given a live board whose last published tally is older than the rows
	svc, m := newTestService(t)
	access := boardAccess(writer, hiddenCards, anonymous)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil)
	state := &model.BoardState{
		Header: access.Header,
		Cards: []model.Card{
			{ID: firstCardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(theirID), Body: secret, Position: "V", CreatedAt: stamp},
			{ID: secondCardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(theirID), Body: secret, Position: "k", CreatedAt: stamp},
		},
	}
	m.boards.EXPECT().GetSnapshot(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(state, nil)
	m.events.tallies[boardID] = projection.HiddenTally{columnID: {theirID: 1}}

	// when
	got, err := svc.GetSnapshot(t.Context(), spec.BoardSnapshotQuery{Principal: me, BoardID: boardID})

	// then the count the board last published wins, so refetching cannot beat the tick
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]int{columnID: 1}, got.HiddenCounts)
	assert.Empty(t, got.Cards)
}
