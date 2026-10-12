package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/model"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

type (
	PrincipalValidator interface {
		Token(r *http.Request) (string, error)
		ValidatePrincipal(ctx context.Context, token string) (model.Principal, error)
	}

	BanChecker interface {
		IsBanned(ctx context.Context, userID uuid.UUID) (bool, error)
	}

	SettingsReader interface {
		Get(ctx context.Context, def *config.SiteSettingDef) (string, error)
	}

	Conn interface {
		ID() uuid.UUID
		Principal() model.Principal
		Send(msg Message) bool
	}

	BoardPresence interface {
		HandleJoin(ctx context.Context, conn Conn, boardID uuid.UUID) (bool, error)
		HandleLeave(ctx context.Context, conn Conn, boardID uuid.UUID) error
		HandleViewerState(ctx context.Context, conn Conn, change ViewerStateChange) error
		Revalidate(ctx context.Context, conn Conn, boardID uuid.UUID) (bool, error)
	}

	HandlerDeps struct {
		Hub      *Hub
		Settings SettingsReader
		Users    PrincipalValidator
		Guests   PrincipalValidator
		Bans     BanChecker
		Boards   BoardPresence
	}

	Handler struct {
		hub        *Hub
		settings   SettingsReader
		validators []PrincipalValidator
		bans       BanChecker
		boards     BoardPresence
		tracer     trace.Tracer
		timings    timings
	}

	timings struct {
		pingInterval    time.Duration
		pongTimeout     time.Duration
		revalidateEvery time.Duration
	}

	credential struct {
		principal model.Principal
		token     string
		validator PrincipalValidator
	}

	refusal struct {
		status  int
		message string
		cause   error
	}

	connection struct {
		handler         *Handler
		cred            credential
		client          *Client
		joined          map[uuid.UUID]struct{}
		limiter         *rate.Limiter
		lastRevalidated time.Time
		lastLimitLog    time.Time
	}
)

const (
	tracerName            = "retrotide/ws"
	maxInboundMessageSize = 8 * 1024
	pingInterval          = 30 * time.Second
	pongTimeout           = 20 * time.Second
	revalidateEvery       = 5 * time.Minute
	inboundPerSecond      = 20
	inboundBurst          = 40
	rateLimitLogEvery     = 5 * time.Second
	maxReportedRTTMS      = 5000
	reportedRTTSlackMS    = 100

	upgradeRequiredMessage = "this endpoint only accepts WebSocket upgrades"
	originRefusedMessage   = "origin not allowed"
	forbiddenMessage       = "forbidden"
	invalidSessionMessage  = "invalid or expired session"
	bannedMessage          = "your account has been banned"
	unavailableMessage     = "service unavailable"
)

var (
	expectedCloses = map[websocket.StatusCode]struct{}{
		websocket.StatusNormalClosure:  {},
		websocket.StatusGoingAway:      {},
		websocket.StatusNoStatusRcvd:   {},
		websocket.StatusServiceRestart: {},
		websocket.StatusTryAgainLater:  {},
	}
)

func NewHandler(deps HandlerDeps) (*Handler, error) {
	if deps.Hub == nil {
		return nil, errMissingHub
	}

	if deps.Settings == nil {
		return nil, errMissingSettings
	}

	if deps.Users == nil {
		return nil, errMissingUsers
	}

	if deps.Bans == nil {
		return nil, errMissingBans
	}

	if deps.Boards == nil {
		return nil, errMissingBoards
	}

	validators := []PrincipalValidator{deps.Users}
	if deps.Guests != nil {
		validators = append(validators, deps.Guests)
	}

	return &Handler{
		hub:        deps.Hub,
		settings:   deps.Settings,
		validators: validators,
		bans:       deps.Bans,
		boards:     deps.Boards,
		tracer:     otel.Tracer(tracerName),
		timings: timings{
			pingInterval:    pingInterval,
			pongTimeout:     pongTimeout,
			revalidateEvery: revalidateEvery,
		},
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state, conn := h.upgrade(w, r)
	if conn == nil {
		return
	}

	h.serve(r.Context(), state, conn)
}

func (h *Handler) upgrade(w http.ResponseWriter, r *http.Request) (*connection, *websocket.Conn) {
	ctx, span := h.tracer.Start(r.Context(), "ws.upgrade", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	if !isUpgradeRequest(r) {
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "Upgrade")
		writeJSON(ctx, w, http.StatusUpgradeRequired, dto.ErrorResponse{Error: upgradeRequiredMessage})
		return nil, nil
	}

	rawBase, base, refused := h.baseURL(ctx)
	if refused != nil {
		refuse(ctx, span, w, refused)
		return nil, nil
	}

	if !OriginAllowed(r.Header.Get("Origin"), rawBase) {
		refuse(ctx, span, w, &refusal{status: http.StatusForbidden, message: originRefusedMessage, cause: errOriginRefused})
		return nil, nil
	}

	cred, refused := h.resolve(ctx, r)
	if refused != nil {
		refuse(ctx, span, w, refused)
		return nil, nil
	}

	span.SetAttributes(attribute.String("ws.principal_kind", kindOf(cred.principal)))

	state := &connection{
		handler:         h,
		cred:            cred,
		joined:          make(map[uuid.UUID]struct{}),
		limiter:         rate.NewLimiter(inboundPerSecond, inboundBurst),
		lastRevalidated: time.Now(),
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:  []string{base.Host},
		CompressionMode: websocket.CompressionDisabled,
		OnPongReceived:  state.onPong,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "accept failed")
		logger.Ctx(ctx).Warn().Err(err).Msg("ws accept failed")
		return nil, nil
	}

	return state, conn
}

