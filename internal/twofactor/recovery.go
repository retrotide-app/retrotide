package twofactor

import (
	"crypto/rand"
	"strings"

	"retrotide/internal/token"
)

const (
	RecoveryCodeCount = 10

	recoveryCodeHalf = 5
)

var (
	recoveryCodeCleaner = strings.NewReplacer(" ", "", "-", "")
)

func GenerateRecoveryCodes() []string {
	codes := make([]string, 0, RecoveryCodeCount)
	for range RecoveryCodeCount {
		drawn := strings.ToLower(rand.Text())

		codes = append(codes, drawn[:recoveryCodeHalf]+"-"+drawn[recoveryCodeHalf:2*recoveryCodeHalf])
	}

	return codes
}

func RecoveryCodeHash(code string) string {
	cleaned := strings.ToLower(recoveryCodeCleaner.Replace(code))

	return token.Hash(cleaned)
}
