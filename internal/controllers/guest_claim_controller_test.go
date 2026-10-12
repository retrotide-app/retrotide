package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/guest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	claimHarness struct {
		*testutil.Harness
		claims *guest.MockClaimService
	}
)

var (
	claimUser   = uuid.MustParse("7a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d")
	claimDevice = uuid.MustParse("8b9c0d1e-2f3a-4b4c-9d5e-6f7a8b9c0d1e")
	claimBoard  = uuid.MustParse("9c0d1e2f-3a4b-4c5d-ae6f-7a8b9c0d1e2f")
	claimOffer  = &dto.GuestClaimOffer{Boards: []dto.GuestClaimBoard{{BoardID: claimBoard, Title: "Sprint 42", DisplayName: "Gil"}}}
)

func newClaimHarness(t *testing.T) *claimHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	ch := &claimHarness{Harness: h, claims: guest.NewMockClaimService(t)}

	s := &Service{GuestClaims: ch.claims, Guests: h.Guests, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllGuestClaimRoutes())

	return ch
}

func (ch *claimHarness) signedIn(method string) *http.Cookie {
	cookie := ch.SignIn(claimUser)
	if method != http.MethodGet {
		ch.Authz.EXPECT().IsLocked(mock.Anything, claimUser).Return(false, nil)
	}

	return cookie
}

func (ch *claimHarness) heldDevice() *http.Cookie {
	ch.GuestRepo.EXPECT().Get(mock.Anything, testutil.TestGuestToken).Return(&model.GuestDevice{ID: claimDevice, LastSeenAt: time.Now()}, nil)

	return &http.Cookie{Name: testutil.GuestCookieName, Value: testutil.TestGuestToken}
}

func TestGuestClaimController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{GuestClaims: guest.NewMockClaimService(t), Guests: h.Guests, Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllGuestClaimRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/auth/guest-claim",
		"HEAD /api/v1/auth/guest-claim",
		"POST /api/v1/auth/guest-claim",
		"DELETE /api/v1/auth/guest-claim",
	}, registeredRoutes(h.Engine))
}

func TestGuestClaimController_EveryRouteRunsTheAuthFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodGet, Path: "/api/v1/auth/guest-claim"},
		{Method: http.MethodPost, Path: "/api/v1/auth/guest-claim", Body: dto.GuestClaimRequest{BoardIDs: []uuid.UUID{claimBoard}}},
		{Method: http.MethodDelete, Path: "/api/v1/auth/guest-claim"},
	}

	for _, route := range routes {
		t.Run(route.Method, func(t *testing.T) {
			testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newClaimHarness(t).Harness
			}, route)
		})
	}
}

func TestGuestClaimController_OfferIsEmptyWithoutAGuestDevice(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(ch *claimHarness) []*http.Cookie
	}{
		{
			name: "no guest cookie",
			arrange: func(ch *claimHarness) []*http.Cookie {
				return []*http.Cookie{ch.signedIn(http.MethodGet)}
			},
		},
		{
			name: "a guest device that is gone",
			arrange: func(ch *claimHarness) []*http.Cookie {
				ch.GuestRepo.EXPECT().Get(mock.Anything, testutil.TestGuestToken).Return(nil, fmt.Errorf("get guest device: %w", dao.ErrNotFound))

				return []*http.Cookie{ch.signedIn(http.MethodGet), {Name: testutil.GuestCookieName, Value: testutil.TestGuestToken}}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ch := newClaimHarness(t)
			request := ch.NewRequest(http.MethodGet, "/api/v1/auth/guest-claim")
			for _, cookie := range tc.arrange(ch) {
				request = request.WithCookie(cookie)
			}

			ch.claims.EXPECT().Offer(mock.Anything, uuid.Nil).Return(&dto.GuestClaimOffer{Boards: []dto.GuestClaimBoard{}}, nil)

			// when
			rec := request.Record()

			// then the claim service is asked for no device, which offers nothing
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.JSONEq(t, `{"boards":[]}`, rec.Body.String())
		})
	}
}

