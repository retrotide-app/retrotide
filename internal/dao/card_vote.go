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
	CardVoteDAO interface {
		SpendBudget(ctx context.Context, s spec.VoteBudgetSpend, tx ...*sql.Tx) (int, error)
		RefundBudget(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) (int, error)
		IncrementVote(ctx context.Context, s spec.CardVoteIncrement, tx ...*sql.Tx) (int, error)
		LowerVote(ctx context.Context, s spec.CardVoteKey, tx ...*sql.Tx) (int, error)
		DropVote(ctx context.Context, s spec.CardVoteKey, tx ...*sql.Tx) error
		List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.CardVote, error)
		ListForParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) ([]model.CardVote, error)
		RefundCard(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) (int64, error)
		RefundColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error)
		CheckColumnBudget(ctx context.Context, s spec.ColumnVoteBudgetCheck, tx ...*sql.Tx) error
		DeleteColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error)
		DeleteAll(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
		DeleteBudgets(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
		HandOverVotes(ctx context.Context, s spec.VoteHandover, tx ...*sql.Tx) (int64, error)
		HandOverBudget(ctx context.Context, s spec.VoteHandover, tx ...*sql.Tx) (int64, error)
	}

	cardVoteDAO struct {
		db *sql.DB
	}
)

func toCardVote(cardID, participantID uuid.UUID, count int16, author *uuid.UUID, columnID uuid.UUID) model.CardVote {
	return model.CardVote{CardID: cardID, ParticipantID: participantID, Count: int(count), CardAuthorParticipantID: author, CardColumnID: columnID}
}

func (r *cardVoteDAO) SpendBudget(ctx context.Context, s spec.VoteBudgetSpend, tx ...*sql.Tx) (int, error) {
	used, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SpendVoteBudget(ctx, sqlcgen.SpendVoteBudgetParams{
		BoardID:       s.BoardID,
		ParticipantID: s.ParticipantID,
		MaxVotes:      clampInt32(s.MaxVotes),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("spend a vote of participant %s on board %s, which allows %d: %w", s.ParticipantID, s.BoardID, s.MaxVotes, ErrVoteBudgetSpent)
	}
	if isForeignKeyViolation(err) {
		return 0, fmt.Errorf("spend a vote of participant %s on board %s: %w: %w", s.ParticipantID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return 0, fmt.Errorf("spend a vote of participant %s on board %s: %w", s.ParticipantID, s.BoardID, err)
	}

	return int(used), nil
}

func (r *cardVoteDAO) RefundBudget(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) (int, error) {
	used, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RefundVoteBudget(ctx, sqlcgen.RefundVoteBudgetParams{BoardID: s.BoardID, ParticipantID: s.ParticipantID})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("refund a vote of participant %s on board %s: %w", s.ParticipantID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("refund a vote of participant %s on board %s: %w", s.ParticipantID, s.BoardID, err)
	}

	return int(used), nil
}

func (r *cardVoteDAO) IncrementVote(ctx context.Context, s spec.CardVoteIncrement, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).IncrementCardVote(ctx, sqlcgen.IncrementCardVoteParams{
		BoardID:       s.BoardID,
		CardID:        s.CardID,
		ParticipantID: s.ParticipantID,
		MaxPerCard:    clampInt32(s.MaxVotesPerCard),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("vote for card %s of board %s as participant %s, which allows %d per card: %w", s.CardID, s.BoardID, s.ParticipantID, s.MaxVotesPerCard, ErrCardVoteCapReached)
	}
	if isForeignKeyViolation(err) {
		return 0, fmt.Errorf("vote for card %s of board %s as participant %s: %w: %w", s.CardID, s.BoardID, s.ParticipantID, ErrNotFound, err)
	}
	if err != nil {
		return 0, fmt.Errorf("vote for card %s of board %s as participant %s: %w", s.CardID, s.BoardID, s.ParticipantID, err)
	}

	return int(count), nil
}

func (r *cardVoteDAO) LowerVote(ctx context.Context, s spec.CardVoteKey, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).LowerCardVote(ctx, sqlcgen.LowerCardVoteParams{BoardID: s.BoardID, CardID: s.CardID, ParticipantID: s.ParticipantID})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("lower the votes of participant %s on card %s of board %s: %w", s.ParticipantID, s.CardID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("lower the votes of participant %s on card %s of board %s: %w", s.ParticipantID, s.CardID, s.BoardID, err)
	}

	return int(count), nil
}

func (r *cardVoteDAO) DropVote(ctx context.Context, s spec.CardVoteKey, tx ...*sql.Tx) error {
	dropped, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DropCardVote(ctx, sqlcgen.DropCardVoteParams{BoardID: s.BoardID, CardID: s.CardID, ParticipantID: s.ParticipantID})
	if err != nil {
		return fmt.Errorf("drop the last vote of participant %s on card %s of board %s: %w", s.ParticipantID, s.CardID, s.BoardID, err)
	}

	if dropped == 0 {
		return fmt.Errorf("drop the last vote of participant %s on card %s of board %s: %w", s.ParticipantID, s.CardID, s.BoardID, ErrNotFound)
	}

	return nil
}

func (r *cardVoteDAO) List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.CardVote, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardVotes(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list the votes of board %s: %w", boardID, err)
	}

	votes := make([]model.CardVote, 0, len(rows))
	for _, row := range rows {
		votes = append(votes, toCardVote(row.CardID, row.ParticipantID, row.Count, row.CardAuthorParticipantID, row.CardColumnID))
	}

	return votes, nil
}

