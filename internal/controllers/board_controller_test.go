package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"testing/synctest"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	boardHarness struct {
		*testutil.Harness
		boards *board.MockService
		live   *live.MockService
	}

	boardCommandCase struct {
		name   string
		method string
		path   string
		body   any
		expect func(bh *boardHarness)
	}
)

var (
	boardUser     = uuid.MustParse("8c9d0e1f-2a3b-4c4d-9e5f-6a7b8c9d0e1f")
	boardOrg      = uuid.MustParse("5d6e7f80-91a2-43b4-b5c6-3b4c5d6e7f80")
	boardTeam     = uuid.MustParse("9d0e1f2a-3b4c-4d5e-8f60-7b8c9d0e1f2a")
	boardID       = uuid.MustParse("0e1f2a3b-4c5d-4e6f-a071-8c9d0e1f2a3b")
	boardColumn   = uuid.MustParse("1f2a3b4c-5d6e-4f70-b182-9d0e1f2a3b4c")
	boardAnchor   = uuid.MustParse("2a3b4c5d-6e7f-4081-8293-0e1f2a3b4c5d")
	boardCard     = uuid.MustParse("3b4c5d6e-7f80-4192-93a4-1f2a3b4c5d6e")
	boardOp       = uuid.MustParse("4c5d6e7f-8091-42a3-a4b5-2a3b4c5d6e7f")
	boardV7CardID = uuid.MustParse("019a6b2c-3d4e-7f50-8a1b-2c3d4e5f6a7b")

	boardPrincipal = model.Principal{Kind: model.PrincipalUser, ID: boardUser}
	boardCommandOf = board.Command{Principal: boardPrincipal, BoardID: boardID, OpID: boardOp}
	boardOpBody    = dto.BoardCommandRequest{OpID: boardOp}
	boardCardBody  = dto.CreateCardRequest{BoardCommandRequest: boardOpBody, ID: boardCard, ColumnID: boardColumn, Body: "Deploys were smooth"}
	boardVersioned = &dto.BoardCommandResponse{Version: 7}
	boardColour    = model.Colour("green")
)

func newBoardHarness(t *testing.T) *boardHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	bh := &boardHarness{Harness: h, boards: board.NewMockService(t), live: live.NewMockService(t)}

	s := &Service{BoardService: bh.boards, BoardLive: bh.live, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), boardRouteLists(s))

	return bh
}

func boardRouteLists(s *Service) []FRoute {
	var all []FRoute
	all = append(all, s.getAllBoardRoutes()...)
	all = append(all, s.getAllBoardColumnRoutes()...)
	all = append(all, s.getAllBoardCardRoutes()...)
	all = append(all, s.getAllBoardVoteRoutes()...)
	all = append(all, s.getAllBoardCommentRoutes()...)
	all = append(all, s.getAllBoardFacilitationRoutes()...)

	return all
}

func boardPath(rest string) string {
	return "/api/v1/boards/" + boardID.String() + rest
}

func (bh *boardHarness) record(method string, path string, body any) (int, string) {
	cookie := bh.SignIn(boardUser)
	if method != http.MethodGet && method != http.MethodHead {
		bh.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)
	}

	request := bh.NewRequest(method, path).WithCookie(cookie)
	if body != nil {
		request = request.WithJSONBody(body)
	}

	rec := request.Record()

	return rec.Code, rec.Body.String()
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err)

	return string(data)
}

