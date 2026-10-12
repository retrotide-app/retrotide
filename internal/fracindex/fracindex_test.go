package fracindex

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBetween_PlacesAKeyStrictlyBetweenItsBounds(t *testing.T) {
	cases := []struct {
		name  string
		lower string
		upper string
		want  string
	}{
		{name: "an empty column", lower: "", upper: "", want: "V"},
		{name: "after the last key", lower: "V", upper: "", want: "W"},
		{name: "after a key of only top digits", lower: "zz", upper: "", want: "zzV"},
		{name: "after a key whose first digit is the top one", lower: "zA", upper: "", want: "zB"},
		{name: "before the first key", lower: "", upper: "V", want: "U"},
		{name: "before a key of ones", lower: "", upper: "1", want: "0V"},
		{name: "before a key with a low prefix", lower: "", upper: "01", want: "00V"},
		{name: "between distant keys", lower: "A", upper: "a", want: "N"},
		{name: "between adjacent digits", lower: "V", upper: "W", want: "VV"},
		{name: "between a key and its extension", lower: "V", upper: "V1", want: "V0V"},
		{name: "between keys sharing a prefix", lower: "a1", upper: "a3", want: "a2"},
		{name: "below a longer upper key", lower: "a", upper: "bV", want: "b"},
		{name: "above a lower key ending in its smallest digit", lower: "a0", upper: "a1", want: "a0V"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := Between(tc.lower, tc.upper)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			if tc.lower != "" {
				assert.Less(t, tc.lower, got)
			}
			if tc.upper != "" {
				assert.Less(t, got, tc.upper)
			}
		})
	}
}

func TestBetween_RefusesBoundsOutOfOrder(t *testing.T) {
	cases := []struct {
		name  string
		lower string
		upper string
	}{
		{name: "equal keys", lower: "V", upper: "V"},
		{name: "reversed keys", lower: "W", upper: "V"},
		{name: "a lower key that extends the upper", lower: "V1", upper: "V"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := Between(tc.lower, tc.upper)

			// then
			assert.ErrorIs(t, err, ErrNotOrdered)
			assert.Empty(t, got)
		})
	}
}

func TestBetween_RefusesKeysItCannotOrder(t *testing.T) {
	cases := []struct {
		name  string
		lower string
		upper string
	}{
		{name: "a lower key outside the alphabet", lower: "a-b", upper: ""},
		{name: "an upper key outside the alphabet", lower: "", upper: "é"},
		{name: "an upper key ending in its smallest digit", lower: "a", upper: "a0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := Between(tc.lower, tc.upper)

			// then
			assert.ErrorIs(t, err, ErrInvalidKey)
			assert.Empty(t, got)
		})
	}
}

func TestBetween_RandomInsertsStayDistinctAndOrdered(t *testing.T) {
	// given a fixed seed, so a failure always reproduces
	random := rand.New(rand.NewPCG(7, 11))
	keys := []string{}

	// when
	for range 2000 {
		at := random.IntN(len(keys) + 1)

		lower := ""
		if at > 0 {
			lower = keys[at-1]
		}

		upper := ""
		if at < len(keys) {
			upper = keys[at]
		}

		key, err := Between(lower, upper)
		require.NoError(t, err)

		keys = slices.Insert(keys, at, key)
	}

	// then
	assert.True(t, slices.IsSorted(keys))
	assert.Len(t, slices.Compact(slices.Clone(keys)), len(keys))
	for _, key := range keys {
		assert.False(t, strings.HasSuffix(key, "0"), key)
	}
}

func TestBetween_RepeatedAppendsAndPrependsGrowSlowly(t *testing.T) {
	// given
	last, first := "", ""

	// when
	for range 1000 {
		next, err := Between(last, "")
		require.NoError(t, err)
		last = next

		previous, err := Between("", first)
		require.NoError(t, err)
		first = previous
	}

	// then a thousand cards at either end of a column stay well inside the rewrite threshold
	assert.LessOrEqual(t, len(last), 40)
	assert.LessOrEqual(t, len(first), 40)
}

func TestBetween_RepeatedInsertsIntoOneGapCrossTheRewriteThreshold(t *testing.T) {
	// given
	lower, upper := "V", "W"

	// when
	var key string
	for range 500 {
		next, err := Between(lower, upper)
		require.NoError(t, err)
		key = next
		upper = next
	}

	// then the caller sees a key past the limit and rewrites the column
	assert.Greater(t, len(key), MaxKeyLength)
}

func TestSpread_GivesDistinctOrderedKeys(t *testing.T) {
	cases := []struct {
		name  string
		n     int
		width int
	}{
		{name: "one key", n: 1, width: 1},
		{name: "a handful of keys", n: 5, width: 1},
		{name: "the most that fit one digit", n: 60, width: 1},
		{name: "one more than fit one digit", n: 61, width: 2},
		{name: "many keys", n: 5000, width: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			keys := Spread(tc.n)

			// then
			require.Len(t, keys, tc.n)
			assert.True(t, slices.IsSorted(keys))
			assert.Len(t, slices.Compact(slices.Clone(keys)), tc.n)
			for _, key := range keys {
				assert.NotEmpty(t, key)
				assert.LessOrEqual(t, len(key), tc.width)
				assert.False(t, strings.HasSuffix(key, "0"), key)
			}
		})
	}
}

func TestSpread_KeysLeaveSpaceAtBothEnds(t *testing.T) {
	// given
	keys := Spread(3)

	// when
	before, beforeErr := Between("", keys[0])
	after, afterErr := Between(keys[len(keys)-1], "")
	inside, insideErr := Between(keys[0], keys[1])

	// then
	require.NoError(t, beforeErr)
	require.NoError(t, afterErr)
	require.NoError(t, insideErr)
	assert.Less(t, before, keys[0])
	assert.Greater(t, after, keys[2])
	assert.Less(t, keys[0], inside)
	assert.Less(t, inside, keys[1])
}

func TestSpread_NoKeysForNothing(t *testing.T) {
	for _, n := range []int{0, -3} {
		// when
		keys := Spread(n)

		// then
		assert.NotNil(t, keys)
		assert.Empty(t, keys)
	}
}
