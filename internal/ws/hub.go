package ws

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/pprof"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"retrotide/internal/logger"
	"retrotide/internal/model"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type (
	Client struct {
		id             uuid.UUID
		principal      model.Principal
		conn           *websocket.Conn
		cancel         context.CancelCauseFunc
		timings        timings
		send           chan []byte
		closing        chan struct{}
		closeRequested atomic.Bool
		closeWith      closeRequest
		done           chan struct{}
		registered     atomic.Bool
		killed         atomic.Bool
		rttPeak        atomic.Int64
	}

	closeRequest struct {
		code   websocket.StatusCode
		reason string
		flush  bool
	}

	boardRecipient struct {
		principalID uuid.UUID
		clients     []*Client
	}

	Hub struct {
		mu          sync.RWMutex
		clients     map[uuid.UUID][]*Client
		anon        map[*Client]struct{}
		boards      map[uuid.UUID]map[uuid.UUID]struct{}
		memberships map[uuid.UUID]map[uuid.UUID]struct{}
		live        sync.WaitGroup
		closed      bool
	}
)

const (
	boardSendBufferSize = 256
	sendBufferSize      = 64
	writeTimeout        = 10 * time.Second
	anonymousKind       = "anonymous"

	closeReasonRestart   = "server restart"
	closeReasonSignedOut = "signed out"
	closeReasonDropped   = "connection dropped"
	closeReasonInternal  = "internal error"
)

func newClient(principal model.Principal, conn *websocket.Conn, cancel context.CancelCauseFunc, t timings) *Client {
	size := sendBufferSize
	if principal.ID != uuid.Nil {
		size = boardSendBufferSize
	}

	return &Client{
		id:        uuid.New(),
		principal: principal,
		conn:      conn,
		cancel:    cancel,
		timings:   t,
		send:      make(chan []byte, size),
		closing:   make(chan struct{}),
		done:      make(chan struct{}),
	}
}

func kindOf(principal model.Principal) string {
	if principal.ID == uuid.Nil {
		return anonymousKind
	}

	return string(principal.Kind)
}

func (c *Client) ID() uuid.UUID {
	return c.id
}

func (c *Client) Principal() model.Principal {
	return c.principal
}

func (c *Client) Send(msg Message) bool {
	data, err := Encode(msg)
	if err != nil {
		logger.Log.Error().Err(err).Msg("ws frame could not be encoded")
		return false
	}

	return c.enqueue(data)
}

func (c *Client) anonymous() bool {
	return c.principal.ID == uuid.Nil
}

func (c *Client) start(ctx context.Context) {
	go pprof.Do(ctx, pprof.Labels("conn", "ws", "loop", "write"), c.writeLoop)
	go pprof.Do(ctx, pprof.Labels("conn", "ws", "loop", "ping"), c.pingLoop)
}

func (c *Client) writeLoop(ctx context.Context) {
	defer close(c.done)
	defer c.cancel(errWriterStopped)

	for {
		select {
		case data := <-c.send:
			if err := c.write(ctx, data); err != nil {
				logger.Ctx(ctx).Debug().Err(err).Msg("ws write failed, so the socket is closing")
				return
			}
		case <-c.closing:
			c.finish(ctx)
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) write(ctx context.Context, data []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	if err := c.conn.Write(writeCtx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("write a frame: %w", err)
	}

	return nil
}

func (c *Client) finish(ctx context.Context) {
	req := c.closeWith

	if req.flush {
		if err := c.flush(ctx); err != nil {
			logger.Ctx(ctx).Debug().Err(err).Msg("ws flush before close failed")
			return
		}
	}

	if ctx.Err() != nil {
		return
	}

	if err := c.conn.Close(req.code, req.reason); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.Ctx(ctx).Debug().Err(err).Msg("ws close handshake did not complete")
	}
}

func (c *Client) flush(ctx context.Context) error {
	for {
		select {
		case data := <-c.send:
			if err := c.write(ctx, data); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (c *Client) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(c.timings.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closing:
			return
		case <-ticker.C:
		}

		err := c.ping(ctx)
		if err == nil {
			continue
		}

		if ctx.Err() != nil || c.isClosing() {
			return
		}

		logger.Ctx(ctx).Debug().Err(err).Msg("ws pong never arrived, so the socket is dropped")
		c.kill(killPongTimeout, fmt.Errorf("%w: %w", errPongTimeout, err))

		if err := c.conn.CloseNow(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.Ctx(ctx).Debug().Err(err).Msg("ws socket could not be dropped")
		}

		return
	}
}

func (c *Client) ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, c.timings.pongTimeout)
	defer cancel()

	started := time.Now()
	if err := c.conn.Ping(pingCtx); err != nil {
		return err
	}

	c.recordRTT(time.Since(started))

	return nil
}

