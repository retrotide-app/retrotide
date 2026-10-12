package images

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		SetCardImage(ctx context.Context, command CardImageCommand) (*dto.BoardCommandResponse, error)
		SetBackground(ctx context.Context, command BackgroundCommand) (*dto.BoardCommandResponse, error)
	}

	Deps struct {
		Boards repository.BoardRepository
		Cards  repository.CardRepository
		Media  repository.MediaRepository
		Live   live.Service
		Events live.Publisher
	}

	service struct {
		board.Runner

		boards repository.BoardRepository
		cards  repository.CardRepository
		media  repository.MediaRepository
		events live.Publisher
	}
)

func NewService(deps Deps) Service {
	return &service{
		Runner: board.NewRunner(deps.Live, deps.Boards, deps.Cards),
		boards: deps.Boards,
		cards:  deps.Cards,
		media:  deps.Media,
		events: deps.Events,
	}
}

func (s *service) attachable(ctx context.Context, access model.BoardAccess, mediaID *uuid.UUID, kind model.MediaKind) error {
	if mediaID == nil {
		return nil
	}

	media, err := s.media.Get(ctx, spec.MediaKey{BoardID: access.Header.ID, MediaID: *mediaID})
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrMediaNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("read image %s of board %s: %w", *mediaID, access.Header.ID, err)
	}

	if media.Kind != kind {
		return fmt.Errorf("%w: image %s is a %s image, not a %s image", ErrWrongKind, media.ID, media.Kind, kind)
	}

	return nil
}
