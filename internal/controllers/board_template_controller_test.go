package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
)

type (
	templateHarness struct {
		*testutil.Harness
		boards *board.MockService
	}
)

var (
	sailboatTemplate = dto.TemplateResponse{
		Key:         "sailboat",
		Name:        "Sailboat",
		Description: "Picture the team as a boat: what drives it, what drags it and what lies ahead.",
		Columns: []dto.TemplateColumnResponse{
			{Title: "Wind", Description: "What pushed us forward?", Colour: model.ColourGreen},
			{Title: "Anchors", Description: "What held us back?", Colour: model.ColourRed},
		},
	}
)

func newTemplateHarness(t *testing.T) *templateHarness {
	t.Helper()

	h := testutil.NewHarness(t)

	th := &templateHarness{Harness: h, boards: board.NewMockService(t)}

	s := &Service{BoardService: th.boards, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardTemplateRoutes())

	return th
}

func TestBoardTemplateController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardService: board.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardTemplateRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/templates",
		"HEAD /api/v1/templates",
		"GET /api/v1/templates/:key",
		"HEAD /api/v1/templates/:key",
	}, registeredRoutes(h.Engine))
}

func TestBoardTemplateController_AnswersAVisitorWithoutASessionWithTheCatalogue(t *testing.T) {
	// given no cookie at all, because the catalogue is the same for everyone and the template pages read it signed out
	th := newTemplateHarness(t)
	th.boards.EXPECT().Templates().Return(&dto.TemplateListResponse{Templates: []dto.TemplateResponse{
		{
			Key:         "mad-sad-glad",
			Name:        "Mad, Sad, Glad",
			Description: "Share how the sprint felt, from frustrations to wins.",
			Columns: []dto.TemplateColumnResponse{
				{Title: "Mad", Description: "What frustrated you?", Colour: model.ColourRed},
				{Title: "Sad", Description: "What disappointed you?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "blank",
			Name:        "Blank",
			Description: "Start with one column, then rename it and add the columns you need.",
			Columns:     []dto.TemplateColumnResponse{{Title: "Notes", Description: "Anything worth raising.", Colour: model.ColourGrey}},
		},
	}})

	// when
	rec := th.NewRequest(http.MethodGet, "/api/v1/templates").Record()

	// then the templates keep the service's order
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"templates":[
		{"key":"mad-sad-glad","name":"Mad, Sad, Glad","description":"Share how the sprint felt, from frustrations to wins.","columns":[
			{"title":"Mad","description":"What frustrated you?","colour":"red"},
			{"title":"Sad","description":"What disappointed you?","colour":"blue"}
		]},
		{"key":"blank","name":"Blank","description":"Start with one column, then rename it and add the columns you need.","columns":[
			{"title":"Notes","description":"Anything worth raising.","colour":"grey"}
		]}
	]}`, rec.Body.String())
}

func TestBoardTemplateController_AnswersAVisitorWithoutASessionWithOneTemplate(t *testing.T) {
	// given
	th := newTemplateHarness(t)
	th.boards.EXPECT().Template("sailboat").Return(&sailboatTemplate, nil)

	// when
	rec := th.NewRequest(http.MethodGet, "/api/v1/templates/sailboat").Record()

	// then the body is one entry of the list, not wrapped in a list
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"key":"sailboat","name":"Sailboat","description":"Picture the team as a boat: what drives it, what drags it and what lies ahead.","columns":[
		{"title":"Wind","description":"What pushed us forward?","colour":"green"},
		{"title":"Anchors","description":"What held us back?","colour":"red"}
	]}`, rec.Body.String())
}

func TestBoardTemplateController_AnswersAnUnknownKeyWith404(t *testing.T) {
	// given
	th := newTemplateHarness(t)
	th.boards.EXPECT().Template("no-such-template").Return(nil, fmt.Errorf("lookup: %w", board.ErrTemplateNotFound))

	// when
	rec := th.NewRequest(http.MethodGet, "/api/v1/templates/no-such-template").Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"template not found"}`, rec.Body.String())
}

func TestBoardTemplateController_ServesTheRealCatalogueThroughTheBoardService(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		status int
	}{
		{name: "a public template", path: "/api/v1/templates/start-stop-continue", status: http.StatusOK},
		{name: "the blank template the create form offers", path: "/api/v1/templates/blank", status: http.StatusOK},
		{name: "an unknown key", path: "/api/v1/templates/nope", status: http.StatusNotFound},
		{name: "a key in another case", path: "/api/v1/templates/Sailboat", status: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the real board service, whose template calls read nothing
			h := testutil.NewHarness(t)
			s := &Service{BoardService: board.NewService(board.Deps{}), Guards: h.Guards}
			RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardTemplateRoutes())

			// when
			rec := h.NewRequest(http.MethodGet, tc.path).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
		})
	}
}
