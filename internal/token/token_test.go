package token

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Draws32RandomBytesAsLowerCaseHex(t *testing.T) {
	// given
	seen := make(map[string]struct{})

	for range 50 {
		// when
		drawn, err := New()

		// then
		require.NoError(t, err)
		assert.Regexp(t, `^[0-9a-f]{64}$`, drawn)

		raw, decodeErr := hex.DecodeString(drawn)
		require.NoError(t, decodeErr)
		assert.Len(t, raw, randomBytes)

		_, repeated := seen[drawn]
		assert.False(t, repeated)
		seen[drawn] = struct{}{}
	}
}

func TestHash_IsTheLowerCaseHexSHA256(t *testing.T) {
	// given
	value := "abc"

	// when
	hash := Hash(value)

	// then the value matches every stored hash column, sessions.token and oidc_flows.session_token_hash included
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", hash)
}

func TestHash_NeverReturnsTheValueItHashes(t *testing.T) {
	// given
	drawn, err := New()
	require.NoError(t, err)

	// when
	hash := Hash(drawn)

	// then
	assert.Regexp(t, `^[0-9a-f]{64}$`, hash)
	assert.NotEqual(t, drawn, hash)
	assert.Equal(t, hash, Hash(drawn))
}
