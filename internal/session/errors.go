package session

import "errors"

var (
	ErrInvalidSession = errors.New("invalid or expired session")
)
