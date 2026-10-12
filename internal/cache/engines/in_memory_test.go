package engines

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"retrotide/internal/cache/engine"
	"retrotide/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	fakeClock struct {
		now time.Time
	}
)

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func smallEntries(n int) int {
	return n * (len("a") + len("1") + entryOverheadBytes)
}

func newClockedInMemory(t *testing.T, maxBytes int) (*InMemory, *fakeClock) {
	t.Helper()

	clock := &fakeClock{now: time.Date(1986, time.October, 4, 12, 0, 0, 0, time.UTC)}

	c := NewInMemory(maxBytes)
	c.now = clock.Now

	return c, clock
}

func TestInMemoryStoresAndReturnsValues(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	// when
	require.NoError(t, c.Set(ctx, "retro", []byte("tide"), 0))

	// then
	got, err := c.Get(ctx, "retro")
	require.NoError(t, err)
	assert.Equal(t, []byte("tide"), got)
}

func TestInMemorySetIfAbsentKeepsAValueAlreadyThere(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()
	require.NoError(t, c.Set(ctx, "setting:site_name", []byte("new"), 0))

	// when a fill that read the old value arrives after the write
	err := c.SetIfAbsent(ctx, "setting:site_name", []byte("old"), 0)

	// then
	require.NoError(t, err)
	got, getErr := c.Get(ctx, "setting:site_name")
	require.NoError(t, getErr)
	assert.Equal(t, []byte("new"), got)
}

func TestInMemorySetIfAbsentFillsAnAbsentOrExpiredKey(t *testing.T) {
	// given
	c, clock := newClockedInMemory(t, 0)
	ctx := t.Context()
	require.NoError(t, c.Set(ctx, "expiring", []byte("old"), time.Minute))
	clock.advance(2 * time.Minute)

	// when
	absentErr := c.SetIfAbsent(ctx, "absent", []byte("filled"), 0)
	expiredErr := c.SetIfAbsent(ctx, "expiring", []byte("refilled"), 0)

	// then
	require.NoError(t, absentErr)
	require.NoError(t, expiredErr)

	absent, err := c.Get(ctx, "absent")
	require.NoError(t, err)
	assert.Equal(t, []byte("filled"), absent)

	expired, err := c.Get(ctx, "expiring")
	require.NoError(t, err)
	assert.Equal(t, []byte("refilled"), expired)
}

func TestInMemoryFlushDropsEveryKey(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()
	require.NoError(t, c.Set(ctx, "a", []byte("1"), 0))
	require.NoError(t, c.Set(ctx, "b", []byte("2"), time.Minute))

	// when
	err := c.Flush(ctx)

	// then
	require.NoError(t, err)
	assert.Zero(t, c.Len())
	assert.Zero(t, c.Bytes())
	_, getErr := c.Get(ctx, "a")
	require.ErrorIs(t, getErr, engine.ErrMiss)
}

func TestInMemoryReturnsMissForUnknownKey(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)

	// when
	_, err := c.Get(t.Context(), "absent")

	// then
	require.ErrorIs(t, err, engine.ErrMiss)
}

func TestInMemoryDefaultsMaxBytes(t *testing.T) {
	tests := []struct {
		name     string
		maxBytes int
		want     int
	}{
		{name: "zero falls back to default", maxBytes: 0, want: defaultInMemoryMaxBytes},
		{name: "negative falls back to default", maxBytes: -5, want: defaultInMemoryMaxBytes},
		{name: "explicit value is kept", maxBytes: 4096, want: 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			maxBytes := tt.maxBytes

			// when
			c := NewInMemory(maxBytes)

			// then
			assert.Equal(t, tt.want, c.maxBytes)
		})
	}
}

