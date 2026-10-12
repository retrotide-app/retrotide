package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/management"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	managementHarness struct {
		*testutil.Harness
		boards *management.MockService
	}
)

var (
	managedTeam     = uuid.MustParse("6e7f8091-a2b3-44c5-96d7-4e5f60718293")
	managedOther    = uuid.MustParse("7f8091a2-b3c4-45d6-a7e8-5f60718293a4")
	managedGuest    = uuid.MustParse("8091a2b3-c4d5-46e7-b8f9-60718293a4b5")
	managedRestore  = management.RestoreCommand{Principal: boardPrincipal, OrgID: boardOrg, BoardID: boardID}
	managedImport   = dto.CSVImportRequest{BoardCommandRequest: boardOpBody, CSV: "Went well\nShipped\n"}
	managedListing  = &dto.DeletedBoardListResponse{Boards: []dto.DeletedBoardItem{{ID: boardID, Title: "Sprint 12", DeletedAt: "2026-10-09T09:00:00.000Z", RestoreUntil: "2026-11-08T09:00:00.000Z"}}}
	managedBulkBody = dto.BulkBoardRequest{Action: management.BulkArchive, BoardIDs: []uuid.UUID{boardID, managedOther}}
)

func newManagementHarness(t *testing.T) *managementHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	mh := &managementHarness{Harness: h, boards: management.NewMockService(t)}

	s := &Service{BoardManagement: mh.boards, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardManagementRoutes())

	return mh
}

func (mh *managementHarness) record(method string, path string, body any) (int, string) {
	cookie := mh.SignIn(boardUser)
	if method != http.MethodGet && method != http.MethodHead {
		mh.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)
	}

	request := mh.NewRequest(method, path).WithCookie(cookie)
	if body != nil {
		request = request.WithJSONBody(body)
	}

	rec := request.Record()

	return rec.Code, rec.Body.String()
}

func (mh *managementHarness) recordInOrg(route testutil.OrgPermissionRoute) (int, string) {
	cookie := mh.SignInToOrg(route, boardUser)

	request := mh.NewRequest(route.Method, route.Path).WithCookie(cookie)
	if route.Body != nil {
		request = request.WithJSONBody(route.Body)
	}

	rec := request.Record()

	return rec.Code, rec.Body.String()
}

func TestBoardManagementController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardManagement: management.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardManagementRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"POST /api/v1/boards/:boardID/archive",
		"POST /api/v1/boards/:boardID/unarchive",
		"DELETE /api/v1/boards/:boardID",
		"PUT /api/v1/boards/:boardID/team",
		"POST /api/v1/boards/:boardID/import",
		"PUT /api/v1/boards/:boardID/pin",
		"DELETE /api/v1/boards/:boardID/pin",
		"GET /api/v1/orgs/:orgID/boards/deleted",
		"HEAD /api/v1/orgs/:orgID/boards/deleted",
		"POST /api/v1/orgs/:orgID/boards/:boardID/restore",
		"POST /api/v1/orgs/:orgID/boards/bulk",
	}, registeredRoutes(h.Engine))
}

func TestBoardManagementController_MountsBesideTheOtherBoardAndExportRoutes(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardService: board.NewMockService(t), BoardLive: live.NewMockService(t), BoardManagement: management.NewMockService(t), Guards: h.Guards}

	// when the lists are mounted together, a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), append(append(boardRouteLists(s), s.getAllExportRoutes()...), s.getAllBoardManagementRoutes()...))

	// then
	assert.Contains(t, registeredRoutes(h.Engine), "POST /api/v1/orgs/:orgID/boards/:boardID/restore")
}