func TestBoardController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardService: board.NewMockService(t), BoardLive: live.NewMockService(t), Guards: h.Guards}

	// when the lists are mounted exactly as production mounts them, so a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), boardRouteLists(s))

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/orgs/:orgID/boards",
		"HEAD /api/v1/orgs/:orgID/boards",
		"POST /api/v1/orgs/:orgID/boards",
		"GET /api/v1/teams/:teamID/boards",
		"HEAD /api/v1/teams/:teamID/boards",
		"POST /api/v1/teams/:teamID/boards",
		"GET /api/v1/boards/:boardID",
		"HEAD /api/v1/boards/:boardID",
		"POST /api/v1/boards/:boardID/participants/me",
		"PUT /api/v1/boards/:boardID/context",
		"GET /api/v1/boards/:boardID/sharing",
		"HEAD /api/v1/boards/:boardID/sharing",
		"PUT /api/v1/boards/:boardID/sharing/link-access",
		"PUT /api/v1/boards/:boardID/sharing/password",
		"POST /api/v1/boards/:boardID/sharing/rotate",
		"POST /api/v1/boards/:boardID/columns",
		"PUT /api/v1/boards/:boardID/columns/:columnID",
		"POST /api/v1/boards/:boardID/columns/:columnID/move",
		"POST /api/v1/boards/:boardID/columns/:columnID/clear",
		"DELETE /api/v1/boards/:boardID/columns/:columnID",
		"POST /api/v1/boards/:boardID/cards",
		"PUT /api/v1/boards/:boardID/cards/:cardID",
		"PUT /api/v1/boards/:boardID/cards/:cardID/colour",
		"POST /api/v1/boards/:boardID/cards/:cardID/move",
		"DELETE /api/v1/boards/:boardID/cards/:cardID",
		"POST /api/v1/boards/:boardID/cards/:cardID/merge",
		"POST /api/v1/boards/:boardID/cards/:cardID/unmerge",
		"POST /api/v1/boards/:boardID/cards/:cardID/votes",
		"DELETE /api/v1/boards/:boardID/cards/:cardID/votes",
		"POST /api/v1/boards/:boardID/votes/reset",
		"PUT /api/v1/boards/:boardID/settings",
		"POST /api/v1/boards/:boardID/cards/:cardID/comments",
		"PUT /api/v1/boards/:boardID/comments/:commentID",
		"DELETE /api/v1/boards/:boardID/comments/:commentID",
		"POST /api/v1/boards/:boardID/reveal",
		"POST /api/v1/boards/:boardID/hide",
		"POST /api/v1/boards/:boardID/timer",
		"POST /api/v1/boards/:boardID/close",
		"POST /api/v1/boards/:boardID/stage",
		"PUT /api/v1/boards/:boardID/participants/:participantID/role",
		"DELETE /api/v1/boards/:boardID/participants/:participantID",
	}, registeredRoutes(h.Engine))
}

func TestBoardController_TeamRoutesRunTheTeamPermissionFailureSuite(t *testing.T) {
	routes := []testutil.TeamPermissionRoute{
		teamRoute(http.MethodGet, boardTeam, "/boards", orgauthz.PermTeamView, nil),
		teamRoute(http.MethodPost, boardTeam, "/boards", orgauthz.PermTeamCreateBoard, dto.CreateBoardRequest{Title: "Sprint 12", TemplateKey: "mad-sad-glad"}),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunTeamPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newBoardHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardController_OrganisationBoardsRunTheOrgPermissionFailureSuite(t *testing.T) {
	routes := []testutil.OrgPermissionRoute{
		orgRoute(http.MethodGet, boardOrg, "/boards", orgauthz.PermOrgView, nil),
		orgRoute(http.MethodPost, boardOrg, "/boards", orgauthz.PermOrgCreateBoard, dto.CreateOrgBoardRequest{Title: "All hands", TemplateKey: "mad-sad-glad"}),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newBoardHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardController_CreateOrganisationBoardHandsTheRequestToTheService(t *testing.T) {
	tests := []struct {
		name   string
		teamID *uuid.UUID
	}{
		{name: "a board for the whole organisation", teamID: nil},
		{name: "a board shared with one team", teamID: new(boardTeam)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			body := dto.CreateOrgBoardRequest{
				Title: "All hands", TemplateKey: "mad-sad-glad", Context: "Quarter close", Anonymity: model.AnonymityNamed,
				TeamID: tc.teamID,
			}
			route := orgRoute(http.MethodPost, boardOrg, "/boards", orgauthz.PermOrgCreateBoard, body)
			cookie := bh.SignInToOrg(route, boardUser)

			created := &dto.BoardSnapshot{BoardID: boardID, Columns: []dto.ColumnView{}, Cards: []dto.CardView{}, HiddenCounts: map[uuid.UUID]int{}, Participants: []dto.ParticipantView{}}
			command := board.CreateCommand{Principal: boardPrincipal, OrgID: boardOrg, TeamID: tc.teamID, Title: body.Title, TemplateKey: body.TemplateKey, Context: body.Context, Anonymity: body.Anonymity}
			bh.boards.EXPECT().Create(mock.Anything, command).Return(created, nil)

			// when
			rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).WithJSONBody(body).Record()

			// then
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.JSONEq(t, mustJSON(t, created), rec.Body.String())
		})
	}
}

func TestBoardController_CreateOrganisationBoardMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "a team of another organisation", err: fmt.Errorf("create: %w", board.ErrTeamNotFound), status: http.StatusNotFound, body: `{"error":"team not found"}`},
		{name: "a team the viewer may not create on", err: fmt.Errorf("create: %w", board.ErrCreateNotAllowed), status: http.StatusForbidden, body: `{"error":"you cannot create a board there","code":"create_not_allowed"}`},
		{name: "an archived team", err: board.ErrTeamArchived, status: http.StatusConflict, body: `{"error":"` + board.ErrTeamArchived.Error() + `","code":"team_archived"}`},
		{name: "a blank title", err: board.ErrTitleRequired, status: http.StatusBadRequest, body: `{"error":"` + board.ErrTitleRequired.Error() + `","code":"board_title_required"}`},
		{name: "database", err: errors.New("the database is unreachable"), status: http.StatusInternalServerError, body: `{"error":"failed to create the board"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			body := dto.CreateOrgBoardRequest{Title: "All hands", TemplateKey: "mad-sad-glad", TeamID: new(boardTeam)}
			route := orgRoute(http.MethodPost, boardOrg, "/boards", orgauthz.PermOrgCreateBoard, body)
			cookie := bh.SignInToOrg(route, boardUser)
			bh.boards.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).WithJSONBody(body).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestBoardController_ListOrganisationBoardsAnswersTheViewersBoards(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	route := orgRoute(http.MethodGet, boardOrg, "/boards", orgauthz.PermOrgView, nil)
	cookie := bh.SignInToOrg(route, boardUser)
	listing := &dto.OrgBoardListResponse{Boards: []dto.OrgBoardListItem{
		{ID: boardAnchor, Title: "All hands", CreatedAt: "2026-10-05T09:00:00.000Z", TemplateKey: "blank"},
		{ID: boardID, TeamID: new(boardTeam), Title: "Sprint 12", CreatedAt: "2026-10-04T09:00:00.000Z", TeamName: "Platform", TemplateKey: "mad-sad-glad"},
		{ID: boardCard, TeamID: new(boardTeam), Title: "Sprint 11", ArchivedAt: "2026-10-03T09:00:00.000Z", TeamName: "Platform", TemplateKey: "blank"},
	}}
	bh.boards.EXPECT().ListForOrganisation(mock.Anything, spec.OrganisationMemberKey{OrgID: boardOrg, UserID: boardUser}).Return(listing, nil)

	// when
	rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

	// then an organisation board carries neither team_id nor team_name
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"boards":[
		{"id":"`+boardAnchor.String()+`","title":"All hands","template_key":"blank","created_at":"2026-10-05T09:00:00.000Z"},
		{"id":"`+boardID.String()+`","team_id":"`+boardTeam.String()+`","team_name":"Platform","title":"Sprint 12","template_key":"mad-sad-glad","created_at":"2026-10-04T09:00:00.000Z"},
		{"id":"`+boardCard.String()+`","team_id":"`+boardTeam.String()+`","team_name":"Platform","title":"Sprint 11","template_key":"blank","archived_at":"2026-10-03T09:00:00.000Z"}
	]}`, rec.Body.String())
}

