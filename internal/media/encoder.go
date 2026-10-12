package media

import (
	"context"
)

type (
	Encoder interface {
		Encode(ctx context.Context, input []byte, maxEdge int) (*Encoded, error)
	}

	Encoded struct {
		Data   []byte
		Width  int
		Height int
	}
)
