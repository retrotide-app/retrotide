package live

import (
	"testing"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/ws"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (f *liveFixture) cardOf(author seat, id uuid.UUID, column uuid.UUID) model.Card {
	card := f.secretCard(author)
	card.ID = id
	card.ColumnID = column
	card.Body = secretBody + "-" + id.String()[:4]

	return card
}

func (f *liveFixture) presenceDue() bool {
	f.svc.mu.Lock()
	defer f.svc.mu.Unlock()

	return f.svc.live[f.boardID].presenceDue
}

func TestHide_SendsEveryViewerTheirOwnCountsAtOnceEvenOnAnAnonymousBoard(t *testing.T) {
	// given a shown anonymous board with a card from Sam and two from Ali
	f := newLiveFixture(t, model.AnonymityAnonymous, false)
	cast := f.cast(t)
	otherColumn := uuid.New()
	cards := []model.Card{
		f.cardOf(cast.sam, uuid.New(), liveColumnID),
		f.cardOf(cast.ali, uuid.New(), liveColumnID),
		f.cardOf(cast.ali, uuid.New(), otherColumn),
	}

	// when the facilitator hides the cards again
	f.header().CardsHidden = true
	f.svc.Publish(f.access(cast.host), projection.CardsHidden(f.header().Board, cards, 8))
	f.timer().flush()

	// then each viewer is told at once to drop everyone else's cards, with counts that leave out their own
	want := map[seat]map[string]any{
		cast.sam:  {liveColumnID.String(): float64(1), otherColumn.String(): float64(1)},
		cast.ali:  {liveColumnID.String(): float64(1)},
		cast.host: {liveColumnID.String(): float64(2), otherColumn.String(): float64(1)},
	}
	for seated, counts := range want {
		frames := f.boardFrames(t, seated)
		require.Len(t, frames, 1)
		assert.Equal(t, int64(8), frames[0].To)
		assert.Equal(t, []map[string]any{{"type": projection.EventCardsHidden, "version": float64(8), "hidden_counts": counts}}, frames[0].Events)
		assertNeverSees(t, f, seated, secretBody, cast.sam.participantID.String(), cast.ali.participantID.String())
	}

	published, ok := f.svc.PublishedTally(f.boardID)
	require.True(t, ok, "a snapshot taken now carries the counts just sent")
	assert.Equal(t, projection.Tally(f.header().Board, cards), published)
}

func TestHide_AViewerWithNoCountsStillHearsTheCardsAreHidden(t *testing.T) {
	// given a named board with only Sam's card
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)

	// when
	f.header().CardsHidden = true
	f.svc.Publish(f.access(cast.host), projection.CardsHidden(f.header().Board, []model.Card{f.secretCard(cast.sam)}, 8))
	f.timer().flush()

	// then Sam, who wrote the only card, still learns the board is hidden
	frames := f.boardFrames(t, cast.sam)
	require.Len(t, frames, 1)
	assert.Equal(t, []map[string]any{{"type": projection.EventCardsHidden, "version": float64(8), "hidden_counts": map[string]any{}}}, frames[0].Events)
}

func TestReveal_ReachesEveryoneAtOnceAndMarksOnlyTheAuthorsOwnCard(t *testing.T) {
	// given an anonymous board in its writing phase
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	card := f.secretCard(cast.sam)

	// when
	f.header().CardsHidden = false
	state := model.BoardState{Header: *f.header(), Cards: []model.Card{card}}
	f.svc.Publish(f.access(cast.host), projection.CardsRevealed(state, 8))
	f.timer().flush()

	// then every viewer receives the card with no author, and only Sam is told it is his
	payloads := map[seat]string{}
	for _, seated := range cast.everyone() {
		frames := f.hub.framesOf(seated.principal.ID)
		require.Len(t, frames, 1)
		assert.Contains(t, frames[0], secretBody)
		assert.NotContains(t, frames[0], cast.sam.participantID.String())
		assert.NotContains(t, frames[0], "created_at")
		payloads[seated] = frames[0]
	}
	assert.Contains(t, payloads[cast.sam], `"is_mine":true`)
	assert.NotContains(t, payloads[cast.ali], "is_mine")
	assert.NotContains(t, payloads[cast.host], "is_mine")
	assert.Equal(t, payloads[cast.ali], payloads[cast.host])

	_, published := f.svc.PublishedTally(f.boardID)
	assert.False(t, published, "a shown board has no hidden tally")
}

func TestReveal_GivesBackOwnershipOfACommentOnSomeoneElsesCard(t *testing.T) {
	// given an anonymous board where Ali commented on Sam's card before it was hidden again
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	card := f.secretCard(cast.sam)
	comment := model.CardComment{ID: uuid.New(), CardID: card.ID, Body: "ALI-COMMENT", AuthorParticipantID: new(cast.ali.participantID)}

	// when
	f.header().CardsHidden = false
	state := model.BoardState{Header: *f.header(), Cards: []model.Card{card}, Comments: []model.CardComment{comment}}
	f.svc.Publish(f.access(cast.host), projection.CardsRevealed(state, 8))
	f.timer().flush()

	// then Ali alone gets the comment back as hers, with no author on it
	ali := f.hub.framesOf(cast.ali.principal.ID)
	require.Len(t, ali, 1)
	assert.Contains(t, ali[0], `"is_mine":true`)
	assert.NotContains(t, ali[0], cast.ali.participantID.String())

	host := f.hub.framesOf(cast.host.principal.ID)
	require.Len(t, host, 1)
	assert.Contains(t, host[0], "ALI-COMMENT")
	assert.NotContains(t, host[0], "is_mine")
}

