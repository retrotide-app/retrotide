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

var (
	facilitatedColumnPath = "/columns/" + boardColumn.String()
	facilitatedColumnOf   = board.ColumnCommand{Command: boardCommandOf, ColumnID: boardColumn}
)

func newColumnFacilitationHarness(t *testing.T) *boardHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	bh := &boardHarness{Harness: h, boards: board.NewMockService(t), live: live.NewMockService(t)}

	s := &Service{BoardService: bh.boards, BoardLive: bh.live, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardColumnFacilitationRoutes())

	return bh
}

func TestBoardColumnFacilitationController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardService: board.NewMockService(t), BoardLive: live.NewMockService(t), Guards: h.Guards}

	// when the list is mounted beside the column routes, so a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), append(s.getAllBoardColumnRoutes(), s.getAllBoardColumnFacilitationRoutes()...))

	// then
	assert.Subset(t, registeredRoutes(h.Engine), []string{
		"POST /api/v1/boards/:boardID/columns/:columnID/reveal",
		"POST /api/v1/boards/:boardID/columns/:columnID/hide",
		"POST /api/v1/boards/:boardID/columns/:columnID/show",
		"PUT /api/v1/boards/:boardID/columns/:columnID/vote-limit",
		"POST /api/v1/boards/:boardID/columns/:columnID/votes/reset",
	})
}

func TestBoardColumnFacilitationController_EveryRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: boardPath(facilitatedColumnPath + "/reveal"), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath(facilitatedColumnPath + "/hide"), Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath(facilitatedColumnPath + "/show"), Body: boardOpBody},
		{Method: http.MethodPut, Path: boardPath(facilitatedColumnPath + "/vote-limit"), Body: dto.ColumnVoteLimitRequest{BoardCommandRequest: boardOpBody, MaxVotes: new(3)}},
		{Method: http.MethodPost, Path: boardPath(facilitatedColumnPath + "/votes/reset"), Body: boardOpBody},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newColumnFacilitationHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardColumnFacilitationController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	tests := []boardCommandCase{
		{
			name:   "reveal the column's cards",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/reveal"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RevealColumn(mock.Anything, facilitatedColumnOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "hide the column",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/hide"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().HideColumn(mock.Anything, facilitatedColumnOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "show the column",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/show"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ShowColumn(mock.Anything, facilitatedColumnOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "limit the column's votes",
			method: http.MethodPut,
			path:   boardPath(facilitatedColumnPath + "/vote-limit"),
			body:   dto.ColumnVoteLimitRequest{BoardCommandRequest: boardOpBody, MaxVotes: new(3)},
			expect: func(bh *boardHarness) {
				limit := board.ColumnVoteLimitCommand{Command: boardCommandOf, ColumnID: boardColumn, MaxVotes: new(3)}
				bh.boards.EXPECT().SetColumnVoteLimit(mock.Anything, limit).Return(boardVersioned, nil)
			},
		},
		{
			name:   "lift the column's vote limit",
			method: http.MethodPut,
			path:   boardPath(facilitatedColumnPath + "/vote-limit"),
			body:   dto.ColumnVoteLimitRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				limit := board.ColumnVoteLimitCommand{Command: boardCommandOf, ColumnID: boardColumn}
				bh.boards.EXPECT().SetColumnVoteLimit(mock.Anything, limit).Return(boardVersioned, nil)
			},
		},
		{
			name:   "reset the column's votes",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/votes/reset"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ResetColumnVotes(mock.Anything, facilitatedColumnOf).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newColumnFacilitationHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardColumnFacilitationController_TheVoteLimitAnswersTheColumn(t *testing.T) {
	// given
	bh := newColumnFacilitationHarness(t)
	answer := &dto.BoardCommandResponse{Version: 8, Column: &dto.ColumnView{ID: boardColumn, Title: "Went well", Colour: boardColour, Position: "V", MaxVotes: new(3)}}
	bh.boards.EXPECT().SetColumnVoteLimit(mock.Anything, mock.Anything).Return(answer, nil)

	// when
	status, body := bh.record(http.MethodPut, boardPath(facilitatedColumnPath+"/vote-limit"), dto.ColumnVoteLimitRequest{BoardCommandRequest: boardOpBody, MaxVotes: new(3)})

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, answer), body)
	assert.Contains(t, body, `"max_votes":3`)
}

