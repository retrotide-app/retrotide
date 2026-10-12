package repository

import (
	"context"
	"database/sql"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	BoardColumnRepository interface {
		dao.BoardColumnDAO

		AddColumn(ctx context.Context, s spec.ColumnCreate, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error)
		EditColumn(ctx context.Context, s spec.ColumnEdit, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error)
		MoveColumn(ctx context.Context, s spec.ColumnPlacement, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error)
		DeleteColumn(ctx context.Context, s spec.ColumnRemoval, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error)
		SetColumnVoteLimit(ctx context.Context, s spec.ColumnVoteLimitWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error)
	}

	boardColumnRepository struct {
		dao.BoardColumnDAO

		db     *sql.DB
		boards dao.BoardDAO
		votes  dao.CardVoteDAO
	}
)

func NewBoardColumnRepo(database *sql.DB, columnDAO dao.BoardColumnDAO, boards dao.BoardDAO, votes dao.CardVoteDAO) BoardColumnRepository {
	return &boardColumnRepository{BoardColumnDAO: columnDAO, db: database, boards: boards, votes: votes}
}

func (r *boardColumnRepository) AddColumn(ctx context.Context, s spec.ColumnCreate, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error) {
	add := func(tx *sql.Tx) (*model.BoardColumn, error) {
		return r.BoardColumnDAO.Create(ctx, spec.NewBoardColumn{BoardID: s.BoardID, BoardColumnSeed: s.Column}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, add)
}

func (r *boardColumnRepository) EditColumn(ctx context.Context, s spec.ColumnEdit, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error) {
	edit := func(tx *sql.Tx) (*model.BoardColumn, error) {
		return r.BoardColumnDAO.Update(ctx, spec.BoardColumnUpdate{
			BoardID:     s.BoardID,
			ColumnID:    s.ColumnID,
			Title:       s.Title,
			Description: s.Description,
			Colour:      s.Colour,
		}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, edit)
}

func (r *boardColumnRepository) MoveColumn(ctx context.Context, s spec.ColumnPlacement, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error) {
	move := func(tx *sql.Tx) (*model.BoardColumn, error) {
		for _, change := range s.Rewrite {
			if _, err := r.BoardColumnDAO.Move(ctx, spec.BoardColumnMove{BoardID: s.BoardID, ColumnID: change.ID, Position: change.Position}, tx); err != nil {
				return nil, err
			}
		}

		return r.BoardColumnDAO.Move(ctx, spec.BoardColumnMove{BoardID: s.BoardID, ColumnID: s.ColumnID, Position: s.Position}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, move)
}

func (r *boardColumnRepository) DeleteColumn(ctx context.Context, s spec.ColumnRemoval, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error) {
	remove := func(tx *sql.Tx) (*model.BoardColumn, error) {
		key := spec.BoardColumnKey{BoardID: s.BoardID, ColumnID: s.ColumnID}

		if _, err := r.votes.RefundColumn(ctx, key, tx); err != nil {
			return nil, err
		}

		return r.BoardColumnDAO.Delete(ctx, key, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, remove)
}

func (r *boardColumnRepository) SetColumnVoteLimit(ctx context.Context, s spec.ColumnVoteLimitWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardColumn], error) {
	limit := func(tx *sql.Tx) (*model.BoardColumn, error) {
		return r.BoardColumnDAO.SetMaxVotes(ctx, spec.BoardColumnMaxVotes{BoardID: s.BoardID, ColumnID: s.ColumnID, MaxVotes: s.MaxVotes}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, limit)
}
