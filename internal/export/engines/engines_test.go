package engines_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"retrotide/internal/dto"
	"retrotide/internal/export"
	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	update = flag.Bool("update", false, "rewrite the golden files in testdata")
)

func namedBoard() dto.BoardExport {
	return dto.BoardExport{
		ID:          uuid.MustParse("8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b"),
		Title:       "Sprint 42",
		Context:     "Payments squad",
		TemplateKey: "went-well",
		Anonymity:   model.AnonymityNamed,
		Stage:       model.StageDiscuss,
		CreatedAt:   "2026-09-22T09:00:00.000Z",
		Columns: []dto.ExportColumn{
			{
				Title:       "Went well",
				Description: "What helped",
				Cards: []dto.ExportCard{
					{
						Body:      "Deploys were smooth, mostly",
						Author:    "Sam",
						Votes:     new(3),
						CreatedAt: "2026-09-22T10:01:02.512Z",
						UpdatedAt: "2026-09-22T10:03:00.250Z",
						Comments: []dto.ExportComment{
							{Body: "Agreed, the new pipeline helped", Author: "Ali", CreatedAt: "2026-09-22T10:04:00.000Z"},
						},
						ActionItems: []dto.ExportActionItem{
							{Title: "Thank @Hana for the pipeline", Assignee: "Hana", DueOn: new("2026-10-20"), Status: model.ActionItemOpen},
						},
						MergedCards: []dto.ExportCard{
							{
								Body:        "CI was fast\nand green",
								Author:      "Ali",
								Votes:       new(1),
								CreatedAt:   "2026-09-22T10:02:40.118Z",
								Comments:    []dto.ExportComment{},
								ActionItems: []dto.ExportActionItem{},
								MergedCards: []dto.ExportCard{},
							},
						},
					},
				},
			},
			{
				Title: "To improve",
				Cards: []dto.ExportCard{
					{
						Body:      `=HYPERLINK("http://evil.example")`,
						Author:    "Gil",
						Votes:     new(0),
						CreatedAt: "2026-09-22T10:06:00.000Z",
						Comments: []dto.ExportComment{
							{Body: "-1 from me", Author: "Sam", CreatedAt: "2026-09-22T10:07:00.000Z"},
						},
						ActionItems: []dto.ExportActionItem{
							{Title: "Escape <b>formulas</b> & *markdown*", Status: model.ActionItemDone},
						},
						MergedCards: []dto.ExportCard{},
					},
				},
			},
		},
		OtherActions: []dto.ExportActionItem{
			{Title: "Book the next retro", Status: model.ActionItemOpen},
		},
	}
}

func anonymousBoard() dto.BoardExport {
	return dto.BoardExport{
		ID:          uuid.MustParse("d4f7a8b9-c0d1-4e2f-9a3b-4c5d6e7f8a9b"),
		Title:       "Quarter review",
		TemplateKey: "blank",
		Anonymity:   model.AnonymityAnonymous,
		Stage:       model.StageWrite,
		CardsHidden: true,
		Columns: []dto.ExportColumn{
			{
				Title:       "Start",
				Description: "Things to begin",
				Cards: []dto.ExportCard{
					{
						Body:        "+ Pair more often",
						Hidden:      true,
						Comments:    []dto.ExportComment{},
						ActionItems: []dto.ExportActionItem{},
						MergedCards: []dto.ExportCard{},
					},
				},
				HiddenCount: 2,
			},
			{Title: "Stop", Cards: []dto.ExportCard{}, HiddenCount: 1},
			{Title: "Continue", Cards: []dto.ExportCard{}},
		},
		OtherActions: []dto.ExportActionItem{
			{Title: "Ask @Olu about the budget", Assignee: "Olu", Status: model.ActionItemOpen},
			{Title: "@channel fix flaky tests", DueOn: new("2026-10-01"), Status: model.ActionItemDone},
		},
	}
}

func singleCardBoard(columnTitle, body string) dto.BoardExport {
	return dto.BoardExport{
		Title: "Defuse",
		Columns: []dto.ExportColumn{
			{
				Title: columnTitle,
				Cards: []dto.ExportCard{
					{Body: body, Votes: new(0), Comments: []dto.ExportComment{}, ActionItems: []dto.ExportActionItem{}, MergedCards: []dto.ExportCard{}},
				},
			},
		},
		OtherActions: []dto.ExportActionItem{},
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.WriteFile(path, got, 0o644))

		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "run go test ./internal/export/engines -update to write %s", path)
	assert.Equal(t, strings.ReplaceAll(string(want), "\r\n", "\n"), string(got))
}