func TestInMemoryEvictsOldestBeyondTheByteBudget(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, smallEntries(2))
	ctx := t.Context()

	// when
	require.NoError(t, c.Set(ctx, "a", []byte("1"), 0))
	require.NoError(t, c.Set(ctx, "b", []byte("2"), 0))
	require.NoError(t, c.Set(ctx, "c", []byte("3"), 0))

	// then
	assert.Equal(t, 2, c.Len())

	_, err := c.Get(ctx, "a")
	require.ErrorIs(t, err, engine.ErrMiss)

	for _, key := range []string{"b", "c"} {
		_, err := c.Get(ctx, key)
		require.NoError(t, err)
	}
}

func TestInMemoryEvictsWhenAnOverwriteGrowsPastTheByteBudget(t *testing.T) {
	// given a full cache of two small entries
	c, _ := newClockedInMemory(t, smallEntries(2))
	ctx := t.Context()

	require.NoError(t, c.Set(ctx, "a", []byte("1"), 0))
	require.NoError(t, c.Set(ctx, "b", []byte("2"), 0))

	// when the newest key is rewritten with a value too large to share the budget
	require.NoError(t, c.Set(ctx, "b", []byte("22"), 0))

	// then the oldest entry makes space, rather than the cache running over its cap
	assert.LessOrEqual(t, c.Bytes(), smallEntries(2))

	_, err := c.Get(ctx, "a")
	require.ErrorIs(t, err, engine.ErrMiss)

	got, err := c.Get(ctx, "b")
	require.NoError(t, err)
	assert.Equal(t, []byte("22"), got)
}

func TestInMemoryReadRefreshesRecency(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, smallEntries(2))
	ctx := t.Context()

	require.NoError(t, c.Set(ctx, "a", []byte("1"), 0))
	require.NoError(t, c.Set(ctx, "b", []byte("2"), 0))

	_, err := c.Get(ctx, "a")
	require.NoError(t, err)

	// when
	require.NoError(t, c.Set(ctx, "c", []byte("3"), 0))

	// then
	_, err = c.Get(ctx, "a")
	require.NoError(t, err)

	_, err = c.Get(ctx, "b")
	require.ErrorIs(t, err, engine.ErrMiss)
}

func TestInMemoryTTLExpiry(t *testing.T) {
	tests := []struct {
		name    string
		ttl     time.Duration
		advance time.Duration
		wantHit bool
	}{
		{name: "before expiry", ttl: time.Minute, advance: 30 * time.Second, wantHit: true},
		{name: "exactly at expiry", ttl: time.Minute, advance: time.Minute, wantHit: true},
		{name: "past expiry", ttl: time.Minute, advance: 90 * time.Second, wantHit: false},
		{name: "zero ttl never expires, matching valkey", ttl: 0, advance: 30 * time.Second, wantHit: true},
		{name: "zero ttl survives a year, because forever means forever", ttl: 0, advance: 365 * 24 * time.Hour, wantHit: true},
		{name: "negative ttl is treated as forever too", ttl: -time.Minute, advance: 365 * 24 * time.Hour, wantHit: true},
		{name: "long ttl is left alone", ttl: 24 * time.Hour, advance: 12 * time.Hour, wantHit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			c, clock := newClockedInMemory(t, 0)
			ctx := t.Context()

			require.NoError(t, c.Set(ctx, "k", []byte("v"), tt.ttl))

			// when
			clock.advance(tt.advance)

			got, err := c.Get(ctx, "k")

			// then
			if !tt.wantHit {
				require.ErrorIs(t, err, engine.ErrMiss)
				assert.Zero(t, c.Len())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, []byte("v"), got)
		})
	}
}