func TestMoveHiddenStack_TellsAChildsAuthorOnAnAnonymousBoardOnlyOnTheTick(t *testing.T) {
	// given an anonymous board with cards hidden, where Ali's card is still stacked under Sam's
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	otherColumn := uuid.New()
	top := f.cardOf(cast.sam, uuid.New(), otherColumn)
	child := f.cardOf(cast.ali, uuid.New(), otherColumn)
	child.ParentCardID = new(top.ID)

	// when Sam moves his card, and the stack with it, to another column
	f.svc.Publish(f.access(cast.sam), projection.CardMoved(f.header().Board, top, liveColumnID, nil, 8), projection.StackedCardMoved(f.header().Board, child, nil, liveColumnID, 8))
	f.timer().flush()

	// then Ali is sent nothing at the moment Sam acts
	assert.Empty(t, f.hub.framesOf(cast.ali.principal.ID))

	// when the tick runs
	f.stored = []model.Card{top, child}
	f.header().Version = 8
	f.boards.EXPECT().BumpVersion(mock.Anything, spec.BoardWrite{OrgID: f.header().OrgID, BoardID: f.boardID, Version: 8}).Return(9, nil).Once()
	f.timer().tick()
	f.timer().flush()

	// then Ali hears where her card went at the tick's version, and nothing names Sam's card
	frames := f.boardFrames(t, cast.ali)
	require.Len(t, frames, 1)
	assert.Equal(t, int64(9), frames[0].To)

	var upserted map[string]any
	for _, event := range frames[0].Events {
		if event["type"] == projection.EventCardUpserted {
			upserted = event["card"].(map[string]any)
		}
	}
	require.NotNil(t, upserted)
	assert.Equal(t, child.ID.String(), upserted["id"])
	assert.Equal(t, otherColumn.String(), upserted["column_id"])
	assert.Nil(t, upserted["parent_card_id"])
	assertNeverSees(t, f, cast.ali, top.ID.String(), top.Body, cast.sam.participantID.String())
}

func TestReveal_PresenceCarriesActivityAgainOnAnAnonymousBoard(t *testing.T) {
	// given an anonymous board in its writing phase whose last presence frame went out
	f := newLiveFixture(t, model.AnonymityAnonymous, true)
	cast := f.cast(t)
	f.timer().presence()
	require.False(t, f.presenceDue())

	// when
	f.header().CardsHidden = false
	f.svc.Publish(f.access(cast.host), projection.CardsRevealed(model.BoardState{Header: *f.header()}, 8))

	// then a presence frame with activity is due
	assert.True(t, f.presenceDue())
}

func TestPublishMeToEveryone_SendsEachSeatedViewerTheirOwnVotes(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	f.header().MaxVotes = 6
	cast := f.cast(t)
	cardID := uuid.New()
	votes := []model.CardVote{
		{CardID: cardID, ParticipantID: cast.sam.participantID, Count: 2, CardAuthorParticipantID: new(cast.ali.participantID)},
		{CardID: cardID, ParticipantID: cast.ali.participantID, Count: 1, CardAuthorParticipantID: new(cast.ali.participantID)},
	}

	// when
	f.svc.PublishMeToEveryone(f.access(cast.host), votes, 8)

	// then
	want := map[seat]dto.BoardMeFrame{
		cast.sam:  {BoardID: f.boardID, Version: 8, VotesRemaining: 4, MyVotes: map[uuid.UUID]int{cardID: 2}},
		cast.ali:  {BoardID: f.boardID, Version: 8, VotesRemaining: 5, MyVotes: map[uuid.UUID]int{cardID: 1}},
		cast.host: {BoardID: f.boardID, Version: 8, VotesRemaining: 6, MyVotes: map[uuid.UUID]int{}},
	}
	for seated, frame := range want {
		direct := f.hub.direct[seated.principal.ID]
		require.Len(t, direct, 1)
		assert.Equal(t, ws.Message{Type: ws.TypeBoardMe, Data: frame}, direct[0])
	}
}

func TestRemoval_TheRemovedParticipantReceivesNothingAfterIt(t *testing.T) {
	// given
	f := newLiveFixture(t, model.AnonymityNamed, false)
	cast := f.cast(t)
	f.rows[cast.sam.principal.ID].RemovedAt = new(time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC))

	// when the board service re-evaluates its viewers and then announces the removal
	require.NoError(t, f.svc.ReevaluateHeld(t.Context(), f.boardID))
	f.svc.Publish(f.access(cast.host), projection.ParticipantRemoved(cast.sam.participantID, 8))
	f.timer().flush()

	// then Sam is told once and nothing more, while everyone else learns Sam left
	assert.Equal(t, []string{ws.TypeBoardJoined, ws.TypeBoardAccessRevoked}, sentTypes(cast.sam.conn))
	assert.Empty(t, f.hub.framesOf(cast.sam.principal.ID))
	assert.False(t, f.hub.member(f.boardID, cast.sam.principal.ID))

	for _, other := range []seat{cast.ali, cast.host} {
		frames := f.boardFrames(t, other)
		require.Len(t, frames, 1)
		assert.Equal(t, []string{projection.EventParticipantRemoved}, eventTypes(frames[0]))
	}
}
