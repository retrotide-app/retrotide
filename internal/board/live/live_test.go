package live

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/ws"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	fakeHub struct {
		mu       sync.Mutex
		boards   map[uuid.UUID]map[uuid.UUID]struct{}
		frames   map[uuid.UUID][]string
		lossy    map[uuid.UUID][]string
		direct   map[uuid.UUID][]ws.Message
		payloads []int
	}

	fakeConn struct {
		id        uuid.UUID
		principal model.Principal
		sent      []ws.Message
	}

	liveFixture struct {
		svc       *service
		boards    *repository.MockBoardRepository
		cards     *repository.MockCardRepository
		orgAuthz  *orgauthz.MockService
		hub       *fakeHub
		headers   map[uuid.UUID]*model.BoardHeader
		headerErr error
		rows      map[uuid.UUID]*model.BoardParticipant
		names     map[uuid.UUID]string
		standings map[uuid.UUID]teamStanding
		stored    []model.Card
		boardID   uuid.UUID
	}

	seat struct {
		principal     model.Principal
		participantID uuid.UUID
		conn          *fakeConn
	}

	frame struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	eventsFrame struct {
		BoardID uuid.UUID        `json:"board_id"`
		From    int64            `json:"from"`
		To      int64            `json:"to"`
		Events  []map[string]any `json:"events"`
	}
)

const (
	secretBody = "SECRET-TEXT-9f2c"
)

var (
	liveColumnID = uuid.MustParse("a7b1c2d3-e4f5-4a6b-9c7d-8e9fa0b1c2d3")
)

func newFakeHub() *fakeHub {
	return &fakeHub{
		boards: make(map[uuid.UUID]map[uuid.UUID]struct{}),
		frames: make(map[uuid.UUID][]string),
		lossy:  make(map[uuid.UUID][]string),
		direct: make(map[uuid.UUID][]ws.Message),
	}
}

func (h *fakeHub) SendToPrincipal(principalID uuid.UUID, msg ws.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.direct[principalID] = append(h.direct[principalID], msg)
}

func (h *fakeHub) JoinBoard(boardID, principalID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.boards[boardID] == nil {
		h.boards[boardID] = make(map[uuid.UUID]struct{})
	}
	h.boards[boardID][principalID] = struct{}{}
}

func (h *fakeHub) LeaveBoard(boardID, principalID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.boards[boardID], principalID)
}

func (h *fakeHub) member(boardID, principalID uuid.UUID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, ok := h.boards[boardID][principalID]

	return ok
}

func (h *fakeHub) SendToBoardClassified(boardID uuid.UUID, classify func(principalID uuid.UUID) int, payloads [][]byte) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.payloads = append(h.payloads, len(payloads))

	sent := 0
	for principalID := range h.boards[boardID] {
		index := classify(principalID)
		if index < 0 || index >= len(payloads) {
			continue
		}

		h.frames[principalID] = append(h.frames[principalID], string(payloads[index]))
		sent++
	}

	return sent
}

func (h *fakeHub) SendToBoardLossy(boardID uuid.UUID, classify func(principalID uuid.UUID) int, payloads [][]byte) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	sent := 0
	for principalID := range h.boards[boardID] {
		index := classify(principalID)
		if index < 0 || index >= len(payloads) {
			continue
		}

		h.lossy[principalID] = append(h.lossy[principalID], string(payloads[index]))
		sent++
	}

	return sent
}

func (h *fakeHub) framesOf(principalID uuid.UUID) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return slices.Clone(h.frames[principalID])
}

func (c *fakeConn) ID() uuid.UUID {
	return c.id
}

func (c *fakeConn) Principal() model.Principal {
	return c.principal
}

func (c *fakeConn) Send(msg ws.Message) bool {
	c.sent = append(c.sent, msg)

	return true
}

