package repository

import (
	"context"
	"database/sql"
	"slices"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	CardRepository interface {
		dao.CardDAO

		AddCard(ctx context.Context, s spec.CardCreate, tx ...*sql.Tx) (*model.Versioned[model.Card], error)
		EditCard(ctx context.Context, s spec.CardBodyWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error)
		RecolourCard(ctx context.Context, s spec.CardColourWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error)
		MoveCard(ctx context.Context, s spec.CardPlacement, tx ...*sql.Tx) (*model.Versioned[model.Card], error)
		MergeCard(ctx context.Context, s spec.CardMergeWrite, tx ...*sql.Tx) (*model.Versioned[model.CardMerge], error)
		DeleteCard(ctx context.Context, s spec.CardRemoval, tx ...*sql.Tx) (*model.Versioned[model.Card], error)
		ClearCards(ctx context.Context, s spec.ColumnRemoval, tx ...*sql.Tx) (*model.Versioned[int64], error)
		PinCard(ctx context.Context, s spec.CardPinWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error)
		SetCardImage(ctx context.Context, s spec.CardImageWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error)
	}

	cardRepository struct {
		dao.CardDAO

		db     *sql.DB
		boards dao.BoardDAO
		votes  dao.CardVoteDAO
	}
)

func NewCardRepo(database *sql.DB, cardDAO dao.CardDAO, boards dao.BoardDAO, votes dao.CardVoteDAO) CardRepository {
	return &cardRepository{CardDAO: cardDAO, db: database, boards: boards, votes: votes}
}