func TestBoardManagementController_EveryBoardRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: boardPath("/archive"), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath("/unarchive"), Body: boardOpBody},
		{Method: http.MethodDelete, Path: boardPath(""), Body: boardOpBody},
		{Method: http.MethodPut, Path: boardPath("/team"), Body: dto.MoveBoardRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodPost, Path: boardPath("/import"), Body: managedImport},
		{Method: http.MethodPut, Path: boardPath("/pin")},
		{Method: http.MethodDelete, Path: boardPath("/pin")},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newManagementHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardManagementController_EveryOrganisationRouteRunsTheOrgPermissionFailureSuite(t *testing.T) {
	routes := []testutil.OrgPermissionRoute{
		orgRoute(http.MethodGet, boardOrg, "/boards/deleted", orgauthz.PermOrgView, nil),
		orgRoute(http.MethodPost, boardOrg, "/boards/"+boardID.String()+"/restore", orgauthz.PermOrgView, nil),
		orgRoute(http.MethodPost, boardOrg, "/boards/bulk", orgauthz.PermOrgView, managedBulkBody),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newManagementHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardManagementController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		expect func(mh *managementHarness)
	}{
		{
			name:   "archive the board",
			method: http.MethodPost,
			path:   boardPath("/archive"),
			body:   boardOpBody,
			expect: func(mh *managementHarness) {
				mh.boards.EXPECT().Archive(mock.Anything, boardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "unarchive the board",
			method: http.MethodPost,
			path:   boardPath("/unarchive"),
			body:   boardOpBody,
			expect: func(mh *managementHarness) {
				mh.boards.EXPECT().Unarchive(mock.Anything, boardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "delete the board",
			method: http.MethodDelete,
			path:   boardPath(""),
			body:   boardOpBody,
			expect: func(mh *managementHarness) {
				mh.boards.EXPECT().Delete(mock.Anything, boardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "move the board to a team",
			method: http.MethodPut,
			path:   boardPath("/team"),
			body:   dto.MoveBoardRequest{BoardCommandRequest: boardOpBody, TeamID: new(managedTeam)},
			expect: func(mh *managementHarness) {
				mh.boards.EXPECT().Move(mock.Anything, management.MoveCommand{Command: boardCommandOf, TeamID: new(managedTeam)}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "move the board to the organisation",
			method: http.MethodPut,
			path:   boardPath("/team"),
			body:   dto.MoveBoardRequest{BoardCommandRequest: boardOpBody},
			expect: func(mh *managementHarness) {
				mh.boards.EXPECT().Move(mock.Anything, management.MoveCommand{Command: boardCommandOf}).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mh := newManagementHarness(t)
			tc.expect(mh)

			// when
			status, body := mh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardManagementController_ImportAnswersTheCounts(t *testing.T) {
	// given
	mh := newManagementHarness(t)
	command := management.ImportCommand{Command: boardCommandOf, CSV: managedImport.CSV}
	mh.boards.EXPECT().ImportCSV(mock.Anything, command).Return(&dto.CSVImportResponse{Version: 9, ColumnsCreated: 1, CardsCreated: 4}, nil)

	// when
	status, body := mh.record(http.MethodPost, boardPath("/import"), managedImport)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":9,"columns_created":1,"cards_created":4}`, body)
}

func TestBoardManagementController_AnArchiveWhoseAuditFailedStillAnswersItsVersion(t *testing.T) {
	// given
	mh := newManagementHarness(t)
	mh.boards.EXPECT().Archive(mock.Anything, boardCommandOf).Return(boardVersioned, fmt.Errorf("%w: %w", management.ErrNotAudited, errServiceDown))

	// when
	status, body := mh.record(http.MethodPost, boardPath("/archive"), boardOpBody)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":7}`, body)
}

func TestBoardManagementController_MapsEveryManagementRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: management.ErrSameTeam, status: http.StatusConflict, body: `{"error":"the board is already there","code":"same_team"}`},
		{err: management.ErrMoveNotAllowed, status: http.StatusForbidden, body: `{"error":"you cannot move a board there","code":"move_not_allowed"}`},
		{err: management.ErrTeamNotFound, status: http.StatusNotFound, body: `{"error":"team not found"}`},
		{err: management.ErrTeamArchived, status: http.StatusConflict, body: `{"error":"the team is archived, so it can no longer hold new boards","code":"team_archived"}`},
		{err: management.ErrCSVEmpty, status: http.StatusBadRequest, body: `{"error":"the CSV holds no cards to import","code":"csv_empty"}`},
		{err: management.ErrCSVInvalid, status: http.StatusBadRequest, body: `{"error":"the CSV could not be read","code":"csv_invalid"}`},
		{err: management.ErrCSVTooManyCards, status: http.StatusBadRequest, body: `{"error":"a CSV import can add at most 500 cards","code":"csv_too_many_cards"}`},
		{err: management.ErrCSVTooManyColumns, status: http.StatusBadRequest, body: `{"error":"a board can have at most 20 columns","code":"csv_too_many_columns"}`},
		{err: board.ErrCardBodyTooLong, status: http.StatusBadRequest, body: `{"error":"a card can be at most 2000 characters","code":"card_body_too_long"}`},
		{err: policy.ErrCardsOutOfStage, status: http.StatusConflict, body: `{"error":"cards can be added, edited, moved and deleted only in the Write stage","code":"wrong_stage"}`},
		{err: policy.ErrColumnHidden, status: http.StatusConflict, body: `{"error":"the column is hidden from participants, so nothing in it can change until it is shown","code":"column_hidden"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to import the cards"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			mh := newManagementHarness(t)
			mh.boards.EXPECT().ImportCSV(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("import: %w", tc.err))

			// when
			status, body := mh.record(http.MethodPost, boardPath("/import"), managedImport)

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardManagementController_AMoveAcrossOrganisationsAnswersNotFound(t *testing.T) {
	// given a team ID belonging to another organisation
	mh := newManagementHarness(t)
	mh.boards.EXPECT().Move(mock.Anything, management.MoveCommand{Command: boardCommandOf, TeamID: new(managedTeam)}).Return(nil, fmt.Errorf("move: %w", management.ErrTeamNotFound))

	// when
	status, body := mh.record(http.MethodPut, boardPath("/team"), dto.MoveBoardRequest{BoardCommandRequest: boardOpBody, TeamID: new(managedTeam)})

	// then
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"team not found"}`, body)
}

func TestBoardManagementController_PinAndUnpinAnswerNoContent(t *testing.T) {
	tests := []struct {
		method string
		pinned bool
	}{
		{method: http.MethodPut, pinned: true},
		{method: http.MethodDelete, pinned: false},
	}

	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			// given
			mh := newManagementHarness(t)
			mh.boards.EXPECT().Pin(mock.Anything, management.PinCommand{Principal: boardPrincipal, BoardID: boardID, Pinned: tc.pinned}).Return(nil)

			// when
			status, body := mh.record(tc.method, boardPath("/pin"), nil)

			// then
			assert.Equal(t, http.StatusNoContent, status)
			assert.Empty(t, body)
		})
	}
}

func TestBoardManagementController_AGuestCannotPinABoard(t *testing.T) {
	// given a guest on the board, whom the service refuses as not a member
	mh := newManagementHarness(t)
	cookie := mh.SignInAsGuest(managedGuest)
	guest := model.Principal{Kind: model.PrincipalGuest, ID: managedGuest}
	mh.boards.EXPECT().Pin(mock.Anything, management.PinCommand{Principal: guest, BoardID: boardID, Pinned: true}).Return(fmt.Errorf("pin: %w", policy.ErrMembersOnly))

	// when
	rec := mh.NewRequest(http.MethodPut, boardPath("/pin")).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"only members of the board's organisation can do that","code":"members_only"}`, rec.Body.String())
}

func TestBoardManagementController_ListsTheBoardsTheViewerCanRestore(t *testing.T) {
	// given
	mh := newManagementHarness(t)
	route := orgRoute(http.MethodGet, boardOrg, "/boards/deleted", orgauthz.PermOrgView, nil)
	mh.boards.EXPECT().ListDeleted(mock.Anything, spec.OrganisationMemberKey{OrgID: boardOrg, UserID: boardUser}).Return(managedListing, nil)

	// when
	status, body := mh.recordInOrg(route)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, managedListing), body)
}

func TestBoardManagementController_RestoreAnswersNoContent(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "restored", err: nil},
		{name: "restored, but its audit entry failed", err: fmt.Errorf("%w: %w", management.ErrNotAudited, errServiceDown)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mh := newManagementHarness(t)
			route := orgRoute(http.MethodPost, boardOrg, "/boards/"+boardID.String()+"/restore", orgauthz.PermOrgView, nil)
			mh.boards.EXPECT().Restore(mock.Anything, managedRestore).Return(tc.err)

			// when
			status, _ := mh.recordInOrg(route)

			// then
			assert.Equal(t, http.StatusNoContent, status)
		})
	}
}

func TestBoardManagementController_MapsTheRestoreRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "a board of another organisation, or past its window", err: management.ErrBoardNotFound, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{name: "neither a manager nor the creator", err: management.ErrRestoreNotAllowed, status: http.StatusForbidden, body: `{"error":"you cannot restore that board","code":"restore_not_allowed"}`},
		{name: "database", err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to restore the board"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mh := newManagementHarness(t)
			route := orgRoute(http.MethodPost, boardOrg, "/boards/"+boardID.String()+"/restore", orgauthz.PermOrgView, nil)
			mh.boards.EXPECT().Restore(mock.Anything, managedRestore).Return(fmt.Errorf("restore: %w", tc.err))

			// when
			status, body := mh.recordInOrg(route)

			// then
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardManagementController_BulkAnswersAResultForEveryBoard(t *testing.T) {
	// given a board archived, one of another organisation, one the viewer may not manage, one unaudited and one that broke
	mh := newManagementHarness(t)
	refused := uuid.MustParse("91a2b3c4-d5e6-47f8-89a0-718293a4b5c6")
	unaudited := uuid.MustParse("a2b3c4d5-e6f7-48a9-9ab1-8293a4b5c6d7")
	broken := uuid.MustParse("b3c4d5e6-f7a8-49ba-abc2-93a4b5c6d7e8")
	body := dto.BulkBoardRequest{Action: management.BulkArchive, BoardIDs: []uuid.UUID{boardID, managedOther, refused, unaudited, broken}}
	route := orgRoute(http.MethodPost, boardOrg, "/boards/bulk", orgauthz.PermOrgView, body)
	command := management.BulkCommand{Principal: boardPrincipal, OrgID: boardOrg, Action: management.BulkArchive, BoardIDs: body.BoardIDs}
	mh.boards.EXPECT().Bulk(mock.Anything, command).Return([]management.BulkOutcome{
		{BoardID: boardID},
		{BoardID: managedOther, Err: fmt.Errorf("archive: %w", live.ErrNoAccess)},
		{BoardID: refused, Err: fmt.Errorf("archive: %w", policy.ErrNotAllowed)},
		{BoardID: unaudited, Err: fmt.Errorf("%w: %w", management.ErrNotAudited, errServiceDown)},
		{BoardID: broken, Err: errServiceDown},
	}, nil)

	// when
	status, got := mh.recordInOrg(route)

	// then the other organisation's board is not touched and answers as if it did not exist
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, dto.BulkBoardResponse{Results: []dto.BulkBoardResult{
		{BoardID: boardID, OK: true},
		{BoardID: managedOther, OK: false, Error: "board not found"},
		{BoardID: refused, OK: false, Error: "your role on this board does not allow that"},
		{BoardID: unaudited, OK: true},
		{BoardID: broken, OK: false, Code: "failed", Error: "failed to update the board"},
	}}), got)
}

func TestBoardManagementController_MapsTheBulkRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: management.ErrInvalidBulkAction, status: http.StatusBadRequest, body: `{"error":"a bulk action must be archive, unarchive or delete","code":"invalid_bulk_action"}`},
		{err: management.ErrBoardsRequired, status: http.StatusBadRequest, body: `{"error":"a bulk action needs at least one board","code":"boards_required"}`},
		{err: management.ErrTooManyBoards, status: http.StatusBadRequest, body: `{"error":"a bulk action takes at most 100 boards","code":"too_many_boards"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			mh := newManagementHarness(t)
			route := orgRoute(http.MethodPost, boardOrg, "/boards/bulk", orgauthz.PermOrgView, managedBulkBody)
			mh.boards.EXPECT().Bulk(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("bulk: %w", tc.err))

			// when
			status, body := mh.recordInOrg(route)

			// then
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func mountCrossTenantManagement(t *testing.T, x *crossTenantHarness) {
	t.Helper()

	orgAuthz := orgauthz.NewService(x.orgRepo, x.teamRepo)
	boardLive := live.New(live.Deps{Boards: x.boardRepo, Cards: x.cardRepo, OrgAuthz: orgAuthz})
	guards := middleware.NewGuards(middleware.GuardDeps{Settings: x.SettingsService, Sessions: x.Sessions, Guests: x.Guests, Authz: x.Authz, OrgAuthz: orgAuthz})

	boards := management.NewService(management.Deps{
		Boards:        x.boardRepo,
		Columns:       x.columnRepo,
		Cards:         x.cardRepo,
		Teams:         x.teamRepo,
		Organisations: x.orgRepo,
		Live:          boardLive,
		Events:        boardLive,
		OrgAuthz:      orgAuthz,
		AuditLog:      repository.NewMockAuditLogRepository(t),
	})

	s := &Service{BoardManagement: boards, BoardLive: boardLive, Guards: guards}
	RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllBoardManagementRoutes())
}

func (x *crossTenantHarness) aliceHasNoAccessToBoardB() {
	x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(tenantBoardBOpen, nil)
	x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)
	x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardB, UserID: tenantAlice}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))
}

func TestBoardManagementController_EveryBoardRouteOfAnotherOrganisationsBoardAnswersNotFound(t *testing.T) {
	routes := []crossTenantRoute{
		{method: http.MethodPost, path: tenantBoardBPath + "/archive", body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/unarchive", body: tenantOpBody},
		{method: http.MethodDelete, path: tenantBoardBPath, body: tenantOpBody},
		{method: http.MethodPut, path: tenantBoardBPath + "/team", body: dto.MoveBoardRequest{BoardCommandRequest: tenantOpBody, TeamID: new(tenantTeamA)}},
		{method: http.MethodPost, path: tenantBoardBPath + "/import", body: dto.CSVImportRequest{BoardCommandRequest: tenantOpBody, CSV: "Went well\nShipped\n"}},
		{method: http.MethodPut, path: tenantBoardBPath + "/pin"},
		{method: http.MethodDelete, path: tenantBoardBPath + "/pin"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given board B belongs to a team of organisation B, where Alice holds nothing and has no seat on the board
			x := newCrossTenantHarness(t)
			mountCrossTenantManagement(t, x)
			x.aliceHasNoAccessToBoardB()

			// when
			status, body := x.recordAsAlice(route)

			// then board B is not even shown to exist, and the strict mocks prove it is never archived, deleted, moved, imported into or pinned
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"board not found"}`, body)
		})
	}
}

func TestBoardManagementController_EveryOrganisationRouteOfAnotherOrganisationAnswersNotFound(t *testing.T) {
	routes := []crossTenantRoute{
		{method: http.MethodGet, path: tenantOrgBPath + "/boards/deleted"},
		{method: http.MethodPost, path: tenantOrgBPath + "/boards/" + tenantBoardB.String() + "/restore"},
		{method: http.MethodPost, path: tenantOrgBPath + "/boards/bulk", body: dto.BulkBoardRequest{Action: management.BulkDelete, BoardIDs: []uuid.UUID{tenantBoardB}}},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given Alice holds nothing in organisation B
			x := newCrossTenantHarness(t)
			mountCrossTenantManagement(t, x)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)

			// when
			status, body := x.recordAsAlice(route)

			// then the guard reads the membership of the path's organisation, and no board of B is listed, restored or changed
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"organisation not found"}`, body)
		})
	}
}

func TestBoardManagementController_RestoringAnotherOrganisationsBoardThroughYourOwnAnswersNotFound(t *testing.T) {
	// given Alice owns organisation A, and asks A's path to restore board B of organisation B
	x := newCrossTenantHarness(t)
	mountCrossTenantManagement(t, x)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil)
	x.boardRepo.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: tenantOrgA, BoardID: tenantBoardB}).Return(nil, fmt.Errorf("get board: %w", dao.ErrNotFound))

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/boards/" + tenantBoardB.String() + "/restore"})

	// then the board is read inside the path's organisation only, so B is never found or restored
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"board not found"}`, body)
}

func TestBoardManagementController_ABulkActionThroughYourOwnOrganisationNeverTouchesAnotherOrganisationsBoard(t *testing.T) {
	// given Alice owns organisation A, and plants board B of organisation B in a bulk delete on A's path
	x := newCrossTenantHarness(t)
	mountCrossTenantManagement(t, x)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil)
	x.aliceHasNoAccessToBoardB()
	route := crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/boards/bulk", body: dto.BulkBoardRequest{Action: management.BulkDelete, BoardIDs: []uuid.UUID{tenantBoardB}}}

	// when
	status, body := x.recordAsAlice(route)

	// then board B answers as if it did not exist, and the strict mocks prove it is never deleted
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"results":[{"board_id":"`+tenantBoardB.String()+`","ok":false,"error":"board not found"}]}`, body)
}