func TestBoardController_ListOrganisationBoardsMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "organisation gone", err: fmt.Errorf("list: %w", board.ErrOrganisationNotFound), status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{name: "database", err: errors.New("the database is unreachable"), status: http.StatusInternalServerError, body: `{"error":"failed to list the boards"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			route := orgRoute(http.MethodGet, boardOrg, "/boards", orgauthz.PermOrgView, nil)
			cookie := bh.SignInToOrg(route, boardUser)
			bh.boards.EXPECT().ListForOrganisation(mock.Anything, spec.OrganisationMemberKey{OrgID: boardOrg, UserID: boardUser}).Return(nil, tc.err)

			// when
			rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestBoardController_BoardRoutesRunThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodGet, Path: boardPath("")},
		{Method: http.MethodPost, Path: boardPath("/participants/me")},
		{Method: http.MethodPut, Path: boardPath("/context"), Body: dto.DescribeBoardRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodGet, Path: boardPath("/sharing")},
		{Method: http.MethodPut, Path: boardPath("/sharing/link-access"), Body: dto.SetLinkAccessRequest{BoardCommandRequest: boardOpBody, LinkAccess: model.LinkAccessGuests}},
		{Method: http.MethodPut, Path: boardPath("/sharing/password"), Body: dto.SetBoardPasswordRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodPost, Path: boardPath("/sharing/rotate"), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath("/columns"), Body: dto.CreateColumnRequest{ColumnFieldsRequest: dto.ColumnFieldsRequest{BoardCommandRequest: boardOpBody}, ID: boardColumn}},
		{Method: http.MethodPut, Path: boardPath("/columns/" + boardColumn.String()), Body: dto.ColumnFieldsRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodPost, Path: boardPath("/columns/" + boardColumn.String() + "/move"), Body: dto.MoveColumnRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodPost, Path: boardPath("/columns/" + boardColumn.String() + "/clear"), Body: boardOpBody},
		{Method: http.MethodDelete, Path: boardPath("/columns/" + boardColumn.String()), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath("/cards"), Body: boardCardBody},
		{Method: http.MethodPut, Path: boardPath("/cards/" + boardCard.String()), Body: dto.EditCardRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodPut, Path: boardPath("/cards/" + boardCard.String() + "/colour"), Body: dto.RecolourCardRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodPost, Path: boardPath("/cards/" + boardCard.String() + "/move"), Body: dto.MoveCardRequest{BoardCommandRequest: boardOpBody, ColumnID: boardColumn}},
		{Method: http.MethodDelete, Path: boardPath("/cards/" + boardCard.String()), Body: boardOpBody},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newBoardHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardController_ListBoardsAnswersTheTeamsBoards(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	route := teamRoute(http.MethodGet, boardTeam, "/boards", orgauthz.PermTeamView, nil)
	cookie := bh.SignInToTeam(route, boardUser)
	listing := &dto.BoardListResponse{Boards: []dto.BoardListItem{{ID: boardID, TeamID: new(boardTeam), Title: "Sprint 12", CreatedAt: "2026-10-04T09:00:00.000Z"}}}
	bh.boards.EXPECT().List(mock.Anything, boardTeam).Return(listing, nil)

	// when
	rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"boards":[{"id":"`+boardID.String()+`","team_id":"`+boardTeam.String()+`","title":"Sprint 12","created_at":"2026-10-04T09:00:00.000Z"}]}`, rec.Body.String())
}

