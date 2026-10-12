package cardpolish

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

func pinCommand(pinned bool) PinCommand {
	return PinCommand{Command: baseCommand(), CardID: cardID, Pinned: pinned}
}

func TestPin_AFacilitatorPinsACardAndEveryoneHearsOfIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	pinned := card(theirID)
	pinned.PinnedAt = new(stamp)
	m.cards.EXPECT().PinCard(mock.Anything, spec.CardPinWrite{BoardWrite: target(), CardID: cardID, Pinned: true}).
		Return(&model.Versioned[model.Card]{Value: pinned, Version: 8}, nil).Once()

	// when
	got, err := svc.Pin(t.Context(), pinCommand(true))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	shared, ok := m.events.published[0].Payload(projection.ClassOthers).(dto.CardPinnedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventCardPinned, shared.Type)
	assert.Equal(t, cardID, shared.CardID)
	assert.True(t, shared.Pinned)
}

func TestPin_AFacilitatorUnpinsACard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	current := card(theirID)
	current.PinnedAt = new(stamp)
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(&current, nil).Once()
	m.cards.EXPECT().PinCard(mock.Anything, spec.CardPinWrite{BoardWrite: target(), CardID: cardID, Pinned: false}).
		Return(&model.Versioned[model.Card]{Value: card(theirID), Version: 8}, nil).Once()

	// when
	_, err := svc.Pin(t.Context(), pinCommand(false))

	// then
	require.NoError(t, err)
	require.Len(t, m.events.published, 1)
	shared, ok := m.events.published[0].Payload(projection.ClassOthers).(dto.CardPinnedEvent)
	require.True(t, ok)
	assert.False(t, shared.Pinned)
}

func TestPin_RefusesAParticipantWhoDoesNotFacilitate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(myID)), nil).Once()

	// when
	_, err := svc.Pin(t.Context(), pinCommand(true))

	// then
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Empty(t, m.events.published)
}

func TestPin_RefusesAHiddenCardEvenForAFacilitator(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hidingCards))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(myID)), nil).Once()

	// when
	_, err := svc.Pin(t.Context(), pinCommand(true))

	// then
	assert.ErrorIs(t, err, policy.ErrCardNotPublic)
	assert.Empty(t, m.events.published)
}

func TestPin_ReplaysAnAppliedOpWithoutWriting(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 9)

	// when
	got, err := svc.Pin(t.Context(), pinCommand(true))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 9, Replayed: true}, got)
	assert.Empty(t, m.events.published)
}