func TestInMemorySetManyStoresEveryEntry(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	entries := map[string][]byte{"a": []byte("1"), "b": []byte("2"), "c": []byte("3")}

	// when
	require.NoError(t, c.SetMany(ctx, entries, 0))

	// then
	assert.Equal(t, 3, c.Len())

	for key, want := range entries {
		got, err := c.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func TestInMemorySetManyIgnoresEmptyInput(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)

	// when
	require.NoError(t, c.SetMany(t.Context(), nil, 0))

	// then
	assert.Zero(t, c.Len())
}

func TestInMemoryDelRemovesOnlyNamedKeys(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	require.NoError(t, c.Set(ctx, "a", []byte("1"), 0))
	require.NoError(t, c.Set(ctx, "b", []byte("2"), 0))

	// when
	require.NoError(t, c.Del(ctx, "a", "never-stored"))

	// then
	assert.Equal(t, 1, c.Len())

	_, err := c.Get(ctx, "a")
	require.ErrorIs(t, err, engine.ErrMiss)

	_, err = c.Get(ctx, "b")
	require.NoError(t, err)
}

func TestInMemoryOverwriteReplacesWithoutGrowing(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	require.NoError(t, c.Set(ctx, "k", []byte("first"), 0))

	// when
	require.NoError(t, c.Set(ctx, "k", []byte("second"), 0))

	// then
	assert.Equal(t, 1, c.Len())

	got, err := c.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("second"), got)
}

func TestInMemoryOverwriteResetsExpiry(t *testing.T) {
	// given
	c, clock := newClockedInMemory(t, 0)
	ctx := t.Context()

	require.NoError(t, c.Set(ctx, "k", []byte("first"), time.Minute))

	// when
	clock.advance(30 * time.Second)
	require.NoError(t, c.Set(ctx, "k", []byte("second"), time.Minute))

	clock.advance(45 * time.Second)

	// then
	got, err := c.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("second"), got)
}

func TestInMemoryCloseClearsEntries(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	require.NoError(t, c.Set(ctx, "k", []byte("v"), 0))

	// when
	require.NoError(t, c.Close())

	// then
	assert.Zero(t, c.Len())

	_, err := c.Get(ctx, "k")
	require.ErrorIs(t, err, engine.ErrMiss)
}

func TestInMemoryHoldsZeroTTLUntilEvicted(t *testing.T) {
	// given a zero ttl entry, which valkey would keep forever
	c, clock := newClockedInMemory(t, 0)
	ctx := t.Context()

	require.NoError(t, c.Set(ctx, "setting:maintenance_mode", []byte("false"), 0))

	// when a long time passes
	clock.advance(24 * time.Hour)

	// then it is still there, so both engines agree on what zero means, and memory stays bounded because the byte budget evicts it rather than the clock
	got, err := c.Get(ctx, "setting:maintenance_mode")
	require.NoError(t, err)
	require.Equal(t, []byte("false"), got)

	blob := make([]byte, 400)
	c.maxBytes = entrySize("filler", blob)
	require.NoError(t, c.Set(ctx, "filler", blob, 0))

	_, err = c.Get(ctx, "setting:maintenance_mode")
	require.ErrorIs(t, err, engine.ErrMiss)
}

func TestInMemoryEvictsOnByteCeiling(t *testing.T) {
	// given a budget with space for exactly two entries once overhead is counted
	c, _ := newClockedInMemory(t, 0)
	blob := make([]byte, 400)
	budget := 2 * entrySize("a", blob)
	c.maxBytes = budget
	ctx := t.Context()

	// when a third arrives
	for _, key := range []string{"a", "b", "c"} {
		require.NoError(t, c.Set(ctx, key, blob, time.Minute))
	}

	// then the oldest is evicted and the budget is respected
	assert.LessOrEqual(t, c.Bytes(), budget)
	assert.Equal(t, 2, c.Len())

	_, err := c.Get(ctx, "a")
	require.ErrorIs(t, err, engine.ErrMiss)
}

func TestInMemoryCountsKeyAndPerEntryOverheadNotJustThePayload(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	// when
	require.NoError(t, c.Set(ctx, "retrotide", make([]byte, 100), time.Minute))

	// then the budget reflects real footprint, so tiny values cannot blow past it unseen
	assert.Equal(t, len("retrotide")+100+entryOverheadBytes, c.Bytes())
}

