package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/actionitem"
	"retrotide/internal/board"
	"retrotide/internal/board/actionitems"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	actionItemHarness struct {
		*testutil.Harness
		boards   *board.MockService
		commands *actionitems.MockService
		items    *actionitem.MockService
	}
)

var (
	boardItem       = uuid.MustParse("5e6f8091-a2b3-4c4d-b5e6-5c6d7e8f90a1")
	boardItemRoute  = "/action-items/" + boardItem.String()
	boardItemFields = dto.ActionItemFieldsRequest{
		BoardCommandRequest: boardOpBody,
		Title:               "Fix the pipeline with @Alice",
		AssigneeUserID:      new(boardUser),
		DueOn:               new("2026-10-20"),
		Mentions:            []uuid.UUID{boardUser},
	}
	boardItemCommandFields = actionitems.Fields{
		Title:          "Fix the pipeline with @Alice",
		AssigneeUserID: new(boardUser),
		DueOn:          new("2026-10-20"),
		Mentions:       []uuid.UUID{boardUser},
	}
)

func newActionItemHarness(t *testing.T) *actionItemHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	ah := &actionItemHarness{Harness: h, boards: board.NewMockService(t), commands: actionitems.NewMockService(t), items: actionitem.NewMockService(t)}

	s := &Service{BoardService: ah.boards, BoardActionItems: ah.commands, ActionItemService: ah.items, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllActionItemRoutes())

	return ah
}

func (ah *actionItemHarness) record(method string, path string, body any) (int, string) {
	cookie := ah.SignIn(boardUser)
	if method != http.MethodGet && method != http.MethodHead {
		ah.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)
	}

	request := ah.NewRequest(method, path).WithCookie(cookie)
	if body != nil {
		request = request.WithJSONBody(body)
	}

	rec := request.Record()

	return rec.Code, rec.Body.String()
}

func TestActionItemController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardService: board.NewMockService(t), BoardActionItems: actionitems.NewMockService(t), ActionItemService: actionitem.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllActionItemRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/orgs/:orgID/action-items",
		"HEAD /api/v1/orgs/:orgID/action-items",
		"GET /api/v1/auth/mentions",
		"HEAD /api/v1/auth/mentions",
		"GET /api/v1/boards/:boardID/people",
		"HEAD /api/v1/boards/:boardID/people",
		"POST /api/v1/boards/:boardID/action-items",
		"PUT /api/v1/boards/:boardID/action-items/:itemID",
		"POST /api/v1/boards/:boardID/action-items/:itemID/status",
		"DELETE /api/v1/boards/:boardID/action-items/:itemID",
		"POST /api/v1/boards/:boardID/previous-actions/:itemID/review",
	}, registeredRoutes(h.Engine))
}

func TestActionItemController_EveryBoardRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodGet, Path: boardPath("/people")},
		{Method: http.MethodPost, Path: boardPath("/action-items"), Body: dto.CreateActionItemRequest{ActionItemFieldsRequest: boardItemFields, ID: boardItem, SourceCardID: new(boardCard)}},
		{Method: http.MethodPut, Path: boardPath(boardItemRoute), Body: boardItemFields},
		{Method: http.MethodPost, Path: boardPath(boardItemRoute + "/status"), Body: dto.ActionItemStatusRequest{BoardCommandRequest: boardOpBody, Status: model.ActionItemDone}},
		{Method: http.MethodDelete, Path: boardPath(boardItemRoute), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath("/previous-actions/" + boardItem.String() + "/review"), Body: dto.ReviewActionItemRequest{BoardCommandRequest: boardOpBody, Outcome: model.ReviewDone}},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newActionItemHarness(t).Harness
			}, route)
		})
	}
}

func TestActionItemController_ThePageRunsTheOrgPermissionFailureSuite(t *testing.T) {
	testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newActionItemHarness(t).Harness
	}, orgRoute(http.MethodGet, boardOrg, "/action-items", orgauthz.PermOrgView, nil))
}

func TestActionItemController_MentionsRunTheAuthFailureSuite(t *testing.T) {
	testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newActionItemHarness(t).Harness
	}, testutil.AuthRoute{Method: http.MethodGet, Path: "/api/v1/auth/mentions"})
}

