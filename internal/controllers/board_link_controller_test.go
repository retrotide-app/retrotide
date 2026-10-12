package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	linkSlug       = "K3V9X2QPLMNORSTUVWXYZ234"
	issuedGuestKey = "5c8e1a3f7b9d2e4c6a8b0d1f3e5a7c9b1d3f5e7a9c1b3d5f7e9a1c3b5d7f9e2a"
)

var (
	linkGuest       = uuid.MustParse("6e7f8091-a2b3-44c5-96d7-4c5d6e7f8091")
	linkGuestPerson = model.Principal{Kind: model.PrincipalGuest, ID: linkGuest}
	linkJoinedBody  = dto.LinkJoinResponse{BoardID: boardID}
)

func newLinkHarness(t *testing.T) *boardHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	bh := &boardHarness{Harness: h, boards: board.NewMockService(t), live: live.NewMockService(t)}

	s := &Service{BoardService: bh.boards, BoardLive: bh.live, Guards: h.Guards, Guests: h.Guests}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardLinkRoutes())

	return bh
}

func linkPath(rest string) string {
	return "/api/v1/b/" + linkSlug + rest
}

func TestBoardLinkController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardLive: live.NewMockService(t), Guards: h.Guards, Guests: h.Guests}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardLinkRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/b/:slug",
		"HEAD /api/v1/b/:slug",
		"POST /api/v1/b/:slug/join",
	}, registeredRoutes(h.Engine))
}

func TestBoardLinkController_PreviewWorksWithoutAnyPrincipal(t *testing.T) {
	// given
	bh := newLinkHarness(t)
	preview := &dto.JoinPreview{RequiresPassword: true, CanJoin: true}
	bh.live.EXPECT().PreviewLink(mock.Anything, live.LinkQuery{Slug: linkSlug}).Return(preview, nil)

	// when
	rec := bh.NewRequest(http.MethodGet, linkPath("")).Record()

	// then the preview carries the three answers and nothing about the board itself
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"requires_password":true,"requires_account":false,"can_join":true}`, rec.Body.String())
}

func TestBoardLinkController_PreviewHandsTheServiceTheCallersPrincipal(t *testing.T) {
	tests := []struct {
		name      string
		arrange   func(bh *boardHarness) *http.Cookie
		principal model.Principal
	}{
		{name: "a signed-in user", arrange: func(bh *boardHarness) *http.Cookie { return bh.SignIn(boardUser) }, principal: boardPrincipal},
		{name: "a returning guest", arrange: func(bh *boardHarness) *http.Cookie { return bh.SignInAsGuest(linkGuest) }, principal: linkGuestPerson},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newLinkHarness(t)
			cookie := tc.arrange(bh)
			bh.live.EXPECT().PreviewLink(mock.Anything, live.LinkQuery{Principal: &tc.principal, Slug: linkSlug}).Return(&dto.JoinPreview{CanJoin: true}, nil)

			// when
			rec := bh.NewRequest(http.MethodGet, linkPath("")).WithCookie(cookie).Record()

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestBoardLinkController_PreviewTreatsAGoneGuestDeviceAsNoPrincipal(t *testing.T) {
	// given a guest cookie whose device was swept or claimed
	bh := newLinkHarness(t)
	bh.GuestRepo.EXPECT().Get(mock.Anything, testutil.TestGuestToken).Return(nil, fmt.Errorf("get guest device: %w", dao.ErrNotFound))
	bh.live.EXPECT().PreviewLink(mock.Anything, live.LinkQuery{Slug: linkSlug}).Return(&dto.JoinPreview{CanJoin: true}, nil)

	// when
	rec := bh.NewRequest(http.MethodGet, linkPath("")).WithCookie(&http.Cookie{Name: testutil.GuestCookieName, Value: testutil.TestGuestToken}).Record()

	// then the visitor is offered the join afresh
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBoardLinkController_PreviewAnswersServiceUnavailableWhenTheGuestCannotBeChecked(t *testing.T) {
	// given
	bh := newLinkHarness(t)
	bh.GuestRepo.EXPECT().Get(mock.Anything, testutil.TestGuestToken).Return(nil, errors.New("guest store down"))

	// when
	rec := bh.NewRequest(http.MethodGet, linkPath("")).WithCookie(&http.Cookie{Name: testutil.GuestCookieName, Value: testutil.TestGuestToken}).Record()

	// then a store that cannot answer is never a silent downgrade to a fresh visitor
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"error":"service unavailable"}`, rec.Body.String())
}