func (h *Handler) baseURL(ctx context.Context) (string, *url.URL, *refusal) {
	raw, err := h.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return "", nil, &refusal{status: http.StatusServiceUnavailable, message: unavailableMessage, cause: fmt.Errorf("read base_url: %w", err)}
	}

	base, err := url.Parse(raw)
	if err != nil {
		return "", nil, &refusal{status: http.StatusForbidden, message: forbiddenMessage, cause: fmt.Errorf("parse base_url: %w", err)}
	}

	if base.Host == "" {
		return "", nil, &refusal{status: http.StatusForbidden, message: forbiddenMessage, cause: fmt.Errorf("base_url %q has no host", raw)}
	}

	return raw, base, nil
}

func (h *Handler) resolve(ctx context.Context, r *http.Request) (credential, *refusal) {
	var invalid *refusal

	for _, validator := range h.validators {
		token, err := validator.Token(r)
		if err != nil {
			return credential{}, &refusal{status: http.StatusServiceUnavailable, message: unavailableMessage, cause: fmt.Errorf("read the credential cookie: %w", err)}
		}

		if token == "" {
			continue
		}

		principal, err := validator.ValidatePrincipal(ctx, token)
		if errors.Is(err, model.ErrInvalidPrincipal) {
			invalid = &refusal{status: http.StatusUnauthorized, message: invalidSessionMessage, cause: err}
			continue
		}
		if err != nil {
			return credential{}, &refusal{status: http.StatusServiceUnavailable, message: unavailableMessage, cause: fmt.Errorf("validate the credential: %w", err)}
		}

		if refused := h.checkBan(ctx, principal); refused != nil {
			return credential{}, refused
		}

		return credential{principal: principal, token: token, validator: validator}, nil
	}

	if invalid != nil {
		return credential{}, invalid
	}

	return credential{}, nil
}

func (h *Handler) checkBan(ctx context.Context, principal model.Principal) *refusal {
	if principal.Kind != model.PrincipalUser {
		return nil
	}

	banned, err := h.bans.IsBanned(ctx, principal.ID)
	if err != nil {
		return &refusal{status: http.StatusServiceUnavailable, message: unavailableMessage, cause: fmt.Errorf("check whether the user is banned: %w", err)}
	}

	if banned {
		return &refusal{status: http.StatusForbidden, message: bannedMessage, cause: errBanned}
	}

	return nil
}

func (h *Handler) serve(requestCtx context.Context, state *connection, conn *websocket.Conn) {
	ctx, cancel := context.WithCancelCause(requestCtx)
	defer cancel(errConnectionEnded)

	conn.SetReadLimit(maxInboundMessageSize)
	client := newClient(state.cred.principal, conn, cancel, h.timings)
	state.client = client

	if !h.hub.Register(client) {
		if err := conn.Close(websocket.StatusServiceRestart, closeReasonRestart); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.Ctx(ctx).Debug().Err(err).Msg("ws socket opened during shutdown could not be closed cleanly")
		}

		return
	}

	client.start(ctx)
	defer state.cleanup(ctx)
	defer state.recoverPanic(ctx)

	state.readLoop(ctx)
}

func (s *connection) readLoop(ctx context.Context) {
	for {
		typ, raw, err := s.client.conn.Read(ctx)
		if err != nil {
			logClose(ctx, err)
			return
		}

		if typ != websocket.MessageText {
			logger.Ctx(ctx).Debug().Msg("ws binary frame ignored")
			continue
		}

		s.receive(ctx, raw)
	}
}

