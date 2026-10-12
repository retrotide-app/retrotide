package engine

import (
	"io"

	"retrotide/internal/dto"
)

type (
	Engine interface {
		Format() Format
		ContentType() string
		Extension() string
		WriteBoard(w io.Writer, board dto.BoardExport) error
	}
)
