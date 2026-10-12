package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	CardDAO interface {
		Create(ctx context.Context, s spec.NewCard, tx ...*sql.Tx) (*model.Card, error)
		GetByID(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) (*model.Card, error)
		List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.Card, error)
		ListInColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) ([]model.Card, error)
		Edit(ctx context.Context, s spec.CardEdit, tx ...*sql.Tx) (*model.Card, error)
		SetColour(ctx context.Context, s spec.CardColourChange, tx ...*sql.Tx) (*model.Card, error)
		Move(ctx context.Context, s spec.CardMove, tx ...*sql.Tx) (*model.Card, error)
		Delete(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) (*model.Card, error)
		ClearColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error)
		ReassignAuthor(ctx context.Context, s spec.CardAuthorReassignment, tx ...*sql.Tx) (int64, error)
		Stack(ctx context.Context, s spec.CardStacking, tx ...*sql.Tx) (*model.Card, error)
		Restack(ctx context.Context, s spec.CardRestack, tx ...*sql.Tx) ([]uuid.UUID, error)
		AddVoteTotal(ctx context.Context, s spec.CardVoteTotalChange, tx ...*sql.Tx) (*model.Card, error)
		ResetVoteTotals(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
		ResetColumnVoteTotals(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error)
		SetPosition(ctx context.Context, s spec.CardPositionChange, tx ...*sql.Tx) error
		SeverAuthors(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
		AddMention(ctx context.Context, s spec.CardMentionKey, tx ...*sql.Tx) error
		RemoveMention(ctx context.Context, s spec.CardMentionKey, tx ...*sql.Tx) error
		ListMentionsForUser(ctx context.Context, s spec.UserMentionsQuery, tx ...*sql.Tx) ([]model.Mention, error)
		ListWrittenByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.WrittenCard, error)
		SetPinned(ctx context.Context, s spec.CardPinChange, tx ...*sql.Tx) (*model.Card, error)
		SetImage(ctx context.Context, s spec.CardImageChange, tx ...*sql.Tx) (*model.Card, error)
	}

	cardDAO struct {
		db *sql.DB
	}
)

func toCard(row sqlcgen.Card) *model.Card {
	return &model.Card{
		ID:                  row.ID,
		BoardID:             row.BoardID,
		ColumnID:            row.ColumnID,
		ParentCardID:        row.ParentCardID,
		AuthorParticipantID: row.AuthorParticipantID,
		Body:                row.Body,
		Colour:              nullableColour(row.Colour),
		Position:            row.Position,
		VoteTotal:           int(row.VoteTotal),
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           nullableTime(row.UpdatedAt),
		PinnedAt:            nullableTime(row.PinnedAt),
		ImageMediaID:        row.ImageMediaID,
	}
}

func toMentionedCard(row sqlcgen.Card, mentions string) (*model.Card, error) {
	people, err := decodePeople(mentions)
	if err != nil {
		return nil, fmt.Errorf("decode the mentions of card %s: %w", row.ID, err)
	}

	card := toCard(row)
	card.Mentions = people

	return card, nil
}

func decodePeople(raw string) ([]model.ActionItemPerson, error) {
	var people []mentionedPerson
	if err := json.Unmarshal([]byte(raw), &people); err != nil {
		return nil, err
	}

	if len(people) == 0 {
		return nil, nil
	}

	named := make([]model.ActionItemPerson, 0, len(people))
	for _, person := range people {
		named = append(named, model.ActionItemPerson{UserID: person.UserID, DisplayName: person.DisplayName})
	}

	return named, nil
}

func nullableColour(value sql.NullString) *model.Colour {
	if !value.Valid {
		return nil
	}

	return new(model.Colour(value.String))
}

func toNullColour(value *model.Colour) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: string(*value), Valid: true}
}

