package images

import (
	"context"
	"fmt"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) SetBackground(ctx context.Context, command BackgroundCommand) (*dto.BoardCommandResponse, error) {
	decorate := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanSetBackground(access); err != nil {
			return nil, err
		}

		if err := s.attachable(ctx, access, command.MediaID, model.MediaKindBackground); err != nil {
			return nil, err
		}

		written, err := s.boards.SetBoardBackground(ctx, spec.BoardBackgroundWrite{BoardWrite: target, MediaID: command.MediaID})
		if err != nil {
			return nil, fmt.Errorf("set the background of board %s: %w", target.BoardID, err)
		}

		changed := access
		changed.Header.Board = written.Value

		s.events.Publish(changed, projection.BoardBackgroundChanged(written.Value, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, decorate)
}
