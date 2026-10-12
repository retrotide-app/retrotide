package boardtemplate

import (
	"regexp"
	"slices"
	"testing"
	"unicode/utf8"

	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	keyPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

func TestAll_HoldsTheFortyBuiltInTemplatesAndTheBlankBoard(t *testing.T) {
	// when
	templates := All()

	// then
	require.Len(t, templates, 41)
	keys := make([]string, 0, len(templates))
	for _, template := range templates {
		keys = append(keys, template.Key)
	}
	assert.Equal(t, []string{
		"start-stop-continue",
		"mad-sad-glad",
		"went-well",
		"four-ls",
		"sailboat",
		"starfish",
		"kalm",
		"daki",
		"rose-thorn-bud",
		"wrap",
		"lean-coffee",
		"three-little-pigs",
		"hot-air-balloon",
		"speed-car",
		"mountain-climb",
		"desert-island",
		"weather-report",
		"traffic-lights",
		"plus-delta",
		"keep-drop-add",
		"good-bad-ugly",
		"fun-done-learn",
		"highs-lows",
		"timeline",
		"learning-matrix",
		"what-so-what-now-what",
		"lessons-learned",
		"post-mortem",
		"incident-review",
		"pre-mortem",
		"swot",
		"pros-cons",
		"six-thinking-hats",
		"kudos",
		"daily-standup",
		"kanban",
		"quarter-in-review",
		"expectations-reality",
		"brainstorm",
		"hopes-and-fears",
		"blank",
	}, keys)
}

func TestLookup_TheBlankBoardStartsWithOneColumn(t *testing.T) {
	// when
	template, ok := Lookup("blank")

	// then the creator renames this column or adds more, as on any board
	require.True(t, ok)
	assert.Equal(t, "Blank", template.Title)
	assert.Equal(t, []Column{{Title: "Notes", Description: "Anything worth raising.", Colour: model.ColourGrey}}, template.Columns)
}

func TestAll_EveryTemplateFitsTheSchema(t *testing.T) {
	for _, template := range All() {
		t.Run(template.Key, func(t *testing.T) {
			// then the key is safe in a /templates/:key path and the columns satisfy board_columns' checks
			assert.Regexp(t, keyPattern, template.Key)
			assert.NotEmpty(t, template.Title)
			assert.NotEmpty(t, template.Description)
			require.NotEmpty(t, template.Columns)

			titles := make([]string, 0, len(template.Columns))
			for _, column := range template.Columns {
				assert.NotEmpty(t, column.Title)
				assert.LessOrEqual(t, utf8.RuneCountInString(column.Title), model.BoardColumnTitleMaxRunes)
				assert.NotEmpty(t, column.Description)
				assert.Contains(t, model.Colours, column.Colour)
				titles = append(titles, column.Title)
			}

			slices.Sort(titles)
			assert.Len(t, slices.Compact(titles), len(template.Columns))
		})
	}
}

func TestLookup_FindsATemplateByKey(t *testing.T) {
	// when
	template, ok := Lookup("sailboat")

	// then
	require.True(t, ok)
	assert.Equal(t, "Sailboat", template.Title)
	assert.Equal(t, []Column{
		{Title: "Wind", Description: "What pushed us forward?", Colour: model.ColourGreen},
		{Title: "Anchors", Description: "What held us back?", Colour: model.ColourRed},
		{Title: "Rocks", Description: "What risks lie ahead?", Colour: model.ColourAmber},
		{Title: "Island", Description: "Where are we heading?", Colour: model.ColourBlue},
	}, template.Columns)
}

func TestLookup_AnUnknownKeyFindsNothing(t *testing.T) {
	for _, key := range []string{"", "Sailboat", "no-such-template"} {
		// when
		template, ok := Lookup(key)

		// then
		assert.False(t, ok, key)
		assert.Equal(t, Template{}, template)
	}
}

func TestLookup_ACallerCannotChangeTheCatalogue(t *testing.T) {
	// given
	template, ok := Lookup("mad-sad-glad")
	require.True(t, ok)

	// when
	template.Columns[0].Title = "Changed"

	// then
	again, ok := Lookup("mad-sad-glad")
	require.True(t, ok)
	assert.Equal(t, "Mad", again.Columns[0].Title)
}

func TestAll_ACallerCannotChangeTheCatalogue(t *testing.T) {
	// given
	templates := All()

	// when
	templates[0].Title = "Changed"
	templates[0].Columns[0].Title = "Changed"

	// then
	again := All()
	assert.Equal(t, "Start, Stop, Continue", again[0].Title)
	assert.Equal(t, "Start", again[0].Columns[0].Title)
}
