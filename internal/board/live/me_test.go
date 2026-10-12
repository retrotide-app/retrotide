package live

import (
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/ws"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishMe_ReachesOnlyTheVotersOwnSockets(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)
	frame := dto.BoardMeFrame{BoardID: f.boardID, Version: 8, VotesRemaining: 5, MyVotes: map[uuid.UUID]int{uuid.New(): 1}}

	// when
	f.svc.PublishMe(f.access(cast.sam), frame)

	// then
	assert.Equal(t, []ws.Message{{Type: ws.TypeBoardMe, Data: frame}}, f.hub.direct[cast.sam.principal.ID])
	assert.Empty(t, f.hub.direct[cast.ali.principal.ID])
	assert.Empty(t, f.hub.direct[cast.host.principal.ID])
}

func TestPublishMe_SendsNothingToSomeoneWhoIsNotOnTheBoard(t *testing.T) {
	// given a seat that never joined over the socket
	f := newLiveFixture(t, model.AnonymityNamed, false)
	f.cast(t)
	absent := f.seat("Rae", teamMember)

	// when
	f.svc.PublishMe(f.access(absent), dto.BoardMeFrame{BoardID: f.boardID, Version: 8, MyVotes: map[uuid.UUID]int{}})

	// then
	assert.Empty(t, f.hub.direct[absent.principal.ID])
}

func TestFlush_AVoteWithHiddenTotalsReachesOnlyTheVoter(t *testing.T) {
	// given a revealed board whose totals are hidden, and Ali voting for Sam's card
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)
	board := f.header().Board
	board.VoteCountsHidden = true
	voted := f.secretCard(cast.sam)
	voted.VoteTotal = 4

	// when
	f.svc.Publish(f.access(cast.ali), projection.VotesChanged(board, voted, cast.ali.participantID, 8))
	f.timer().flush()

	// then
	aliFrames := f.boardFrames(t, cast.ali)
	require.Len(t, aliFrames, 1)
	assert.Equal(t, []string{projection.EventVotesChanged}, eventTypes(aliFrames[0]))
	assert.Nil(t, aliFrames[0].Events[0]["total"])
	assert.Empty(t, f.boardFrames(t, cast.sam))
	assert.Empty(t, f.boardFrames(t, cast.host))
}

func TestFlush_AVoteOnAHiddenCardReachesOnlyItsAuthor(t *testing.T) {
	// given Sam voting for his own card while cards are hidden
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	voted := f.secretCard(cast.sam)
	voted.VoteTotal = 1

	// when
	f.svc.Publish(f.access(cast.sam), projection.VotesChanged(f.header().Board, voted, cast.sam.participantID, 8))
	f.timer().flush()

	// then nobody else learns that a hidden card was voted for
	samFrames := f.boardFrames(t, cast.sam)
	require.Len(t, samFrames, 1)
	assert.Equal(t, float64(1), samFrames[0].Events[0]["total"])
	assert.Empty(t, f.boardFrames(t, cast.ali))
	assert.Empty(t, f.boardFrames(t, cast.host))
	assertNeverSees(t, f, cast.ali, voted.ID.String())
}
