package fracindex

import "errors"

var (
	ErrNotOrdered = errors.New("the lower key must sort before the upper key")
	ErrInvalidKey = errors.New("the key is not a valid ordering key")
)
