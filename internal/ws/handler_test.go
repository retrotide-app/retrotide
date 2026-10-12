package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/model"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	socketHarness struct {
		hub      *Hub
		handler  *Handler
		server   *httptest.Server
		settings *MockSettingsReader
		users    *MockPrincipalValidator
		bans     *MockBanChecker
		boards   *MockBoardPresence
		served   sync.WaitGroup
	}

	frame struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
)

const (
	testSessionCookie = "rt_session"
	testToken         = "session-token"
	frameWait         = 5 * time.Second
)

func newSocketHarness(t *testing.T) *socketHarness {
	t.Helper()

	h := &socketHarness{
		hub:      NewHub(),
		settings: NewMockSettingsReader(t),
		users:    NewMockPrincipalValidator(t),
		bans:     NewMockBanChecker(t),
		boards:   NewMockBoardPresence(t),
	}

	handler, err := NewHandler(HandlerDeps{Hub: h.hub, Settings: h.settings, Users: h.users, Bans: h.bans, Boards: h.boards})
	require.NoError(t, err)
	handler.timings = timings{pingInterval: time.Hour, pongTimeout: time.Hour, revalidateEvery: time.Hour}
	h.handler = handler

	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.served.Add(1)
		defer h.served.Done()

		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(func() {
		h.served.Wait()
		h.server.Close()
	})

	h.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(h.server.URL, nil).Maybe()
	h.users.EXPECT().Token(mock.Anything).RunAndReturn(func(r *http.Request) (string, error) {
		found, err := r.Cookie(testSessionCookie)
		if errors.Is(err, http.ErrNoCookie) {
			return "", nil
		}
		if err != nil {
			return "", err
		}

		return found.Value, nil
	}).Maybe()

	return h
}

func (h *socketHarness) url() string {
	return "ws" + strings.TrimPrefix(h.server.URL, "http")
}

func (h *socketHarness) signedInHeader() http.Header {
	header := http.Header{}
	header.Set("Origin", h.server.URL)
	header.Set("Cookie", testSessionCookie+"="+testToken)

	return header
}

func (h *socketHarness) anonymousHeader() http.Header {
	header := http.Header{}
	header.Set("Origin", h.server.URL)

	return header
}

func (h *socketHarness) expectUser(t *testing.T) model.Principal {
	t.Helper()

	principal := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(principal, nil)
	h.bans.EXPECT().IsBanned(mock.Anything, principal.ID).Return(false, nil)

	return principal
}

func (h *socketHarness) dialWith(t *testing.T, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), frameWait)
	defer cancel()

	opts.HTTPClient = h.server.Client()

	conn, resp, err := websocket.Dial(ctx, h.url(), opts)
	if conn != nil {
		t.Cleanup(func() {
			_ = conn.CloseNow()
		})
	}

	return conn, resp, err
}

func (h *socketHarness) dial(t *testing.T, header http.Header) *websocket.Conn {
	t.Helper()

	conn, _, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: header})
	require.NoError(t, err)

	return conn
}

func send(t *testing.T, conn *websocket.Conn, msgType string, data any) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), frameWait)
	defer cancel()

	require.NoError(t, wsjson.Write(ctx, conn, map[string]any{"type": msgType, "data": data}))
}

func receive(t *testing.T, conn *websocket.Conn) frame {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), frameWait)
	defer cancel()

	var got frame
	require.NoError(t, wsjson.Read(ctx, conn, &got))

	return got
}

func closeStatusOf(t *testing.T, conn *websocket.Conn) websocket.StatusCode {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), frameWait)
	defer cancel()

	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			require.NoError(t, ctx.Err(), "the socket never closed")
			return websocket.CloseStatus(err)
		}
	}
}

