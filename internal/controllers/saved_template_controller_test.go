package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board/live"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/savedtemplate"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	savedTemplateHarness struct {
		*testutil.Harness
		templates *savedtemplate.MockService
	}
)

var (
	savedTemplateUser  = uuid.MustParse("6f708192-a3b4-45c6-87d8-5d6e7f8091a2")
	savedTemplateOrg   = uuid.MustParse("708192a3-b4c5-46d7-98e9-6e7f8091a2b3")
	savedTemplateBoard = uuid.MustParse("8192a3b4-c5d6-47e8-a9fa-7f8091a2b3c4")
	savedTemplateID    = uuid.MustParse("92a3b4c5-d6e7-48f9-bafb-8091a2b3c4d5")
	savedTemplateTeam  = uuid.MustParse("a3b4c5d6-e7f8-490a-8b0c-91a2b3c4d5e6")

	savedTemplateResponse = dto.SavedTemplateResponse{
		ID:          savedTemplateID,
		Scope:       "team",
		TeamID:      new(savedTemplateTeam),
		TeamName:    "Platform",
		Name:        "Our retro",
		Description: "The format we like",
		Columns:     []dto.TemplateColumnResponse{{Title: "Went well", Description: "What went well?", Colour: model.ColourGreen}},
		CreatedAt:   "2026-10-12T09:30:00Z",
		CanDelete:   true,
	}
	savedTemplateJSON = `{"id":"` + savedTemplateID.String() + `","scope":"team","team_id":"` + savedTemplateTeam.String() + `","team_name":"Platform","name":"Our retro","description":"The format we like","columns":[{"title":"Went well","description":"What went well?","colour":"green"}],"created_at":"2026-10-12T09:30:00Z","can_delete":true}`
)

func newSavedTemplateHarness(t *testing.T) *savedTemplateHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	sh := &savedTemplateHarness{Harness: h, templates: savedtemplate.NewMockService(t)}

	s := &Service{SavedTemplateService: sh.templates, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllSavedTemplateRoutes())

	return sh
}

func savedTemplateBoardPath() string {
	return "/api/v1/boards/" + savedTemplateBoard.String() + "/saved-templates"
}

func (sh *savedTemplateHarness) save(body any) (int, string) {
	cookie := sh.SignIn(savedTemplateUser)
	sh.Authz.EXPECT().IsLocked(mock.Anything, savedTemplateUser).Return(false, nil)

	rec := sh.NewRequest(http.MethodPost, savedTemplateBoardPath()).WithCookie(cookie).WithJSONBody(body).Record()

	return rec.Code, rec.Body.String()
}

func TestSavedTemplateController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{SavedTemplateService: savedtemplate.NewMockService(t), Guards: h.Guards}

	// when the list is mounted exactly as production mounts it, so a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllSavedTemplateRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/orgs/:orgID/saved-templates",
		"HEAD /api/v1/orgs/:orgID/saved-templates",
		"POST /api/v1/boards/:boardID/saved-templates",
		"DELETE /api/v1/orgs/:orgID/saved-templates/:templateID",
	}, registeredRoutes(h.Engine))
}

func TestSavedTemplateController_OrganisationRoutesRunTheOrgPermissionFailureSuite(t *testing.T) {
	routes := []testutil.OrgPermissionRoute{
		orgRoute(http.MethodGet, savedTemplateOrg, "/saved-templates", orgauthz.PermOrgView, nil),
		orgRoute(http.MethodDelete, savedTemplateOrg, "/saved-templates/"+savedTemplateID.String(), orgauthz.PermOrgView, nil),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newSavedTemplateHarness(t).Harness
			}, route)
		})
	}
}

func TestSavedTemplateController_SaveRunsThePrincipalFailureSuite(t *testing.T) {
	route := testutil.AuthRoute{Method: http.MethodPost, Path: savedTemplateBoardPath(), Body: dto.SaveTemplateRequest{Name: "Our retro", Scope: "personal"}}

	testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newSavedTemplateHarness(t).Harness
	}, route)
}

