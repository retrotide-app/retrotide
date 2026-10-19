package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	MediaDAO interface {
		Create(ctx context.Context, s spec.NewMedia, tx ...*sql.Tx) (*model.Media, error)
		Get(ctx context.Context, s spec.MediaKey, tx ...*sql.Tx) (*model.Media, error)
		Open(ctx context.Context, s spec.MediaKey, tx ...*sql.Tx) (*model.MediaFile, error)
		PurgeUnattachedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error)
	}

	mediaDAO struct {
		db *sql.DB
	}
)

func toMedia(row sqlcgen.GetMediaRow) *model.Media {
	return &model.Media{
		ID:          row.ID,
		OrgID:       row.OrgID,
		BoardID:     row.BoardID,
		Kind:        model.MediaKind(row.Kind),
		ContentType: row.ContentType,
		Width:       int(row.Width),
		Height:      int(row.Height),
		ByteSize:    int(row.ByteSize),
		CreatedAt:   row.CreatedAt,
	}
}

func (r *mediaDAO) Create(ctx context.Context, s spec.NewMedia, tx ...*sql.Tx) (*model.Media, error) {
	action := fmt.Sprintf("store a %s image of %dx%d on board %s", s.Kind, s.Width, s.Height, s.BoardID)

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateMediaWithinQuota(ctx, sqlcgen.CreateMediaWithinQuotaParams{
		OrgID:          s.OrgID,
		BoardID:        s.BoardID,
		Kind:           string(s.Kind),
		Width:          clampInt32(s.Width),
		Height:         clampInt32(s.Height),
		ByteSize:       clampInt32(len(s.Data)),
		Data:           s.Data,
		MaxStoredBytes: int64(s.MaxStoredBytes),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%s: %w", action, ErrQuotaExceeded)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	return toMedia(sqlcgen.GetMediaRow(row)), nil
}

func (r *mediaDAO) Get(ctx context.Context, s spec.MediaKey, tx ...*sql.Tx) (*model.Media, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetMedia(ctx, sqlcgen.GetMediaParams{BoardID: s.BoardID, ID: s.MediaID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get image %s of board %s: %w", s.MediaID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get image %s of board %s: %w", s.MediaID, s.BoardID, err)
	}

	return toMedia(row), nil
}

func (r *mediaDAO) Open(ctx context.Context, s spec.MediaKey, tx ...*sql.Tx) (*model.MediaFile, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).OpenMedia(ctx, sqlcgen.OpenMediaParams{BoardID: s.BoardID, ID: s.MediaID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("open image %s of board %s: %w", s.MediaID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("open image %s of board %s: %w", s.MediaID, s.BoardID, err)
	}

	media := toMedia(sqlcgen.GetMediaRow{
		ID:          row.ID,
		OrgID:       row.OrgID,
		BoardID:     row.BoardID,
		Kind:        row.Kind,
		ContentType: row.ContentType,
		Width:       row.Width,
		Height:      row.Height,
		ByteSize:    row.ByteSize,
		CreatedAt:   row.CreatedAt,
	})

	return &model.MediaFile{Media: *media, Data: row.Data}, nil
}

func (r *mediaDAO) PurgeUnattachedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error) {
	purged, err := sqlcgen.New(db.TxOrDB(r.db, tx)).PurgeUnattachedMedia(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge the images uploaded before %s that no card or board uses: %w", cutoff.Format(time.RFC3339), err)
	}

	return purged, nil
}
