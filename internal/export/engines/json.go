package engines

import (
	"encoding/json"
	"fmt"
	"io"

	"retrotide/internal/dto"
	"retrotide/internal/export/engine"
)

type (
	JSON struct{}
)

func (JSON) Format() engine.Format {
	return engine.FormatJSON
}

func (JSON) ContentType() string {
	return "application/json"
}

func (JSON) Extension() string {
	return "json"
}

func (JSON) WriteBoard(w io.Writer, board dto.BoardExport) error {
	if err := newJSONEncoder(w).Encode(board); err != nil {
		return fmt.Errorf("write the JSON board export: %w", err)
	}

	return nil
}

func (JSON) WriteDocument(w io.Writer, document any) error {
	if err := newJSONEncoder(w).Encode(document); err != nil {
		return fmt.Errorf("write the JSON export: %w", err)
	}

	return nil
}

func newJSONEncoder(w io.Writer) *json.Encoder {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	return encoder
}
