package board

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) SetColumnVoteLimit(ctx context.Context, command ColumnVoteLimitCommand) (*dto.BoardCommandResponse, error) {
	if command.MaxVotes != nil && (*command.MaxVotes < 1 || *command.MaxVotes > model.BoardMaxVotesLimit) {
		return nil, fmt.Errorf("%w: %d", ErrInvalidColumnVoteLimit, *command.MaxVotes)
	}

	limit := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		column, err := s.readColumn(ctx, access, command.ColumnID)
		if err != nil {
			return nil, err
		}

		if sameLimit(column.MaxVotes, command.MaxVotes) {
			response := projection.ColumnResponse(access.Header.Board, *column, access.Header.Version)

			return &response, nil
		}

		written, err := s.columns.SetColumnVoteLimit(ctx, spec.ColumnVoteLimitWrite{BoardWrite: target, ColumnID: command.ColumnID, MaxVotes: command.MaxVotes})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrColumnNotFound, err)
		}
		if err != nil {
			return nil, fmt.Errorf("set the vote limit of column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ColumnUpserted(access.Header.Board, written.Value, written.Version))

		response := projection.ColumnResponse(access.Header.Board, written.Value, written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, limit)
}

func (s *service) ResetColumnVotes(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error) {
	reset := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		if err := s.requireColumn(ctx, access, command.ColumnID); err != nil {
			return nil, err
		}

		if err := policy.CanUseColumn(access, command.ColumnID); err != nil {
			return nil, err
		}

		written, err := s.votes.ResetColumnVotes(ctx, spec.ColumnVotesResetWrite{BoardWrite: target, ColumnID: command.ColumnID})
		if err != nil {
			return nil, fmt.Errorf("reset the votes in column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ColumnVotesReset(command.ColumnID, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, reset)
}

func (s *service) readColumn(ctx context.Context, access model.BoardAccess, columnID uuid.UUID) (*model.BoardColumn, error) {
	if err := columnConcealed(access, columnID); err != nil {
		return nil, err
	}

	column, err := s.columns.GetByID(ctx, spec.BoardColumnKey{BoardID: access.Header.ID, ColumnID: columnID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrColumnNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read column %s of board %s: %w", columnID, access.Header.ID, err)
	}

	return column, nil
}

func sameLimit(current, wanted *int) bool {
	if current == nil || wanted == nil {
		return current == nil && wanted == nil
	}

	return *current == *wanted
}