func (r *cardVoteDAO) ListForParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) ([]model.CardVote, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListParticipantCardVotes(ctx, sqlcgen.ListParticipantCardVotesParams{BoardID: s.BoardID, ParticipantID: s.ParticipantID})
	if err != nil {
		return nil, fmt.Errorf("list the votes of participant %s on board %s: %w", s.ParticipantID, s.BoardID, err)
	}

	votes := make([]model.CardVote, 0, len(rows))
	for _, row := range rows {
		votes = append(votes, toCardVote(row.CardID, row.ParticipantID, row.Count, row.CardAuthorParticipantID, row.CardColumnID))
	}

	return votes, nil
}

func (r *cardVoteDAO) RefundCard(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) (int64, error) {
	refunded, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RefundCardVoteBudgets(ctx, sqlcgen.RefundCardVoteBudgetsParams{BoardID: s.BoardID, CardID: s.CardID})
	if err != nil {
		return 0, fmt.Errorf("refund the votes on card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	return refunded, nil
}

func (r *cardVoteDAO) RefundColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error) {
	refunded, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RefundColumnVoteBudgets(ctx, sqlcgen.RefundColumnVoteBudgetsParams{BoardID: s.BoardID, ColumnID: s.ColumnID})
	if err != nil {
		return 0, fmt.Errorf("refund the votes on the cards of column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return refunded, nil
}

func (r *cardVoteDAO) CheckColumnBudget(ctx context.Context, s spec.ColumnVoteBudgetCheck, tx ...*sql.Tx) error {
	used, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountColumnVotes(ctx, sqlcgen.CountColumnVotesParams{BoardID: s.BoardID, ParticipantID: s.ParticipantID, ColumnID: s.ColumnID})
	if err != nil {
		return fmt.Errorf("count the votes of participant %s in column %s of board %s: %w", s.ParticipantID, s.ColumnID, s.BoardID, err)
	}

	if int(used) >= s.MaxVotes {
		return fmt.Errorf("vote in column %s of board %s as participant %s, which allows %d: %w", s.ColumnID, s.BoardID, s.ParticipantID, s.MaxVotes, ErrColumnVoteBudgetSpent)
	}

	return nil
}

func (r *cardVoteDAO) DeleteColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteColumnCardVotes(ctx, sqlcgen.DeleteColumnCardVotesParams{BoardID: s.BoardID, ColumnID: s.ColumnID})
	if err != nil {
		return 0, fmt.Errorf("delete the votes on the cards of column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return deleted, nil
}

func (r *cardVoteDAO) DeleteAll(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteBoardCardVotes(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("delete the votes of board %s: %w", boardID, err)
	}

	return deleted, nil
}

func (r *cardVoteDAO) DeleteBudgets(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteBoardVoteBudgets(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("delete the vote budgets of board %s: %w", boardID, err)
	}

	return deleted, nil
}

func (r *cardVoteDAO) HandOverVotes(ctx context.Context, s spec.VoteHandover, tx ...*sql.Tx) (int64, error) {
	moved, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MergeParticipantCardVotes(ctx, sqlcgen.MergeParticipantCardVotesParams{
		ToParticipantID:   s.ToParticipantID,
		BoardID:           s.BoardID,
		FromParticipantID: s.FromParticipantID,
	})
	if err != nil {
		return 0, fmt.Errorf("hand the votes of participant %s to participant %s on board %s: %w", s.FromParticipantID, s.ToParticipantID, s.BoardID, err)
	}

	return moved, nil
}

func (r *cardVoteDAO) HandOverBudget(ctx context.Context, s spec.VoteHandover, tx ...*sql.Tx) (int64, error) {
	moved, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MergeParticipantVoteBudget(ctx, sqlcgen.MergeParticipantVoteBudgetParams{
		ToParticipantID:   s.ToParticipantID,
		BoardID:           s.BoardID,
		FromParticipantID: s.FromParticipantID,
	})
	if err != nil {
		return 0, fmt.Errorf("hand the vote budget of participant %s to participant %s on board %s: %w", s.FromParticipantID, s.ToParticipantID, s.BoardID, err)
	}

	return moved, nil
}
