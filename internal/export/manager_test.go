package export_test

import (
	"testing"

	"retrotide/internal/export"
	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineFactory_CreatesTheEngineForEachFormat(t *testing.T) {
	tests := []struct {
		format engine.Format
		want   engine.Engine
	}{
		{format: engine.FormatCSV, want: engines.CSV{}},
		{format: engine.FormatMarkdown, want: engines.Markdown{}},
		{format: engine.FormatJSON, want: engines.JSON{}},
	}

	for _, tc := range tests {
		t.Run(string(tc.format), func(t *testing.T) {
			// when
			got, err := export.NewEngineFactory().Create(tc.format)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.format, got.Format())
		})
	}
}

func TestEngineFactory_RefusesAnUnknownFormat(t *testing.T) {
	for _, format := range []engine.Format{"", "pdf", "CSV", "md", " json"} {
		t.Run(string(format), func(t *testing.T) {
			// when
			_, err := export.NewEngineFactory().Create(format)

			// then
			require.ErrorIs(t, err, export.ErrUnknownFormat)
			assert.ErrorContains(t, err, "want one of csv, json, markdown")
		})
	}
}

func TestEngineManager_HandsOutTheSameEngineForAFormatEachTime(t *testing.T) {
	// given
	manager := export.NewEngineManager()

	// when
	first, firstErr := manager.Engine(engine.FormatMarkdown)
	second, secondErr := manager.Engine(engine.FormatMarkdown)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.Equal(t, engines.Markdown{}, first)
	assert.Equal(t, first, second)
}

func TestEngineManager_PassesOnTheFactorysRefusal(t *testing.T) {
	// when
	_, err := export.NewEngineManager().Engine("pdf")

	// then
	require.ErrorIs(t, err, export.ErrUnknownFormat)
}
