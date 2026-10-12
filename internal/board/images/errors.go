package images

import (
	"errors"
)

var (
	ErrMediaNotFound = errors.New("image not found")
	ErrWrongKind     = errors.New("that image was uploaded for something else, so it cannot be used here")
)