func TestBoardController_CreateBoardHandsTheRequestToTheServiceAndAnswersCreated(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	body := dto.CreateBoardRequest{Title: "Sprint 12", TemplateKey: "mad-sad-glad", Context: "Two weeks of deploys", Anonymity: model.AnonymityAnonymous, CardsShown: true}
	route := teamRoute(http.MethodPost, boardTeam, "/boards", orgauthz.PermTeamCreateBoard, body)
	cookie := bh.SignInToTeam(route, boardUser)

	created := &dto.BoardSnapshot{BoardID: boardID, Version: 0, Columns: []dto.ColumnView{}, Cards: []dto.CardView{}, HiddenCounts: map[uuid.UUID]int{}, Participants: []dto.ParticipantView{}}
	command := board.CreateCommand{Principal: boardPrincipal, TeamID: new(boardTeam), Title: body.Title, TemplateKey: body.TemplateKey, Context: body.Context, Anonymity: body.Anonymity, CardsShown: true}
	bh.boards.EXPECT().Create(mock.Anything, command).Return(created, nil)

	// when
	rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).WithJSONBody(body).Record()

	// then
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, mustJSON(t, created), rec.Body.String())
}

func TestBoardController_CreateBoardPassesTheSavedTemplate(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	savedTemplate := uuid.MustParse("5e6f7081-92a3-44b5-86c7-4c5d6e7f8091")
	body := dto.CreateBoardRequest{Title: "Sprint 12", SavedTemplateID: new(savedTemplate)}
	route := teamRoute(http.MethodPost, boardTeam, "/boards", orgauthz.PermTeamCreateBoard, body)
	cookie := bh.SignInToTeam(route, boardUser)
	created := &dto.BoardSnapshot{BoardID: boardID, Columns: []dto.ColumnView{}, Cards: []dto.CardView{}, HiddenCounts: map[uuid.UUID]int{}, Participants: []dto.ParticipantView{}}
	command := board.CreateCommand{Principal: boardPrincipal, TeamID: new(boardTeam), Title: "Sprint 12", SavedTemplateID: new(savedTemplate)}
	bh.boards.EXPECT().Create(mock.Anything, command).Return(created, nil)

	// when
	rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).WithJSONBody(body).Record()

	// then
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, mustJSON(t, created), rec.Body.String())
}

func TestBoardController_CreateOrganisationBoardPassesTheSavedTemplate(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	savedTemplate := uuid.MustParse("5e6f7081-92a3-44b5-86c7-4c5d6e7f8091")
	body := dto.CreateOrgBoardRequest{CreateBoardRequest: dto.CreateBoardRequest{Title: "All hands", SavedTemplateID: new(savedTemplate)}}
	route := orgRoute(http.MethodPost, boardOrg, "/boards", orgauthz.PermOrgCreateBoard, body)
	cookie := bh.SignInToOrg(route, boardUser)
	created := &dto.BoardSnapshot{BoardID: boardID, Columns: []dto.ColumnView{}, Cards: []dto.CardView{}, HiddenCounts: map[uuid.UUID]int{}, Participants: []dto.ParticipantView{}}
	command := board.CreateCommand{Principal: boardPrincipal, OrgID: boardOrg, Title: "All hands", SavedTemplateID: new(savedTemplate)}
	bh.boards.EXPECT().Create(mock.Anything, command).Return(created, nil)

	// when
	rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).WithJSONBody(body).Record()

	// then
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, mustJSON(t, created), rec.Body.String())
}

