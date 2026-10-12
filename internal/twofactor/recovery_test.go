package twofactor

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	recoveryCodeShape = regexp.MustCompile(`^[a-z2-7]{5}-[a-z2-7]{5}$`)
)

func TestGenerateRecoveryCodes_DrawsTenDistinctReadableCodes(t *testing.T) {
	// when
	codes := GenerateRecoveryCodes()

	// then
	require.Len(t, codes, RecoveryCodeCount)

	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		assert.Regexp(t, recoveryCodeShape, code)
		seen[code] = struct{}{}
	}
	assert.Len(t, seen, RecoveryCodeCount)
}

func TestRecoveryCodeHash_IgnoresCaseSpacesAndTheHyphen(t *testing.T) {
	tests := []struct {
		name  string
		typed string
	}{
		{name: "as shown", typed: "abcde-fgh23"},
		{name: "in capitals", typed: "ABCDE-FGH23"},
		{name: "without the hyphen", typed: "abcdefgh23"},
		{name: "with spaces", typed: " abcde fgh23 "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := RecoveryCodeHash(tc.typed)

			// then
			assert.Equal(t, RecoveryCodeHash("abcde-fgh23"), got)
			assert.Len(t, got, 64)
		})
	}
}

func TestRecoveryCodeHash_TellsDifferentCodesApart(t *testing.T) {
	// when
	first, second := RecoveryCodeHash("abcde-fgh23"), RecoveryCodeHash("abcde-fgh24")

	// then
	assert.NotEqual(t, first, second)
}
