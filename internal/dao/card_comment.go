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
	CardCommentDAO interface {
		Create(ctx context.Context, s spec.NewCardComment, tx ...*sql.Tx) (*model.CardComment, error)
		GetByID(ctx context.Context, s spec.CardCommentRef, tx ...*sql.Tx) (*model.CardComment, error)
		List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.CardComment, error)
		Edit(ctx context.Context, s spec.CardCommentEdit, tx ...*sql.Tx) (*model.CardComment, error)
		Delete(ctx context.Context, s spec.CardCommentRef, tx ...*sql.Tx) (*model.CardComment, error)
		ReassignAuthor(ctx context.Context, s spec.CardAuthorReassignment, tx ...*sql.Tx) (int64, error)
		SeverAuthors(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
		AddMention(ctx context.Context, s spec.CommentMentionKey, tx ...*sql.Tx) error
		RemoveMention(ctx context.Context, s spec.CommentMentionKey, tx ...*sql.Tx) error
		ListMentionsForUser(ctx context.Context, s spec.UserMentionsQuery, tx ...*sql.Tx) ([]model.Mention, error)
		ListWrittenByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.WrittenComment, error)
	}

	cardCommentDAO struct {
		db *sql.DB
	}
)

func toCardComment(row sqlcgen.CardComment) *model.CardComment {
	return &model.CardComment{
		ID:                  row.ID,
		BoardID:             row.BoardID,
		CardID:              row.CardID,
		AuthorParticipantID: row.AuthorParticipantID,
		Body:                row.Body,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           nullableTime(row.UpdatedAt),
	}
}

func cardComment(row sqlcgen.CardComment, err error, action string) (*model.CardComment, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	return toCardComment(row), nil
}

func (r *cardCommentDAO) Create(ctx context.Context, s spec.NewCardComment, tx ...*sql.Tx) (*model.CardComment, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateCardComment(ctx, sqlcgen.CreateCardCommentParams{
		ID:                  s.ID,
		BoardID:             s.BoardID,
		CardID:              s.CardID,
		AuthorParticipantID: s.AuthorParticipantID,
		Body:                s.Body,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create comment %s on board %s: %w: %w", s.ID, s.BoardID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create comment %s on card %s of board %s: %w: %w", s.ID, s.CardID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create comment %s on board %s: %w", s.ID, s.BoardID, err)
	}

	return toCardComment(row), nil
}

func toMentionedComment(row sqlcgen.CardComment, mentions string) (*model.CardComment, error) {
	people, err := decodePeople(mentions)
	if err != nil {
		return nil, fmt.Errorf("decode the mentions of comment %s: %w", row.ID, err)
	}

	comment := toCardComment(row)
	comment.Mentions = people

	return comment, nil
}

func (r *cardCommentDAO) GetByID(ctx context.Context, s spec.CardCommentRef, tx ...*sql.Tx) (*model.CardComment, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetCardComment(ctx, sqlcgen.GetCardCommentParams{BoardID: s.BoardID, ID: s.CommentID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get comment %s of board %s: %w", s.CommentID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get comment %s of board %s: %w", s.CommentID, s.BoardID, err)
	}

	return toMentionedComment(row.CardComment, row.Mentions)
}

func (r *cardCommentDAO) List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.CardComment, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardComments(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list the comments of board %s: %w", boardID, err)
	}

	comments := make([]model.CardComment, 0, len(rows))
	for _, row := range rows {
		comment, err := toMentionedComment(row.CardComment, row.Mentions)
		if err != nil {
			return nil, err
		}

		comments = append(comments, *comment)
	}

	return comments, nil
}

func (r *cardCommentDAO) Edit(ctx context.Context, s spec.CardCommentEdit, tx ...*sql.Tx) (*model.CardComment, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).EditCardComment(ctx, sqlcgen.EditCardCommentParams{Body: s.Body, BoardID: s.BoardID, ID: s.CommentID})

	return cardComment(row, err, fmt.Sprintf("edit comment %s of board %s", s.CommentID, s.BoardID))
}

func (r *cardCommentDAO) Delete(ctx context.Context, s spec.CardCommentRef, tx ...*sql.Tx) (*model.CardComment, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteCardComment(ctx, sqlcgen.DeleteCardCommentParams{BoardID: s.BoardID, ID: s.CommentID})

	return cardComment(row, err, fmt.Sprintf("delete comment %s of board %s", s.CommentID, s.BoardID))
}

func (r *cardCommentDAO) ReassignAuthor(ctx context.Context, s spec.CardAuthorReassignment, tx ...*sql.Tx) (int64, error) {
	moved, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ReassignCardCommentAuthor(ctx, sqlcgen.ReassignCardCommentAuthorParams{
		ToParticipantID:   s.ToParticipantID,
		BoardID:           s.BoardID,
		FromParticipantID: s.FromParticipantID,
	})
	if err != nil {
		return 0, fmt.Errorf("move the comments of participant %s to participant %s on board %s: %w", s.FromParticipantID, s.ToParticipantID, s.BoardID, err)
	}

	return moved, nil
}

func (r *cardCommentDAO) SeverAuthors(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	severed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SeverCardCommentAuthors(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("sever the authors of the comments of board %s: %w", boardID, err)
	}

	return severed, nil
}

func (r *cardCommentDAO) AddMention(ctx context.Context, s spec.CommentMentionKey, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddCardCommentMention(ctx, sqlcgen.AddCardCommentMentionParams{OrgID: s.OrgID, BoardID: s.BoardID, CommentID: s.CommentID, UserID: s.UserID})
	if isForeignKeyViolation(err) {
		return fmt.Errorf("mention user %s on comment %s of board %s: %w: %w", s.UserID, s.CommentID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("mention user %s on comment %s of board %s: %w", s.UserID, s.CommentID, s.BoardID, err)
	}

	return nil
}

func (r *cardCommentDAO) RemoveMention(ctx context.Context, s spec.CommentMentionKey, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveCardCommentMention(ctx, sqlcgen.RemoveCardCommentMentionParams{BoardID: s.BoardID, CommentID: s.CommentID, UserID: s.UserID})
	if err != nil {
		return fmt.Errorf("remove the mention of user %s from comment %s of board %s: %w", s.UserID, s.CommentID, s.BoardID, err)
	}

	return nil
}

func (r *cardCommentDAO) ListMentionsForUser(ctx context.Context, s spec.UserMentionsQuery, tx ...*sql.Tx) ([]model.Mention, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardCommentMentionsForUser(ctx, sqlcgen.ListCardCommentMentionsForUserParams{UserID: s.UserID, RowLimit: clampInt32(s.Limit)})
	if err != nil {
		return nil, fmt.Errorf("list the comment mentions of user %s: %w", s.UserID, err)
	}

	mentions := make([]model.Mention, 0, len(rows))
	for _, row := range rows {
		mentions = append(mentions, model.Mention{
			Kind:        model.MentionComment,
			CardID:      row.CardID,
			CommentID:   row.CommentID,
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

func (r *cardCommentDAO) ListWrittenByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.WrittenComment, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListCardCommentsWrittenByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the comments written by user %s: %w", userID, err)
	}

	comments := make([]model.WrittenComment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, model.WrittenComment{
			OrgName:    row.OrgName,
			BoardTitle: row.BoardTitle,
			Body:       row.Body,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  nullableTime(row.UpdatedAt),
		})
	}

	return comments, nil
}
