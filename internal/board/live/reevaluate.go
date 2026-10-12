package live

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/ws"

	"github.com/google/uuid"
)

func (s *service) ReevaluateHeld(ctx context.Context, boardID uuid.UUID) error {
	principals := s.seated(boardID)
	if len(principals) == 0 {
		return nil
	}

	header, err := s.boards.GetHeader(ctx, boardID)
	if errors.Is(err, dao.ErrNotFound) {
		for _, principal := range principals {
			s.revoke(boardID, principal.ID, ws.RejectNotParticipant)
		}

		return nil
	}
	if err != nil {
		return fmt.Errorf("read the header of board %s to re-evaluate its viewers: %w", boardID, err)
	}

	for _, principal := range principals {
		access, err := s.resolve(ctx, principal, *header)
		if err == nil && header.Archived() {
			err = fmt.Errorf("%w: board %s", ErrArchived, boardID)
		}

		if err == nil {
			s.reseat(*access)
			continue
		}

		reason, known := rejectReason(err)
		if !known {
			return err
		}

		s.revoke(boardID, principal.ID, reason)
	}

	return nil
}

func (s *service) seated(boardID uuid.UUID) []model.Principal {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[boardID]
	if !ok {
		return nil
	}

	principals := make([]model.Principal, 0, len(live.viewers))
	for _, member := range live.viewers {
		principals = append(principals, member.access.Principal)
	}

	return principals
}

func (s *service) reseat(access model.BoardAccess) {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[access.Header.ID]
	if !ok {
		return
	}

	live.board = access.Header.Board

	member, ok := live.viewers[access.Principal.ID]
	if !ok {
		return
	}

	member.access = access
}

func (s *service) revoke(boardID, principalID uuid.UUID, reason ws.RejectReason) {
	s.mu.Lock()
	var tabs []ws.Conn
	live, ok := s.live[boardID]
	if ok {
		if member, seated := live.viewers[principalID]; seated {
			for _, tab := range member.tabs {
				tabs = append(tabs, tab)
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
