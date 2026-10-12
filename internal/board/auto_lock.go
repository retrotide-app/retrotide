package board

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) expireTimer(ctx context.Context, due model.TimerDue) error {
	release, err := s.live.Acquire(ctx, due.BoardID)
	if err != nil {
		return fmt.Errorf("take the gate of board %s to lock it at its timer's end: %w", due.BoardID, err)
	}
	defer release()

	header, err := s.boards.GetHeader(ctx, due.BoardID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read board %s to lock it at its timer's end: %w", due.BoardID, err)
	}

	if timerLockStale(*header, due) {
		return nil
	}

	if s.now().Before(due.EndsAt) {
		s.timers.Schedule(due)

		return nil
	}

	lock := spec.BoardTimerLockWrite{
		BoardWrite: spec.BoardWrite{OrgID: header.OrgID, BoardID: header.ID, Version: header.Version},
		EndsAt:     due.EndsAt,
	}

	written, err := s.boards.LockBoardAtTimerEnd(ctx, lock)
	if errors.Is(err, dao.ErrTimerMoved) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("lock board %s at its timer's end: %w", due.BoardID, err)
	}

	access := model.BoardAccess{Header: *header}
	access.Header.Board = written.Value

	s.events.Publish(access, projection.Resync(written.Version))

	if err := s.live.ReevaluateHeld(ctx, header.ID); err != nil {
		return fmt.Errorf("re-evaluate the viewers of board %s after its timer locked it: %w", header.ID, err)
	}

	return nil
}

func (s *service) RestoreTimers(ctx context.Context) error {
	dues, err := s.boards.ListAutoLockTimers(ctx)
	if err != nil {
		return fmt.Errorf("list the board timers that lock their boards: %w", err)
	}

	for _, due := range dues {
		s.timers.Schedule(due)
	}

	return nil
}

func (s *service) StopTimers(ctx context.Context) error {
	if err := s.timers.Stop(ctx); err != nil {
		return fmt.Errorf("stop the board timers: %w", err)
	}

	return nil
}

func (s *service) scheduleAutoLock(board model.Board) {
	due, ok := board.TimerDue()
	if !ok || !due.EndsAt.After(s.now()) {
		s.timers.Cancel(board.ID)

		return
	}

	s.timers.Schedule(due)
}

func timerLockStale(header model.BoardHeader, due model.TimerDue) bool {
	if header.Gone() || header.Closed() || header.Archived() || header.Locked() || !header.TimerAutoLock {
		return true
	}

	return header.TimerEndsAt == nil || !header.TimerEndsAt.Equal(due.EndsAt)
}