func TestGuestClaimController_OfferListsTheBoardsTheDeviceJoined(t *testing.T) {
	// given a browser that joined as a guest and has just signed in with Google
	ch := newClaimHarness(t)
	session := ch.signedIn(http.MethodGet)
	device := ch.heldDevice()
	ch.claims.EXPECT().Offer(mock.Anything, claimDevice).Return(claimOffer, nil)

	// when
	rec := ch.NewRequest(http.MethodGet, "/api/v1/auth/guest-claim").WithCookie(session).WithCookie(device).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"boards":[{"board_id":"9c0d1e2f-3a4b-4c5d-ae6f-7a8b9c0d1e2f","title":"Sprint 42","display_name":"Gil"}]}`, rec.Body.String())
}

func TestGuestClaimController_ClaimHandsTheServiceTheUserTheDeviceAndTheChosenBoards(t *testing.T) {
	// given
	ch := newClaimHarness(t)
	session := ch.signedIn(http.MethodPost)
	device := ch.heldDevice()
	claim := spec.GuestClaim{UserID: claimUser, DeviceID: claimDevice, BoardIDs: []uuid.UUID{claimBoard}}
	ch.claims.EXPECT().Claim(mock.Anything, claim).Return(&dto.GuestClaimResult{Claimed: []uuid.UUID{claimBoard}}, nil)

	// when
	rec := ch.NewRequest(http.MethodPost, "/api/v1/auth/guest-claim").WithCookie(session).WithCookie(device).WithJSONBody(dto.GuestClaimRequest{BoardIDs: []uuid.UUID{claimBoard}}).Record()

	// then the guest cookie is cleared, so the tab reconnects as the user
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"claimed":["9c0d1e2f-3a4b-4c5d-ae6f-7a8b9c0d1e2f"]}`, rec.Body.String())
	cleared := testutil.ResponseCookie(t, rec, testutil.GuestCookieName)
	assert.Equal(t, -1, cleared.MaxAge)
}

func TestGuestClaimController_AnUnauditedClaimStillAnswersAndClears(t *testing.T) {
	// given
	ch := newClaimHarness(t)
	session := ch.signedIn(http.MethodPost)
	device := ch.heldDevice()
	ch.claims.EXPECT().Claim(mock.Anything, mock.Anything).Return(&dto.GuestClaimResult{Claimed: []uuid.UUID{claimBoard}}, fmt.Errorf("%w: %w", guest.ErrNotAudited, errors.New("audit store down")))

	// when
	rec := ch.NewRequest(http.MethodPost, "/api/v1/auth/guest-claim").WithCookie(session).WithCookie(device).WithJSONBody(dto.GuestClaimRequest{BoardIDs: []uuid.UUID{claimBoard}}).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, testutil.ResponseCookie(t, rec, testutil.GuestCookieName))
}

func TestGuestClaimController_ClaimRefusesABrowserWithoutAGuestDevice(t *testing.T) {
	// given
	ch := newClaimHarness(t)
	session := ch.signedIn(http.MethodPost)

	// when
	rec := ch.NewRequest(http.MethodPost, "/api/v1/auth/guest-claim").WithCookie(session).WithJSONBody(dto.GuestClaimRequest{BoardIDs: []uuid.UUID{claimBoard}}).Record()

	// then
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"this browser holds no guest identity to claim","code":"no_guest"}`, rec.Body.String())
}

func TestGuestClaimController_ClaimMapsTheRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: guest.ErrNothingClaimed, status: http.StatusBadRequest, body: `{"error":"choose at least one board to claim","code":"nothing_claimed"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
		{err: errors.New("participants store down"), status: http.StatusInternalServerError, body: `{"error":"failed to claim the guest boards"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			ch := newClaimHarness(t)
			session := ch.signedIn(http.MethodPost)
			device := ch.heldDevice()
			ch.claims.EXPECT().Claim(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("claim: %w", tc.err))

			// when
			rec := ch.NewRequest(http.MethodPost, "/api/v1/auth/guest-claim").WithCookie(session).WithCookie(device).WithJSONBody(dto.GuestClaimRequest{}).Record()

			// then the guest cookie stays, so the offer can be made again
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Result().Cookies())
		})
	}
}

func TestGuestClaimController_DeclineOnlyClearsTheCookie(t *testing.T) {
	// given
	ch := newClaimHarness(t)
	session := ch.signedIn(http.MethodDelete)

	// when
	rec := ch.NewRequest(http.MethodDelete, "/api/v1/auth/guest-claim").WithCookie(session).WithCookie(&http.Cookie{Name: testutil.GuestCookieName, Value: testutil.TestGuestToken}).Record()

	// then nothing is claimed and the device is left for the sweep
	assert.Equal(t, http.StatusNoContent, rec.Code)
	cleared := testutil.ResponseCookie(t, rec, testutil.GuestCookieName)
	assert.Equal(t, -1, cleared.MaxAge)
}
