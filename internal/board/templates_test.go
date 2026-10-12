package board

import (
	"testing"

	"retrotide/internal/boardtemplate"
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplates_ListsTheCatalogueInOrder(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	listing := svc.Templates()

	// then every template is listed once, in catalogue order, without touching a repository
	catalogue := boardtemplate.All()
	require.Len(t, listing.Templates, len(catalogue))
	for i, template := range catalogue {
		assert.Equal(t, template.Key, listing.Templates[i].Key)
		assert.Equal(t, template.Title, listing.Templates[i].Name)
		assert.Equal(t, template.Description, listing.Templates[i].Description)
		assert.Len(t, listing.Templates[i].Columns, len(template.Columns))
	}
}

func TestTemplates_CarriesEachColumnOfATemplate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	listing := svc.Templates()

	// then
	var sailboat dto.TemplateResponse
	for _, template := range listing.Templates {
		if template.Key == "sailboat" {
			sailboat = template
		}
	}

	assert.Equal(t, dto.TemplateResponse{
		Key:         "sailboat",
		Name:        "Sailboat",
		Description: "Picture the team as a boat: what drives it, what drags it and what lies ahead.",
		Columns: []dto.TemplateColumnResponse{
			{Title: "Wind", Description: "What pushed us forward?", Colour: model.ColourGreen},
			{Title: "Anchors", Description: "What held us back?", Colour: model.ColourRed},
			{Title: "Rocks", Description: "What risks lie ahead?", Colour: model.ColourAmber},
			{Title: "Island", Description: "Where are we heading?", Colour: model.ColourBlue},
		},
	}, sailboat)
}

func TestTemplate_AnswersOneEntryOfTheListing(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	listing := svc.Templates()

	for _, want := range listing.Templates {
		t.Run(want.Key, func(t *testing.T) {
			// when
			got, err := svc.Template(want.Key)

			// then the single template has exactly the shape of its list entry
			require.NoError(t, err)
			assert.Equal(t, want, *got)
		})
	}
}

func TestTemplate_RefusesAnUnknownKey(t *testing.T) {
	tests := []string{"no-such-template", "", "Sailboat", "sailboat/"}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.Template(key)

			// then keys match exactly, so a differently cased key is unknown
			require.ErrorIs(t, err, ErrTemplateNotFound)
			assert.Nil(t, got)
		})
	}
}

func TestTemplates_IncludesTheBlankBoard(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	listing := svc.Templates()

	// then
	last := listing.Templates[len(listing.Templates)-1]
	assert.Equal(t, dto.TemplateResponse{
		Key:         "blank",
		Name:        "Blank",
		Description: "Start with one column, then rename it and add the columns you need.",
		Columns:     []dto.TemplateColumnResponse{{Title: "Notes", Description: "Anything worth raising.", Colour: model.ColourGrey}},
	}, last)
}