func TestBoardColumnFacilitationController_AColumnPathIDThatIsNotAUUIDIsRefused(t *testing.T) {
	// given
	bh := newColumnFacilitationHarness(t)

	// when
	status, body := bh.record(http.MethodPost, boardPath("/columns/not-a-uuid/reveal"), boardOpBody)

	// then the strict board mock proves the service is never asked
	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"invalid columnID"}`, body)
}

func TestBoardColumnFacilitationController_MapsEveryColumnRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: policy.ErrColumnHidden, status: http.StatusConflict, body: `{"error":"the column is hidden from participants, so nothing in it can change until it is shown","code":"column_hidden"}`},
		{err: board.ErrColumnVoteLimitReached, status: http.StatusConflict, body: `{"error":"you have used every vote this column allows","code":"column_vote_limit_reached"}`},
		{err: board.ErrInvalidColumnVoteLimit, status: http.StatusBadRequest, body: `{"error":"a column vote limit must be between 1 and 99, or none","code":"invalid_column_vote_limit"}`},
		{err: board.ErrColumnNotFound, status: http.StatusNotFound, body: `{"error":"column not found"}`},
		{err: board.ErrOpRequired, status: http.StatusBadRequest, body: `{"error":"every board command needs an op_id","code":"op_required"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newColumnFacilitationHarness(t)
			bh.boards.EXPECT().SetColumnVoteLimit(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("limit the column: %w", tc.err))

			// when
			status, body := bh.record(http.MethodPut, boardPath(facilitatedColumnPath+"/vote-limit"), dto.ColumnVoteLimitRequest{BoardCommandRequest: boardOpBody, MaxVotes: new(3)})

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardColumnFacilitationController_AnUnexpectedFailureAnswersTheRoutesOwnMessage(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		expect func(bh *boardHarness)
		want   string
	}{
		{
			name:   "reveal",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/reveal"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RevealColumn(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to reveal the column's cards"}`,
		},
		{
			name:   "hide",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/hide"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().HideColumn(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to hide the column"}`,
		},
		{
			name:   "show",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/show"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ShowColumn(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to show the column"}`,
		},
		{
			name:   "vote limit",
			method: http.MethodPut,
			path:   boardPath(facilitatedColumnPath + "/vote-limit"),
			body:   dto.ColumnVoteLimitRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().SetColumnVoteLimit(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to set the column's vote limit"}`,
		},
		{
			name:   "vote reset",
			method: http.MethodPost,
			path:   boardPath(facilitatedColumnPath + "/votes/reset"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ResetColumnVotes(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to reset the column's votes"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newColumnFacilitationHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.JSONEq(t, tc.want, body)
		})
	}
}

func TestBoardColumnFacilitationController_EveryRouteOfAnotherOrganisationsBoardAnswersNotFound(t *testing.T) {
	columnB := tenantBoardBPath + "/columns/" + tenantColumnB.String()
	routes := []crossTenantRoute{
		{method: http.MethodPost, path: columnB + "/reveal", body: tenantOpBody},
		{method: http.MethodPost, path: columnB + "/hide", body: tenantOpBody},
		{method: http.MethodPost, path: columnB + "/show", body: tenantOpBody},
		{method: http.MethodPut, path: columnB + "/vote-limit", body: dto.ColumnVoteLimitRequest{BoardCommandRequest: tenantOpBody, MaxVotes: new(3)}},
		{method: http.MethodPost, path: columnB + "/votes/reset", body: tenantOpBody},
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
			RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllBoardColumnFacilitationRoutes())
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(tenantBoardBOpen, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardB, UserID: tenantAlice}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))

			// when
			status, body := x.recordAsAlice(route)

			// then board B is not even shown to exist, and the strict mocks prove none of its columns or votes are written
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"board not found"}`, body)
		})
	}
}

func TestBoardColumnFacilitationController_AnotherBoardsColumnReachedThroughYourOwnBoardAnswersNotFound(t *testing.T) {
	columnBThroughA := tenantBoardAPath + "/columns/" + tenantColumnB.String()
	routes := []crossTenantRoute{
		{method: http.MethodPost, path: columnBThroughA + "/reveal", body: tenantOpBody},
		{method: http.MethodPost, path: columnBThroughA + "/hide", body: tenantOpBody},
		{method: http.MethodPost, path: columnBThroughA + "/show", body: tenantOpBody},
		{method: http.MethodPut, path: columnBThroughA + "/vote-limit", body: dto.ColumnVoteLimitRequest{BoardCommandRequest: tenantOpBody, MaxVotes: new(3)}},
		{method: http.MethodPost, path: columnBThroughA + "/votes/reset", body: tenantOpBody},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given Alice owns organisation A and facilitates board A, and names a column of board B
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
			RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllBoardColumnFacilitationRoutes())
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardA).Return(tenantBoardAOpen, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamA).Return(&model.Team{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardA, UserID: tenantAlice}).Return(tenantAliceSeat, nil)
			x.boardRepo.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: tenantBoardA, OpID: tenantOp}).Return(nil, fmt.Errorf("get op: %w", dao.ErrNotFound))
			x.columnRepo.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: tenantBoardA, ColumnID: tenantColumnB}).Return(nil, fmt.Errorf("get column: %w", dao.ErrNotFound))

			// when
			status, body := x.recordAsAlice(route)

			// then every read is scoped to board A, so board B's column is not even shown to exist and nothing is written
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"column not found"}`, body)
		})
	}
}