func TestBoardController_AnUnusableSavedTemplateIsAnUnknownTemplate(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	body := dto.CreateBoardRequest{Title: "Sprint 12", SavedTemplateID: new(uuid.New())}
	route := teamRoute(http.MethodPost, boardTeam, "/boards", orgauthz.PermTeamCreateBoard, body)
	cookie := bh.SignInToTeam(route, boardUser)
	bh.boards.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("%w: saved template belongs to another member", board.ErrUnknownTemplate))

	// when
	rec := bh.NewRequest(route.Method, route.Path).WithCookie(cookie).WithJSONBody(body).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"`+board.ErrUnknownTemplate.Error()+`","code":"unknown_template"}`, rec.Body.String())
}

func TestBoardController_GetBoardAnswersTheViewersSnapshot(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	snapshot := &dto.BoardSnapshot{BoardID: boardID, Version: 12, Columns: []dto.ColumnView{}, Cards: []dto.CardView{}, HiddenCounts: map[uuid.UUID]int{boardColumn: 2}, Participants: []dto.ParticipantView{}}
	bh.boards.EXPECT().GetSnapshot(mock.Anything, spec.BoardSnapshotQuery{Principal: boardPrincipal, BoardID: boardID}).Return(snapshot, nil)

	// when
	status, body := bh.record(http.MethodGet, boardPath(""), nil)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, snapshot), body)
}

func TestBoardController_GetBoardAnswersNoContentWhenTheKnownVersionIsCurrent(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	query := spec.BoardSnapshotQuery{Principal: boardPrincipal, BoardID: boardID, KnownVersion: new(int64(12))}
	bh.boards.EXPECT().GetSnapshot(mock.Anything, query).Return(nil, board.ErrNotModified)

	// when
	status, body := bh.record(http.MethodGet, boardPath("?known_version=12&reason=reconnect"), nil)

	// then
	assert.Equal(t, http.StatusNoContent, status)
	assert.Empty(t, body)
}

func TestBoardController_GetBoardRefusesAKnownVersionThatIsNotANumber(t *testing.T) {
	// given
	bh := newBoardHarness(t)

	// when
	status, body := bh.record(http.MethodGet, boardPath("?known_version=latest"), nil)

	// then the service is never asked
	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"invalid known_version"}`, body)
}

func TestBoardController_JoinBoardAddsTheMemberAndAnswersNoContent(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	access := &model.BoardAccess{Principal: boardPrincipal, ParticipantID: new(boardAnchor), Role: model.BoardRoleParticipant}
	bh.live.EXPECT().JoinAsMember(mock.Anything, spec.BoardAccessQuery{Principal: boardPrincipal, BoardID: boardID}).Return(access, nil)

	// when
	status, body := bh.record(http.MethodPost, boardPath("/participants/me"), nil)

	// then
	assert.Equal(t, http.StatusNoContent, status)
	assert.Empty(t, body)
}

