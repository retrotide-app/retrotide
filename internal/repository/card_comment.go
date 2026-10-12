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
	CardCommentRepository interface {
		dao.CardCommentDAO

		AddComment(ctx context.Context, s spec.CommentCreate, tx ...*sql.Tx) (*model.Versioned[model.CardComment], error)
		EditComment(ctx context.Context, s spec.CommentBodyWrite, tx ...*sql.Tx) (*model.Versioned[model.CardComment], error)
		DeleteComment(ctx context.Context, s spec.CommentRemoval, tx ...*sql.Tx) (*model.Versioned[model.CardComment], error)
	}

	cardCommentRepository struct {
		dao.CardCommentDAO

		db     *sql.DB
		boards dao.BoardDAO
	}
)

func NewCardCommentRepo(database *sql.DB, commentDAO dao.CardCommentDAO, boards dao.BoardDAO) CardCommentRepository {
	return &cardCommentRepository{CardCommentDAO: commentDAO, db: database, boards: boards}
}

func (r *cardCommentRepository) AddComment(ctx context.Context, s spec.CommentCreate, tx ...*sql.Tx) (*model.Versioned[model.CardComment], error) {
	add := func(tx *sql.Tx) (*model.CardComment, error) {
		_, err := r.CardCommentDAO.Create(ctx, spec.NewCardComment{
			ID:                  s.CommentID,
			BoardID:             s.BoardID,
			CardID:              s.CardID,
			AuthorParticipantID: s.AuthorParticipantID,
			Body:                s.Body,
		}, tx)
		if err != nil {
			return nil, err
		}

		for _, userID := range s.Mentions {
			if err := r.CardCommentDAO.AddMention(ctx, spec.CommentMentionKey{OrgID: s.OrgID, BoardID: s.BoardID, CommentID: s.CommentID, UserID: userID}, tx); err != nil {
				return nil, err
			}
		}

		return r.CardCommentDAO.GetByID(ctx, spec.CardCommentRef{BoardID: s.BoardID, CommentID: s.CommentID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, add)
}

func (r *cardCommentRepository) EditComment(ctx context.Context, s spec.CommentBodyWrite, tx ...*sql.Tx) (*model.Versioned[model.CardComment], error) {
	edit := func(tx *sql.Tx) (*model.CardComment, error) {
		ref := spec.CardCommentRef{BoardID: s.BoardID, CommentID: s.CommentID}

		current, err := r.CardCommentDAO.GetByID(ctx, ref, tx)
		if err != nil {
			return nil, err
		}

		if _, err := r.CardCommentDAO.Edit(ctx, spec.CardCommentEdit{BoardID: s.BoardID, CommentID: s.CommentID, Body: s.Body}, tx); err != nil {
			return nil, err
		}

		for _, person := range current.Mentions {
			if slices.Contains(s.Mentions, person.UserID) {
				continue
			}

			if err := r.CardCommentDAO.RemoveMention(ctx, spec.CommentMentionKey{OrgID: s.OrgID, BoardID: s.BoardID, CommentID: s.CommentID, UserID: person.UserID}, tx); err != nil {
				return nil, err
			}
		}

		for _, userID := range s.Mentions {
			if err := r.CardCommentDAO.AddMention(ctx, spec.CommentMentionKey{OrgID: s.OrgID, BoardID: s.BoardID, CommentID: s.CommentID, UserID: userID}, tx); err != nil {
				return nil, err
			}
		}

		return r.CardCommentDAO.GetByID(ctx, ref, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, edit)
}

func (r *cardCommentRepository) DeleteComment(ctx context.Context, s spec.CommentRemoval, tx ...*sql.Tx) (*model.Versioned[model.CardComment], error) {
	remove := func(tx *sql.Tx) (*model.CardComment, error) {
		return r.CardCommentDAO.Delete(ctx, spec.CardCommentRef{BoardID: s.BoardID, CommentID: s.CommentID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, remove)
}
