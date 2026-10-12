package twofactor

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	secretBytes  = 20
	codeDigits   = 6
	codeModulus  = 1000000
	stepSeconds  = 30
	stepWindow   = 1
	counterBytes = 8

	uriAlgorithm = "SHA1"
)

var (
	secretEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)
)

func GenerateSecret() (string, error) {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("draw a two-factor secret: %w", err)
	}

	return secretEncoding.EncodeToString(raw), nil
}

func code(key []byte, step int64) string {
	counter := make([]byte, counterBytes)
	binary.BigEndian.PutUint64(counter, uint64(step))

	mac := hmac.New(sha1.New, key)
	mac.Write(counter)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	return fmt.Sprintf("%0*d", codeDigits, truncated%codeModulus)
}

func Match(secret string, presented string, now time.Time) (int64, bool) {
	cleaned := strings.ReplaceAll(presented, " ", "")
	if !wellFormedCode(cleaned) {
		return 0, false
	}

	key, err := secretEncoding.DecodeString(secret)
	if err != nil {
		return 0, false
	}

	current := now.Unix() / stepSeconds

	var matched int64
	found := false

	for step := current - stepWindow; step <= current+stepWindow; step++ {
		if step < 0 {
			continue
		}

		if subtle.ConstantTimeCompare([]byte(code(key, step)), []byte(cleaned)) == 1 && !found {
			matched = step
			found = true
		}
	}

	return matched, found
}

func URI(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)

	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=%s&digits=%d&period=%d", label, secret, url.QueryEscape(issuer), uriAlgorithm, codeDigits, stepSeconds)
}

func wellFormedCode(candidate string) bool {
	if len(candidate) != codeDigits {
		return false
	}

	for _, r := range candidate {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
