package management

import (
	"context"
	"fmt"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) Delete(ctx context.Context, command board.Command) (*dto.BoardCommandResponse, error) {
	remove := s.deleter(ctx, uuid.Nil)

	return s.Run(ctx, command, board.VersionReplay, remove)
}

func (s *service) deleter(ctx context.Context, orgID uuid.UUID) board.Write {
	return func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := inOrganisation(access, orgID); err != nil {
			return nil, err
		}

		if err := policy.CanDeleteBoard(access); err != nil {
			return nil, err
		}

		written, err := s.boards.DeleteBoard(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("delete board %s: %w", target.BoardID, err)
		}

		if err := s.settle(ctx, access, written); err != nil {
			return nil, err
		}

		response := projection.VersionResponse(written.Version)

		return s.audited(ctx, &response, boardEntry(access, audit.ActionBoardDeleted))
	}
}

func (s *service) PurgeDeleted(ctx context.Context) (int64, error) {
	cutoff := s.now().Add(-model.BoardRestoreWindow)

	purged, err := s.boards.PurgeDeletedBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge the boards deleted before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return purged, nil
}
