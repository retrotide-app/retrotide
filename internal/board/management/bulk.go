package management

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"retrotide/internal/board"

	"github.com/google/uuid"
)

var (
	bulkActions = []string{BulkArchive, BulkUnarchive, BulkDelete}
)

func CheckBulkBoards(boardIDs []uuid.UUID) error {
	if len(boardIDs) == 0 {
		return ErrBoardsRequired
	}

	if len(boardIDs) > MaxBulkBoards {
		return fmt.Errorf("%w: got %d", ErrTooManyBoards, len(boardIDs))
	}

	return nil
}

func (s *service) Bulk(ctx context.Context, command BulkCommand) ([]BulkOutcome, error) {
	if !slices.Contains(bulkActions, command.Action) {
		return nil, fmt.Errorf("%w: got %q", ErrInvalidBulkAction, command.Action)
	}

	if err := CheckBulkBoards(command.BoardIDs); err != nil {
		return nil, err
	}

	boardIDs := slices.Clone(command.BoardIDs)
	slices.SortFunc(boardIDs, func(left, right uuid.UUID) int {
		return bytes.Compare(left[:], right[:])
	})
	boardIDs = slices.Compact(boardIDs)

	outcomes := make([]BulkOutcome, 0, len(boardIDs))
	for _, boardID := range boardIDs {
		write := s.bulkWrite(ctx, command.Action, command.OrgID)
		each := board.Command{Principal: command.Principal, BoardID: boardID, OpID: uuid.New()}

		_, err := s.Run(ctx, each, board.VersionReplay, write)
		outcomes = append(outcomes, BulkOutcome{BoardID: boardID, Err: err})
	}

	return outcomes, nil
}

func (s *service) bulkWrite(ctx context.Context, action string, orgID uuid.UUID) board.Write {
	switch action {
	case BulkArchive:
		return s.archiver(ctx, true, orgID)
	case BulkUnarchive:
		return s.archiver(ctx, false, orgID)
	default:
		return s.deleter(ctx, orgID)
	}
}
