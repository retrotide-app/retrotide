package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/cardpolish"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	cardPolishHarness struct {
		*testutil.Harness
		polish *cardpolish.MockService
	}
)

var (
	cardPolishPinBody = dto.CardPinRequest{BoardCommandRequest: boardOpBody, Pinned: true}
)

func newCardPolishHarness(t *testing.T) *cardPolishHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	ph := &cardPolishHarness{Harness: h, polish: cardpolish.NewMockService(t)}

	s := &Service{BoardCardPolish: ph.polish, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardCardPolishRoutes())

	return ph
}

func (ph *cardPolishHarness) record(method string, path string, body any) (int, string) {
	cookie := ph.SignIn(boardUser)
	ph.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)

	request := ph.NewRequest(method, path).WithCookie(cookie)
	if body != nil {
		request = request.WithJSONBody(body)
	}

	rec := request.Record()

	return rec.Code, rec.Body.String()
}

func TestBoardCardPolishController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardCardPolish: cardpolish.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardCardPolishRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"PUT /api/v1/boards/:boardID/cards/:cardID/reactions/:reaction",
		"DELETE /api/v1/boards/:boardID/cards/:cardID/reactions/:reaction",
		"PUT /api/v1/boards/:boardID/cards/:cardID/pin",
	}, registeredRoutes(h.Engine))
}

func TestBoardCardPolishController_EveryRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPut, Path: boardPath("/cards/" + boardCard.String() + "/reactions/heart"), Body: boardOpBody},
		{Method: http.MethodDelete, Path: boardPath("/cards/" + boardCard.String() + "/reactions/heart"), Body: boardOpBody},
		{Method: http.MethodPut, Path: boardPath("/cards/" + boardCard.String() + "/pin"), Body: cardPolishPinBody},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newCardPolishHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardCardPolishController_AddingAReactionAnswersTheCardsCounts(t *testing.T) {
	// given
	ph := newCardPolishHarness(t)
	command := cardpolish.ReactCommand{Command: boardCommandOf, CardID: boardCard, Reaction: model.ReactionHeart}
	ph.polish.EXPECT().React(mock.Anything, command).Return(&dto.BoardCommandResponse{
		Version: 7,
		Reactions: &dto.CardReactionsView{
			CardID:      boardCard,
			Reactions:   []dto.ReactionCountView{{Reaction: model.ReactionHeart, Count: 2}},
			MyReactions: []model.Reaction{model.ReactionHeart},
		},
	}, nil)

	// when
	status, body := ph.record(http.MethodPut, boardPath("/cards/"+boardCard.String()+"/reactions/heart"), boardOpBody)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":7,"reactions":{"card_id":"`+boardCard.String()+`","reactions":[{"reaction":"heart","count":2}],"my_reactions":["heart"]}}`, body)
}

func TestBoardCardPolishController_RemovingAReactionBuildsTheCommandFromThePath(t *testing.T) {
	// given
	ph := newCardPolishHarness(t)
	command := cardpolish.ReactCommand{Command: boardCommandOf, CardID: boardCard, Reaction: model.ReactionRocket}
	ph.polish.EXPECT().Unreact(mock.Anything, command).Return(&dto.BoardCommandResponse{
		Version:   7,
		Reactions: &dto.CardReactionsView{CardID: boardCard, Reactions: []dto.ReactionCountView{}, MyReactions: []model.Reaction{}},
	}, nil)

	// when
	status, body := ph.record(http.MethodDelete, boardPath("/cards/"+boardCard.String()+"/reactions/rocket"), boardOpBody)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":7,"reactions":{"card_id":"`+boardCard.String()+`","reactions":[],"my_reactions":[]}}`, body)
}