func TestActionItemController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	type itemCase struct {
		name   string
		method string
		path   string
		body   any
		expect func(ah *actionItemHarness)
	}

	tests := []itemCase{
		{
			name:   "add an action item from a card",
			method: http.MethodPost,
			path:   boardPath("/action-items"),
			body:   dto.CreateActionItemRequest{ActionItemFieldsRequest: boardItemFields, ID: boardItem, SourceCardID: new(boardCard)},
			expect: func(ah *actionItemHarness) {
				add := actionitems.AddCommand{Command: boardCommandOf, Fields: boardItemCommandFields, ItemID: boardItem, SourceCardID: new(boardCard)}
				ah.commands.EXPECT().Add(mock.Anything, add).Return(boardVersioned, nil)
			},
		},
		{
			name:   "edit an action item",
			method: http.MethodPut,
			path:   boardPath(boardItemRoute),
			body:   boardItemFields,
			expect: func(ah *actionItemHarness) {
				edit := actionitems.EditCommand{Command: boardCommandOf, Fields: boardItemCommandFields, ItemID: boardItem}
				ah.commands.EXPECT().Edit(mock.Anything, edit).Return(boardVersioned, nil)
			},
		},
		{
			name:   "mark an action item done",
			method: http.MethodPost,
			path:   boardPath(boardItemRoute + "/status"),
			body:   dto.ActionItemStatusRequest{BoardCommandRequest: boardOpBody, Status: model.ActionItemDone},
			expect: func(ah *actionItemHarness) {
				mark := actionitems.StatusCommand{Command: boardCommandOf, ItemID: boardItem, Status: model.ActionItemDone}
				ah.commands.EXPECT().SetStatus(mock.Anything, mark).Return(boardVersioned, nil)
			},
		},
		{
			name:   "delete an action item",
			method: http.MethodDelete,
			path:   boardPath(boardItemRoute),
			body:   boardOpBody,
			expect: func(ah *actionItemHarness) {
				ah.commands.EXPECT().Delete(mock.Anything, actionitems.ItemCommand{Command: boardCommandOf, ItemID: boardItem}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "carry a previous action over",
			method: http.MethodPost,
			path:   boardPath("/previous-actions/" + boardItem.String() + "/review"),
			body:   dto.ReviewActionItemRequest{BoardCommandRequest: boardOpBody, Outcome: model.ReviewCarried},
			expect: func(ah *actionItemHarness) {
				review := actionitems.ReviewCommand{Command: boardCommandOf, ItemID: boardItem, Outcome: model.ReviewCarried}
				ah.commands.EXPECT().Review(mock.Anything, review).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ah := newActionItemHarness(t)
			tc.expect(ah)

			// when
			status, body := ah.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestActionItemController_AddRefusesAnIDThatIsNotV4(t *testing.T) {
	// given
	ah := newActionItemHarness(t)

	// when
	status, _ := ah.record(http.MethodPost, boardPath("/action-items"), dto.CreateActionItemRequest{ActionItemFieldsRequest: boardItemFields, ID: boardV7CardID, SourceCardID: new(boardCard)})

	// then the strict mock proves the service was never asked
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestActionItemController_AddWithoutACardAnswersCardRequired(t *testing.T) {
	// given
	ah := newActionItemHarness(t)
	add := actionitems.AddCommand{Command: boardCommandOf, Fields: boardItemCommandFields, ItemID: boardItem}
	ah.commands.EXPECT().Add(mock.Anything, add).Return(nil, actionitems.ErrCardRequired)

	// when
	status, body := ah.record(http.MethodPost, boardPath("/action-items"), dto.CreateActionItemRequest{ActionItemFieldsRequest: boardItemFields, ID: boardItem})

	// then
	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"an action item is made from a card","code":"card_required"}`, body)
}

func TestActionItemController_PeopleAnswersTheBoardsPeople(t *testing.T) {
	// given
	ah := newActionItemHarness(t)
	people := &dto.BoardPeopleResponse{People: []dto.PersonView{{UserID: boardUser, DisplayName: "Alice"}}}
	ah.boards.EXPECT().People(mock.Anything, spec.BoardAccessQuery{Principal: boardPrincipal, BoardID: boardID}).Return(people, nil)

	// when
	status, body := ah.record(http.MethodGet, boardPath("/people"), nil)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, people), body)
}

func TestActionItemController_MapsEveryActionItemRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: actionitems.ErrTitleRequired, status: http.StatusBadRequest, body: `{"error":"an action item needs a title","code":"action_item_title_required"}`},
		{err: actionitems.ErrTitleTooLong, status: http.StatusBadRequest, body: `{"error":"an action item title can be at most 500 characters","code":"action_item_title_too_long"}`},
		{err: actionitems.ErrInvalidDueDate, status: http.StatusBadRequest, body: `{"error":"a due date must be a calendar date written as YYYY-MM-DD","code":"invalid_due_date"}`},
		{err: board.ErrTooManyMentions, status: http.StatusBadRequest, body: `{"error":"an action item, card or comment can mention at most 20 people","code":"too_many_mentions"}`},
		{err: board.ErrNotBoardMember, status: http.StatusBadRequest, body: `{"error":"an assignee or a mention must be a member who can open the board","code":"not_board_member"}`},
		{err: actionitems.ErrCardRequired, status: http.StatusBadRequest, body: `{"error":"an action item is made from a card","code":"card_required"}`},
		{err: board.ErrIDConflict, status: http.StatusConflict, body: `{"error":"that ID is already taken by another command","code":"id_conflict"}`},
		{err: policy.ErrActionItemsOutOfStage, status: http.StatusConflict, body: `{"error":"action items can be added, edited and marked only in the Actions stage","code":"wrong_stage"}`},
		{err: policy.ErrActionItemNotFound, status: http.StatusNotFound, body: `{"error":"action item not found"}`},
		{err: policy.ErrCardNotFound, status: http.StatusNotFound, body: `{"error":"card not found"}`},
		{err: policy.ErrMembersOnly, status: http.StatusForbidden, body: `{"error":"only members of the board's organisation can do that","code":"members_only"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			ah := newActionItemHarness(t)
			ah.commands.EXPECT().Add(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("add the action item: %w", tc.err))

			// when
			status, body := ah.record(http.MethodPost, boardPath("/action-items"), dto.CreateActionItemRequest{ActionItemFieldsRequest: boardItemFields, ID: boardItem, SourceCardID: new(boardCard)})

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestActionItemController_MapsTheStatusAndReviewRefusals(t *testing.T) {
	// given
	ah := newActionItemHarness(t)
	ah.commands.EXPECT().SetStatus(mock.Anything, mock.Anything).Return(nil, actionitems.ErrInvalidStatus)
	ah.commands.EXPECT().Review(mock.Anything, mock.Anything).Return(nil, actionitems.ErrInvalidReviewOutcome)

	// when
	markStatus, markBody := ah.record(http.MethodPost, boardPath(boardItemRoute+"/status"), dto.ActionItemStatusRequest{BoardCommandRequest: boardOpBody, Status: "finished"})
	reviewStatus, reviewBody := ah.record(http.MethodPost, boardPath("/previous-actions/"+boardItem.String()+"/review"), dto.ReviewActionItemRequest{BoardCommandRequest: boardOpBody, Outcome: "ignored"})

	// then
	assert.Equal(t, http.StatusBadRequest, markStatus)
	assert.JSONEq(t, `{"error":"an action item's status must be open, done or dropped","code":"invalid_action_item_status"}`, markBody)
	assert.Equal(t, http.StatusBadRequest, reviewStatus)
	assert.JSONEq(t, `{"error":"a review must mark the action done, carried or dropped","code":"invalid_review_outcome"}`, reviewBody)
}

func TestActionItemController_ThePageReadsItsFiltersIntoTheQuery(t *testing.T) {
	other := uuid.MustParse("6f708192-a3b4-4c5d-a6e7-6d7e8f90a1b2")
	done := model.ActionItemDone
	viewer := spec.OrganisationMemberKey{OrgID: boardOrg, UserID: boardUser}

	tests := []struct {
		name  string
		query string
		want  actionitem.ListQuery
	}{
		{name: "no filter", query: "", want: actionitem.ListQuery{Viewer: viewer}},
		{name: "my finished items", query: "?status=done&assignee=me", want: actionitem.ListQuery{Viewer: viewer, Status: &done, AssigneeUserID: new(boardUser)}},
		{name: "nobody's items on organisation boards", query: "?assignee=none&team=organisation", want: actionitem.ListQuery{Viewer: viewer, Unassigned: true, OrganisationBoards: true}},
		{name: "someone's items on one team, paged", query: "?assignee=" + other.String() + "&team=" + boardTeam.String() + "&limit=20&offset=40", want: actionitem.ListQuery{Viewer: viewer, AssigneeUserID: new(other), TeamID: new(boardTeam), Limit: 20, Offset: 40}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ah := newActionItemHarness(t)
			route := orgRoute(http.MethodGet, boardOrg, "/action-items", orgauthz.PermOrgView, nil)
			cookie := ah.SignInToOrg(route, boardUser)
			listing := &dto.ActionItemListResponse{ActionItems: []dto.ActionItemListItem{}, Limit: 50}
			ah.items.EXPECT().List(mock.Anything, tc.want).Return(listing, nil)

			// when
			rec := ah.NewRequest(http.MethodGet, route.Path+tc.query).WithCookie(cookie).Record()

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.JSONEq(t, mustJSON(t, listing), rec.Body.String())
		})
	}
}

func TestActionItemController_ThePageRefusesAMalformedFilter(t *testing.T) {
	tests := []struct {
		query string
		body  string
	}{
		{query: "?assignee=someone", body: `{"error":"invalid assignee"}`},
		{query: "?team=everyone", body: `{"error":"invalid team"}`},
		{query: "?limit=lots", body: `{"error":"invalid limit"}`},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			// given
			ah := newActionItemHarness(t)
			route := orgRoute(http.MethodGet, boardOrg, "/action-items", orgauthz.PermOrgView, nil)
			cookie := ah.SignInToOrg(route, boardUser)

			// when
			rec := ah.NewRequest(http.MethodGet, route.Path+tc.query).WithCookie(cookie).Record()

			// then the strict mock proves the service was never asked
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestActionItemController_ThePageMapsItsRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: actionitem.ErrOrganisationNotFound, status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{err: actionitem.ErrInvalidStatus, status: http.StatusBadRequest, body: `{"error":"an action item's status must be open, done or dropped","code":"invalid_action_item_status"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to list the action items"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			ah := newActionItemHarness(t)
			route := orgRoute(http.MethodGet, boardOrg, "/action-items", orgauthz.PermOrgView, nil)
			cookie := ah.SignInToOrg(route, boardUser)
			ah.items.EXPECT().List(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("list: %w", tc.err))

			// when
			rec := ah.NewRequest(http.MethodGet, route.Path).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestActionItemController_MentionsAnswerTheSignedInUsersMentions(t *testing.T) {
	// given
	ah := newActionItemHarness(t)
	mentions := &dto.MentionListResponse{Mentions: []dto.MentionView{
		{
			Kind:                  model.MentionActionItem,
			ActionItemMentionView: &dto.ActionItemMentionView{ActionItemID: boardItem, Title: "Fix the pipeline", Status: model.ActionItemOpen},
			OrgID:                 boardOrg,
			OrgSlug:               "acme",
			OrgName:               "Acme",
			BoardID:               new(boardID),
			BoardTitle:            "Sprint 12",
		},
		{
			Kind:            model.MentionComment,
			TextMentionView: &dto.TextMentionView{CardID: boardCard, CommentID: new(boardComment), Body: "Agreed, @Sam"},
			OrgID:           boardOrg,
			OrgSlug:         "acme",
			OrgName:         "Acme",
			BoardID:         new(boardID),
			BoardTitle:      "Sprint 12",
		},
	}}
	ah.items.EXPECT().Mentions(mock.Anything, boardUser).Return(mentions, nil)

	// when
	status, body := ah.record(http.MethodGet, "/api/v1/auth/mentions", nil)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, mentions), body)
	assert.Contains(t, body, `"kind":"comment","card_id":"`+boardCard.String()+`"`)
}
