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

	"github.com/google/uuid"
)

func (s *service) AddColumn(ctx context.Context, command AddColumnCommand) (*dto.BoardCommandResponse, error) {
	if err := ClientID(command.ColumnID); err != nil {
		return nil, err
	}

	fields, err := CleanColumn(command.ColumnFields)
	if err != nil {
		return nil, err
	}

	add := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanShapeColumns(access); err != nil {
			return nil, err
		}

		siblings, err := s.columnOrder(ctx, access)
		if err != nil {
			return nil, err
		}

		position, err := appendKey(siblings)
		if err != nil {
			return nil, fmt.Errorf("place column %s on board %s: %w", command.ColumnID, target.BoardID, err)
		}

		written, err := s.columns.AddColumn(ctx, spec.ColumnCreate{
			BoardWrite: target,
			Column: spec.BoardColumnSeed{
				ID:          command.ColumnID,
				Title:       fields.Title,
				Description: fields.Description,
				Colour:      fields.Colour,
				Position:    position,
			},
		})
		if errors.Is(err, dao.ErrConflict) {
			return nil, fmt.Errorf("%w: column %s: %w", ErrIDConflict, command.ColumnID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("add column %s to board %s: %w", command.ColumnID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ColumnUpserted(access.Header.Board, written.Value, written.Version))

		response := projection.ColumnResponse(access.Header.Board, written.Value, written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, createdReplay(command.ColumnID), add)
}

func (s *service) EditColumn(ctx context.Context, command EditColumnCommand) (*dto.BoardCommandResponse, error) {
	fields, err := CleanColumn(command.ColumnFields)
	if err != nil {
		return nil, err
	}

	edit := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanShapeColumns(access); err != nil {
			return nil, err
		}

		if err := columnConcealed(access, command.ColumnID); err != nil {
			return nil, err
		}

		written, err := s.columns.EditColumn(ctx, spec.ColumnEdit{
			BoardWrite:  target,
			ColumnID:    command.ColumnID,
			Title:       fields.Title,
			Description: fields.Description,
			Colour:      fields.Colour,
		})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrColumnNotFound, err)
		}
		if err != nil {
			return nil, fmt.Errorf("edit column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ColumnUpserted(access.Header.Board, written.Value, written.Version))

		response := projection.ColumnResponse(access.Header.Board, written.Value, written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, edit)
}

func (s *service) MoveColumn(ctx context.Context, command MoveColumnCommand) (*dto.BoardCommandResponse, error) {
	move := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanShapeColumns(access); err != nil {
			return nil, err
		}

		if err := columnConcealed(access, command.ColumnID); err != nil {
			return nil, err
		}

		order, err := s.columnOrder(ctx, access)
		if err != nil {
			return nil, err
		}

		siblings := slices.DeleteFunc(slices.Clone(order), func(sibling placed) bool {
			return sibling.id == command.ColumnID
		})
		if len(siblings) == len(order) {
			return nil, fmt.Errorf("%w: column %s is not on board %s", ErrColumnNotFound, command.ColumnID, target.BoardID)
		}

		placing := place(siblings, command.AfterColumnID)

		written, err := s.columns.MoveColumn(ctx, spec.ColumnPlacement{
			BoardWrite: target,
			ColumnID:   command.ColumnID,
			Position:   placing.position,
			Rewrite:    placing.rewrite,
		})
		if err != nil {
			return nil, fmt.Errorf("move column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ColumnMoved(access.Header.Board, written.Value, placing.rewrite, written.Version))

		response := projection.ColumnResponse(access.Header.Board, written.Value, written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, move)
}

func (s *service) DeleteColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error) {
	remove := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageColumns(access); err != nil {
			return nil, err
		}

		written, err := s.columns.DeleteColumn(ctx, spec.ColumnRemoval{BoardWrite: target, ColumnID: command.ColumnID})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrColumnNotFound, err)
		}
		if err != nil {
			return nil, fmt.Errorf("delete column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ColumnDeleted(access.Header.Board, command.ColumnID, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, remove)
}

func (s *service) ClearColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error) {
	empty := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageColumns(access); err != nil {
			return nil, err
		}

		if err := s.requireColumn(ctx, access, command.ColumnID); err != nil {
			return nil, err
		}

		written, err := s.cards.ClearCards(ctx, spec.ColumnRemoval{BoardWrite: target, ColumnID: command.ColumnID})
		if err != nil {
			return nil, fmt.Errorf("clear column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		s.events.Publish(access, projection.ColumnCleared(access.Header.Board, command.ColumnID, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, empty)
}

func (s *service) columnOrder(ctx context.Context, access model.BoardAccess) ([]placed, error) {
	columns, err := s.columns.List(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the columns of board %s: %w", access.Header.ID, err)
	}

	order := make([]placed, 0, len(columns))
	for _, column := range columns {
		order = append(order, placed{id: column.ID, position: column.Position})
	}

	return order, nil
}

func columnConcealed(access model.BoardAccess, columnID uuid.UUID) error {
	if slices.Contains(access.Header.HiddenColumnIDs, columnID) && !slices.Contains(projection.ViewerFor(access).Capabilities, model.BoardCapabilityFacilitate) {
		return fmt.Errorf("%w: column %s of board %s is hidden from the viewer", ErrColumnNotFound, columnID, access.Header.ID)
	}

	return nil
}

func (s *service) requireColumn(ctx context.Context, access model.BoardAccess, columnID uuid.UUID) error {
	if err := columnConcealed(access, columnID); err != nil {
		return err
	}

	_, err := s.columns.GetByID(ctx, spec.BoardColumnKey{BoardID: access.Header.ID, ColumnID: columnID})
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrColumnNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("read column %s of board %s: %w", columnID, access.Header.ID, err)
	}

	return nil
}
