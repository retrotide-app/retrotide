package dao

import (
	"context"
	"database/sql"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	CardReactionDAO interface {
		Add(ctx context.Context, s spec.CardReactionKey, tx ...*sql.Tx) (bool, error)
		Remove(ctx context.Context, s spec.CardReactionKey, tx ...*sql.Tx) (bool, error)
		ListForCard(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) ([]model.CardReaction, error)
		List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.CardReaction, error)
		HandOver(ctx context.Context, s spec.ParticipantHandover, tx ...*sql.Tx) (int64, error)
		Sever(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
	}

	cardReactionDAO struct {
		db *sql.DB
	}
)

func toCardReaction(boardID, cardID uuid.UUID, participantID *uuid.UUID, reaction string) model.CardReaction {
	return model.CardReaction{BoardID: boardID, CardID: cardID, ParticipantID: participantID, Reaction: model.Reaction(reaction)}
}

func (r *cardReactionDAO) Add(ctx context.Context, s spec.CardReactionKey, tx ...*sql.Tx) (bool, error) {
	action := fmt.Sprintf("react %s to card %s of board %s as participant %s", s.Reaction, s.CardID, s.BoardID, s.ParticipantID)

	added, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddCardReaction(ctx, sqlcgen.AddCardReactionParams{
		OrgID:         s.OrgID,
		BoardID:       s.BoardID,
		CardID:        s.CardID,
		ParticipantID: s.ParticipantID,
		Reaction:      string(s.Reaction),
	})
	if isForeignKeyViolation(err) {
		return false, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}
	if err != nil {
		return false, fmt.Errorf("%s: %w", action, err)
	}

	return added > 0, nil
}

func (r *cardReactionDAO) Remove(ctx context.Context, s spec.CardReactionKey, tx ...*sql.Tx) (bool, error) {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveCardReaction(ctx, sqlcgen.RemoveCardReactionParams{
		OrgID:         s.OrgID,
		BoardID:       s.BoardID,
		CardID:        s.CardID,
		ParticipantID: s.ParticipantID,
		Reaction:      string(s.Reaction),
	})
	if err != nil {
		return false, fmt.Errorf("take back the %s reaction of participant %s on card %s of board %s: %w", s.Reaction, s.ParticipantID, s.CardID, s.BoardID, err)
	}

	return removed > 0, nil
}

func (r *cardReactionDAO) ListForCard(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) ([]model.CardReaction, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardReactionsForCard(ctx, sqlcgen.ListCardReactionsForCardParams{BoardID: s.BoardID, CardID: s.CardID})
	if err != nil {
		return nil, fmt.Errorf("list the reactions on card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	reactions := make([]model.CardReaction, 0, len(rows))
	for _, row := range rows {
		reactions = append(reactions, toCardReaction(row.BoardID, row.CardID, row.ParticipantID, row.Reaction))
	}

	return reactions, nil
}

func (r *cardReactionDAO) List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.CardReaction, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardReactions(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list the card reactions of board %s: %w", boardID, err)
	}

	reactions := make([]model.CardReaction, 0, len(rows))
	for _, row := range rows {
		reactions = append(reactions, toCardReaction(row.BoardID, row.CardID, row.ParticipantID, row.Reaction))
	}

	return reactions, nil
}

func (r *cardReactionDAO) HandOver(ctx context.Context, s spec.ParticipantHandover, tx ...*sql.Tx) (int64, error) {
	handed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).HandOverCardReactions(ctx, sqlcgen.HandOverCardReactionsParams{
		ToParticipantID:   s.ToParticipantID,
		BoardID:           s.BoardID,
		FromParticipantID: s.FromParticipantID,
	})
	if err != nil {
		return 0, fmt.Errorf("hand the card reactions of participant %s to participant %s on board %s: %w", s.FromParticipantID, s.ToParticipantID, s.BoardID, err)
	}

	return handed.Moved, nil
}

func (r *cardReactionDAO) Sever(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	severed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SeverCardReactions(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("sever the card reactions of board %s from their participants: %w", boardID, err)
	}

	return severed, nil
}
