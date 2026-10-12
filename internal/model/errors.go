package model

import "errors"

var (
	ErrInvalidPrincipal = errors.New("invalid or expired credentials")
)