func (c *Client) recordRTT(rtt time.Duration) {
	wsPingRTT.Observe(rtt.Seconds())

	sample := rtt.Milliseconds()
	if sample < 0 {
		return
	}

	peak := c.rttPeak.Load()
	if peak <= 0 || sample >= peak {
		c.rttPeak.Store(sample)
		return
	}

	c.rttPeak.Store(peak - (peak-sample)/5)
}

func (c *Client) enqueue(data []byte) bool {
	if c.isClosing() {
		return false
	}

	select {
	case c.send <- data:
		return true
	case <-c.closing:
		return false
	default:
		c.kill(killSlowConsumer, errSlowConsumer)
		return false
	}
}

func (c *Client) enqueueLossy(data []byte) bool {
	if len(c.send) >= cap(c.send)/2 {
		return false
	}

	select {
	case c.send <- data:
		return true
	case <-c.closing:
		return false
	default:
		return false
	}
}

func (c *Client) isClosing() bool {
	select {
	case <-c.closing:
		return true
	default:
		return false
	}
}

func (c *Client) requestClose(req closeRequest) {
	if !c.closeRequested.CompareAndSwap(false, true) {
		return
	}

	c.closeWith = req
	close(c.closing)
}

func (c *Client) kill(reason string, cause error) {
	if !c.killed.CompareAndSwap(false, true) {
		return
	}

	wsClientsKilled.WithLabelValues(reason).Inc()
	c.requestClose(closeRequest{code: websocket.StatusPolicyViolation, reason: closeReasonDropped})
	c.cancel(cause)
}

func NewHub() *Hub {
	return &Hub{
		clients:     make(map[uuid.UUID][]*Client),
		anon:        make(map[*Client]struct{}),
		boards:      make(map[uuid.UUID]map[uuid.UUID]struct{}),
		memberships: make(map[uuid.UUID]map[uuid.UUID]struct{}),
	}
}

func (h *Hub) Register(client *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return false
	}

	if client.anonymous() {
		h.anon[client] = struct{}{}
	} else {
		h.clients[client.principal.ID] = append(h.clients[client.principal.ID], client)
	}

	client.registered.Store(true)
	h.live.Add(1)
	recordConnectionOpened(kindOf(client.principal))

	return true
}

func (h *Hub) Unregister(client *Client) {
	if !client.registered.CompareAndSwap(true, false) {
		return
	}

	h.mu.Lock()
	h.remove(client)
	h.mu.Unlock()

	wsConnections.WithLabelValues(kindOf(client.principal)).Dec()
	h.live.Done()
}

func (h *Hub) remove(client *Client) {
	if client.anonymous() {
		delete(h.anon, client)
		return
	}

	principalID := client.principal.ID
	remaining := slices.DeleteFunc(h.clients[principalID], func(candidate *Client) bool {
		return candidate == client
	})

	if len(remaining) > 0 {
		h.clients[principalID] = remaining
		return
	}

	delete(h.clients, principalID)

	for boardID := range h.memberships[principalID] {
		h.dropMember(boardID, principalID)
	}

	delete(h.memberships, principalID)
}

func (h *Hub) dropMember(boardID, principalID uuid.UUID) {
	members := h.boards[boardID]
	delete(members, principalID)

	if len(members) == 0 {
		delete(h.boards, boardID)
	}
}

func (h *Hub) JoinBoard(boardID, principalID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.boards[boardID] == nil {
		h.boards[boardID] = make(map[uuid.UUID]struct{})
	}
	h.boards[boardID][principalID] = struct{}{}

	if h.memberships[principalID] == nil {
		h.memberships[principalID] = make(map[uuid.UUID]struct{})
	}
	h.memberships[principalID][boardID] = struct{}{}
}

func (h *Hub) LeaveBoard(boardID, principalID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.dropMember(boardID, principalID)

	boards := h.memberships[principalID]
	delete(boards, boardID)

	if len(boards) == 0 {
		delete(h.memberships, principalID)
	}
}

func (h *Hub) IsOnBoard(boardID, principalID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, member := h.boards[boardID][principalID]

	return member
}

func (h *Hub) IsOnline(principalID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients[principalID]) > 0
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}

func (h *Hub) SendToPrincipal(principalID uuid.UUID, msg Message) {
	conns := h.principalClients(principalID)
	h.deliver(conns, msg)
}

func (h *Hub) SendToUser(userID uuid.UUID, msg Message) {
	conns := h.userClients([]uuid.UUID{userID})
	h.deliver(conns, msg)
}

func (h *Hub) SendToUsers(userIDs []uuid.UUID, msg Message) {
	conns := h.userClients(userIDs)
	h.deliver(conns, msg)
}

