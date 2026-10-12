package repository

import (
	"context"
	"database/sql"
	"errors"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	CardVoteRepository interface {
		dao.CardVoteDAO

		AddVote(ctx context.Context, s spec.CardVote, tx ...*sql.Tx) (*model.Versioned[model.CardVoteResult], error)
		RemoveVote(ctx context.Context, s spec.CardVote, tx ...*sql.Tx) (*model.Versioned[model.CardVoteResult], error)
		ResetVotes(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (*model.Versioned[int64], error)
		ResetColumnVotes(ctx context.Context, s spec.ColumnVotesResetWrite, tx ...*sql.Tx) (*model.Versioned[int64], error)
	}

	CardVoteRepoDeps struct {
		Boards dao.BoardDAO
		Cards  dao.CardDAO
	}

	cardVoteRepository struct {
		dao.CardVoteDAO

		db     *sql.DB
		boards dao.BoardDAO
		cards  dao.CardDAO
	}
)

func NewCardVoteRepo(database *sql.DB, voteDAO dao.CardVoteDAO, deps CardVoteRepoDeps) CardVoteRepository {
	return &cardVoteRepository{CardVoteDAO: voteDAO, db: database, boards: deps.Boards, cards: deps.Cards}
}

func (r *cardVoteRepository) AddVote(ctx context.Context, s spec.CardVote, tx ...*sql.Tx) (*model.Versioned[model.CardVoteResult], error) {
	add := func(tx *sql.Tx) (*model.CardVoteResult, error) {
		if s.ColumnMaxVotes > 0 {
			check := spec.ColumnVoteBudgetCheck{BoardID: s.BoardID, ColumnID: s.ColumnID, ParticipantID: s.ParticipantID, MaxVotes: s.ColumnMaxVotes}
			if err := r.CardVoteDAO.CheckColumnBudget(ctx, check, tx); err != nil {
				return nil, err
			}
		}

		if _, err := r.CardVoteDAO.SpendBudget(ctx, spec.VoteBudgetSpend{BoardID: s.BoardID, ParticipantID: s.ParticipantID, MaxVotes: s.MaxVotes}, tx); err != nil {
			return nil, err
		}

		count, err := r.CardVoteDAO.IncrementVote(ctx, spec.CardVoteIncrement{
			BoardID:         s.BoardID,
			CardID:          s.CardID,
			ParticipantID:   s.ParticipantID,
			MaxVotesPerCard: s.MaxVotesPerCard,
		}, tx)
		if err != nil {
			return nil, err
		}

		return r.settle(ctx, s, count, 1, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, add)
}

func (r *cardVoteRepository) RemoveVote(ctx context.Context, s spec.CardVote, tx ...*sql.Tx) (*model.Versioned[model.CardVoteResult], error) {
	remove := func(tx *sql.Tx) (*model.CardVoteResult, error) {
		key := spec.CardVoteKey{BoardID: s.BoardID, CardID: s.CardID, ParticipantID: s.ParticipantID}

		count, err := r.CardVoteDAO.LowerVote(ctx, key, tx)
		if errors.Is(err, dao.ErrNotFound) {
			count, err = 0, r.CardVoteDAO.DropVote(ctx, key, tx)
		}
		if err != nil {
			return nil, err
		}

		if _, err := r.CardVoteDAO.RefundBudget(ctx, spec.BoardParticipantKey{BoardID: s.BoardID, ParticipantID: s.ParticipantID}, tx); err != nil {
			return nil, err
		}

		return r.settle(ctx, s, count, -1, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, remove)
}

func (r *cardVoteRepository) ResetVotes(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (*model.Versioned[int64], error) {
	reset := func(tx *sql.Tx) (*int64, error) {
		removed, err := r.CardVoteDAO.DeleteAll(ctx, s.BoardID, tx)
		if err != nil {
			return nil, err
		}

		if _, err := r.CardVoteDAO.DeleteBudgets(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		if _, err := r.cards.ResetVoteTotals(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		return &removed, nil
	}

	return writeBoard(ctx, r.db, r.boards, s, tx, reset)
}

func (r *cardVoteRepository) ResetColumnVotes(ctx context.Context, s spec.ColumnVotesResetWrite, tx ...*sql.Tx) (*model.Versioned[int64], error) {
	reset := func(tx *sql.Tx) (*int64, error) {
		key := spec.BoardColumnKey{BoardID: s.BoardID, ColumnID: s.ColumnID}

		if _, err := r.CardVoteDAO.RefundColumn(ctx, key, tx); err != nil {
			return nil, err
		}

		removed, err := r.CardVoteDAO.DeleteColumn(ctx, key, tx)
		if err != nil {
			return nil, err
		}

		if _, err := r.cards.ResetColumnVoteTotals(ctx, key, tx); err != nil {
			return nil, err
		}

		return &removed, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, reset)
}

func (r *cardVoteRepository) settle(ctx context.Context, s spec.CardVote, count, delta int, tx *sql.Tx) (*model.CardVoteResult, error) {
	card, err := r.cards.AddVoteTotal(ctx, spec.CardVoteTotalChange{BoardID: s.BoardID, CardID: s.CardID, Delta: delta}, tx)
	if err != nil {
		return nil, err
	}

	mine, err := r.CardVoteDAO.ListForParticipant(ctx, spec.BoardParticipantKey{BoardID: s.BoardID, ParticipantID: s.ParticipantID}, tx)
	if err != nil {
		return nil, err
	}

	return &model.CardVoteResult{Card: *card, Mine: mine, Count: count}, nil
}
