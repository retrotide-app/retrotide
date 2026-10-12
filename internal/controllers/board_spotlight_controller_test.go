package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newSpotlightHarness(t *testing.T) *boardHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	bh := &boardHarness{Harness: h, boards: board.NewMockService(t), live: live.NewMockService(t)}

	s := &Service{BoardService: bh.boards, BoardLive: bh.live, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardSpotlightRoutes())

	return bh
}

func TestBoardSpotlightController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardService: board.NewMockService(t), BoardLive: live.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardSpotlightRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"POST /api/v1/boards/:boardID/presentation",
		"POST /api/v1/boards/:boardID/highlight",
		"POST /api/v1/boards/:boardID/focus",
	}, registeredRoutes(h.Engine))
}

func TestBoardSpotlightController_EveryRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: boardPath("/presentation"), Body: dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody, CardID: new(boardCard)}},
		{Method: http.MethodPost, Path: boardPath("/highlight"), Body: dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody, CardID: new(boardCard)}},
		{Method: http.MethodPost, Path: boardPath("/focus"), Body: dto.ColumnFocusRequest{BoardCommandRequest: boardOpBody, ColumnID: new(boardColumn)}},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newSpotlightHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardSpotlightController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	tests := []boardCommandCase{
		{
			name:   "present a card",
			method: http.MethodPost,
			path:   boardPath("/presentation"),
			body:   dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody, CardID: new(boardCard)},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Present(mock.Anything, board.CardSpotlightCommand{Command: boardCommandOf, CardID: new(boardCard)}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "end the presentation",
			method: http.MethodPost,
			path:   boardPath("/presentation"),
			body:   dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Present(mock.Anything, board.CardSpotlightCommand{Command: boardCommandOf}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "highlight a card",
			method: http.MethodPost,
			path:   boardPath("/highlight"),
			body:   dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody, CardID: new(boardCard)},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Highlight(mock.Anything, board.CardSpotlightCommand{Command: boardCommandOf, CardID: new(boardCard)}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "remove the highlight",
			method: http.MethodPost,
			path:   boardPath("/highlight"),
			body:   dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Highlight(mock.Anything, board.CardSpotlightCommand{Command: boardCommandOf}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "focus a column",
			method: http.MethodPost,
			path:   boardPath("/focus"),
			body:   dto.ColumnFocusRequest{BoardCommandRequest: boardOpBody, ColumnID: new(boardColumn)},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Focus(mock.Anything, board.ColumnFocusCommand{Command: boardCommandOf, ColumnID: new(boardColumn)}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "show every column again",
			method: http.MethodPost,
			path:   boardPath("/focus"),
			body:   dto.ColumnFocusRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Focus(mock.Anything, board.ColumnFocusCommand{Command: boardCommandOf}).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newSpotlightHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardSpotlightController_ABodyThatIsNotTheRequestIsRefused(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "a card ID that is not a UUID", path: boardPath("/presentation"), body: `{"op_id":"` + boardOp.String() + `","card_id":"card-1"}`},
		{name: "a highlight that is not JSON", path: boardPath("/highlight"), body: `{"op_id":`},
		{name: "a column ID that is not a UUID", path: boardPath("/focus"), body: `{"op_id":"` + boardOp.String() + `","column_id":42}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newSpotlightHarness(t)
			cookie := bh.SignIn(boardUser)
			bh.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)

			// when
			rec := bh.NewRequest(http.MethodPost, tc.path).WithCookie(cookie).WithRawBody(tc.body, "application/json").Record()

			// then the strict board mock proves the service is never asked
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"invalid request body"}`, rec.Body.String())
		})
	}
}

func TestBoardSpotlightController_MapsEverySpotlightRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: policy.ErrCardNotShown, status: http.StatusConflict, body: `{"error":"only a card everyone can see can be presented or highlighted","code":"card_not_shown"}`},
		{err: policy.ErrColumnHidden, status: http.StatusConflict, body: `{"error":"the column is hidden from participants, so nothing in it can change until it is shown","code":"column_hidden"}`},
		{err: policy.ErrCardNotFound, status: http.StatusNotFound, body: `{"error":"card not found"}`},
		{err: board.ErrColumnNotFound, status: http.StatusNotFound, body: `{"error":"column not found"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{err: board.ErrOpRequired, status: http.StatusBadRequest, body: `{"error":"every board command needs an op_id","code":"op_required"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newSpotlightHarness(t)
			bh.boards.EXPECT().Present(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("present the card: %w", tc.err))

			// when
			status, body := bh.record(http.MethodPost, boardPath("/presentation"), dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody, CardID: new(boardCard)})

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardSpotlightController_AFocusOnAHiddenColumnAnswersColumnHidden(t *testing.T) {
	// given
	bh := newSpotlightHarness(t)
	bh.boards.EXPECT().Focus(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("focus: %w", policy.ErrColumnHidden))

	// when
	status, body := bh.record(http.MethodPost, boardPath("/focus"), dto.ColumnFocusRequest{BoardCommandRequest: boardOpBody, ColumnID: new(boardColumn)})

	// then
	assert.Equal(t, http.StatusConflict, status)
	assert.JSONEq(t, `{"error":"the column is hidden from participants, so nothing in it can change until it is shown","code":"column_hidden"}`, body)
}

func TestBoardSpotlightController_AnUnexpectedFailureAnswersTheRoutesOwnMessage(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   any
		expect func(bh *boardHarness)
		want   string
	}{
		{
			name: "presentation",
			path: boardPath("/presentation"),
			body: dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Present(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to change the presentation"}`,
		},
		{
			name: "highlight",
			path: boardPath("/highlight"),
			body: dto.CardSpotlightRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Highlight(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to change the highlight"}`,
		},
		{
			name: "focus",
			path: boardPath("/focus"),
			body: dto.ColumnFocusRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().Focus(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to change the focused column"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newSpotlightHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(http.MethodPost, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.JSONEq(t, tc.want, body)
		})
	}
}

func TestBoardSpotlightController_EveryRouteOfAnotherOrganisationsBoardAnswersNotFound(t *testing.T) {
	routes := []crossTenantRoute{
		{method: http.MethodPost, path: tenantBoardBPath + "/presentation", body: dto.CardSpotlightRequest{BoardCommandRequest: tenantOpBody, CardID: new(tenantCardB)}},
		{method: http.MethodPost, path: tenantBoardBPath + "/highlight", body: dto.CardSpotlightRequest{BoardCommandRequest: tenantOpBody, CardID: new(tenantCardB)}},
		{method: http.MethodPost, path: tenantBoardBPath + "/focus", body: dto.ColumnFocusRequest{BoardCommandRequest: tenantOpBody, ColumnID: new(tenantColumnB)}},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given board B belongs to a team of organisation B, where Alice holds nothing and has no seat on the board
			x := newCrossTenantHarness(t)
			orgAuthz := orgauthz.NewService(x.orgRepo, x.teamRepo)
			boardLive := live.New(live.Deps{Boards: x.boardRepo, Cards: x.cardRepo, OrgAuthz: orgAuthz})
			boards := board.NewService(board.Deps{
				Boards:         x.boardRepo,
				Columns:        x.columnRepo,
				Cards:          x.cardRepo,
				Votes:          x.voteRepo,
				Comments:       x.noteRepo,
				Teams:          x.teamRepo,
				Organisations:  x.orgRepo,
				Live:           boardLive,
				Events:         boardLive,
				OrgAuthz:       orgAuthz,
				SavedTemplates: x.savedRepo,
			})
			s := &Service{BoardService: boards, BoardLive: boardLive, Guards: x.Guards}
			RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllBoardSpotlightRoutes())
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(tenantBoardBOpen, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardB, UserID: tenantAlice}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))

			// when
			status, body := x.recordAsAlice(route)

			// then board B is not even shown to exist, and the strict board mock proves its spotlight is never written
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"board not found"}`, body)
		})
	}
}