func newLiveFixture(t *testing.T, anonymity model.Anonymity, hidden bool) *liveFixture {
	t.Helper()

	f := &liveFixture{
		boards:    repository.NewMockBoardRepository(t),
		cards:     repository.NewMockCardRepository(t),
		orgAuthz:  orgauthz.NewMockService(t),
		hub:       newFakeHub(),
		headers:   make(map[uuid.UUID]*model.BoardHeader),
		rows:      make(map[uuid.UUID]*model.BoardParticipant),
		names:     make(map[uuid.UUID]string),
		standings: make(map[uuid.UUID]teamStanding),
	}

	f.boardID = f.addBoard(anonymity, hidden)

	f.svc = New(Deps{Boards: f.boards, Cards: f.cards, OrgAuthz: f.orgAuthz, Hub: f.hub}).(*service)
	f.svc.timings = timings{flush: time.Hour, presence: time.Hour, tick: time.Hour, tickTimeout: time.Minute}

	f.boards.EXPECT().GetHeader(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, id uuid.UUID, _ ...*sql.Tx) (*model.BoardHeader, error) {
		if f.headerErr != nil {
			return nil, f.headerErr
		}

		header, ok := f.headers[id]
		if !ok {
			return nil, errNoRow
		}

		copied := *header

		return &copied, nil
	}).Maybe()

	f.orgAuthz.EXPECT().CanInTeam(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, check spec.TeamPermissionCheck) (orgauthz.Decision, error) {
		standing := f.standings[check.UserID]
		if check.Permission == orgauthz.PermTeamView {
			return orgauthz.Decision{Member: standing.orgMember, Allowed: standing.onTeam}, nil
		}

		return orgauthz.Decision{Member: standing.orgMember, Allowed: standing.lead}, nil
	}).Maybe()

	f.boards.EXPECT().GetParticipantForUser(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key spec.BoardParticipantUser, _ ...*sql.Tx) (*model.BoardParticipant, error) {
		row, ok := f.rows[key.UserID]
		if !ok || row.BoardID != key.BoardID {
			return nil, errNoRow
		}

		copied := *row

		return &copied, nil
	}).Maybe()

	f.boards.EXPECT().ListParticipants(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, boardID uuid.UUID, _ ...*sql.Tx) ([]model.BoardParticipantProfile, error) {
		var profiles []model.BoardParticipantProfile
		for userID, row := range f.rows {
			if row.BoardID == boardID {
				profiles = append(profiles, model.BoardParticipantProfile{BoardParticipant: *row, AccountName: f.names[userID]})
			}
		}

		return profiles, nil
	}).Maybe()

	f.cards.EXPECT().List(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, boardID uuid.UUID, _ ...*sql.Tx) ([]model.Card, error) {
		return slices.Clone(f.stored), nil
	}).Maybe()

	return f
}

func (f *liveFixture) addBoard(anonymity model.Anonymity, hidden bool) uuid.UUID {
	header := liveHeader()
	header.Anonymity = anonymity
	header.CardsHidden = hidden
	header.Stage = model.StageWrite
	if !hidden {
		header.Stage = model.StageVote
	}
	f.headers[header.ID] = &header

	return header.ID
}

func (f *liveFixture) header() *model.BoardHeader {
	return f.headers[f.boardID]
}

func (f *liveFixture) seat(name string, standing teamStanding) seat {
	principal := user()
	row := storedRow(*f.header(), principal, model.BoardRoleParticipant)

	f.rows[principal.ID] = row
	f.names[principal.ID] = name
	f.standings[principal.ID] = standing

	return seat{principal: principal, participantID: row.ID, conn: &fakeConn{id: uuid.New(), principal: principal}}
}

func (f *liveFixture) join(t *testing.T, seated seat) {
	t.Helper()

	joined, err := f.svc.HandleJoin(t.Context(), seated.conn, f.boardID)
	require.NoError(t, err)
	require.True(t, joined)
}

func (f *liveFixture) access(seated seat) model.BoardAccess {
	return model.BoardAccess{Principal: seated.principal, Header: *f.header(), ParticipantID: new(seated.participantID), Role: model.BoardRoleParticipant}
}

func (f *liveFixture) timer() boardTimer {
	return boardTimer{s: f.svc, boardID: f.boardID}
}

func (f *liveFixture) secretCard(author seat) model.Card {
	return model.Card{
		ID:                  uuid.MustParse("c1e2f3a4-b5c6-4d7e-8f90-a1b2c3d4e5f6"),
		BoardID:             f.boardID,
		ColumnID:            liveColumnID,
		AuthorParticipantID: new(author.participantID),
		Body:                secretBody,
		Position:            "a0V",
		CreatedAt:           time.Date(2026, 10, 4, 9, 30, 0, 0, time.UTC),
	}
}

func (f *liveFixture) boardFrames(t *testing.T, seated seat) []eventsFrame {
	t.Helper()

	var frames []eventsFrame
	for _, raw := range f.hub.framesOf(seated.principal.ID) {
		var envelope frame
		require.NoError(t, json.Unmarshal([]byte(raw), &envelope))
		require.Equal(t, ws.TypeBoardEvents, envelope.Type)

		var decoded eventsFrame
		require.NoError(t, json.Unmarshal(envelope.Data, &decoded))
		frames = append(frames, decoded)
	}

	return frames
}

func (f *liveFixture) viewerOf(seated seat) *viewer {
	f.svc.mu.Lock()
	defer f.svc.mu.Unlock()

	live, ok := f.svc.live[f.boardID]
	if !ok {
		return nil
	}

	return live.viewers[seated.principal.ID]
}

func sentTypes(conn *fakeConn) []string {
	types := make([]string, 0, len(conn.sent))
	for _, msg := range conn.sent {
		types = append(types, msg.Type)
	}

	return types
}

func eventTypes(frame eventsFrame) []string {
	types := make([]string, 0, len(frame.Events))
	for _, event := range frame.Events {
		types = append(types, event["type"].(string))
	}

	return types
}
