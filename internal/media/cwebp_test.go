package media

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSlottedEncoder(slots int) *cwebpEncoder {
	return &cwebpEncoder{timeout: cwebpTimeout, queueWait: 20 * time.Millisecond, slots: make(chan struct{}, slots)}
}

func TestCWebPEncoder_Acquire_TakesAFreeSlot(t *testing.T) {
	// given
	encoder := newSlottedEncoder(2)

	// when
	err := encoder.acquire(t.Context())

	// then
	require.NoError(t, err)
	assert.Len(t, encoder.slots, 1)
}

func TestCWebPEncoder_Acquire_RefusesOnceEverySlotIsBusyForTheWholeWait(t *testing.T) {
	// given every slot held by another conversion
	encoder := newSlottedEncoder(1)
	encoder.slots <- struct{}{}

	// when
	err := encoder.acquire(t.Context())

	// then
	require.ErrorIs(t, err, ErrEncoderUnavailable)
	assert.Len(t, encoder.slots, 1)
}

func TestCWebPEncoder_Acquire_StopsWaitingWhenTheRequestEnds(t *testing.T) {
	// given every slot held, and a request that has already gone away
	encoder := newSlottedEncoder(1)
	encoder.queueWait = time.Minute
	encoder.slots <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// when
	err := encoder.acquire(ctx)

	// then
	require.ErrorIs(t, err, ErrEncoderUnavailable)
	require.ErrorIs(t, err, context.Canceled)
}

func TestNewCWebPEncoder_RunsAtMostTwoConversionsAtOnce(t *testing.T) {
	// when
	encoder := NewCWebPEncoder().(*cwebpEncoder)

	// then
	assert.Equal(t, 2, cap(encoder.slots))
}