func boardRef() BoardRef {
	return BoardRef{BoardID: uuid.New()}
}

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed string
		want    bool
	}{
		{name: "exact match", origin: "https://retrotide.app", allowed: "https://retrotide.app", want: true},
		{name: "localhost dev", origin: "http://localhost:5174", allowed: "http://localhost:5174", want: true},
		{name: "different host", origin: "https://evil.example.com", allowed: "https://retrotide.app", want: false},
		{name: "different scheme", origin: "http://retrotide.app", allowed: "https://retrotide.app", want: false},
		{name: "different port", origin: "https://retrotide.app:8080", allowed: "https://retrotide.app", want: false},
		{name: "a browser origin with a trailing slash is still refused", origin: "https://retrotide.app/", allowed: "https://retrotide.app", want: false},
		{name: "a base_url with a trailing slash is tolerated", origin: "https://retrotide.app", allowed: "https://retrotide.app/", want: true},
		{name: "an empty origin is refused", origin: "", allowed: "https://retrotide.app", want: false},
		{name: "an empty base_url refuses everything", origin: "https://retrotide.app", allowed: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := OriginAllowed(tt.origin, tt.allowed)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePing(t *testing.T) {
	nonce := int64(3)

	tests := []struct {
		name string
		data string
		want pingData
	}{
		{name: "a nonce and a reported round trip are kept", data: `{"nonce":3,"rtt":240}`, want: pingData{Nonce: &nonce, RTT: 240}},
		{name: "a keepalive carries no nonce", data: `{}`, want: pingData{}},
		{name: "an absent payload carries no nonce", data: ``, want: pingData{}},
		{name: "a round trip beyond any real network is discarded", data: `{"nonce":3,"rtt":600000}`, want: pingData{Nonce: &nonce}},
		{name: "a negative round trip is discarded", data: `{"nonce":3,"rtt":-5}`, want: pingData{Nonce: &nonce}},
		{name: "an unreadable payload yields an empty ping", data: `[1,2,3]`, want: pingData{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := parsePing(json.RawMessage(tt.data))

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBoundReportedRTT(t *testing.T) {
	tests := []struct {
		name     string
		reported int
		measured int64
		want     int
	}{
		{name: "without a measurement the report stands", reported: 300, measured: 0, want: 300},
		{name: "a report within the ceiling stands", reported: 150, measured: 100, want: 150},
		{name: "a report over twice the measurement plus slack is capped", reported: 5000, measured: 100, want: 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := boundReportedRTT(tt.reported, tt.measured)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsUpgradeRequest(t *testing.T) {
	tests := []struct {
		name       string
		connection string
		upgrade    string
		want       bool
	}{
		{name: "a WebSocket upgrade", connection: "Upgrade", upgrade: "websocket", want: true},
		{name: "a browser's keep-alive plus upgrade", connection: "keep-alive, Upgrade", upgrade: "WebSocket", want: true},
		{name: "a plain GET", connection: "keep-alive", upgrade: "", want: false},
		{name: "an upgrade to another protocol", connection: "Upgrade", upgrade: "h2c", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			r := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
			r.Header.Set("Connection", tt.connection)
			r.Header.Set("Upgrade", tt.upgrade)

			// when
			got := isUpgradeRequest(r)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewHandler_RefusesMissingDependencies(t *testing.T) {
	// given
	hub := NewHub()
	settings := NewMockSettingsReader(t)
	users := NewMockPrincipalValidator(t)
	bans := NewMockBanChecker(t)
	boards := NewMockBoardPresence(t)

	tests := []struct {
		name string
		deps HandlerDeps
		want error
	}{
		{name: "no hub", deps: HandlerDeps{Settings: settings, Users: users, Bans: bans, Boards: boards}, want: errMissingHub},
		{name: "no settings", deps: HandlerDeps{Hub: hub, Users: users, Bans: bans, Boards: boards}, want: errMissingSettings},
		{name: "no user validator", deps: HandlerDeps{Hub: hub, Settings: settings, Bans: bans, Boards: boards}, want: errMissingUsers},
		{name: "no ban checker", deps: HandlerDeps{Hub: hub, Settings: settings, Users: users, Boards: boards}, want: errMissingBans},
		{name: "no board presence", deps: HandlerDeps{Hub: hub, Settings: settings, Users: users, Bans: bans}, want: errMissingBoards},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			_, err := NewHandler(tt.deps)

			// then
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestHandler_AnswersAPlainRequestWith426(t *testing.T) {
	// given
	h := newSocketHarness(t)

	// when
	resp, err := h.server.Client().Get(h.server.URL)

	// then
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUpgradeRequired, resp.StatusCode)
	assert.Equal(t, "websocket", resp.Header.Get("Upgrade"))
	assert.Equal(t, "Upgrade", resp.Header.Get("Connection"))
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

func TestHandler_RefusesAForeignOrMissingOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
	}{
		{name: "a foreign origin", origin: "https://evil.example.com"},
		{name: "no origin", origin: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			h := newSocketHarness(t)
			header := http.Header{}
			if tt.origin != "" {
				header.Set("Origin", tt.origin)
			}

			// when
			_, resp, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: header})

			// then
			require.Error(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

func TestHandler_RefusesAnUpgradeItCannotCheck(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(model.Principal{}, errDatabaseDown)

	// when
	_, resp, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: h.signedInHeader()})

	// then
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "a database blip is never quietly an anonymous socket")
}

func TestHandler_RefusesAnInvalidSessionWith401(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(model.Principal{}, fmt.Errorf("%w: expired", model.ErrInvalidPrincipal))

	// when
	_, resp, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: h.signedInHeader()})

	// then
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandler_RefusesABannedUserWith403(t *testing.T) {
	// given
	h := newSocketHarness(t)
	principal := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(principal, nil)
	h.bans.EXPECT().IsBanned(mock.Anything, principal.ID).Return(true, nil)

	// when
	_, resp, err := h.dialWith(t, &websocket.DialOptions{HTTPHeader: h.signedInHeader()})

	// then
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandler_AnswersAPingWithItsNonceAndTheServerClock(t *testing.T) {
	// given
	h := newSocketHarness(t)
	conn := h.dial(t, h.anonymousHeader())
	before := time.Now().UTC().Add(-time.Second)

	// when
	send(t, conn, TypePing, map[string]any{"nonce": 42, "rtt": 80})
	got := receive(t, conn)

	// then
	require.Equal(t, TypePong, got.Type)
	var pong struct {
		Nonce     *int64 `json:"nonce"`
		ServerNow string `json:"server_now"`
	}
	require.NoError(t, json.Unmarshal(got.Data, &pong))
	require.NotNil(t, pong.Nonce)
	assert.Equal(t, int64(42), *pong.Nonce)
	serverNow, err := time.Parse(serverNowLayout, pong.ServerNow)
	require.NoError(t, err)
	assert.True(t, serverNow.After(before), "server_now is the current UTC time")
}

func TestHandler_AnswersAKeepaliveWithoutANonce(t *testing.T) {
	// given
	h := newSocketHarness(t)
	conn := h.dial(t, h.anonymousHeader())

	// when
	send(t, conn, TypePing, map[string]any{})
	got := receive(t, conn)

	// then
	require.Equal(t, TypePong, got.Type)
	assert.NotContains(t, string(got.Data), "nonce")
	assert.Contains(t, string(got.Data), "server_now")
}

func TestHandler_RefusesABoardJoinFromAnAnonymousSocket(t *testing.T) {
	// given
	h := newSocketHarness(t)
	conn := h.dial(t, h.anonymousHeader())
	ref := boardRef()

	// when
	send(t, conn, TypeBoardJoin, ref)
	got := receive(t, conn)

	// then
	require.Equal(t, TypeBoardJoinRejected, got.Type)
	var rejected BoardJoinRejected
	require.NoError(t, json.Unmarshal(got.Data, &rejected))
	assert.Equal(t, BoardJoinRejected{BoardRef: ref, Reason: RejectAccountRequired}, rejected)
	h.boards.AssertNotCalled(t, "HandleJoin", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_JoinsABoardAndDeliversItsFramesUntilTheSocketCloses(t *testing.T) {
	// given
	h := newSocketHarness(t)
	principal := h.expectUser(t)
	ref := boardRef()
	left := make(chan error, 1)
	h.boards.EXPECT().HandleJoin(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(ctx context.Context, conn Conn, boardID uuid.UUID) (bool, error) {
		h.hub.JoinBoard(boardID, conn.Principal().ID)
		conn.Send(BoardJoinedMessage(BoardJoined{BoardID: boardID, Version: 41, Role: model.BoardRoleParticipant}))

		return true, nil
	}).Once()
	h.boards.EXPECT().HandleLeave(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(ctx context.Context, conn Conn, boardID uuid.UUID) error {
		h.hub.LeaveBoard(boardID, conn.Principal().ID)
		left <- ctx.Err()

		return nil
	}).Once()
	conn := h.dial(t, h.signedInHeader())

	// when
	send(t, conn, TypeBoardJoin, ref)
	joined := receive(t, conn)
	sent := h.hub.SendToBoardClassified(ref.BoardID, func(principalID uuid.UUID) int {
		if principalID == principal.ID {
			return 0
		}

		return -1
	}, [][]byte{[]byte(`{"type":"board_events","data":{"from":42,"to":42}}`)})
	events := receive(t, conn)
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))

	// then
	assert.Equal(t, TypeBoardJoined, joined.Type)
	assert.JSONEq(t, fmt.Sprintf(`{"board_id":%q,"version":41,"participant_id":null,"role":"participant"}`, ref.BoardID), string(joined.Data))
	assert.Equal(t, 1, sent)
	assert.Equal(t, TypeBoardEvents, events.Type)
	select {
	case err := <-left:
		assert.NoError(t, err, "the leave on close runs on a context that is no longer cancelled")
	case <-time.After(frameWait):
		t.Fatal("the board was never left when the socket closed")
	}
}

func TestHandler_NeverDeliversAnotherBoardsFramesToASocketThatDidNotJoin(t *testing.T) {
	// given
	h := newSocketHarness(t)
	principal := h.expectUser(t)
	h.hub.JoinBoard(uuid.New(), principal.ID)
	conn := h.dial(t, h.signedInHeader())
	boardID := uuid.New()

	// when
	h.hub.SendToBoardClassified(boardID, func(uuid.UUID) int {
		return 0
	}, [][]byte{[]byte(`{"type":"board_events"}`)})
	send(t, conn, TypePing, map[string]any{})
	got := receive(t, conn)

	// then
	assert.Equal(t, TypePong, got.Type, "the first frame the outsider sees is its own pong")
}

func TestHandler_LeavesABoardJoinedWhileTheConnectionWasClosing(t *testing.T) {
	// given a join still at the gate when the socket closes, which the reader only notices once the join returns
	h := newSocketHarness(t)
	h.expectUser(t)
	ref := boardRef()
	waiting := make(chan struct{})
	release := make(chan struct{})
	left := make(chan struct{})
	h.boards.EXPECT().HandleJoin(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(context.Context, Conn, uuid.UUID) (bool, error) {
		close(waiting)
		<-release

		return true, nil
	}).Once()
	h.boards.EXPECT().HandleLeave(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(context.Context, Conn, uuid.UUID) error {
		close(left)

		return nil
	}).Once()
	conn := h.dial(t, h.signedInHeader())
	send(t, conn, TypeBoardJoin, ref)
	<-waiting

	// when
	require.NoError(t, conn.CloseNow())
	close(release)

	// then the board the join took is left again, so no viewer is stranded
	select {
	case <-left:
	case <-time.After(frameWait):
		t.Fatal("the board joined while the socket was closing was never left")
	}
}

func TestHandler_LeavesABoardOnRequestAndNotAgainOnClose(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.expectUser(t)
	ref := boardRef()
	left := make(chan struct{}, 2)
	h.boards.EXPECT().HandleJoin(mock.Anything, mock.Anything, ref.BoardID).Return(true, nil).Once()
	h.boards.EXPECT().HandleLeave(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(context.Context, Conn, uuid.UUID) error {
		left <- struct{}{}

		return nil
	}).Once()
	conn := h.dial(t, h.signedInHeader())
	send(t, conn, TypeBoardJoin, ref)

	// when
	send(t, conn, TypeBoardLeave, ref)
	<-left
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
	h.served.Wait()

	// then
	assert.Empty(t, left, "a board already left is not left again on close")
}

func TestHandler_RelaysAViewerStateForAJoinedBoardOnly(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.expectUser(t)
	joinedRef := boardRef()
	strangerRef := boardRef()
	relayed := make(chan ViewerStateChange, 2)
	h.boards.EXPECT().HandleJoin(mock.Anything, mock.Anything, joinedRef.BoardID).Return(true, nil).Once()
	h.boards.EXPECT().HandleViewerState(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ Conn, change ViewerStateChange) error {
		relayed <- change

		return nil
	}).Once()
	h.boards.EXPECT().HandleLeave(mock.Anything, mock.Anything, joinedRef.BoardID).Return(nil).Once()
	conn := h.dial(t, h.signedInHeader())
	send(t, conn, TypeBoardJoin, joinedRef)

	// when
	send(t, conn, TypeBoardViewerState, ViewerStateChange{BoardRef: strangerRef, State: ViewerStateIdle})
	send(t, conn, TypeBoardViewerState, ViewerStateChange{BoardRef: joinedRef, State: "asleep"})
	send(t, conn, TypeBoardViewerState, ViewerStateChange{BoardRef: joinedRef, State: ViewerStateIdle})

	// then
	select {
	case change := <-relayed:
		assert.Equal(t, ViewerStateChange{BoardRef: joinedRef, State: ViewerStateIdle}, change)
	case <-time.After(frameWait):
		t.Fatal("the viewer state was never relayed")
	}
}

func TestHandler_ClosesASocketWhosePongNeverArrives(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.handler.timings = timings{pingInterval: 20 * time.Millisecond, pongTimeout: 50 * time.Millisecond, revalidateEvery: time.Hour}
	killedBefore := testutil.ToFloat64(wsClientsKilled.WithLabelValues(killPongTimeout))
	conn, _, err := h.dialWith(t, &websocket.DialOptions{
		HTTPHeader: h.anonymousHeader(),
		OnPingReceived: func(context.Context, []byte) bool {
			return false
		},
	})
	require.NoError(t, err)

	// when
	status := closeStatusOf(t, conn)
	h.served.Wait()

	// then
	assert.NotEqual(t, websocket.StatusNormalClosure, status)
	assert.Equal(t, killedBefore+1, testutil.ToFloat64(wsClientsKilled.WithLabelValues(killPongTimeout)))
}

func TestHandler_ClosesTheSocketWhenRevalidationFindsTheSessionRevoked(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.handler.timings = timings{pingInterval: 20 * time.Millisecond, pongTimeout: time.Second, revalidateEvery: 0}
	principal := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(principal, nil).Once()
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(model.Principal{}, fmt.Errorf("%w: revoked", model.ErrInvalidPrincipal))
	h.bans.EXPECT().IsBanned(mock.Anything, principal.ID).Return(false, nil).Once()
	killedBefore := testutil.ToFloat64(wsClientsKilled.WithLabelValues(killRevalidation))
	conn := h.dial(t, h.signedInHeader())

	// when
	closeStatusOf(t, conn)
	h.served.Wait()

	// then
	assert.Equal(t, killedBefore+1, testutil.ToFloat64(wsClientsKilled.WithLabelValues(killRevalidation)))
}

func TestHandler_ClosesTheSocketWhenRevalidationFindsTheUserBanned(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.handler.timings = timings{pingInterval: 20 * time.Millisecond, pongTimeout: time.Second, revalidateEvery: 0}
	principal := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	h.users.EXPECT().ValidatePrincipal(mock.Anything, testToken).Return(principal, nil)
	h.bans.EXPECT().IsBanned(mock.Anything, principal.ID).Return(false, nil).Once()
	h.bans.EXPECT().IsBanned(mock.Anything, principal.ID).Return(true, nil)
	killedBefore := testutil.ToFloat64(wsClientsKilled.WithLabelValues(killRevalidation))
	conn := h.dial(t, h.signedInHeader())

	// when
	closeStatusOf(t, conn)
	h.served.Wait()

	// then
	assert.Equal(t, killedBefore+1, testutil.ToFloat64(wsClientsKilled.WithLabelValues(killRevalidation)))
}

func TestHandler_RevalidationDropsARevokedBoardButKeepsTheSocket(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.handler.timings = timings{pingInterval: 20 * time.Millisecond, pongTimeout: time.Second, revalidateEvery: 0}
	h.expectUser(t)
	ref := boardRef()
	revoked := make(chan struct{})
	h.boards.EXPECT().HandleJoin(mock.Anything, mock.Anything, ref.BoardID).Return(true, nil).Once()
	h.boards.EXPECT().Revalidate(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(_ context.Context, conn Conn, boardID uuid.UUID) (bool, error) {
		conn.Send(BoardAccessRevokedMessage(BoardAccessRevoked{BoardID: boardID, Reason: RejectRemoved}))
		close(revoked)

		return false, nil
	}).Once()
	conn := h.dial(t, h.signedInHeader())
	go func() {
		ctx, cancel := context.WithTimeout(t.Context(), frameWait)
		defer cancel()

		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()
	send(t, conn, TypeBoardJoin, ref)

	// when
	select {
	case <-revoked:
	case <-time.After(frameWait):
		t.Fatal("the joined board was never revalidated")
	}
	send(t, conn, TypePing, map[string]any{})

	// then
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
	h.served.Wait()
	h.boards.AssertNotCalled(t, "HandleLeave", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_ClosesWithMessageTooBigOnAnOversizedFrame(t *testing.T) {
	// given
	h := newSocketHarness(t)
	conn := h.dial(t, h.anonymousHeader())
	ctx, cancel := context.WithTimeout(t.Context(), frameWait)
	defer cancel()

	// when
	require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", maxInboundMessageSize+1))))
	status := closeStatusOf(t, conn)

	// then
	assert.Equal(t, websocket.StatusMessageTooBig, status)
}

func TestHandler_ShutdownClosesTheSocketWithServiceRestart(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.expectUser(t)
	conn := h.dial(t, h.signedInHeader())
	send(t, conn, TypePing, map[string]any{})
	receive(t, conn)
	shutdown := make(chan error, 1)

	// when
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), frameWait)
		defer cancel()

		shutdown <- h.hub.Shutdown(ctx)
	}()
	status := closeStatusOf(t, conn)

	// then
	assert.Equal(t, websocket.StatusServiceRestart, status)
	require.NoError(t, <-shutdown)
}

func TestHandler_DisconnectUserClosesTheUsersSockets(t *testing.T) {
	// given
	h := newSocketHarness(t)
	principal := h.expectUser(t)
	conn := h.dial(t, h.signedInHeader())
	send(t, conn, TypePing, map[string]any{})
	receive(t, conn)

	// when
	closed := h.hub.DisconnectUser(principal.ID)
	status := closeStatusOf(t, conn)

	// then
	assert.Equal(t, 1, closed)
	assert.Equal(t, websocket.StatusPolicyViolation, status)
}

func TestHandler_RecoversAPanicWithAnErrorFrameAndAnInternalErrorClose(t *testing.T) {
	// given
	h := newSocketHarness(t)
	h.expectUser(t)
	ref := boardRef()
	h.boards.EXPECT().HandleJoin(mock.Anything, mock.Anything, ref.BoardID).RunAndReturn(func(context.Context, Conn, uuid.UUID) (bool, error) {
		panic("board exploded")
	}).Once()
	conn := h.dial(t, h.signedInHeader())

	// when
	send(t, conn, TypeBoardJoin, ref)
	got := receive(t, conn)
	status := closeStatusOf(t, conn)

	// then
	assert.Equal(t, TypeError, got.Type)
	assert.JSONEq(t, `{"error":"internal error"}`, string(got.Data))
	assert.Equal(t, websocket.StatusInternalError, status)
}