func (r *cardRepository) AddCard(ctx context.Context, s spec.CardCreate, tx ...*sql.Tx) (*model.Versioned[model.Card], error) {
	add := func(tx *sql.Tx) (*model.Card, error) {
		_, err := r.CardDAO.Create(ctx, spec.NewCard{
			ID:                  s.CardID,
			BoardID:             s.BoardID,
			ColumnID:            s.ColumnID,
			AuthorParticipantID: s.AuthorParticipantID,
			Body:                s.Body,
			Colour:              s.Colour,
			Position:            s.Position,
		}, tx)
		if err != nil {
			return nil, err
		}

		for _, userID := range s.Mentions {
			if err := r.CardDAO.AddMention(ctx, spec.CardMentionKey{OrgID: s.OrgID, BoardID: s.BoardID, CardID: s.CardID, UserID: userID}, tx); err != nil {
				return nil, err
			}
		}

		return r.CardDAO.GetByID(ctx, spec.CardRef{BoardID: s.BoardID, CardID: s.CardID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, add)
}

func (r *cardRepository) EditCard(ctx context.Context, s spec.CardBodyWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error) {
	edit := func(tx *sql.Tx) (*model.Card, error) {
		ref := spec.CardRef{BoardID: s.BoardID, CardID: s.CardID}

		current, err := r.CardDAO.GetByID(ctx, ref, tx)
		if err != nil {
			return nil, err
		}

		if _, err := r.CardDAO.Edit(ctx, spec.CardEdit{BoardID: s.BoardID, CardID: s.CardID, Body: s.Body}, tx); err != nil {
			return nil, err
		}

		for _, person := range current.Mentions {
			if slices.Contains(s.Mentions, person.UserID) {
				continue
			}

			if err := r.CardDAO.RemoveMention(ctx, spec.CardMentionKey{OrgID: s.OrgID, BoardID: s.BoardID, CardID: s.CardID, UserID: person.UserID}, tx); err != nil {
				return nil, err
			}
		}

		for _, userID := range s.Mentions {
			if err := r.CardDAO.AddMention(ctx, spec.CardMentionKey{OrgID: s.OrgID, BoardID: s.BoardID, CardID: s.CardID, UserID: userID}, tx); err != nil {
				return nil, err
			}
		}

		return r.CardDAO.GetByID(ctx, ref, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, edit)
}

func (r *cardRepository) RecolourCard(ctx context.Context, s spec.CardColourWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error) {
	recolour := func(tx *sql.Tx) (*model.Card, error) {
		if _, err := r.CardDAO.SetColour(ctx, spec.CardColourChange{BoardID: s.BoardID, CardID: s.CardID, Colour: s.Colour}, tx); err != nil {
			return nil, err
		}

		return r.CardDAO.GetByID(ctx, spec.CardRef{BoardID: s.BoardID, CardID: s.CardID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, recolour)
}

func (r *cardRepository) PinCard(ctx context.Context, s spec.CardPinWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error) {
	pin := func(tx *sql.Tx) (*model.Card, error) {
		if _, err := r.CardDAO.SetPinned(ctx, spec.CardPinChange{BoardID: s.BoardID, CardID: s.CardID, Pinned: s.Pinned}, tx); err != nil {
			return nil, err
		}

		return r.CardDAO.GetByID(ctx, spec.CardRef{BoardID: s.BoardID, CardID: s.CardID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, pin)
}

func (r *cardRepository) SetCardImage(ctx context.Context, s spec.CardImageWrite, tx ...*sql.Tx) (*model.Versioned[model.Card], error) {
	illustrate := func(tx *sql.Tx) (*model.Card, error) {
		if _, err := r.CardDAO.SetImage(ctx, spec.CardImageChange{BoardID: s.BoardID, CardID: s.CardID, MediaID: s.MediaID}, tx); err != nil {
			return nil, err
		}

		return r.CardDAO.GetByID(ctx, spec.CardRef{BoardID: s.BoardID, CardID: s.CardID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, illustrate)
}

func (r *cardRepository) MoveCard(ctx context.Context, s spec.CardPlacement, tx ...*sql.Tx) (*model.Versioned[model.Card], error) {
	move := func(tx *sql.Tx) (*model.Card, error) {
		for _, change := range s.Rewrite {
			if _, err := r.CardDAO.Move(ctx, spec.CardMove{BoardID: s.BoardID, CardID: change.ID, ColumnID: s.ColumnID, Position: change.Position}, tx); err != nil {
				return nil, err
			}
		}

		moved, err := r.CardDAO.Move(ctx, spec.CardMove{BoardID: s.BoardID, CardID: s.CardID, ColumnID: s.ColumnID, Position: s.Position}, tx)
		if err != nil {
			return nil, err
		}

		restack := spec.CardRestack{BoardID: s.BoardID, FromParentCardID: s.CardID, ToParentCardID: s.CardID, ColumnID: s.ColumnID}
		if _, err := r.CardDAO.Restack(ctx, restack, tx); err != nil {
			return nil, err
		}

		return moved, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, move)
}

func (r *cardRepository) MergeCard(ctx context.Context, s spec.CardMergeWrite, tx ...*sql.Tx) (*model.Versioned[model.CardMerge], error) {
	merge := func(tx *sql.Tx) (*model.CardMerge, error) {
		restacked, err := r.CardDAO.Restack(ctx, spec.CardRestack{BoardID: s.BoardID, FromParentCardID: s.CardID, ToParentCardID: s.ParentCardID, ColumnID: s.ColumnID}, tx)
		if err != nil {
			return nil, err
		}

		stacked, err := r.CardDAO.Stack(ctx, spec.CardStacking{
			BoardID:      s.BoardID,
			CardID:       s.CardID,
			ParentCardID: s.ParentCardID,
			ColumnID:     s.ColumnID,
			Position:     s.Position,
		}, tx)
		if err != nil {
			return nil, err
		}

		return &model.CardMerge{Card: *stacked, Restacked: restacked}, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, merge)
}

func (r *cardRepository) DeleteCard(ctx context.Context, s spec.CardRemoval, tx ...*sql.Tx) (*model.Versioned[model.Card], error) {
	remove := func(tx *sql.Tx) (*model.Card, error) {
		ref := spec.CardRef{BoardID: s.BoardID, CardID: s.CardID}

		if _, err := r.votes.RefundCard(ctx, ref, tx); err != nil {
			return nil, err
		}

		return r.CardDAO.Delete(ctx, ref, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, remove)
}

func (r *cardRepository) ClearCards(ctx context.Context, s spec.ColumnRemoval, tx ...*sql.Tx) (*model.Versioned[int64], error) {
	empty := func(tx *sql.Tx) (*int64, error) {
		key := spec.BoardColumnKey{BoardID: s.BoardID, ColumnID: s.ColumnID}

		if _, err := r.votes.RefundColumn(ctx, key, tx); err != nil {
			return nil, err
		}

		cleared, err := r.CardDAO.ClearColumn(ctx, key, tx)
		if err != nil {
			return nil, err
		}

		return &cleared, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, empty)
}
