package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	BoardColumnDAO interface {
		Create(ctx context.Context, s spec.NewBoardColumn, tx ...*sql.Tx) (*model.BoardColumn, error)
		GetByID(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (*model.BoardColumn, error)
		List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.BoardColumn, error)
		Update(ctx context.Context, s spec.BoardColumnUpdate, tx ...*sql.Tx) (*model.BoardColumn, error)
		Move(ctx context.Context, s spec.BoardColumnMove, tx ...*sql.Tx) (*model.BoardColumn, error)
		Delete(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (*model.BoardColumn, error)
		SetMaxVotes(ctx context.Context, s spec.BoardColumnMaxVotes, tx ...*sql.Tx) (*model.BoardColumn, error)
	}

	boardColumnDAO struct {
		db *sql.DB
	}
)

func toBoardColumn(row sqlcgen.BoardColumn) *model.BoardColumn {
	return &model.BoardColumn{
		ID:          row.ID,
		BoardID:     row.BoardID,
		Title:       row.Title,
		Description: row.Description,
		Colour:      model.Colour(row.Colour),
		Position:    row.Position,
		CreatedAt:   row.CreatedAt,
		MaxVotes:    nullableInt16(row.MaxVotes),
	}
}

func nullableInt16(value sql.NullInt16) *int {
	if !value.Valid {
		return nil
	}

	return new(int(value.Int16))
}

func (r *boardColumnDAO) Create(ctx context.Context, s spec.NewBoardColumn, tx ...*sql.Tx) (*model.BoardColumn, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateBoardColumn(ctx, sqlcgen.CreateBoardColumnParams{
		ID:          s.ID,
		BoardID:     s.BoardID,
		Title:       s.Title,
		Description: s.Description,
		Colour:      string(s.Colour),
		Position:    s.Position,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create column %s on board %s: %w: %w", s.ID, s.BoardID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create column %s on board %s: %w: %w", s.ID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create column %s on board %s: %w", s.ID, s.BoardID, err)
	}

	return toBoardColumn(row), nil
}

func (r *boardColumnDAO) GetByID(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (*model.BoardColumn, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoardColumn(ctx, sqlcgen.GetBoardColumnParams{BoardID: s.BoardID, ID: s.ColumnID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get column %s of board %s: %w", s.ColumnID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return toBoardColumn(row), nil
}

func (r *boardColumnDAO) List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.BoardColumn, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListBoardColumns(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list columns of board %s: %w", boardID, err)
	}

	columns := make([]model.BoardColumn, 0, len(rows))
	for _, row := range rows {
		columns = append(columns, *toBoardColumn(row))
	}

	return columns, nil
}

func (r *boardColumnDAO) Update(ctx context.Context, s spec.BoardColumnUpdate, tx ...*sql.Tx) (*model.BoardColumn, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UpdateBoardColumn(ctx, sqlcgen.UpdateBoardColumnParams{
		BoardID:     s.BoardID,
		ID:          s.ColumnID,
		Title:       s.Title,
		Description: s.Description,
		Colour:      string(s.Colour),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("update column %s of board %s: %w", s.ColumnID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("update column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return toBoardColumn(row), nil
}

func (r *boardColumnDAO) Move(ctx context.Context, s spec.BoardColumnMove, tx ...*sql.Tx) (*model.BoardColumn, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MoveBoardColumn(ctx, sqlcgen.MoveBoardColumnParams{BoardID: s.BoardID, ID: s.ColumnID, Position: s.Position})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("move column %s of board %s: %w", s.ColumnID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("move column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return toBoardColumn(row), nil
}

func (r *boardColumnDAO) Delete(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (*model.BoardColumn, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteBoardColumn(ctx, sqlcgen.DeleteBoardColumnParams{BoardID: s.BoardID, ID: s.ColumnID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("delete column %s of board %s: %w", s.ColumnID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("delete column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return toBoardColumn(row), nil
}

func (r *boardColumnDAO) SetMaxVotes(ctx context.Context, s spec.BoardColumnMaxVotes, tx ...*sql.Tx) (*model.BoardColumn, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardColumnMaxVotes(ctx, sqlcgen.SetBoardColumnMaxVotesParams{
		MaxVotes: toNullInt16(s.MaxVotes),
		BoardID:  s.BoardID,
		ID:       s.ColumnID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("set the vote limit of column %s of board %s: %w", s.ColumnID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("set the vote limit of column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return toBoardColumn(row), nil
}
