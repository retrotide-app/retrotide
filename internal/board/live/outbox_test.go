package live

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/ws"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	boardCast struct {
		sam  seat
		ali  seat
		host seat
	}
)

func (f *liveFixture) cast(t *testing.T) boardCast {
	t.Helper()

	cast := boardCast{
		sam:  f.seat("Sam", teamMember),
		ali:  f.seat("Ali", teamMember),
		host: f.seat("Hana", teamLead),
	}

	f.join(t, cast.sam)
	f.join(t, cast.ali)
	f.join(t, cast.host)

	return cast
}

func (c boardCast) everyone() []seat {
	return []seat{c.sam, c.ali, c.host}
}

func assertNeverSees(t *testing.T, f *liveFixture, seated seat, secrets ...string) {
	t.Helper()

	for _, raw := range f.hub.framesOf(seated.principal.ID) {
		for _, hidden := range secrets {
			assert.NotContains(t, raw, hidden)
		}
	}
}

func TestFlush_AHiddenCardOnANamedBoardReachesItsAuthorAndCountsForEveryoneElse(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, true)
	cast := f.cast(t)
	card := f.secretCard(cast.sam)

	// when
	f.svc.Publish(f.access(cast.sam), projection.CardAdded(f.header().Board, card, nil, 8))
	f.timer().flush()

	// then the author gets the card itself
	samFrames := f.boardFrames(t, cast.sam)
	require.Len(t, samFrames, 1)
	assert.Equal(t, int64(8), samFrames[0].From)
	assert.Equal(t, int64(8), samFrames[0].To)
	assert.Equal(t, f.boardID, samFrames[0].BoardID)
	assert.Equal(t, []string{projection.EventCardUpserted}, eventTypes(samFrames[0]))
	assert.Equal(t, secretBody, samFrames[0].Events[0]["card"].(map[string]any)["body"])

	for _, other := range []seat{cast.ali, cast.host} {
		frames := f.boardFrames(t, other)
		require.Len(t, frames, 1, "everyone else, the facilitator included, gets only a count at the same version")
		assert.Equal(t, int64(8), frames[0].From)
		assert.Equal(t, int64(8), frames[0].To)
		assert.Equal(t, []map[string]any{{"type": "hidden_counts", "version": float64(8), "hidden_counts": map[string]any{liveColumnID.String(): float64(1)}}}, frames[0].Events)
		assertNeverSees(t, f, other, secretBody, card.ID.String(), cast.sam.participantID.String(), `"a0V"`)
	}
}

func TestFlush_AHiddenCardOnAnAnonymousBoardWaitsForTheTick(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	card := f.secretCard(cast.sam)

	// when the card is saved
	f.svc.Publish(f.access(cast.sam), projection.CardAdded(f.header().Board, card, nil, 8))
	f.timer().flush()

	// then only its author hears of it, and the published tally has not moved
	require.Len(t, f.boardFrames(t, cast.sam), 1)
	assert.Empty(t, f.hub.framesOf(cast.ali.principal.ID), "nobody is sent a frame when someone else saves a hidden card")
	assert.Empty(t, f.hub.framesOf(cast.host.principal.ID))
	published, ok := f.svc.PublishedTally(f.boardID)
	require.True(t, ok)
	assert.Empty(t, published, "a refetch cannot beat the tick")

	// when the tick runs
	f.stored = []model.Card{card}
	f.header().Version = 8
	f.boards.EXPECT().BumpVersion(mock.Anything, spec.BoardWrite{OrgID: f.header().OrgID, BoardID: f.boardID, Version: 8}).Return(9, nil).Once()
	f.timer().tick()
	f.timer().flush()

	// then the others get their new counts at a version of their own
	for _, other := range []seat{cast.ali, cast.host} {
		frames := f.boardFrames(t, other)
		require.Len(t, frames, 1)
		assert.Equal(t, int64(8), frames[0].From, "the frame covers the version they were not shown")
		assert.Equal(t, int64(9), frames[0].To)
		assert.Equal(t, []map[string]any{{"type": "hidden_counts", "version": float64(9), "hidden_counts": map[string]any{liveColumnID.String(): float64(1)}}}, frames[0].Events)
		assertNeverSees(t, f, other, secretBody, card.ID.String(), cast.sam.participantID.String())
	}

	assert.Len(t, f.boardFrames(t, cast.sam), 1, "the author's own counts did not change")
	published, ok = f.svc.PublishedTally(f.boardID)
	require.True(t, ok)
	assert.Equal(t, projection.HiddenTally{liveColumnID: {cast.sam.participantID: 1}}, published)
}

