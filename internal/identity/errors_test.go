package identity

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	errDatabaseDown = errors.New("database down")
	errSMTPRefused  = errors.New("smtp refused the message")
)

func TestRefusals_MapTheM12cReasonsToTheirCodes(t *testing.T) {
	tests := []struct {
		name   string
		reason error
		want   string
	}{
		{name: "an account the provider adapter does not allow", reason: fmt.Errorf("microsoft: tenant not allowed: %w", ErrAccountNotAllowed), want: CodeAccountNotAllowed},
		{name: "a reauthentication by an identity the account does not hold", reason: fmt.Errorf("reauthenticate: %w", errReauthenticationFailed), want: CodeReauthenticationFailed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			code := CodeFailed

			// when
			for _, known := range refusals {
				if errors.Is(tc.reason, known.reason) {
					code = known.code

					break
				}
			}

			// then
			assert.Equal(t, tc.want, code)
		})
	}
}