func TestSavedTemplateController_ListAnswersTheViewersTemplates(t *testing.T) {
	// given
	sh := newSavedTemplateHarness(t)
	route := orgRoute(http.MethodGet, savedTemplateOrg, "/saved-templates", orgauthz.PermOrgView, nil)
	cookie := sh.SignInToOrg(route, savedTemplateUser)
	listing := &dto.SavedTemplateListResponse{Templates: []dto.SavedTemplateResponse{savedTemplateResponse}}
	sh.templates.EXPECT().List(mock.Anything, spec.OrganisationMemberKey{OrgID: savedTemplateOrg, UserID: savedTemplateUser}).Return(listing, nil)

	// when
	rec := sh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"templates":[`+savedTemplateJSON+`]}`, rec.Body.String())
}

func TestSavedTemplateController_ListMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "a deleted organisation", err: fmt.Errorf("list: %w", savedtemplate.ErrOrganisationNotFound), status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{name: "database", err: errors.New("the database is unreachable"), status: http.StatusInternalServerError, body: `{"error":"failed to list the saved templates"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sh := newSavedTemplateHarness(t)
			route := orgRoute(http.MethodGet, savedTemplateOrg, "/saved-templates", orgauthz.PermOrgView, nil)
			cookie := sh.SignInToOrg(route, savedTemplateUser)
			sh.templates.EXPECT().List(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := sh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestSavedTemplateController_SaveHandsTheRequestToTheServiceAndAnswersCreated(t *testing.T) {
	// given
	sh := newSavedTemplateHarness(t)
	body := dto.SaveTemplateRequest{Name: "Our retro", Description: "The format we like", Scope: "team"}
	command := savedtemplate.SaveCommand{
		Principal:   model.Principal{Kind: model.PrincipalUser, ID: savedTemplateUser},
		BoardID:     savedTemplateBoard,
		Name:        "Our retro",
		Description: "The format we like",
		Scope:       model.SavedTemplateTeam,
	}
	sh.templates.EXPECT().Save(mock.Anything, command).Return(new(savedTemplateResponse), nil)

	// when
	status, answer := sh.save(body)

	// then
	assert.Equal(t, http.StatusCreated, status)
	assert.JSONEq(t, savedTemplateJSON, answer)
}

func TestSavedTemplateController_SaveMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "a blank name", err: savedtemplate.ErrNameRequired, status: http.StatusBadRequest, body: `{"error":"` + savedtemplate.ErrNameRequired.Error() + `","code":"template_name_required"}`},
		{name: "a long name", err: savedtemplate.ErrNameTooLong, status: http.StatusBadRequest, body: `{"error":"` + savedtemplate.ErrNameTooLong.Error() + `","code":"template_name_too_long"}`},
		{name: "a long description", err: savedtemplate.ErrDescriptionTooLong, status: http.StatusBadRequest, body: `{"error":"` + savedtemplate.ErrDescriptionTooLong.Error() + `","code":"template_description_too_long"}`},
		{name: "an unknown scope", err: fmt.Errorf("%w: %q", savedtemplate.ErrInvalidScope, "org"), status: http.StatusBadRequest, body: `{"error":"` + savedtemplate.ErrInvalidScope.Error() + `","code":"invalid_template_scope"}`},
		{name: "a team template from an organisation board", err: savedtemplate.ErrNotTeamBoard, status: http.StatusConflict, body: `{"error":"` + savedtemplate.ErrNotTeamBoard.Error() + `","code":"not_team_board"}`},
		{name: "a board without columns", err: savedtemplate.ErrNoColumns, status: http.StatusConflict, body: `{"error":"` + savedtemplate.ErrNoColumns.Error() + `","code":"board_has_no_columns"}`},
		{name: "someone who may not manage the board", err: fmt.Errorf("save: %w", savedtemplate.ErrNotAllowed), status: http.StatusForbidden, body: `{"error":"` + savedtemplate.ErrNotAllowed.Error() + `","code":"save_not_allowed"}`},
		{name: "an unreachable board", err: fmt.Errorf("resolve: %w", live.ErrNoAccess), status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{name: "a fourth template on Free", err: fmt.Errorf("check: %w", &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitSavedTemplates, Allowed: 3, Current: 3}), status: http.StatusPaymentRequired, body: `{"error":"upgrade required","code":"upgrade_required","limit":"saved_templates","allowed":3,"current":3,"plan":"free"}`},
		{name: "database", err: errors.New("the database is unreachable"), status: http.StatusInternalServerError, body: `{"error":"failed to save the template"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sh := newSavedTemplateHarness(t)
			sh.templates.EXPECT().Save(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			status, answer := sh.save(dto.SaveTemplateRequest{Name: "Our retro", Scope: "personal"})

			// then
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, answer)
		})
	}
}

func TestSavedTemplateController_DeleteAnswersNoContent(t *testing.T) {
	// given
	sh := newSavedTemplateHarness(t)
	route := orgRoute(http.MethodDelete, savedTemplateOrg, "/saved-templates/"+savedTemplateID.String(), orgauthz.PermOrgView, nil)
	cookie := sh.SignInToOrg(route, savedTemplateUser)
	sh.templates.EXPECT().Delete(mock.Anything, spec.SavedTemplateRemoval{OrgID: savedTemplateOrg, ID: savedTemplateID, UserID: savedTemplateUser}).Return(nil)

	// when
	rec := sh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestSavedTemplateController_DeleteMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "an unknown or invisible template", err: fmt.Errorf("delete: %w", savedtemplate.ErrTemplateNotFound), status: http.StatusNotFound, body: `{"error":"template not found"}`},
		{name: "a template the viewer may see but not delete", err: fmt.Errorf("delete: %w", savedtemplate.ErrDeleteNotAllowed), status: http.StatusForbidden, body: `{"error":"` + savedtemplate.ErrDeleteNotAllowed.Error() + `","code":"delete_not_allowed"}`},
		{name: "database", err: errors.New("the database is unreachable"), status: http.StatusInternalServerError, body: `{"error":"failed to delete the template"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sh := newSavedTemplateHarness(t)
			route := orgRoute(http.MethodDelete, savedTemplateOrg, "/saved-templates/"+savedTemplateID.String(), orgauthz.PermOrgView, nil)
			cookie := sh.SignInToOrg(route, savedTemplateUser)
			sh.templates.EXPECT().Delete(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := sh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestSavedTemplateController_APathIDThatIsNotAUUIDIsRefused(t *testing.T) {
	// given
	sh := newSavedTemplateHarness(t)
	route := orgRoute(http.MethodDelete, savedTemplateOrg, "/saved-templates/not-a-uuid", orgauthz.PermOrgView, nil)
	cookie := sh.SignInToOrg(route, savedTemplateUser)

	// when
	rec := sh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

	// then the strict mock proves the service was never asked
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