func TestTick_AnUnchangedTallyBumpsNoVersion(t *testing.T) {
	// given a card saved and deleted again inside one tick
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	card := f.secretCard(cast.sam)
	f.svc.Publish(f.access(cast.sam), projection.CardAdded(f.header().Board, card, nil, 8), projection.CardDeleted(f.header().Board, card, 9))
	f.timer().flush()

	// when
	f.timer().tick()
	f.timer().flush()

	// then nobody else is sent anything and BumpVersion was never called
	assert.Empty(t, f.hub.framesOf(cast.ali.principal.ID))
	assert.Empty(t, f.hub.framesOf(cast.host.principal.ID))
}

func TestTick_AFailedReadIsRetriedOnTheNextTick(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	f.svc.Publish(f.access(cast.sam), projection.CardAdded(f.header().Board, f.secretCard(cast.sam), nil, 8))
	f.headerErr = errDatabaseDown

	// when
	f.timer().tick()

	// then the counts are still due
	f.svc.mu.Lock()
	defer f.svc.mu.Unlock()
	assert.True(t, f.svc.live[f.boardID].countsDue)
	assert.True(t, f.svc.live[f.boardID].tickDue)
}

func TestFlush_ARevealedCardReachesEveryoneWithIsMineOnlyForItsAuthor(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		t.Run(string(anonymity), func(t *testing.T) {
			// given
			f := newLiveFixture(t, anonymity, false)
			cast := f.cast(t)
			card := f.secretCard(cast.sam)
			author := &model.BoardParticipantProfile{BoardParticipant: *f.rows[cast.sam.principal.ID], AccountName: "Sam"}
			if anonymity == model.AnonymityAnonymous {
				author = nil
			}

			// when
			f.svc.Publish(f.access(cast.sam), projection.CardAdded(f.header().Board, card, author, 8))
			f.timer().flush()

			// then
			for _, seated := range cast.everyone() {
				frames := f.boardFrames(t, seated)
				require.Len(t, frames, 1)
				cardObject := frames[0].Events[0]["card"].(map[string]any)
				assert.Equal(t, secretBody, cardObject["body"])
				_, mine := cardObject["is_mine"]
				assert.Equal(t, seated.principal == cast.sam.principal, mine)

				if anonymity == model.AnonymityAnonymous {
					assert.Nil(t, cardObject["author"])
					assert.NotContains(t, cardObject, "created_at")
					assertNeverSees(t, f, seated, cast.sam.participantID.String())
				}
			}
		})
	}
}

func TestFlush_ViewersInOneClassShareOneMarshalledFrame(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)

	// when
	f.svc.Publish(f.access(cast.sam), projection.CardAdded(f.header().Board, f.secretCard(cast.sam), nil, 8))
	f.timer().flush()

	// then one frame for the author and one for everyone else
	assert.Equal(t, []int{2}, f.hub.payloads)
	assert.Equal(t, f.hub.framesOf(cast.ali.principal.ID), f.hub.framesOf(cast.host.principal.ID))
	assert.NotEqual(t, f.hub.framesOf(cast.sam.principal.ID), f.hub.framesOf(cast.ali.principal.ID))
}

