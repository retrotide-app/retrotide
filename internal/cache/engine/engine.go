package engine

import (
	"context"
	"time"
)

type (
	Engine interface {
		Kind() Kind
		Enabled() bool
		Generation() uint64
		Get(ctx context.Context, key string) ([]byte, error)
		Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
		SetIfAbsent(ctx context.Context, key string, data []byte, ttl time.Duration) error
		SetMany(ctx context.Context, entries map[string][]byte, ttl time.Duration) error
		Del(ctx context.Context, keys ...string) error
		Flush(ctx context.Context) error
		Ping(ctx context.Context) error
		Close() error
	}
)
