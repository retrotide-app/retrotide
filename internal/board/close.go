package board

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/audit"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

const (
	idleBatch = 100

	closeReasonDetail = "reason"
	closeReasonIdle   = "idle"
)

func (s *service) Close(ctx context.Context, command Command) (*dto.BoardCommandResponse, error) {
	end := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		return s.end(ctx, access, target)
	}

	return s.Run(ctx, command, VersionReplay, end)
}

func (s *service) end(ctx context.Context, access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
	written, err := s.closeBoard(ctx, access, target)
	if err != nil {
		return nil, err
	}

	response := projection.VersionResponse(written.Version)

	entry := audit.NewEntry{
		OrgID:      access.Header.OrgID,
		ActorID:    access.Principal.ID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionBoardClosed,
		TargetType: audit.TargetBoard,
		TargetID:   access.Header.ID.String(),
		SubjectID:  access.Principal.ID,
	}
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return &response, fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return &response, nil
}

func (s *service) EndIdle(ctx context.Context) (int64, error) {
	idle, err := s.boards.ListIdle(ctx, spec.IdleBoardsQuery{IdleSince: s.now().Add(-model.RetroIdleLimit), Limit: idleBatch})
	if err != nil {
		return 0, fmt.Errorf("list the retros idle for %s: %w", model.RetroIdleLimit, err)
	}

	var ended int64
	var failures []error

	for _, ref := range idle {
		if ctx.Err() != nil {
			failures = append(failures, fmt.Errorf("stopped after ending %d idle retros: %w", ended, context.Cause(ctx)))
			break
		}

		closed, err := s.endIdle(ctx, ref)
		if closed {
			ended++
		}

		if err != nil {
			failures = append(failures, err)
		}
	}

	return ended, errors.Join(failures...)
}

func (s *service) endIdle(ctx context.Context, ref model.BoardRef) (bool, error) {
	release, err := s.live.Acquire(ctx, ref.ID)
	if err != nil {
		return false, fmt.Errorf("take the gate of idle board %s: %w", ref.ID, err)
	}
	defer release()

	work := context.WithoutCancel(ctx)

	header, err := s.boards.GetHeader(work, ref.ID)
	if errors.Is(err, dao.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the header of idle board %s: %w", ref.ID, err)
	}

	if header.Gone() || header.Closed() || !header.UpdatedAt.Before(s.now().Add(-model.RetroIdleLimit)) {
		return false, nil
	}

	access := model.BoardAccess{Header: *header}
	target := spec.BoardWrite{OrgID: header.OrgID, BoardID: header.ID, Version: header.Version}

	if _, err := s.closeBoard(work, access, target); err != nil {
		return false, err
	}

	entry := audit.NewEntry{
		OrgID:      header.OrgID,
		ActorKind:  audit.ActorSystem,
		Action:     audit.ActionBoardClosed,
		TargetType: audit.TargetBoard,
		TargetID:   header.ID.String(),
		Details:    map[string]string{closeReasonDetail: closeReasonIdle},
	}
	if err := s.auditLog.Create(work, entry); err != nil {
		return true, fmt.Errorf("%w: board %s: %w", ErrNotAudited, header.ID, err)
	}

	return true, nil
}

func (s *service) closeBoard(ctx context.Context, access model.BoardAccess, target spec.BoardWrite) (*model.Versioned[model.Board], error) {
	positions, err := s.shuffledPositions(ctx, access)
	if err != nil {
		return nil, err
	}

	written, err := s.boards.CloseBoard(ctx, spec.BoardCloseWrite{BoardWrite: target, Positions: positions, Sever: access.Header.Anonymous()})
	if err != nil {
		return nil, fmt.Errorf("close board %s: %w", target.BoardID, err)
	}

	closed := access
	closed.Header.Board = written.Value

	s.events.Publish(closed, projection.Resync(written.Version))

	if err := s.live.ReevaluateHeld(ctx, target.BoardID); err != nil {
		return nil, fmt.Errorf("re-evaluate the viewers of board %s after it closed: %w", target.BoardID, err)
	}

	return written, nil
}
