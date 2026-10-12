package repository

import (
	"context"
	"database/sql"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	CardReactionRepository interface {
		dao.CardReactionDAO

		React(ctx context.Context, s spec.CardReactionWrite, tx ...*sql.Tx) (*model.Versioned[[]model.CardReaction], error)
	}

	cardReactionRepository struct {
		dao.CardReactionDAO

		db     *sql.DB
		boards dao.BoardDAO
	}
)

func NewCardReactionRepo(database *sql.DB, reactionDAO dao.CardReactionDAO, boards dao.BoardDAO) CardReactionRepository {
	return &cardReactionRepository{CardReactionDAO: reactionDAO, db: database, boards: boards}
}

func (r *cardReactionRepository) React(ctx context.Context, s spec.CardReactionWrite, tx ...*sql.Tx) (*model.Versioned[[]model.CardReaction], error) {
	react := func(tx *sql.Tx) (*[]model.CardReaction, error) {
		key := spec.CardReactionKey{OrgID: s.OrgID, BoardID: s.BoardID, CardID: s.CardID, ParticipantID: s.ParticipantID, Reaction: s.Reaction}

		if s.Add {
			if _, err := r.CardReactionDAO.Add(ctx, key, tx); err != nil {
				return nil, err
			}
		} else {
			if _, err := r.CardReactionDAO.Remove(ctx, key, tx); err != nil {
				return nil, err
			}
		}

		reactions, err := r.CardReactionDAO.ListForCard(ctx, spec.CardRef{BoardID: s.BoardID, CardID: s.CardID}, tx)
		if err != nil {
			return nil, err
		}

		return &reactions, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, react)
}