func TestFlush_FramesCoverEveryVersionOnceAndInOrder(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, true)
	cast := f.cast(t)
	board := f.header().Board
	card := f.secretCard(cast.sam)
	edited := card
	edited.Body = "SECRET-EDIT-77aa"

	// when a public change, then an edit only its author may see, then another public change
	f.svc.Publish(f.access(cast.host), projection.ColumnCleared(f.header().Board, uuid.New(), 8))
	f.timer().flush()
	f.svc.Publish(f.access(cast.sam), projection.CardChanged(board, edited, nil, 9))
	f.timer().flush()
	f.svc.Publish(f.access(cast.host), projection.ColumnCleared(f.header().Board, uuid.New(), 10))
	f.timer().flush()

	// then the author is sent every version, and the others skip 9 without a gap
	sam := f.boardFrames(t, cast.sam)
	require.Len(t, sam, 3)
	assert.Equal(t, [][2]int64{{8, 8}, {9, 9}, {10, 10}}, [][2]int64{{sam[0].From, sam[0].To}, {sam[1].From, sam[1].To}, {sam[2].From, sam[2].To}})

	ali := f.boardFrames(t, cast.ali)
	require.Len(t, ali, 2)
	assert.Equal(t, [][2]int64{{8, 8}, {9, 10}}, [][2]int64{{ali[0].From, ali[0].To}, {ali[1].From, ali[1].To}})
	assert.Equal(t, []string{projection.EventColumnCleared}, eventTypes(ali[1]))
	assertNeverSees(t, f, cast.ali, "SECRET-EDIT-77aa")
	assertNeverSees(t, f, cast.host, "SECRET-EDIT-77aa")
}

func TestFlush_OneWindowBatchesEveryEventIntoOneFrame(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)

	// when
	f.svc.Publish(f.access(cast.host), projection.ColumnCleared(f.header().Board, uuid.New(), 8))
	f.svc.Publish(f.access(cast.host), projection.ColumnDeleted(f.header().Board, uuid.New(), 9))
	f.svc.Publish(f.access(cast.host), projection.Resync(10))
	f.timer().flush()

	// then
	frames := f.boardFrames(t, cast.ali)
	require.Len(t, frames, 1)
	assert.Equal(t, int64(8), frames[0].From)
	assert.Equal(t, int64(10), frames[0].To)
	assert.Equal(t, []string{projection.EventColumnCleared, projection.EventColumnDeleted, projection.EventResync}, eventTypes(frames[0]))
}

func TestFlush_AViewerWhoJoinsAfterAnEventSkipsIt(t *testing.T) {
	// given an event published before Ali joined at its version
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	ali := f.seat("Ali", teamMember)
	f.join(t, sam)
	f.svc.Publish(f.access(sam), projection.ColumnCleared(f.header().Board, liveColumnID, 8))
	f.header().Version = 8
	f.join(t, ali)

	// when
	f.timer().flush()

	// then the snapshot Ali reads after joining already holds it
	assert.Len(t, f.boardFrames(t, sam), 1)
	assert.Empty(t, f.hub.framesOf(ali.principal.ID))
	assert.Equal(t, int64(8), ali.conn.sent[0].Data.(ws.BoardJoined).Version)
}

func TestFlush_ASocketOnlyReceivesTheBoardsItsPrincipalJoined(t *testing.T) {
	// given Ali on one board and Sam on another
	f := newLiveFixture(t, model.AnonymityNamed, false)
	otherBoard := f.addBoard(model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	ali := f.seat("Ali", teamMember)
	f.join(t, ali)
	joined, err := f.svc.HandleJoin(t.Context(), sam.conn, otherBoard)
	require.NoError(t, err)
	require.True(t, joined, "Sam watches only the other board")

	// when
	f.svc.Publish(f.access(ali), projection.ColumnCleared(f.header().Board, liveColumnID, 8))
	f.timer().flush()

	// then
	assert.Len(t, f.hub.framesOf(ali.principal.ID), 1)
	assert.Empty(t, f.hub.framesOf(sam.principal.ID))
}

func TestPublish_AnUnwatchedBoardQueuesNothing(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)

	// when
	f.svc.Publish(f.access(sam), projection.ColumnCleared(f.header().Board, liveColumnID, 8))
	f.timer().flush()

	// then
	assert.False(t, f.svc.watched(f.boardID))
	assert.Empty(t, f.hub.payloads)
}