func TestWriteBoard_MatchesTheGoldenFiles(t *testing.T) {
	boards := []struct {
		name  string
		board dto.BoardExport
	}{
		{name: "named", board: namedBoard()},
		{name: "anonymous", board: anonymousBoard()},
	}

	factory := export.NewEngineFactory()

	for _, board := range boards {
		for _, format := range factory.Formats() {
			t.Run(board.name+"/"+string(format), func(t *testing.T) {
				// given
				created, err := factory.Create(format)
				require.NoError(t, err)

				var out bytes.Buffer

				// when
				err = created.WriteBoard(&out, board.board)

				// then
				require.NoError(t, err)
				assertGolden(t, board.name+"."+created.Extension(), out.Bytes())
			})
		}
	}
}

func TestEngines_DescribeTheirFiles(t *testing.T) {
	tests := []struct {
		engine      engine.Engine
		format      engine.Format
		contentType string
		extension   string
	}{
		{engine: engines.CSV{}, format: engine.FormatCSV, contentType: "text/csv; charset=utf-8", extension: "csv"},
		{engine: engines.Markdown{}, format: engine.FormatMarkdown, contentType: "text/markdown; charset=utf-8", extension: "md"},
		{engine: engines.JSON{}, format: engine.FormatJSON, contentType: "application/json", extension: "json"},
	}

	for _, tc := range tests {
		t.Run(string(tc.format), func(t *testing.T) {
			// then
			assert.Equal(t, tc.format, tc.engine.Format())
			assert.Equal(t, tc.contentType, tc.engine.ContentType())
			assert.Equal(t, tc.extension, tc.engine.Extension())
		})
	}
}

func TestCSV_DefusesEveryCellThatASpreadsheetWouldRunAsAFormula(t *testing.T) {
	tests := []struct {
		name   string
		column string
		body   string
		want   string
	}{
		{name: "equals", column: "Ideas", body: "=1+1", want: "Ideas\n'=1+1 (0 votes)\n"},
		{name: "plus", column: "Ideas", body: "+1", want: "Ideas\n'+1 (0 votes)\n"},
		{name: "minus", column: "Ideas", body: "-1", want: "Ideas\n'-1 (0 votes)\n"},
		{name: "at", column: "Ideas", body: "@SUM(A1)", want: "Ideas\n'@SUM(A1) (0 votes)\n"},
		{name: "tab", column: "Ideas", body: "\t=1", want: "Ideas\n'\t=1 (0 votes)\n"},
		{name: "carriage return", column: "Ideas", body: "\r=1", want: "Ideas\n\"'\r=1 (0 votes)\"\n"},
		{name: "a column title", column: "=SUM(A1)", body: "fine", want: "'=SUM(A1)\nfine (0 votes)\n"},
		{name: "a formula later in the cell", column: "Ideas", body: "x=1", want: "Ideas\nx=1 (0 votes)\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var out bytes.Buffer

			// when
			err := engines.CSV{}.WriteBoard(&out, singleCardBoard(tc.column, tc.body))

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, out.String())
		})
	}
}

func TestMarkdown_EscapesMarkdownAndHTMLInEveryUserString(t *testing.T) {
	// given
	board := singleCardBoard("<script>alert(1)</script>", "**bold** [link](javascript:x) `code` # not a heading")
	board.Title = "# Title & <img src=x>"

	// when
	var out bytes.Buffer
	err := engines.Markdown{}.WriteBoard(&out, board)

	// then
	require.NoError(t, err)
	assert.Equal(t, "# \\# Title &amp; &lt;img src\\=x&gt;\n"+
		"\n## &lt;script&gt;alert\\(1\\)&lt;/script&gt;\n"+
		"\n- \\*\\*bold\\*\\* \\[link\\]\\(javascript:x\\) \\`code\\` \\# not a heading (0 votes)\n", out.String())
}

func TestJSON_WriteDocumentKeepsMarkupReadable(t *testing.T) {
	// given
	var out bytes.Buffer

	// when
	err := engines.JSON{}.WriteDocument(&out, map[string]string{"title": "<b>Q3</b> & more"})

	// then
	require.NoError(t, err)
	assert.Equal(t, "{\n  \"title\": \"<b>Q3</b> & more\"\n}\n", out.String())
}
