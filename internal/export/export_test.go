package export_test

import (
	"testing"
	"time"

	"retrotide/internal/export"

	"github.com/stretchr/testify/assert"
)

var (
	exportDate = time.Date(2026, 10, 11, 15, 4, 5, 0, time.UTC)
)

func TestFileName(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		extension string
		want      string
	}{
		{name: "a plain title", title: "Sprint 42", extension: "csv", want: "RetroTide - Sprint 42 - 2026-10-11.csv"},
		{name: "Markdown", title: "Sprint 42", extension: "md", want: "RetroTide - Sprint 42 - 2026-10-11.md"},
		{name: "characters a file system refuses", title: `Q3: plan/review "final"`, extension: "json", want: "RetroTide - Q3- plan-review -final- - 2026-10-11.json"},
		{name: "control characters and runs of spaces", title: "Line\none  \t two", extension: "csv", want: "RetroTide - Line one two - 2026-10-11.csv"},
		{name: "letters beyond ASCII", title: "Rétro été", extension: "csv", want: "RetroTide - Rétro été - 2026-10-11.csv"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			name := export.FileName(tc.title, exportDate, tc.extension)

			// then
			assert.Equal(t, tc.want, name)
		})
	}
}

func TestFileName_DatesTheExportInUTC(t *testing.T) {
	// given half past midnight in London, still the evening before in UTC
	london := time.FixedZone("BST", 60*60)
	date := time.Date(2026, 10, 12, 0, 30, 0, 0, london)

	// when
	name := export.FileName("Sprint 42", date, "csv")

	// then
	assert.Equal(t, "RetroTide - Sprint 42 - 2026-10-11.csv", name)
}