func (r *cardDAO) Create(ctx context.Context, s spec.NewCard, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateCard(ctx, sqlcgen.CreateCardParams{
		ID:                  s.ID,
		BoardID:             s.BoardID,
		ColumnID:            s.ColumnID,
		AuthorParticipantID: s.AuthorParticipantID,
		Body:                s.Body,
		Colour:              toNullColour(s.Colour),
		Position:            s.Position,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create card %s on board %s: %w: %w", s.ID, s.BoardID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create card %s in column %s of board %s: %w: %w", s.ID, s.ColumnID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create card %s on board %s: %w", s.ID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) GetByID(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetCard(ctx, sqlcgen.GetCardParams{BoardID: s.BoardID, ID: s.CardID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get card %s of board %s: %w", s.CardID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	return toMentionedCard(row.Card, row.Mentions)
}

func (r *cardDAO) List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.Card, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCards(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list cards of board %s: %w", boardID, err)
	}

	cards := make([]model.Card, 0, len(rows))
	for _, row := range rows {
		card, err := toMentionedCard(row.Card, row.Mentions)
		if err != nil {
			return nil, err
		}

		cards = append(cards, *card)
	}

	return cards, nil
}

func (r *cardDAO) ListInColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) ([]model.Card, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardsInColumn(ctx, sqlcgen.ListCardsInColumnParams{BoardID: s.BoardID, ColumnID: s.ColumnID})
	if err != nil {
		return nil, fmt.Errorf("list cards in column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	cards := make([]model.Card, 0, len(rows))
	for _, row := range rows {
		card, err := toMentionedCard(row.Card, row.Mentions)
		if err != nil {
			return nil, err
		}

		cards = append(cards, *card)
	}

	return cards, nil
}

func (r *cardDAO) Edit(ctx context.Context, s spec.CardEdit, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).EditCard(ctx, sqlcgen.EditCardParams{BoardID: s.BoardID, ID: s.CardID, Body: s.Body})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("edit card %s of board %s: %w", s.CardID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("edit card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) SetColour(ctx context.Context, s spec.CardColourChange, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetCardColour(ctx, sqlcgen.SetCardColourParams{BoardID: s.BoardID, ID: s.CardID, Colour: toNullColour(s.Colour)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("set the colour of card %s of board %s: %w", s.CardID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("set the colour of card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) Move(ctx context.Context, s spec.CardMove, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MoveCard(ctx, sqlcgen.MoveCardParams{BoardID: s.BoardID, ID: s.CardID, ColumnID: s.ColumnID, Position: s.Position})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("move card %s of board %s: %w", s.CardID, s.BoardID, ErrNotFound)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("move card %s to column %s of board %s: %w: %w", s.CardID, s.ColumnID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("move card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) SetPinned(ctx context.Context, s spec.CardPinChange, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetCardPinned(ctx, sqlcgen.SetCardPinnedParams{Pinned: s.Pinned, BoardID: s.BoardID, ID: s.CardID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("set whether card %s of board %s is pinned: %w", s.CardID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("set whether card %s of board %s is pinned: %w", s.CardID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) SetImage(ctx context.Context, s spec.CardImageChange, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetCardImage(ctx, sqlcgen.SetCardImageParams{ImageMediaID: s.MediaID, BoardID: s.BoardID, ID: s.CardID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("set the image of card %s of board %s: %w", s.CardID, s.BoardID, ErrNotFound)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("set the image of card %s of board %s: %w: %w", s.CardID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("set the image of card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) Delete(ctx context.Context, s spec.CardRef, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteCard(ctx, sqlcgen.DeleteCardParams{BoardID: s.BoardID, ID: s.CardID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("delete card %s of board %s: %w", s.CardID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("delete card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) ClearColumn(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error) {
	cleared, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ClearBoardColumnCards(ctx, sqlcgen.ClearBoardColumnCardsParams{BoardID: s.BoardID, ColumnID: s.ColumnID})
	if err != nil {
		return 0, fmt.Errorf("clear column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return cleared, nil
}

func (r *cardDAO) ReassignAuthor(ctx context.Context, s spec.CardAuthorReassignment, tx ...*sql.Tx) (int64, error) {
	moved, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ReassignCardAuthor(ctx, sqlcgen.ReassignCardAuthorParams{
		ToParticipantID:   s.ToParticipantID,
		BoardID:           s.BoardID,
		FromParticipantID: s.FromParticipantID,
	})
	if err != nil {
		return 0, fmt.Errorf("move the cards of participant %s to participant %s on board %s: %w", s.FromParticipantID, s.ToParticipantID, s.BoardID, err)
	}

	return moved, nil
}

func (r *cardDAO) Stack(ctx context.Context, s spec.CardStacking, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MergeCard(ctx, sqlcgen.MergeCardParams{
		ParentCardID: s.ParentCardID,
		ColumnID:     s.ColumnID,
		Position:     s.Position,
		BoardID:      s.BoardID,
		ID:           s.CardID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("stack card %s onto card %s of board %s: %w", s.CardID, s.ParentCardID, s.BoardID, ErrNotFound)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("stack card %s onto card %s of board %s: %w: %w", s.CardID, s.ParentCardID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("stack card %s onto card %s of board %s: %w", s.CardID, s.ParentCardID, s.BoardID, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) Restack(ctx context.Context, s spec.CardRestack, tx ...*sql.Tx) ([]uuid.UUID, error) {
	moved, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RestackCards(ctx, sqlcgen.RestackCardsParams{
		ToParentCardID:   s.ToParentCardID,
		ColumnID:         s.ColumnID,
		BoardID:          s.BoardID,
		FromParentCardID: s.FromParentCardID,
	})
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("move the stack under card %s of board %s to card %s: %w: %w", s.FromParentCardID, s.BoardID, s.ToParentCardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("move the stack under card %s of board %s to card %s: %w", s.FromParentCardID, s.BoardID, s.ToParentCardID, err)
	}

	return moved, nil
}

func (r *cardDAO) AddVoteTotal(ctx context.Context, s spec.CardVoteTotalChange, tx ...*sql.Tx) (*model.Card, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddCardVoteTotal(ctx, sqlcgen.AddCardVoteTotalParams{Delta: int32(s.Delta), BoardID: s.BoardID, ID: s.CardID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("change the vote total of card %s of board %s by %d: %w", s.CardID, s.BoardID, s.Delta, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("change the vote total of card %s of board %s by %d: %w", s.CardID, s.BoardID, s.Delta, err)
	}

	return toCard(row), nil
}

func (r *cardDAO) ResetVoteTotals(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	reset, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ResetCardVoteTotals(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("reset the vote totals of board %s: %w", boardID, err)
	}

	return reset, nil
}

func (r *cardDAO) ResetColumnVoteTotals(ctx context.Context, s spec.BoardColumnKey, tx ...*sql.Tx) (int64, error) {
	reset, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ResetColumnVoteTotals(ctx, sqlcgen.ResetColumnVoteTotalsParams{BoardID: s.BoardID, ColumnID: s.ColumnID})
	if err != nil {
		return 0, fmt.Errorf("reset the vote totals of column %s of board %s: %w", s.ColumnID, s.BoardID, err)
	}

	return reset, nil
}

func (r *cardDAO) SetPosition(ctx context.Context, s spec.CardPositionChange, tx ...*sql.Tx) error {
	placed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetCardPosition(ctx, sqlcgen.SetCardPositionParams{Position: s.Position, BoardID: s.BoardID, ID: s.CardID})
	if err != nil {
		return fmt.Errorf("set the position of card %s of board %s: %w", s.CardID, s.BoardID, err)
	}

	if placed == 0 {
		return fmt.Errorf("set the position of card %s of board %s: %w", s.CardID, s.BoardID, ErrNotFound)
	}

	return nil
}

func (r *cardDAO) SeverAuthors(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	severed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SeverCardAuthors(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("sever the authors of the cards of board %s: %w", boardID, err)
	}

	return severed, nil
}

func (r *cardDAO) AddMention(ctx context.Context, s spec.CardMentionKey, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddCardMention(ctx, sqlcgen.AddCardMentionParams{OrgID: s.OrgID, BoardID: s.BoardID, CardID: s.CardID, UserID: s.UserID})
	if isForeignKeyViolation(err) {
		return fmt.Errorf("mention user %s on card %s of board %s: %w: %w", s.UserID, s.CardID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("mention user %s on card %s of board %s: %w", s.UserID, s.CardID, s.BoardID, err)
	}

	return nil
}

func (r *cardDAO) RemoveMention(ctx context.Context, s spec.CardMentionKey, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveCardMention(ctx, sqlcgen.RemoveCardMentionParams{BoardID: s.BoardID, CardID: s.CardID, UserID: s.UserID})
	if err != nil {
		return fmt.Errorf("remove the mention of user %s from card %s of board %s: %w", s.UserID, s.CardID, s.BoardID, err)
	}

	return nil
}

func (r *cardDAO) ListMentionsForUser(ctx context.Context, s spec.UserMentionsQuery, tx ...*sql.Tx) ([]model.Mention, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardMentionsForUser(ctx, sqlcgen.ListCardMentionsForUserParams{UserID: s.UserID, RowLimit: clampInt32(s.Limit)})
	if err != nil {
		return nil, fmt.Errorf("list the card mentions of user %s: %w", s.UserID, err)
	}

	mentions := make([]model.Mention, 0, len(rows))
	for _, row := range rows {
		mentions = append(mentions, model.Mention{
			Kind:        model.MentionCard,
			CardID:      row.CardID,
			OrgID:       row.OrgID,
			OrgSlug:     row.OrgSlug,
			OrgName:     row.OrgName,
			TeamID:      row.BoardTeamID,
			BoardID:     new(row.BoardID),
			BoardTitle:  row.BoardTitle,
			Body:        row.Body,
			CardsHidden: row.CardsHidden,
			OwnCard:     row.OwnCard,
			MentionedAt: row.MentionedAt,
		})
	}

	return mentions, nil
}

func (r *cardDAO) ListWrittenByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.WrittenCard, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardsWrittenByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the cards written by user %s: %w", userID, err)
	}

	cards := make([]model.WrittenCard, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, model.WrittenCard{
			OrgName:     row.OrgName,
			BoardTitle:  row.BoardTitle,
			ColumnTitle: row.ColumnTitle,
			Body:        row.Body,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   nullableTime(row.UpdatedAt),
		})
	}

	return cards, nil
}
