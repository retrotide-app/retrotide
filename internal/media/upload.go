package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

const (
	pngContentType  = "image/png"
	jpegContentType = "image/jpeg"
	webpContentType = "image/webp"
	maxStoredEdge   = 4096
)

func (s *service) Upload(ctx context.Context, command UploadCommand) (*dto.MediaView, error) {
	maxEdge, err := maxEdgeFor(command.Kind)
	if err != nil {
		return nil, err
	}

	access, err := s.live.Resolve(ctx, spec.BoardAccessQuery{Principal: command.Principal, BoardID: command.BoardID})
	if err != nil {
		return nil, err
	}

	if err := policy.CanUploadImage(*access); err != nil {
		return nil, err
	}

	plan, err := s.entitlements.For(ctx, access.Header.OrgID)
	if err != nil {
		return nil, fmt.Errorf("read the plan of organisation %s: %w", access.Header.OrgID, err)
	}

	allowed, known := plan.Allowance(model.LimitCardImageBytes)
	if !known {
		return nil, fmt.Errorf("%w: %q on the %s plan", entitlements.ErrUnknownLimit, model.LimitCardImageBytes, plan.Code)
	}

	if command.ContentLength > int64(allowed) {
		return nil, &entitlements.DeniedError{Plan: plan.Code, Limit: model.LimitCardImageBytes, Allowed: allowed, Current: int(command.ContentLength)}
	}

	data, err := io.ReadAll(io.LimitReader(command.Body, int64(allowed)+1))
	if err != nil {
		return nil, fmt.Errorf("read the image uploaded to board %s: %w", access.Header.ID, err)
	}

	if len(data) > allowed {
		return nil, &entitlements.DeniedError{Plan: plan.Code, Limit: model.LimitCardImageBytes, Allowed: allowed, Current: len(data)}
	}

	width, height, err := imageSize(data)
	if err != nil {
		return nil, err
	}

	if int64(width)*int64(height) > model.MediaMaxPixels {
		return nil, fmt.Errorf("%w: the image is %dx%d", ErrImageDimensions, width, height)
	}

	encoded, err := s.encoder.Encode(ctx, data, maxEdge)
	if err != nil {
		return nil, fmt.Errorf("re-encode a %dx%d image for board %s: %w", width, height, access.Header.ID, err)
	}

	if !storableEdge(encoded.Width) || !storableEdge(encoded.Height) {
		return nil, fmt.Errorf("%w: the re-encoded image is %dx%d", ErrImageDimensions, encoded.Width, encoded.Height)
	}

	storable, known := plan.Allowance(model.LimitMediaBytes)
	if !known {
		return nil, fmt.Errorf("%w: %q on the %s plan", entitlements.ErrUnknownLimit, model.LimitMediaBytes, plan.Code)
	}

	stored, err := s.media.Create(ctx, spec.NewMedia{
		OrgID:          access.Header.OrgID,
		BoardID:        access.Header.ID,
		Kind:           command.Kind,
		Width:          encoded.Width,
		Height:         encoded.Height,
		Data:           encoded.Data,
		MaxStoredBytes: storable,
	})
	if errors.Is(err, dao.ErrQuotaExceeded) {
		return nil, &entitlements.DeniedError{Plan: plan.Code, Limit: model.LimitMediaBytes, Allowed: storable, Current: storable}
	}
	if err != nil {
		return nil, fmt.Errorf("store the %s image for board %s: %w", command.Kind, access.Header.ID, err)
	}

	view := projection.MediaResponse(*stored)

	return &view, nil
}

func maxEdgeFor(kind model.MediaKind) (int, error) {
	switch kind {
	case model.MediaKindCard:
		return model.CardImageMaxEdge, nil
	case model.MediaKindBackground:
		return model.BackgroundMaxEdge, nil
	}

	return 0, fmt.Errorf("%w: got %q", ErrInvalidKind, kind)
}

func imageSize(data []byte) (int, int, error) {
	width, height, err := headerSize(data)
	if err != nil {
		return 0, 0, err
	}

	if width < 1 || height < 1 {
		return 0, 0, fmt.Errorf("%w: the image claims to be %dx%d", ErrUnsupportedImage, width, height)
	}

	return width, height, nil
}

func headerSize(data []byte) (int, int, error) {
	contentType := http.DetectContentType(data)

	switch contentType {
	case pngContentType, jpegContentType:
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return 0, 0, fmt.Errorf("%w: read the %s header: %w", ErrUnsupportedImage, contentType, err)
		}

		return config.Width, config.Height, nil
	case webpContentType:
		return webpSize(data)
	}

	return 0, 0, fmt.Errorf("%w: the upload is %s", ErrUnsupportedImage, contentType)
}

func storableEdge(edge int) bool {
	return edge >= 1 && edge <= maxStoredEdge
}
