package board

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) AddComment(ctx context.Context, command AddCommentCommand) (*dto.BoardCommandResponse, error) {
	if err := ClientID(command.CommentID); err != nil {
		return nil, err
	}

	body, err := cleanComment(command.Body)
	if err != nil {
		return nil, err
	}

	mentions, err := CleanMentions(command.Mentions)
	if err != nil {
		return nil, err
	}

	add := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanComment(access, *card); err != nil {
			return nil, err
		}

		if err := s.members.CheckNamed(ctx, access, mentions); err != nil {
			return nil, err
		}

		written, err := s.comments.AddComment(ctx, spec.CommentCreate{
			BoardWrite:          target,
			CommentID:           command.CommentID,
			CardID:              card.ID,
			AuthorParticipantID: access.ParticipantID,
			Body:                body,
			Mentions:            mentions,
		})
		if errors.Is(err, dao.ErrConflict) {
			return nil, fmt.Errorf("%w: comment %s: %w", ErrIDConflict, command.CommentID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("add comment %s to card %s of board %s: %w", command.CommentID, card.ID, target.BoardID, err)
		}

		return s.announceComment(ctx, access, *card, written)
	}

	return s.Run(ctx, command.Command, s.commentReplay(ctx, command.CommentID), add)
}

func (s *service) EditComment(ctx context.Context, command EditCommentCommand) (*dto.BoardCommandResponse, error) {
	body, err := cleanComment(command.Body)
	if err != nil {
		return nil, err
	}

	mentions, err := CleanMentions(command.Mentions)
	if err != nil {
		return nil, err
	}

	edit := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		comment, card, err := s.commentOnCard(ctx, access, command.CommentID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanEditComment(access, *card, *comment); err != nil {
			return nil, err
		}

		if err := s.members.CheckNamed(ctx, access, NewMentions(mentions, comment.Mentions)); err != nil {
			return nil, err
		}

		written, err := s.comments.EditComment(ctx, spec.CommentBodyWrite{BoardWrite: target, CommentID: comment.ID, Body: body, Mentions: mentions})
		if err != nil {
			return nil, fmt.Errorf("edit comment %s of board %s: %w", comment.ID, target.BoardID, err)
		}

		return s.announceComment(ctx, access, *card, written)
	}

	return s.Run(ctx, command.Command, VersionReplay, edit)
}

func (s *service) DeleteComment(ctx context.Context, command CommentCommand) (*dto.BoardCommandResponse, error) {
	remove := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		comment, card, err := s.commentOnCard(ctx, access, command.CommentID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanDeleteComment(access, *card, *comment); err != nil {
			return nil, err
		}

		written, err := s.comments.DeleteComment(ctx, spec.CommentRemoval{BoardWrite: target, CommentID: comment.ID})
		if err != nil {
			return nil, fmt.Errorf("delete comment %s of board %s: %w", comment.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.CommentDeleted(access.Header.Board, *card, written.Value, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, remove)
}

func (s *service) announceComment(ctx context.Context, access model.BoardAccess, card model.Card, written *model.Versioned[model.CardComment]) (*dto.BoardCommandResponse, error) {
	author, err := s.profile(ctx, access, written.Value.AuthorParticipantID)
	if err != nil {
		s.events.Publish(access, projection.Resync(written.Version))

		return nil, err
	}

	s.events.Publish(access, projection.CommentUpserted(access.Header.Board, card, written.Value, author, written.Version))

	response := projection.CommentResponse(access.Header.Board, card, written.Value, author, projection.ViewerFor(access), written.Version)

	return &response, nil
}

func (s *service) commentReplay(ctx context.Context, commentID uuid.UUID) Replay {
	return func(access model.BoardAccess, version int64) (*dto.BoardCommandResponse, error) {
		created := projection.ReplayResponse(version, &commentID)

		comment, err := s.comments.GetByID(ctx, spec.CardCommentRef{BoardID: access.Header.ID, CommentID: commentID})
		if errors.Is(err, dao.ErrNotFound) {
			return &created, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read the replayed comment %s of board %s: %w", commentID, access.Header.ID, err)
		}

		viewer := projection.ViewerFor(access)
		if !projection.OwnsComment(*comment, viewer) {
			return &created, nil
		}

		card, err := s.cards.GetByID(ctx, spec.CardRef{BoardID: access.Header.ID, CardID: comment.CardID})
		if errors.Is(err, dao.ErrNotFound) {
			return &created, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read card %s of the replayed comment %s: %w", comment.CardID, commentID, err)
		}

		author, err := s.profile(ctx, access, comment.AuthorParticipantID)
		if err != nil {
			return nil, err
		}

		response := projection.ReplayCommentResponse(access.Header.Board, *card, *comment, author, viewer, version)

		return &response, nil
	}
}

func (s *service) commentOnCard(ctx context.Context, access model.BoardAccess, commentID uuid.UUID) (*model.CardComment, *model.Card, error) {
	comment, err := s.comments.GetByID(ctx, spec.CardCommentRef{BoardID: access.Header.ID, CommentID: commentID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, nil, fmt.Errorf("%w: %w", policy.ErrCommentNotFound, err)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read comment %s of board %s: %w", commentID, access.Header.ID, err)
	}

	card, err := s.cards.GetByID(ctx, spec.CardRef{BoardID: access.Header.ID, CardID: comment.CardID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, nil, fmt.Errorf("%w: card %s is gone: %w", policy.ErrCommentNotFound, comment.CardID, err)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read card %s of comment %s: %w", comment.CardID, commentID, err)
	}

	return comment, card, nil
}

func cleanComment(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", ErrCommentBodyRequired
	}

	if utf8.RuneCountInString(body) > model.CardCommentBodyMaxRunes {
		return "", ErrCommentBodyTooLong
	}

	return body, nil
}
