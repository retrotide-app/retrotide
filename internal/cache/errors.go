package cache

import (
	"errors"

	"retrotide/internal/cache/engine"
)

var (
	ErrMiss          = engine.ErrMiss
	ErrUnknownEngine = errors.New("cache: unknown engine")
)
