package export

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines"
)

type (
	EngineFactory struct {
		constructors map[engine.Format]func() engine.Engine
		formats      []engine.Format
	}
)

func NewEngineFactory() *EngineFactory {
	constructors := map[engine.Format]func() engine.Engine{
		engine.FormatCSV:      func() engine.Engine { return engines.CSV{} },
		engine.FormatMarkdown: func() engine.Engine { return engines.Markdown{} },
		engine.FormatJSON:     func() engine.Engine { return engines.JSON{} },
	}

	return &EngineFactory{constructors: constructors, formats: slices.Sorted(maps.Keys(constructors))}
}

func (f *EngineFactory) Formats() []engine.Format {
	return f.formats
}

func (f *EngineFactory) Create(format engine.Format) (engine.Engine, error) {
	construct, ok := f.constructors[format]
	if !ok {
		return nil, fmt.Errorf("%w: got %q, want one of %s", ErrUnknownFormat, format, joinFormats(f.formats))
	}

	return construct(), nil
}

func joinFormats(formats []engine.Format) string {
	names := make([]string, 0, len(formats))
	for _, format := range formats {
		names = append(names, string(format))
	}

	return strings.Join(names, ", ")
}