func (h *Hub) Broadcast(msg Message) {
	h.mu.RLock()
	var conns []*Client
	for _, principalConns := range h.clients {
		for _, client := range principalConns {
			if client.principal.Kind == model.PrincipalUser {
				conns = append(conns, client)
			}
		}
	}
	h.mu.RUnlock()

	h.deliver(conns, msg)
}

func (h *Hub) BroadcastPublic(msg Message) {
	h.mu.RLock()
	conns := make([]*Client, 0, len(h.clients)+len(h.anon))
	for _, principalConns := range h.clients {
		conns = append(conns, principalConns...)
	}
	for client := range h.anon {
		conns = append(conns, client)
	}
	h.mu.RUnlock()

	h.deliver(conns, msg)
}

func (h *Hub) SendToBoardClassified(boardID uuid.UUID, classify func(principalID uuid.UUID) int, payloads [][]byte) int {
	recipients := h.boardRecipients(boardID)

	sent := 0
	var dead []*Client
	for _, recipient := range recipients {
		index := classify(recipient.principalID)
		if !payloadIndexUsable(index, payloads) {
			continue
		}

		for _, client := range recipient.clients {
			if client.enqueue(payloads[index]) {
				sent++
				continue
			}

			if client.killed.Load() {
				dead = append(dead, client)
			}
		}
	}

	h.reap(dead)

	return sent
}

func (h *Hub) SendToBoardLossy(boardID uuid.UUID, classify func(principalID uuid.UUID) int, payloads [][]byte) int {
	recipients := h.boardRecipients(boardID)

	sent := 0
	for _, recipient := range recipients {
		index := classify(recipient.principalID)
		if !payloadIndexUsable(index, payloads) {
			continue
		}

		for _, client := range recipient.clients {
			if client.enqueueLossy(payloads[index]) {
				sent++
				continue
			}

			wsOutboundFramesDropped.Inc()
		}
	}

	return sent
}

func payloadIndexUsable(index int, payloads [][]byte) bool {
	if index < 0 {
		return false
	}

	if index >= len(payloads) {
		logger.Log.Error().Int("index", index).Int("payloads", len(payloads)).Msg("ws board classifier chose a payload that does not exist, so the recipient was skipped")
		return false
	}

	return true
}

func (h *Hub) DisconnectUser(userID uuid.UUID) int {
	conns := h.principalClients(userID)
	for _, client := range conns {
		wsClientsKilled.WithLabelValues(killDisconnected).Inc()
		client.requestClose(closeRequest{code: websocket.StatusPolicyViolation, reason: closeReasonSignedOut})
	}

	return len(conns)
}

func (h *Hub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	h.closed = true
	var conns []*Client
	for _, principalConns := range h.clients {
		conns = append(conns, principalConns...)
	}
	for client := range h.anon {
		conns = append(conns, client)
	}
	h.mu.Unlock()

	for _, client := range conns {
		client.requestClose(closeRequest{code: websocket.StatusServiceRestart, reason: closeReasonRestart, flush: true})
	}

	drained := make(chan struct{})
	go func() {
		h.live.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		return nil
	case <-ctx.Done():
	}

	cause := context.Cause(ctx)
	for _, client := range conns {
		if !client.registered.Load() {
			continue
		}

		wsClientsKilled.WithLabelValues(killShutdownExpiry).Inc()
		client.cancel(fmt.Errorf("%w: %w", errShutdownExpired, cause))
	}

	return fmt.Errorf("close the open sockets: %w", cause)
}

func (h *Hub) principalClients(principalID uuid.UUID) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return slices.Clone(h.clients[principalID])
}

func (h *Hub) userClients(userIDs []uuid.UUID) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var conns []*Client
	for _, userID := range userIDs {
		for _, client := range h.clients[userID] {
			if client.principal.Kind == model.PrincipalUser {
				conns = append(conns, client)
			}
		}
	}

	return conns
}

func (h *Hub) boardRecipients(boardID uuid.UUID) []boardRecipient {
	h.mu.RLock()
	defer h.mu.RUnlock()

	members := h.boards[boardID]
	recipients := make([]boardRecipient, 0, len(members))
	for principalID := range members {
		recipients = append(recipients, boardRecipient{principalID: principalID, clients: slices.Clone(h.clients[principalID])})
	}

	return recipients
}

func (h *Hub) deliver(conns []*Client, msg Message) {
	if len(conns) == 0 {
		return
	}

	data, err := Encode(msg)
	if err != nil {
		logger.Log.Error().Err(err).Msg("ws frame could not be encoded")
		return
	}

	var dead []*Client
	for _, client := range conns {
		if !client.enqueue(data) && client.killed.Load() {
			dead = append(dead, client)
		}
	}

	h.reap(dead)
}

func (h *Hub) reap(dead []*Client) {
	for _, client := range dead {
		h.Unregister(client)
	}
}