func TestPresence_ListsWhoIsOnTheBoardAndHidesActivityWhileAnAnonymousBoardHidesCards(t *testing.T) {
	tests := []struct {
		anonymity model.Anonymity
		hidden    bool
		wantState bool
	}{
		{anonymity: model.AnonymityNamed, hidden: true, wantState: true},
		{anonymity: model.AnonymityAnonymous, hidden: false, wantState: true},
		{anonymity: model.AnonymityAnonymous, hidden: true, wantState: false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s/hidden=%t", tc.anonymity, tc.hidden), func(t *testing.T) {
			// given
			f := newLiveFixture(t, tc.anonymity, tc.hidden)
			cast := f.cast(t)
			require.NoError(t, f.svc.HandleViewerState(t.Context(), cast.ali.conn, ws.ViewerStateChange{BoardID: f.boardID, State: ws.ViewerStateIdle}))

			// when
			f.timer().presence()

			// then
			for _, seated := range cast.everyone() {
				require.Len(t, f.hub.lossy[seated.principal.ID], 1)

				var envelope frame
				require.NoError(t, json.Unmarshal([]byte(f.hub.lossy[seated.principal.ID][0]), &envelope))
				assert.Equal(t, ws.TypeBoardPresence, envelope.Type)

				var presence struct {
					BoardID uuid.UUID        `json:"board_id"`
					Viewers []map[string]any `json:"viewers"`
				}
				require.NoError(t, json.Unmarshal(envelope.Data, &presence))
				assert.Equal(t, f.boardID, presence.BoardID)
				require.Len(t, presence.Viewers, 3)

				names := make([]string, 0, len(presence.Viewers))
				for _, entry := range presence.Viewers {
					names = append(names, entry["display_name"].(string))
					_, hasState := entry["state"]
					assert.Equal(t, tc.wantState, hasState)
				}
				assert.Equal(t, []string{"Ali", "Hana", "Sam"}, names)
			}
		})
	}
}

func TestJoinAsMember_AnnouncesTheNewParticipantToTheLiveBoard(t *testing.T) {
	// given Sam already watching, and Ali watching before taking a seat
	f := newLiveFixture(t, model.AnonymityNamed, false)
	sam := f.seat("Sam", teamMember)
	ali := f.seat("Ali", teamMember)
	aliRow := f.rows[ali.principal.ID]
	delete(f.rows, ali.principal.ID)
	f.join(t, sam)
	f.join(t, ali)
	require.Nil(t, f.viewerOf(ali).access.ParticipantID)

	f.boards.EXPECT().JoinParticipant(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ spec.BoardParticipantJoin, _ ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
		f.rows[ali.principal.ID] = aliRow

		return &model.Versioned[model.BoardParticipant]{Value: *aliRow, Version: 8}, nil
	}).Once()

	// when
	access, err := f.svc.JoinAsMember(t.Context(), spec.BoardAccessQuery{Principal: ali.principal, BoardID: f.boardID})
	f.timer().flush()

	// then
	require.NoError(t, err)
	assert.Equal(t, aliRow.ID, *access.ParticipantID)
	assert.Equal(t, aliRow.ID, *f.viewerOf(ali).access.ParticipantID, "the roster seats Ali before the next event is projected")

	frames := f.boardFrames(t, sam)
	require.Len(t, frames, 1)
	assert.Equal(t, []map[string]any{{
		"type":        "participant_upserted",
		"version":     float64(8),
		"participant": map[string]any{"participant_id": aliRow.ID.String(), "display_name": "Ali", "role": "participant"},
	}}, frames[0].Events)
}