func TestBoardController_JoinBoardMapsTheLiveBoardsRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{err: live.ErrRemoved, status: http.StatusForbidden, body: `{"error":"you were removed from this board","code":"removed"}`},
		{err: live.ErrLinkDisabled, status: http.StatusForbidden, body: `{"error":"the board's link is switched off","code":"link_disabled"}`},
		{err: live.ErrPasswordRequired, status: http.StatusForbidden, body: `{"error":"the board needs its password","code":"password_required"}`},
		{err: live.ErrAccountRequired, status: http.StatusForbidden, body: `{"error":"the board needs you to sign in","code":"account_required"}`},
		{err: live.ErrArchived, status: http.StatusConflict, body: `{"error":"the board is archived","code":"archived"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
		{err: errors.New("participants store down"), status: http.StatusInternalServerError, body: `{"error":"failed to join the board"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			bh.live.EXPECT().JoinAsMember(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("join: %w", tc.err))

			// when
			status, body := bh.record(http.MethodPost, boardPath("/participants/me"), nil)

			// then
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	columnCommandOf := board.ColumnCommand{Command: boardCommandOf, ColumnID: boardColumn}
	cardCommandOf := board.CardCommand{Command: boardCommandOf, CardID: boardCard}
	fields := dto.ColumnFieldsRequest{BoardCommandRequest: boardOpBody, Title: "Went well", Description: "What we keep", Colour: boardColour}
	columnFields := board.ColumnFields{Title: "Went well", Description: "What we keep", Colour: boardColour}

	tests := []boardCommandCase{
		{
			name:   "describe the board",
			method: http.MethodPut,
			path:   boardPath("/context"),
			body:   dto.DescribeBoardRequest{BoardCommandRequest: boardOpBody, Context: "Two weeks of deploys"},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Describe(mock.Anything, board.DescribeCommand{Command: boardCommandOf, Context: "Two weeks of deploys"}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "add a column",
			method: http.MethodPost,
			path:   boardPath("/columns"),
			body:   dto.CreateColumnRequest{ColumnFieldsRequest: fields, ID: boardColumn},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().AddColumn(mock.Anything, board.AddColumnCommand{Command: boardCommandOf, ColumnFields: columnFields, ColumnID: boardColumn}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "edit a column",
			method: http.MethodPut,
			path:   boardPath("/columns/" + boardColumn.String()),
			body:   fields,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().EditColumn(mock.Anything, board.EditColumnCommand{Command: boardCommandOf, ColumnFields: columnFields, ColumnID: boardColumn}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "move a column",
			method: http.MethodPost,
			path:   boardPath("/columns/" + boardColumn.String() + "/move"),
			body:   dto.MoveColumnRequest{BoardCommandRequest: boardOpBody, AfterColumnID: new(boardAnchor)},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().MoveColumn(mock.Anything, board.MoveColumnCommand{Command: boardCommandOf, ColumnID: boardColumn, AfterColumnID: new(boardAnchor)}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "move a column to the front",
			method: http.MethodPost,
			path:   boardPath("/columns/" + boardColumn.String() + "/move"),
			body:   dto.MoveColumnRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().MoveColumn(mock.Anything, board.MoveColumnCommand{Command: boardCommandOf, ColumnID: boardColumn}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "clear a column",
			method: http.MethodPost,
			path:   boardPath("/columns/" + boardColumn.String() + "/clear"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ClearColumn(mock.Anything, columnCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "delete a column",
			method: http.MethodDelete,
			path:   boardPath("/columns/" + boardColumn.String()),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().DeleteColumn(mock.Anything, columnCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "add a card",
			method: http.MethodPost,
			path:   boardPath("/cards"),
			body:   dto.CreateCardRequest{BoardCommandRequest: boardOpBody, ID: boardCard, ColumnID: boardColumn, Body: "Deploys were smooth", Colour: new(boardColour)},
			expect: func(bh *boardHarness) {
				add := board.AddCardCommand{Command: boardCommandOf, CardID: boardCard, ColumnID: boardColumn, Body: "Deploys were smooth", Colour: new(boardColour)}
				bh.boards.EXPECT().AddCard(mock.Anything, add).Return(boardVersioned, nil)
			},
		},
		{
			name:   "edit a card",
			method: http.MethodPut,
			path:   boardPath("/cards/" + boardCard.String()),
			body:   dto.EditCardRequest{BoardCommandRequest: boardOpBody, Body: "Deploys were mostly smooth"},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().EditCard(mock.Anything, board.EditCardCommand{Command: boardCommandOf, CardID: boardCard, Body: "Deploys were mostly smooth"}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "add a card that mentions someone",
			method: http.MethodPost,
			path:   boardPath("/cards"),
			body:   dto.CreateCardRequest{BoardCommandRequest: boardOpBody, ID: boardCard, ColumnID: boardColumn, Body: "Thanks @Sam", Mentions: []uuid.UUID{boardUser}},
			expect: func(bh *boardHarness) {
				add := board.AddCardCommand{Command: boardCommandOf, CardID: boardCard, ColumnID: boardColumn, Body: "Thanks @Sam", Mentions: []uuid.UUID{boardUser}}
				bh.boards.EXPECT().AddCard(mock.Anything, add).Return(boardVersioned, nil)
			},
		},
		{
			name:   "edit a card's mentions",
			method: http.MethodPut,
			path:   boardPath("/cards/" + boardCard.String()),
			body:   dto.EditCardRequest{BoardCommandRequest: boardOpBody, Body: "Thanks @Sam", Mentions: []uuid.UUID{boardUser}},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().EditCard(mock.Anything, board.EditCardCommand{Command: boardCommandOf, CardID: boardCard, Body: "Thanks @Sam", Mentions: []uuid.UUID{boardUser}}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "recolour a card",
			method: http.MethodPut,
			path:   boardPath("/cards/" + boardCard.String() + "/colour"),
			body:   dto.RecolourCardRequest{BoardCommandRequest: boardOpBody, Colour: new(boardColour)},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RecolourCard(mock.Anything, board.RecolourCardCommand{Command: boardCommandOf, CardID: boardCard, Colour: new(boardColour)}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "clear a card's colour",
			method: http.MethodPut,
			path:   boardPath("/cards/" + boardCard.String() + "/colour"),
			body:   dto.RecolourCardRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RecolourCard(mock.Anything, board.RecolourCardCommand{Command: boardCommandOf, CardID: boardCard}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "move a card",
			method: http.MethodPost,
			path:   boardPath("/cards/" + boardCard.String() + "/move"),
			body:   dto.MoveCardRequest{BoardCommandRequest: boardOpBody, ColumnID: boardColumn, AfterCardID: new(boardAnchor)},
			expect: func(bh *boardHarness) {
				move := board.MoveCardCommand{Command: boardCommandOf, CardID: boardCard, ColumnID: boardColumn, AfterCardID: new(boardAnchor)}
				bh.boards.EXPECT().MoveCard(mock.Anything, move).Return(boardVersioned, nil)
			},
		},
		{
			name:   "delete a card",
			method: http.MethodDelete,
			path:   boardPath("/cards/" + boardCard.String()),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().DeleteCard(mock.Anything, cardCommandOf).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardController_ACommandAnswersTheProjectedResponseAsItIs(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	replayed := &dto.BoardCommandResponse{Version: 9, Replayed: true, ID: new(boardCard)}
	bh.boards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(replayed, nil)

	// when
	status, body := bh.record(http.MethodPost, boardPath("/cards"), boardCardBody)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":9,"replayed":true,"id":"`+boardCard.String()+`"}`, body)
}

func TestBoardController_AClientIDThatIsNotVersion4IsRefusedBeforeTheService(t *testing.T) {
	tests := []struct {
		name string
		path string
		body any
	}{
		{name: "a card", path: boardPath("/cards"), body: dto.CreateCardRequest{BoardCommandRequest: boardOpBody, ID: boardV7CardID, ColumnID: boardColumn, Body: "Deploys were smooth"}},
		{name: "a card with no ID", path: boardPath("/cards"), body: dto.CreateCardRequest{BoardCommandRequest: boardOpBody, ColumnID: boardColumn, Body: "Deploys were smooth"}},
		{name: "a column", path: boardPath("/columns"), body: dto.CreateColumnRequest{ColumnFieldsRequest: dto.ColumnFieldsRequest{BoardCommandRequest: boardOpBody, Title: "Went well"}, ID: boardV7CardID}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)

			// when
			status, body := bh.record(http.MethodPost, tc.path, tc.body)

			// then the strict board mock proves the service is never asked
			assert.Equal(t, http.StatusBadRequest, status)
			assert.JSONEq(t, `{"error":"a board content ID must be a version 4 UUID","code":"invalid_id"}`, body)
		})
	}
}

func TestBoardController_APathIDThatIsNotAUUIDIsRefused(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "board", method: http.MethodGet, path: "/api/v1/boards/not-a-uuid", body: `{"error":"invalid boardID"}`},
		{name: "column", method: http.MethodDelete, path: boardPath("/columns/not-a-uuid"), body: `{"error":"invalid columnID"}`},
		{name: "card", method: http.MethodDelete, path: boardPath("/cards/not-a-uuid"), body: `{"error":"invalid cardID"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)

			// when
			status, body := bh.record(tc.method, tc.path, boardOpBody)

			// then
			assert.Equal(t, http.StatusBadRequest, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardController_MapsEveryBoardErrorToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: board.ErrInvalidID, status: http.StatusBadRequest, body: `{"error":"a board content ID must be a version 4 UUID","code":"invalid_id"}`},
		{err: board.ErrOpRequired, status: http.StatusBadRequest, body: `{"error":"every board command needs an op_id","code":"op_required"}`},
		{err: board.ErrTitleRequired, status: http.StatusBadRequest, body: `{"error":"a board title is required","code":"board_title_required"}`},
		{err: board.ErrTitleTooLong, status: http.StatusBadRequest, body: `{"error":"a board title can be at most 200 characters","code":"board_title_too_long"}`},
		{err: board.ErrUnknownTemplate, status: http.StatusBadRequest, body: `{"error":"there is no template with that key","code":"unknown_template"}`},
		{err: board.ErrInvalidAnonymity, status: http.StatusBadRequest, body: `{"error":"anonymity must be named or anonymous","code":"invalid_anonymity"}`},
		{err: board.ErrContextTooLong, status: http.StatusBadRequest, body: `{"error":"a board description can be at most 500 characters","code":"board_context_too_long"}`},
		{err: board.ErrColumnTitleRequired, status: http.StatusBadRequest, body: `{"error":"a column title is required","code":"column_title_required"}`},
		{err: board.ErrColumnTitleTooLong, status: http.StatusBadRequest, body: `{"error":"a column title can be at most 80 characters","code":"column_title_too_long"}`},
		{err: board.ErrColumnDescriptionTooLong, status: http.StatusBadRequest, body: `{"error":"a column description can be at most 200 characters","code":"column_description_too_long"}`},
		{err: board.ErrInvalidColour, status: http.StatusBadRequest, body: `{"error":"that is not one of the board colours","code":"invalid_colour"}`},
		{err: board.ErrCardBodyRequired, status: http.StatusBadRequest, body: `{"error":"a card needs some text","code":"card_body_required"}`},
		{err: board.ErrCardBodyTooLong, status: http.StatusBadRequest, body: `{"error":"a card can be at most 2000 characters","code":"card_body_too_long"}`},
		{err: board.ErrInvalidLinkAccess, status: http.StatusBadRequest, body: `{"error":"link access must be off, guests or accounts","code":"invalid_link_access"}`},
		{err: board.ErrPasswordBlank, status: http.StatusBadRequest, body: `{"error":"a board password cannot be blank","code":"board_password_blank"}`},
		{err: board.ErrPasswordTooLong, status: http.StatusBadRequest, body: `{"error":"a board password can be at most 72 bytes","code":"board_password_too_long"}`},
		{err: live.ErrWrongPassword, status: http.StatusForbidden, body: `{"error":"that is not the board's password","code":"wrong_password"}`},
		{err: live.ErrGuestsNotAllowed, status: http.StatusForbidden, body: `{"error":"this organisation does not let guests join its boards","code":"guests_not_allowed"}`},
		{err: live.ErrLinkDisabled, status: http.StatusForbidden, body: `{"error":"the board's link is switched off","code":"link_disabled"}`},
		{err: live.ErrPasswordRequired, status: http.StatusForbidden, body: `{"error":"the board needs its password","code":"password_required"}`},
		{err: live.ErrAccountRequired, status: http.StatusForbidden, body: `{"error":"the board needs you to sign in","code":"account_required"}`},
		{err: live.ErrBoardFull, status: http.StatusTooManyRequests, body: `{"error":"the board already holds as many participants as it can","code":"board_full"}`},
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{err: board.ErrTeamNotFound, status: http.StatusNotFound, body: `{"error":"team not found"}`},
		{err: board.ErrOrganisationNotFound, status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{err: board.ErrColumnNotFound, status: http.StatusNotFound, body: `{"error":"column not found"}`},
		{err: board.ErrTemplateNotFound, status: http.StatusNotFound, body: `{"error":"template not found"}`},
		{err: policy.ErrCardNotFound, status: http.StatusNotFound, body: `{"error":"card not found"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: board.ErrCreateNotAllowed, status: http.StatusForbidden, body: `{"error":"you cannot create a board there","code":"create_not_allowed"}`},
		{err: live.ErrRemoved, status: http.StatusForbidden, body: `{"error":"you were removed from this board","code":"removed"}`},
		{err: board.ErrIDConflict, status: http.StatusConflict, body: `{"error":"that ID is already taken by another command","code":"id_conflict"}`},
		{err: board.ErrTeamArchived, status: http.StatusConflict, body: `{"error":"the team is archived, so it can no longer hold new boards","code":"team_archived"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{err: policy.ErrCardsLocked, status: http.StatusConflict, body: `{"error":"cards are locked on this board","code":"cards_locked"}`},
		{err: policy.ErrMovesLocked, status: http.StatusConflict, body: `{"error":"moving cards is locked on this board","code":"moves_locked"}`},
		{err: policy.ErrNotParticipant, status: http.StatusConflict, body: `{"error":"join the board before writing on it","code":"not_participant"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
		{err: dao.ErrVersionConflict, status: http.StatusInternalServerError, body: `{"error":"failed to add the card"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			bh.boards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("add card %s: %w", boardCard, tc.err))

			// when
			status, body := bh.record(http.MethodPost, boardPath("/cards"), boardCardBody)

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardController_BoardCommandsShareOneWriteBudgetPerPrincipal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a user who has spent the burst across different commands
		bh := newBoardHarness(t)
		bh.boards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(boardVersioned, nil)
		bh.boards.EXPECT().DeleteCard(mock.Anything, mock.Anything).Return(boardVersioned, nil)

		for i := range middleware.BoardWriteBurst {
			if i%2 == 0 {
				status, _ := bh.record(http.MethodPost, boardPath("/cards"), boardCardBody)
				require.Equal(t, http.StatusOK, status, "write "+strconv.Itoa(i))

				continue
			}

			status, _ := bh.record(http.MethodDelete, boardPath("/cards/"+boardCard.String()), boardOpBody)
			require.Equal(t, http.StatusOK, status, "write "+strconv.Itoa(i))
		}

		// when
		status, body := bh.record(http.MethodPut, boardPath("/context"), dto.DescribeBoardRequest{BoardCommandRequest: boardOpBody})

		// then the limiter refuses before the strict mock is asked to describe anything
		assert.Equal(t, http.StatusTooManyRequests, status)
		assert.JSONEq(t, `{"error":"too many requests, please try again later"}`, body)
	})
}
