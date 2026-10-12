package live

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/ws"

	"github.com/google/uuid"
)

type (
	rejection struct {
		cause  error
		reason ws.RejectReason
	}
)

var (
	rejections = []rejection{
		{cause: ErrRemoved, reason: ws.RejectRemoved},
		{cause: ErrLinkDisabled, reason: ws.RejectLinkDisabled},
		{cause: ErrPasswordRequired, reason: ws.RejectPasswordRequired},
		{cause: ErrAccountRequired, reason: ws.RejectAccountRequired},
		{cause: ErrGuestsNotAllowed, reason: ws.RejectAccountRequired},
		{cause: ErrArchived, reason: ws.RejectArchived},
		{cause: ErrNoAccess, reason: ws.RejectNotParticipant},
	}
)

func (s *service) HandleJoin(ctx context.Context, conn ws.Conn, boardID uuid.UUID) (bool, error) {
	principal := conn.Principal()

	release, err := s.Acquire(ctx, boardID)
	if err != nil {
		return false, err
	}
	defer release()

	access, err := s.socketAccess(ctx, principal, boardID)
	if err != nil {
		return s.refuse(conn, boardID, err)
	}

	name, err := s.viewerName(ctx, *access)
	if err != nil {
		return false, err
	}

	tally, err := s.joinTally(ctx, *access)
	if err != nil {
		return false, err
	}

	s.mu.Lock()
	live := s.enter(*access, tally)
	member, returning := live.viewers[principal.ID]
	if !returning {
		member = &viewer{tabs: make(map[uuid.UUID]ws.Conn), state: ws.ViewerStateActive, lastTo: access.Header.Version}
		live.viewers[principal.ID] = member
	}

	member.access = *access
	member.displayName = name
	member.tabs[conn.ID()] = conn
	if !returning && live.tally != nil {
		member.counts = live.tally.For(live.board, projection.ViewerFor(*access))
	}

	s.hub.JoinBoard(boardID, principal.ID)
	s.schedulePresence(live)
	s.mu.Unlock()

	conn.Send(ws.BoardJoinedMessage(ws.BoardJoined{
		BoardID:       boardID,
		Version:       access.Header.Version,
		ParticipantID: access.ParticipantID,
		Role:          access.Role,
	}))

	return true, nil
}

func (s *service) HandleLeave(_ context.Context, conn ws.Conn, boardID uuid.UUID) error {
	principalID := conn.Principal().ID

	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[boardID]
	if !ok {
		return nil
	}

	member, ok := live.viewers[principalID]
	if !ok {
		return nil
	}

	delete(member.tabs, conn.ID())
	if len(member.tabs) > 0 {
		return nil
	}

	s.dropViewer(live, principalID)

	return nil
}

func (s *service) HandleViewerState(_ context.Context, conn ws.Conn, change ws.ViewerStateChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[change.BoardID]
	if !ok {
		return nil
	}

	member, ok := live.viewers[conn.Principal().ID]
	if !ok || member.state == change.State {
		return nil
	}

	member.state = change.State

	if live.board.Anonymous() && live.board.CardsHidden {
		return nil
	}

	s.schedulePresence(live)

	return nil
}

func (s *service) Revalidate(ctx context.Context, conn ws.Conn, boardID uuid.UUID) (bool, error) {
	if _, err := s.socketAccess(ctx, conn.Principal(), boardID); err != nil {
		reason, known := rejectReason(err)
		if !known {
			return false, err
		}

		s.evict(conn, boardID, reason)
		conn.Send(ws.BoardAccessRevokedMessage(ws.BoardAccessRevoked{BoardID: boardID, Reason: reason}))

		return false, nil
	}

	return true, nil
}

func (s *service) socketAccess(ctx context.Context, principal model.Principal, boardID uuid.UUID) (*model.BoardAccess, error) {
	access, err := s.Resolve(ctx, spec.BoardAccessQuery{Principal: principal, BoardID: boardID})
	if err != nil {
		return nil, err
	}

	if access.Header.Archived() {
		return nil, fmt.Errorf("%w: board %s", ErrArchived, boardID)
	}

	return access, nil
}

func (s *service) refuse(conn ws.Conn, boardID uuid.UUID, cause error) (bool, error) {
	reason, known := rejectReason(cause)
	if !known {
		return false, cause
	}

	s.evict(conn, boardID, reason)
	conn.Send(ws.BoardJoinRejectedMessage(ws.BoardJoinRejected{BoardID: boardID, Reason: reason}))

	return false, nil
}

func (s *service) evict(conn ws.Conn, boardID uuid.UUID, reason ws.RejectReason) {
	principalID := conn.Principal().ID

	s.mu.Lock()
	var tabs []ws.Conn
	live, ok := s.live[boardID]
	if ok {
		if member, seated := live.viewers[principalID]; seated {
			for id, tab := range member.tabs {
				if id != conn.ID() {
					tabs = append(tabs, tab)
				}
			}

			s.dropViewer(live, principalID)
		}
	}

	s.hub.LeaveBoard(boardID, principalID)
	s.mu.Unlock()

	revoked := ws.BoardAccessRevokedMessage(ws.BoardAccessRevoked{BoardID: boardID, Reason: reason})
	for _, tab := range tabs {
		tab.Send(revoked)
	}
}

func (s *service) dropViewer(live *liveBoard, principalID uuid.UUID) {
	delete(live.viewers, principalID)
	s.hub.LeaveBoard(live.id, principalID)

	if len(live.viewers) == 0 {
		delete(s.live, live.id)
		return
	}

	s.schedulePresence(live)
}

func (s *service) enter(access model.BoardAccess, tally projection.HiddenTally) *liveBoard {
	live, ok := s.live[access.Header.ID]
	if !ok {
		live = &liveBoard{id: access.Header.ID, viewers: make(map[uuid.UUID]*viewer), sending: make(chan struct{}, 1)}
		s.live[access.Header.ID] = live
	}

	live.board = access.Header.Board
	if live.tally == nil && access.Header.CardsHidden {
		live.tally = tally
	}

	return live
}

func (s *service) viewerName(ctx context.Context, access model.BoardAccess) (string, error) {
	if access.ParticipantID == nil {
		return "", nil
	}

	profiles, err := s.boards.ListParticipants(ctx, access.Header.ID)
	if err != nil {
		return "", fmt.Errorf("list the participants of board %s: %w", access.Header.ID, err)
	}

	for _, profile := range profiles {
		if profile.ID == *access.ParticipantID {
			return projection.ParticipantName(access.Header.Board, profile), nil
		}
	}

	return "", nil
}

func (s *service) joinTally(ctx context.Context, access model.BoardAccess) (projection.HiddenTally, error) {
	if !access.Header.CardsHidden {
		return nil, nil
	}

	s.mu.Lock()
	live, ok := s.live[access.Header.ID]
	known := ok && live.tally != nil
	s.mu.Unlock()

	if known {
		return nil, nil
	}

	cards, err := s.cards.List(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("tally the hidden cards of board %s: %w", access.Header.ID, err)
	}

	return projection.Tally(access.Header.Board, cards), nil
}

func rejectReason(err error) (ws.RejectReason, bool) {
	for _, candidate := range rejections {
		if errors.Is(err, candidate.cause) {
			return candidate.reason, true
		}
	}

	return "", false
}
