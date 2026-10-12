package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStage_PreviousIsTheStageStraightBeforeAndEndIsFinal(t *testing.T) {
	tests := []struct {
		stage Stage
		want  Stage
		ok    bool
	}{
		{stage: StageWrite},
		{stage: StageVote, want: StageWrite, ok: true},
		{stage: StageDiscuss, want: StageVote, ok: true},
		{stage: StageActions, want: StageDiscuss, ok: true},
		{stage: StageEnd},
		{stage: "review"},
	}

	for _, tc := range tests {
		t.Run(string(tc.stage), func(t *testing.T) {
			// when
			got, ok := tc.stage.Previous()

			// then
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestReviewOutcome_StatusIsWhatTheReviewLeavesTheItemIn(t *testing.T) {
	// then
	assert.Equal(t, ActionItemDone, ReviewDone.Status())
	assert.Equal(t, ActionItemOpen, ReviewCarried.Status())
	assert.Equal(t, ActionItemDropped, ReviewDropped.Status())
}

func TestActionItemCounts_TheCompletionRateLeavesOutDroppedItems(t *testing.T) {
	// given
	counts := ActionItemCounts{Open: 1, Done: 3, Dropped: 4}

	// when
	rate := counts.CompletionRate()

	// then
	require.NotNil(t, rate)
	assert.InDelta(t, 0.75, *rate, 0.0001)
	assert.Equal(t, 8, counts.Total())
	assert.Equal(t, 4, counts.Of(ActionItemDropped))
	assert.Nil(t, ActionItemCounts{Dropped: 2}.CompletionRate())
}