func TestInMemoryResizeMBEvictsImmediately(t *testing.T) {
	// given a cache holding more than the new budget will allow
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	for _, key := range []string{"a", "b", "c"} {
		require.NoError(t, c.Set(ctx, key, make([]byte, 400*1024), time.Minute))
	}

	require.Equal(t, 3, c.Len())

	// when the limit is shrunk to one megabyte
	c.ResizeMB(1)

	// then it evicts on the spot rather than waiting for the next write
	assert.LessOrEqual(t, c.Bytes(), bytesPerMB)
	assert.Less(t, c.Len(), 3)
}

func TestInMemoryResizeMBClampsOutOfRangeValues(t *testing.T) {
	tests := []struct {
		name string
		mb   int
		want int
	}{
		{name: "zero falls back to the default", mb: 0, want: defaultInMemoryMaxMB * bytesPerMB},
		{name: "negative falls back to the default", mb: -3, want: defaultInMemoryMaxMB * bytesPerMB},
		{name: "absurdly large is clamped", mb: 1 << 20, want: maxInMemoryMaxMB * bytesPerMB},
		{name: "a sane value is kept", mb: 64, want: 64 * bytesPerMB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			c, _ := newClockedInMemory(t, 0)

			// when
			c.ResizeMB(tt.mb)

			// then
			assert.Equal(t, tt.want, c.maxBytes)
		})
	}
}

func TestInMemoryOnSettingChangedOnlyReactsToItsOwnKey(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	before := c.maxBytes

	// when an unrelated setting changes
	c.OnSettingChanged(config.SettingValkeyURL.Key, "redis://localhost:6379")

	// then
	assert.Equal(t, before, c.maxBytes)
}

func TestInMemoryOnSettingChangedKeepsTheLimitWhenTheValueIsNotANumber(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	c.ResizeMB(64)
	before := c.maxBytes

	// when
	c.OnSettingChanged(config.SettingCacheInMemoryMaxMB.Key, "not-a-number")

	// then a bad value must never shrink the cache to nothing
	assert.Equal(t, before, c.maxBytes)
}

func TestInMemoryOnSettingChangedAppliesTheNewLimit(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)

	// when
	c.OnSettingChanged(config.SettingCacheInMemoryMaxMB.Key, "32")

	// then
	assert.Equal(t, 32*bytesPerMB, c.maxBytes)
}

func TestInMemoryBytesTracksOverwriteAndDelete(t *testing.T) {
	// given
	c, _ := newClockedInMemory(t, 0)
	ctx := t.Context()

	// when
	require.NoError(t, c.Set(ctx, "k", make([]byte, 500), time.Minute))

	// then
	assert.Equal(t, entrySize("k", make([]byte, 500)), c.Bytes())

	// when
	require.NoError(t, c.Set(ctx, "k", make([]byte, 100), time.Minute))

	// then
	assert.Equal(t, entrySize("k", make([]byte, 100)), c.Bytes())

	// when
	require.NoError(t, c.Del(ctx, "k"))

	// then
	assert.Zero(t, c.Bytes())

	// when
	require.NoError(t, c.Set(ctx, "k", make([]byte, 50), time.Minute))
	require.NoError(t, c.Close())

	// then
	assert.Zero(t, c.Bytes())
}

func TestInMemoryIsAlwaysAvailable(t *testing.T) {
	// given
	c := NewInMemory(0)

	// when
	err := c.Ping(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, engine.KindInMemory, c.Kind())
	assert.True(t, c.Enabled())
}

func TestInMemoryHandlesConcurrentAccess(t *testing.T) {
	// given
	c := NewInMemory(0)
	ctx := t.Context()

	var wg sync.WaitGroup

	// when
	for i := range 64 {
		wg.Go(func() {
			key := strconv.Itoa(i % 16)

			assert.NoError(t, c.Set(ctx, key, []byte(key), 0))

			_, err := c.Get(ctx, key)
			if err != nil {
				assert.ErrorIs(t, err, engine.ErrMiss)
			}

			assert.NoError(t, c.Del(ctx, key))
		})
	}

	wg.Wait()

	// then
	assert.LessOrEqual(t, c.Len(), 8)
}