func (s *connection) receive(ctx context.Context, raw []byte) {
	kind := kindOf(s.cred.principal)

	tokens := s.limiter.Tokens()
	if !s.limiter.Allow() {
		wsInboundDropped.WithLabelValues(kind).Inc()

		if now := time.Now(); now.Sub(s.lastLimitLog) >= rateLimitLogEvery {
			s.lastLimitLog = now
			logger.Ctx(ctx).Warn().Str("kind", kind).Float64("tokens", tokens).Msg("ws inbound rate limit exceeded")
		}

		return
	}

	var msg incomingMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		recordInbound(unknownMessageType, tokens)
		logger.Ctx(ctx).Debug().Err(err).Msg("ws frame is not a JSON envelope, so it was ignored")
		return
	}

	label := inboundLabel(msg.Type)
	recordInbound(label, tokens)

	frameCtx, span := s.handler.tracer.Start(ctx, "ws."+label,
		trace.WithNewRoot(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("ws.message_type", label),
			attribute.String("ws.principal_kind", kind),
		),
	)
	defer span.End()

	s.dispatch(frameCtx, msg)
}

func (s *connection) dispatch(ctx context.Context, msg incomingMessage) {
	switch msg.Type {
	case TypePing:
		s.pong(msg.Data)
	case TypeBoardJoin:
		s.join(ctx, msg.Data)
	case TypeBoardLeave:
		s.leave(ctx, msg.Data)
	case TypeBoardViewerState:
		s.viewerState(ctx, msg.Data)
	default:
		logger.Ctx(ctx).Debug().Msg("ws frame of an unknown type ignored")
	}
}

func (s *connection) pong(raw json.RawMessage) {
	in := parsePing(raw)
	recordClientRTT(boundReportedRTT(in.RTT, s.client.rttPeak.Load()))

	s.client.Send(Message{Type: TypePong, Data: pongData{Nonce: in.Nonce, ServerNow: time.Now().UTC().Format(serverNowLayout)}})
}

func (s *connection) join(ctx context.Context, raw json.RawMessage) {
	boardID, ok := decodeBoardRef(ctx, raw)
	if !ok {
		return
	}

	if s.client.anonymous() {
		s.client.Send(BoardJoinRejectedMessage(BoardJoinRejected{BoardID: boardID, Reason: RejectAccountRequired}))
		return
	}

	joined, err := s.handler.boards.HandleJoin(ctx, s.client, boardID)
	if err != nil {
		logBoardFailure(ctx, err, "ws board join failed")
		return
	}

	if !joined {
		delete(s.joined, boardID)
		return
	}

	s.joined[boardID] = struct{}{}
}

func (s *connection) leave(ctx context.Context, raw json.RawMessage) {
	boardID, ok := decodeBoardRef(ctx, raw)
	if !ok {
		return
	}

	if _, joined := s.joined[boardID]; !joined {
		return
	}

	if err := s.handler.boards.HandleLeave(ctx, s.client, boardID); err != nil {
		logBoardFailure(ctx, err, "ws board leave failed")
		return
	}

	delete(s.joined, boardID)
}

func (s *connection) viewerState(ctx context.Context, raw json.RawMessage) {
	var change ViewerStateChange
	if err := json.Unmarshal(raw, &change); err != nil {
		logger.Ctx(ctx).Debug().Err(err).Msg("ws viewer state frame could not be read")
		return
	}

	if !change.BoardRef.valid() || !change.State.valid() {
		logger.Ctx(ctx).Debug().Msg("ws viewer state frame names no board or no known state")
		return
	}

	if _, joined := s.joined[change.BoardID]; !joined {
		return
	}

	if err := s.handler.boards.HandleViewerState(ctx, s.client, change); err != nil {
		logBoardFailure(ctx, err, "ws viewer state update failed")
	}
}

func (s *connection) onPong(ctx context.Context, _ []byte) {
	if time.Since(s.lastRevalidated) < s.handler.timings.revalidateEvery {
		return
	}

	s.lastRevalidated = time.Now()

	if err := s.revalidate(ctx); err != nil {
		logger.Ctx(ctx).Info().Err(err).Str("kind", kindOf(s.cred.principal)).Msg("ws access no longer holds, so the socket is closing")
		s.client.kill(killRevalidation, fmt.Errorf("%w: %w", errRevalidationFailed, err))
	}
}

func (s *connection) revalidate(ctx context.Context) error {
	if s.cred.validator == nil {
		return nil
	}

	principal, err := s.cred.validator.ValidatePrincipal(ctx, s.cred.token)
	if err != nil {
		return fmt.Errorf("revalidate the credential: %w", err)
	}

	if principal != s.cred.principal {
		return errPrincipalChanged
	}

	if refused := s.handler.checkBan(ctx, principal); refused != nil {
		return refused.cause
	}

	for boardID := range s.joined {
		kept, err := s.handler.boards.Revalidate(ctx, s.client, boardID)
		if err != nil {
			return fmt.Errorf("revalidate board %s: %w", boardID, err)
		}

		if !kept {
			delete(s.joined, boardID)
		}
	}

	return nil
}

