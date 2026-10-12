package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		Upload(ctx context.Context, command UploadCommand) (*dto.MediaView, error)
		Open(ctx context.Context, query OpenQuery) (*model.MediaFile, error)
		PurgeUnattached(ctx context.Context) (int64, error)
	}

	UploadCommand struct {
		Principal     model.Principal
		BoardID       uuid.UUID
		Kind          model.MediaKind
		ContentLength int64
		Body          io.Reader
	}

	OpenQuery struct {
		Principal model.Principal
		BoardID   uuid.UUID
		MediaID   uuid.UUID
	}

	Deps struct {
		Media        repository.MediaRepository
		Live         live.Service
		Entitlements entitlements.Service
		Encoder      Encoder
		Now          func() time.Time
	}

	service struct {
		media        repository.MediaRepository
		live         live.Service
		entitlements entitlements.Service
		encoder      Encoder
		now          func() time.Time
	}
)

func NewService(deps Deps) Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}

	return &service{
		media:        deps.Media,
		live:         deps.Live,
		entitlements: deps.Entitlements,
		encoder:      deps.Encoder,
		now:          now,
	}
}

func (s *service) Open(ctx context.Context, query OpenQuery) (*model.MediaFile, error) {
	access, err := s.live.Resolve(ctx, spec.BoardAccessQuery{Principal: query.Principal, BoardID: query.BoardID})
	if err != nil {
		return nil, err
	}

	file, err := s.media.Open(ctx, spec.MediaKey{BoardID: access.Header.ID, MediaID: query.MediaID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrMediaNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("open image %s of board %s: %w", query.MediaID, access.Header.ID, err)
	}

	return file, nil
}

func (s *service) PurgeUnattached(ctx context.Context) (int64, error) {
	cutoff := s.now().Add(-model.MediaUnattachedLimit)

	return s.media.PurgeUnattachedBefore(ctx, cutoff)
}
