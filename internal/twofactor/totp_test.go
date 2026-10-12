package twofactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rfcKey    = "12345678901234567890"
	rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
)

func TestCode_MatchesTheRFC6238SHA1Vectors(t *testing.T) {
	tests := []struct {
		name string
		unix int64
		want string
	}{
		{name: "T=59", unix: 59, want: "287082"},
		{name: "T=1111111109", unix: 1111111109, want: "081804"},
		{name: "T=1234567890", unix: 1234567890, want: "005924"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			step := tc.unix / stepSeconds

			// when
			got := code([]byte(rfcKey), step)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMatch_AcceptsTheRFC6238VectorsAndAnswersTheirStep(t *testing.T) {
	tests := []struct {
		name     string
		unix     int64
		code     string
		wantStep int64
	}{
		{name: "T=59", unix: 59, code: "287082", wantStep: 1},
		{name: "T=1111111109", unix: 1111111109, code: "081804", wantStep: 37037036},
		{name: "T=1234567890", unix: 1234567890, code: "005924", wantStep: 41152263},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			now := time.Unix(tc.unix, 0)

			// when
			step, matched := Match(rfcSecret, tc.code, now)

			// then
			assert.True(t, matched)
			assert.Equal(t, tc.wantStep, step)
		})
	}
}

func TestMatch_AllowsOneStepEitherSideAndNoMore(t *testing.T) {
	tests := []struct {
		name        string
		offset      time.Duration
		wantMatched bool
	}{
		{name: "two steps early", offset: -60 * time.Second, wantMatched: false},
		{name: "one step early", offset: -30 * time.Second, wantMatched: true},
		{name: "on time", offset: 0, wantMatched: true},
		{name: "one step late", offset: 30 * time.Second, wantMatched: true},
		{name: "two steps late", offset: 60 * time.Second, wantMatched: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the code for T=1111111109, read by a clock that is off by the offset
			now := time.Unix(1111111109, 0).Add(tc.offset)

			// when
			step, matched := Match(rfcSecret, "081804", now)

			// then the matched step is always the code's own step, so replay protection keys on it
			assert.Equal(t, tc.wantMatched, matched)
			if tc.wantMatched {
				assert.Equal(t, int64(37037036), step)
			}
		})
	}
}

func TestMatch_StripsSpacesFromTheCode(t *testing.T) {
	// given
	now := time.Unix(1111111109, 0)

	// when
	step, matched := Match(rfcSecret, " 081 804 ", now)

	// then
	assert.True(t, matched)
	assert.Equal(t, int64(37037036), step)
}

func TestMatch_RefusesMalformedCodes(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{name: "empty", code: ""},
		{name: "too short", code: "08180"},
		{name: "too long", code: "0818040"},
		{name: "a letter", code: "08180a"},
		{name: "a hyphen", code: "081-804"},
		{name: "full-width digits", code: "０８１８０４"},
		{name: "the wrong digits", code: "081805"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			now := time.Unix(1111111109, 0)

			// when
			step, matched := Match(rfcSecret, tc.code, now)

			// then
			assert.False(t, matched)
			assert.Zero(t, step)
		})
	}
}

func TestMatch_RefusesASecretThatIsNotBase32(t *testing.T) {
	// given
	now := time.Unix(1111111109, 0)

	// when
	_, matched := Match("not base32!", "081804", now)

	// then
	assert.False(t, matched)
}

func TestGenerateSecret_DrawsTwentyBytesAsUnpaddedBase32(t *testing.T) {
	// when
	first, err := GenerateSecret()
	require.NoError(t, err)

	second, err := GenerateSecret()
	require.NoError(t, err)

	// then the secret fits the 32-character column and is fresh each time
	assert.Len(t, first, 32)
	assert.NotContains(t, first, "=")
	decoded, err := secretEncoding.DecodeString(first)
	require.NoError(t, err)
	assert.Len(t, decoded, secretBytes)
	assert.NotEqual(t, first, second)
}

func TestURI_NamesTheIssuerAndAccountForAuthenticatorApps(t *testing.T) {
	// when
	got := URI("Retro Tide", "alice@example.com", rfcSecret)

	// then
	assert.Equal(t, "otpauth://totp/Retro%20Tide:alice@example.com?secret="+rfcSecret+"&issuer=Retro+Tide&algorithm=SHA1&digits=6&period=30", got)
}