func TestBoardLinkController_PreviewMapsTheRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{err: live.ErrLinkDisabled, status: http.StatusForbidden, body: `{"error":"the board's link is switched off","code":"link_disabled"}`},
		{err: live.ErrRemoved, status: http.StatusForbidden, body: `{"error":"you were removed from this board","code":"removed"}`},
		{err: live.ErrArchived, status: http.StatusConflict, body: `{"error":"the board is archived","code":"archived"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newLinkHarness(t)
			bh.live.EXPECT().PreviewLink(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("preview: %w", tc.err))

			// when
			rec := bh.NewRequest(http.MethodGet, linkPath("")).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestBoardLinkController_JoinIssuesTheGuestCookieToAVisitor(t *testing.T) {
	// given
	bh := newLinkHarness(t)
	command := live.LinkJoinCommand{Slug: linkSlug, DisplayName: "Gil", Password: "hunter2"}
	bh.live.EXPECT().JoinByLink(mock.Anything, command).Return(&live.LinkJoined{Response: linkJoinedBody, GuestToken: issuedGuestKey}, nil)

	// when
	rec := bh.NewRequest(http.MethodPost, linkPath("/join")).WithJSONBody(dto.JoinBoardRequest{DisplayName: "Gil", Password: "hunter2"}).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, mustJSON(t, linkJoinedBody), rec.Body.String())

	cookie := testutil.ResponseCookie(t, rec, testutil.GuestCookieName)
	assert.Equal(t, issuedGuestKey, cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, "/", cookie.Path)
	assert.Empty(t, cookie.Domain)
	assert.Equal(t, testutil.TestGuestDays*24*int(time.Hour/time.Second), cookie.MaxAge)
}

func TestBoardLinkController_JoinSetsNoCookieForAKnownPrincipal(t *testing.T) {
	tests := []struct {
		name      string
		arrange   func(bh *boardHarness) *http.Cookie
		principal model.Principal
	}{
		{
			name: "a signed-in user",
			arrange: func(bh *boardHarness) *http.Cookie {
				cookie := bh.SignIn(boardUser)
				bh.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)

				return cookie
			},
			principal: boardPrincipal,
		},
		{name: "a returning guest", arrange: func(bh *boardHarness) *http.Cookie { return bh.SignInAsGuest(linkGuest) }, principal: linkGuestPerson},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newLinkHarness(t)
			cookie := tc.arrange(bh)
			command := live.LinkJoinCommand{Principal: &tc.principal, Slug: linkSlug}
			bh.live.EXPECT().JoinByLink(mock.Anything, command).Return(&live.LinkJoined{Response: linkJoinedBody}, nil)

			// when
			rec := bh.NewRequest(http.MethodPost, linkPath("/join")).WithCookie(cookie).WithJSONBody(dto.JoinBoardRequest{}).Record()

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Result().Cookies())
		})
	}
}

func TestBoardLinkController_JoinMapsTheRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{err: live.ErrGuestsNotAllowed, status: http.StatusForbidden, body: `{"error":"this organisation does not let guests join its boards","code":"guests_not_allowed"}`},
		{err: live.ErrLinkDisabled, status: http.StatusForbidden, body: `{"error":"the board's link is switched off","code":"link_disabled"}`},
		{err: live.ErrAccountRequired, status: http.StatusForbidden, body: `{"error":"the board needs you to sign in","code":"account_required"}`},
		{err: live.ErrPasswordRequired, status: http.StatusForbidden, body: `{"error":"the board needs its password","code":"password_required"}`},
		{err: live.ErrWrongPassword, status: http.StatusForbidden, body: `{"error":"that is not the board's password","code":"wrong_password"}`},
		{err: live.ErrRemoved, status: http.StatusForbidden, body: `{"error":"you were removed from this board","code":"removed"}`},
		{err: live.ErrArchived, status: http.StatusConflict, body: `{"error":"the board is archived","code":"archived"}`},
		{err: live.ErrBoardFull, status: http.StatusTooManyRequests, body: `{"error":"the board already holds as many participants as it can","code":"board_full"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
		{err: errors.New("participants store down"), status: http.StatusInternalServerError, body: `{"error":"failed to join the board"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newLinkHarness(t)
			bh.live.EXPECT().JoinByLink(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("join: %w", tc.err))

			// when
			rec := bh.NewRequest(http.MethodPost, linkPath("/join")).WithJSONBody(dto.JoinBoardRequest{}).Record()

			// then no guest cookie is ever set for a refused join
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Result().Cookies())
		})
	}
}

func TestBoardLinkController_JoinAnswersASpentPasswordBudgetWithRetryAfter(t *testing.T) {
	// given
	bh := newLinkHarness(t)
	spent := live.NewPasswordAttemptsError(boardID, 41500*time.Millisecond)
	bh.live.EXPECT().JoinByLink(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("join: %w", spent))

	// when
	rec := bh.NewRequest(http.MethodPost, linkPath("/join")).WithJSONBody(dto.JoinBoardRequest{Password: "guess"}).Record()

	// then
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "42", rec.Header().Get("Retry-After"))
	assert.JSONEq(t, `{"error":"too many wrong passwords for this board, so try again later","code":"password_attempts"}`, rec.Body.String())
}

func TestBoardLinkController_JoinLimitIsPerSlugAndPrincipalWithAnIPCeiling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a guest who has spent the per-link budget
		bh := newLinkHarness(t)
		bh.GuestRepo.EXPECT().Get(mock.Anything, testutil.TestGuestToken).Return(&model.GuestDevice{ID: linkGuest, LastSeenAt: time.Now()}, nil)
		bh.live.EXPECT().JoinByLink(mock.Anything, mock.Anything).Return(&live.LinkJoined{Response: linkJoinedBody}, nil)
		guestCookie := &http.Cookie{Name: testutil.GuestCookieName, Value: testutil.TestGuestToken}

		for range middleware.BoardJoinsPerMinute {
			rec := bh.NewRequest(http.MethodPost, linkPath("/join")).WithCookie(guestCookie).WithJSONBody(dto.JoinBoardRequest{}).Record()
			require.Equal(t, http.StatusOK, rec.Code)
		}

		// when the same guest tries again, and a fresh visitor behind the same address joins
		limited := bh.NewRequest(http.MethodPost, linkPath("/join")).WithCookie(guestCookie).WithJSONBody(dto.JoinBoardRequest{}).Record()
		fresh := bh.NewRequest(http.MethodPost, linkPath("/join")).WithJSONBody(dto.JoinBoardRequest{}).Record()

		// then only the guest is refused, so a whole office behind one address can still join
		assert.Equal(t, http.StatusTooManyRequests, limited.Code)
		assert.Equal(t, http.StatusOK, fresh.Code)
	})
}

func TestBoardLinkController_AGuestReachesTheBoardRoutesAsItself(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	cookie := bh.SignInAsGuest(linkGuest)
	snapshot := &dto.BoardSnapshot{BoardID: boardID, Version: 3, Columns: []dto.ColumnView{}, Cards: []dto.CardView{}, HiddenCounts: map[uuid.UUID]int{}, Participants: []dto.ParticipantView{}}
	bh.boards.EXPECT().GetSnapshot(mock.Anything, spec.BoardSnapshotQuery{Principal: linkGuestPerson, BoardID: boardID}).Return(snapshot, nil)

	// when
	rec := bh.NewRequest(http.MethodGet, boardPath("")).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, mustJSON(t, snapshot), rec.Body.String())
}
