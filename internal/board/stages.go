package board

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) ChangeStage(ctx context.Context, command StageCommand) (*dto.BoardCommandResponse, error) {
	if !slices.Contains(model.Stages, command.Stage) {
		return nil, ErrInvalidStage
	}

	advance := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		current := access.Header.Stage
		if command.Stage == current {
			response := projection.VersionResponse(access.Header.Version)

			return &response, nil
		}

		next, forward := current.Next()
		if forward && command.Stage == next && next == model.StageEnd {
			return s.end(ctx, access, target)
		}

		if forward && command.Stage == next {
			return s.enterStage(ctx, access, target, next)
		}

		previous, backward := current.Previous()
		if backward && command.Stage == previous {
			return s.enterStage(ctx, access, target, previous)
		}

		return nil, fmt.Errorf("%w: board %s is in %s, neither straight before nor straight after %s", ErrStageNotNext, target.BoardID, current, command.Stage)
	}

	return s.Run(ctx, command.Command, VersionReplay, advance)
}

func (s *service) enterStage(ctx context.Context, access model.BoardAccess, target spec.BoardWrite, stage model.Stage) (*dto.BoardCommandResponse, error) {
	reveal := access.Header.CardsHidden

	positions, err := s.shuffledPositions(ctx, access)
	if err != nil {
		return nil, err
	}

	written, err := s.boards.AdvanceStage(ctx, spec.BoardStageWrite{
		BoardWrite: target,
		From:       access.Header.Stage,
		To:         stage,
		Positions:  positions,
		Reveal:     reveal,
	})
	if errors.Is(err, dao.ErrStageMoved) {
		return nil, fmt.Errorf("%w: %w", ErrStageNotNext, err)
	}
	if err != nil {
		return nil, fmt.Errorf("move board %s from stage %s to %s: %w", target.BoardID, access.Header.Stage, stage, err)
	}

	moved := access
	moved.Header.Board = written.Value.Header.Board

	switch {
	case reveal:
		s.events.Publish(moved, projection.CardsRevealed(written.Value, written.Version))
		s.events.PublishMeToEveryone(moved, written.Value.Votes, written.Version)
	case access.Header.TotalsHidden() != moved.Header.TotalsHidden():
		s.events.Publish(moved, projection.Resync(written.Version))
	default:
		s.events.Publish(moved, projection.StageChanged(moved.Header.Board, written.Version))
	}

	response := projection.VersionResponse(written.Version)

	return &response, nil
}
