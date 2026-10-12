package ws

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"retrotide/internal/model"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testGuestCookie = "rt_guest"
	testGuestToken  = "guest-token"
)

func newGuestSocketHarness(t *testing.T) (*socketHarness, *MockPrincipalValidator) {
	t.Helper()

	h := newSocketHarness(t)
	guests := NewMockPrincipalValidator(t)

	handler, err := NewHandler(HandlerDeps{Hub: h.hub, Settings: h.settings, Users: h.users, Guests: guests, Bans: h.bans, Boards: h.boards})
	require.NoError(t, err)
	handler.timings = h.handler.timings
	*h.handler = *handler

	guests.EXPECT().Token(mock.Anything).RunAndReturn(func(r *http.Request) (string, error) {
		found, err := r.Cookie(testGuestCookie)
		if errors.Is(err, http.ErrNoCookie) {
			return "", nil
		}
		if err != nil {
			return "", err
		}

		return found.Value, nil
	}).Maybe()

	return h, guests
}

func (h *socketHarness) guestHeader(withSession bool) http.Header {
	header := http.Header{}
	header.Set("Origin", h.server.URL)

	cookie := testGuestCookie + "=" + testGuestToken
	if withSession {
		cookie = testSessionCookie + "=" + testToken + "; " + cookie
	}
	header.Set("Cookie", cookie)

	return header
}

func TestHandler_OpensAGuestSocketFromTheGuestCookieAlone(t *testing.T) {
	// given
	h, guests := newGuestSocketHarness(t)
	principal := model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()}
	guests.EXPECT().ValidatePrincipal(mock.Anything, testGuestToken).Return(principal, nil)
	ref := boardRef()
	joined := make(chan model.Principal, 1)
	h.boards.EXPECT().HandleJoin(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(_ context.Context, conn Conn, _ uuid.UUID) (bool, error) {
		joined <- conn.Principal()

		return true, nil
	}).Once()
	h.boards.EXPECT().HandleLeave(mock.Anything, mock.Anything, ref.BoardID).Return(nil).Maybe()
	conn := h.dial(t, h.guestHeader(false))

	// when
	send(t, conn, TypeBoardJoin, ref)

	// then the guest joins as itself, and no ban is ever checked for a guest
	select {
	case got := <-joined:
		assert.Equal(t, principal, got)
	case <-time.After(frameWait):
		require.Fail(t, "the join never reached the board service")
	}
}

func TestHandler_FallsBackToTheGuestBehindAnInvalidSession(t *testing.T) {
	// given a browser whose session row is gone but whose guest device is live
	h, guests := newGuestSocketHarness(t)
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(model.Principal{}, fmt.Errorf("%w: expired", model.ErrInvalidPrincipal))
	guests.EXPECT().ValidatePrincipal(mock.Anything, testGuestToken).Return(model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()}, nil)

	// when
	conn, resp, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: h.guestHeader(true)})

	// then
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

func TestHandler_RefusesAnInvalidGuestWith401(t *testing.T) {
	// given
	h, guests := newGuestSocketHarness(t)
	guests.EXPECT().ValidatePrincipal(mock.Anything, testGuestToken).Return(model.Principal{}, fmt.Errorf("%w: expired", model.ErrInvalidPrincipal))

	// when
	_, resp, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: h.guestHeader(false)})

	// then
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandler_RefusesAGuestItCannotCheckWith503(t *testing.T) {
	// given
	h, guests := newGuestSocketHarness(t)
	guests.EXPECT().ValidatePrincipal(mock.Anything, testGuestToken).Return(model.Principal{}, errDatabaseDown)

	// when
	_, resp, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: h.guestHeader(false)})

	// then a database blip is never quietly an anonymous socket
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestHandler_ClosesAGuestSocketWhenRevalidationFindsTheDeviceGone(t *testing.T) {
	// given
	h, guests := newGuestSocketHarness(t)
	h.handler.timings = timings{pingInterval: 20 * time.Millisecond, pongTimeout: time.Second, revalidateEvery: 0}
	principal := model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()}
	guests.EXPECT().ValidatePrincipal(mock.Anything, testGuestToken).Return(principal, nil).Once()
	guests.EXPECT().ValidatePrincipal(mock.Anything, testGuestToken).Return(model.Principal{}, fmt.Errorf("%w: claimed", model.ErrInvalidPrincipal))
	killedBefore := testutil.ToFloat64(wsClientsKilled.WithLabelValues(killRevalidation))
	conn := h.dial(t, h.guestHeader(false))

	// when
	closeStatusOf(t, conn)
	h.served.Wait()

	// then
	assert.Equal(t, killedBefore+1, testutil.ToFloat64(wsClientsKilled.WithLabelValues(killRevalidation)))
}