func TestDrain_FlushesEveryOutboxAndStopsScheduling(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)
	f.svc.Publish(f.access(cast.host), projection.ColumnCleared(f.header().Board, liveColumnID, 8))

	// when
	err := f.svc.Drain(t.Context())

	// then
	require.NoError(t, err)
	for _, seated := range cast.everyone() {
		assert.Len(t, f.hub.framesOf(seated.principal.ID), 1)
	}

	f.svc.Publish(f.access(cast.host), projection.ColumnCleared(f.header().Board, liveColumnID, 9))
	f.svc.mu.Lock()
	defer f.svc.mu.Unlock()
	assert.False(t, f.svc.live[f.boardID].flushDue, "nothing new is scheduled once draining")
}

func TestFlush_EveryFrameIsTypedBoardEventsAndCarriesItsBoard(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)

	// when
	f.svc.Publish(f.access(cast.host), projection.ColumnCleared(f.header().Board, liveColumnID, 8))
	f.timer().flush()

	// then
	raw := f.hub.framesOf(cast.ali.principal.ID)
	require.Len(t, raw, 1)
	assert.True(t, strings.HasPrefix(raw[0], `{"type":"board_events","data":{"board_id":"`+f.boardID.String()+`","from":8,"to":8,"events":[`))
}

func TestFlush_AnEventInAHiddenColumnReachesOnlyFacilitators(t *testing.T) {
	// given a board showing its cards, with the live column hidden from participants
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)
	f.header().HiddenColumnIDs = []uuid.UUID{liveColumnID}
	card := f.secretCard(cast.sam)

	// when Sam's card in the hidden column changes, then a public change follows
	f.svc.Publish(f.access(cast.host), projection.CardChanged(f.header().Board, card, nil, 8))
	f.svc.Publish(f.access(cast.host), projection.Resync(9))
	f.timer().flush()

	// then the facilitator reads the card, and everyone else, its author included, skips its version
	host := f.boardFrames(t, cast.host)
	require.Len(t, host, 1)
	assert.Equal(t, []string{projection.EventCardUpserted, projection.EventResync}, eventTypes(host[0]))

	for _, other := range []seat{cast.sam, cast.ali} {
		frames := f.boardFrames(t, other)
		require.Len(t, frames, 1)
		assert.Equal(t, int64(8), frames[0].From)
		assert.Equal(t, int64(9), frames[0].To)
		assert.Equal(t, []string{projection.EventResync}, eventTypes(frames[0]))
		assertNeverSees(t, f, other, secretBody, card.ID.String(), liveColumnID.String())
	}
}

func TestFlush_FacilitatorsGetTheStaffPayloadAndEveryoneElseTheFilteredOne(t *testing.T) {
	// given a hidden column beside the live one
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)
	hiddenColumn := uuid.New()
	f.header().HiddenColumnIDs = []uuid.UUID{hiddenColumn}
	moved := model.BoardColumn{ID: liveColumnID, BoardID: f.boardID, Position: "a5"}
	rewritten := []spec.PositionChange{{ID: liveColumnID, Position: "a5"}, {ID: hiddenColumn, Position: "a6"}}

	// when a move rewrites both columns' keys
	f.svc.Publish(f.access(cast.host), projection.ColumnMoved(f.header().Board, moved, rewritten, 8))
	f.timer().flush()

	// then
	host := f.boardFrames(t, cast.host)
	require.Len(t, host, 1)
	assert.Len(t, host[0].Events[0]["rewritten"], 2)
	assert.Contains(t, f.hub.framesOf(cast.host.principal.ID)[0], hiddenColumn.String())

	for _, other := range []seat{cast.sam, cast.ali} {
		frames := f.boardFrames(t, other)
		require.Len(t, frames, 1)
		assert.Equal(t, []any{map[string]any{"id": liveColumnID.String(), "position": "a5"}}, frames[0].Events[0]["rewritten"])
		assertNeverSees(t, f, other, hiddenColumn.String())
	}
}