func TestBoardCardPolishController_PinningBuildsTheCommandFromThePathAndTheBody(t *testing.T) {
	tests := []struct {
		name   string
		pinned bool
	}{
		{name: "pin", pinned: true},
		{name: "unpin", pinned: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ph := newCardPolishHarness(t)
			ph.polish.EXPECT().Pin(mock.Anything, cardpolish.PinCommand{Command: boardCommandOf, CardID: boardCard, Pinned: tc.pinned}).Return(boardVersioned, nil)

			// when
			status, body := ph.record(http.MethodPut, boardPath("/cards/"+boardCard.String()+"/pin"), dto.CardPinRequest{BoardCommandRequest: boardOpBody, Pinned: tc.pinned})

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardCardPolishController_ABadPathOrBodyIsRefusedBeforeTheService(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   string
	}{
		{name: "a card ID that is not a UUID", method: http.MethodPut, path: boardPath("/cards/card-1/reactions/heart"), body: `{"op_id":"` + boardOp.String() + `"}`, want: `{"error":"invalid cardID"}`},
		{name: "a board ID that is not a UUID", method: http.MethodDelete, path: "/api/v1/boards/board-1/cards/" + boardCard.String() + "/reactions/heart", body: `{"op_id":"` + boardOp.String() + `"}`, want: `{"error":"invalid boardID"}`},
		{name: "a reaction body that is not JSON", method: http.MethodPut, path: boardPath("/cards/" + boardCard.String() + "/reactions/heart"), body: `{"op_id":`, want: `{"error":"invalid request body"}`},
		{name: "a pin flag that is not a boolean", method: http.MethodPut, path: boardPath("/cards/" + boardCard.String() + "/pin"), body: `{"op_id":"` + boardOp.String() + `","pinned":"yes"}`, want: `{"error":"invalid request body"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ph := newCardPolishHarness(t)
			cookie := ph.SignIn(boardUser)
			ph.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)

			// when
			rec := ph.NewRequest(tc.method, tc.path).WithCookie(cookie).WithRawBody(tc.body, "application/json").Record()

			// then the strict mock proves the service is never asked
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestBoardCardPolishController_MapsEveryRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: cardpolish.ErrUnknownReaction, status: http.StatusBadRequest, body: `{"error":"that is not one of the board reactions","code":"unknown_reaction"}`},
		{err: policy.ErrCardNotPublic, status: http.StatusConflict, body: `{"error":"only a card everyone can see can be reacted to or pinned","code":"card_not_public"}`},
		{err: policy.ErrCardNotFound, status: http.StatusNotFound, body: `{"error":"card not found"}`},
		{err: policy.ErrNotParticipant, status: http.StatusConflict, body: `{"error":"join the board before writing on it","code":"not_participant"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{err: board.ErrOpRequired, status: http.StatusBadRequest, body: `{"error":"every board command needs an op_id","code":"op_required"}`},
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			ph := newCardPolishHarness(t)
			ph.polish.EXPECT().React(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("react: %w", tc.err))

			// when
			status, body := ph.record(http.MethodPut, boardPath("/cards/"+boardCard.String()+"/reactions/heart"), boardOpBody)

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardCardPolishController_PinningAHiddenCardAnswersCardNotPublic(t *testing.T) {
	// given
	ph := newCardPolishHarness(t)
	ph.polish.EXPECT().Pin(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("pin: %w", policy.ErrCardNotPublic))

	// when
	status, body := ph.record(http.MethodPut, boardPath("/cards/"+boardCard.String()+"/pin"), cardPolishPinBody)

	// then
	assert.Equal(t, http.StatusConflict, status)
	assert.JSONEq(t, `{"error":"only a card everyone can see can be reacted to or pinned","code":"card_not_public"}`, body)
}

func TestBoardCardPolishController_AnUnexpectedFailureAnswersTheRoutesOwnMessage(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		expect func(ph *cardPolishHarness)
		want   string
	}{
		{
			name:   "add a reaction",
			method: http.MethodPut,
			path:   boardPath("/cards/" + boardCard.String() + "/reactions/heart"),
			body:   boardOpBody,
			expect: func(ph *cardPolishHarness) {
				ph.polish.EXPECT().React(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to add the reaction"}`,
		},
		{
			name:   "remove a reaction",
			method: http.MethodDelete,
			path:   boardPath("/cards/" + boardCard.String() + "/reactions/heart"),
			body:   boardOpBody,
			expect: func(ph *cardPolishHarness) {
				ph.polish.EXPECT().Unreact(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to remove the reaction"}`,
		},
		{
			name:   "pin",
			method: http.MethodPut,
			path:   boardPath("/cards/" + boardCard.String() + "/pin"),
			body:   cardPolishPinBody,
			expect: func(ph *cardPolishHarness) {
				ph.polish.EXPECT().Pin(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to pin the card"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ph := newCardPolishHarness(t)
			tc.expect(ph)

			// when
			status, body := ph.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.JSONEq(t, tc.want, body)
		})
	}
}

func TestBoardCardPolishController_EveryRouteOfAnotherOrganisationsBoardAnswersNotFound(t *testing.T) {
	cardPath := tenantBoardBPath + "/cards/" + tenantCardB.String()
	routes := []crossTenantRoute{
		{method: http.MethodPut, path: cardPath + "/reactions/heart", body: tenantOpBody},
		{method: http.MethodDelete, path: cardPath + "/reactions/heart", body: tenantOpBody},
		{method: http.MethodPut, path: cardPath + "/pin", body: dto.CardPinRequest{BoardCommandRequest: tenantOpBody, Pinned: true}},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given board B belongs to a team of organisation B, where Alice holds nothing and has no seat on the board
			x := newCrossTenantHarness(t)
			orgAuthz := orgauthz.NewService(x.orgRepo, x.teamRepo)
			boardLive := live.New(live.Deps{Boards: x.boardRepo, Cards: x.cardRepo, OrgAuthz: orgAuthz})
			polish := cardpolish.NewService(cardpolish.Deps{
				Boards:    x.boardRepo,
				Cards:     x.cardRepo,
				Reactions: repository.NewMockCardReactionRepository(t),
				Live:      boardLive,
				Events:    boardLive,
			})
			s := &Service{BoardCardPolish: polish, BoardLive: boardLive, Guards: x.Guards}
			RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllBoardCardPolishRoutes())
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(tenantBoardBOpen, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardB, UserID: tenantAlice}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))

			// when
			status, body := x.recordAsAlice(route)

			// then board B is not even shown to exist, and the strict mocks prove no reaction or pin is ever written
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"board not found"}`, body)
		})
	}
}