func (s *connection) cleanup(ctx context.Context) {
	leaveCtx := context.WithoutCancel(ctx)

	for boardID := range s.joined {
		if err := s.handler.boards.HandleLeave(leaveCtx, s.client, boardID); err != nil {
			logger.Ctx(leaveCtx).Warn().Err(err).Str("board_id", boardID.String()).Msg("ws board leave on close failed")
		}
	}

	s.handler.hub.Unregister(s.client)
	s.client.requestClose(closeRequest{code: websocket.StatusNormalClosure})
	<-s.client.done
}

func (s *connection) recoverPanic(ctx context.Context) {
	recovered := recover()
	if recovered == nil {
		return
	}

	logger.Ctx(ctx).Error().Err(fmt.Errorf("panic: %v", recovered)).Bytes("stack", debug.Stack()).Msg("ws handler panic")

	s.client.Send(Message{Type: TypeError, Data: errorData{Error: closeReasonInternal}})
	s.client.requestClose(closeRequest{code: websocket.StatusInternalError, reason: closeReasonInternal, flush: true})
}

func decodeBoardRef(ctx context.Context, raw json.RawMessage) (uuid.UUID, bool) {
	var ref BoardRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		logger.Ctx(ctx).Debug().Err(err).Msg("ws board frame could not be read")
		return uuid.Nil, false
	}

	if !ref.valid() {
		logger.Ctx(ctx).Debug().Msg("ws board frame names no board")
		return uuid.Nil, false
	}

	return ref.BoardID, true
}

func logBoardFailure(ctx context.Context, err error, msg string) {
	if ctx.Err() != nil {
		logger.Ctx(ctx).Debug().Err(err).Msg(msg + " because the connection ended")
		return
	}

	logger.Ctx(ctx).Warn().Err(err).Msg(msg)
}

func logClose(ctx context.Context, err error) {
	if cause := context.Cause(ctx); cause != nil {
		logger.Ctx(ctx).Debug().Err(cause).Msg("ws socket closed by the server")
		return
	}

	status := websocket.CloseStatus(err)
	if _, expected := expectedCloses[status]; expected {
		logger.Ctx(ctx).Debug().Int("status", int(status)).Msg("ws socket closed")
		return
	}

	if status == -1 && (errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)) {
		logger.Ctx(ctx).Debug().Err(err).Msg("ws socket dropped without a close frame")
		return
	}

	logger.Ctx(ctx).Warn().Err(err).Int("status", int(status)).Msg("ws socket closed unexpectedly")
}

func refuse(ctx context.Context, span trace.Span, w http.ResponseWriter, refused *refusal) {
	span.SetAttributes(attribute.Int("http.response.status_code", refused.status))

	if refused.status >= http.StatusInternalServerError {
		span.RecordError(refused.cause)
		span.SetStatus(codes.Error, refused.message)
		logger.Ctx(ctx).Error().Err(refused.cause).Int("status", refused.status).Msg("ws upgrade could not be checked")
	} else {
		logger.Ctx(ctx).Info().Err(refused.cause).Int("status", refused.status).Msg("ws upgrade refused")
	}

	writeJSON(ctx, w, refused.status, dto.ErrorResponse{Error: refused.message})
}

func writeJSON(ctx context.Context, w http.ResponseWriter, status int, body dto.ErrorResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.Ctx(ctx).Debug().Err(err).Msg("ws refusal body could not be written")
	}
}

func OriginAllowed(origin, allowed string) bool {
	if origin == "" || allowed == "" {
		return false
	}

	return origin == strings.TrimSuffix(allowed, "/")
}

func isUpgradeRequest(r *http.Request) bool {
	return headerHasToken(r.Header, "Connection", "upgrade") && headerHasToken(r.Header, "Upgrade", "websocket")
}

func headerHasToken(header http.Header, name string, want string) bool {
	for _, value := range header.Values(name) {
		for token := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}

	return false
}

func parsePing(raw json.RawMessage) pingData {
	var in pingData
	if len(raw) == 0 {
		return in
	}

	if err := json.Unmarshal(raw, &in); err != nil {
		return pingData{}
	}

	if in.RTT < 0 || in.RTT > maxReportedRTTMS {
		in.RTT = 0
	}

	return in
}

func boundReportedRTT(reported int, measured int64) int {
	if measured <= 0 {
		return reported
	}

	ceiling := int(measured)*2 + reportedRTTSlackMS
	if reported > ceiling {
		return ceiling
	}

	return reported
}
