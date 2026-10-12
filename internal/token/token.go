package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	randomBytes = 32
)

func New() (string, error) {
	raw := make([]byte, randomBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("draw a random token: %w", err)
	}

	return hex.EncodeToString(raw), nil
}

func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))

	return hex.EncodeToString(sum[:])
}
