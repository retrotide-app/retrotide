package repository

import (
	"context"
	"database/sql"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	MediaRepository interface {
		dao.MediaDAO
	}

	mediaRepository struct {
		dao.MediaDAO

		db            *sql.DB
		organisations dao.OrganisationDAO
	}
)

func NewMediaRepo(database *sql.DB, mediaDAO dao.MediaDAO, organisations dao.OrganisationDAO) MediaRepository {
	return &mediaRepository{MediaDAO: mediaDAO, db: database, organisations: organisations}
}

func (r *mediaRepository) Create(ctx context.Context, s spec.NewMedia, tx ...*sql.Tx) (*model.Media, error) {
	var created *model.Media

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if err := r.organisations.Lock(ctx, s.OrgID, tx); err != nil {
			return err
		}

		stored, err := r.MediaDAO.Create(ctx, s, tx)
		if err != nil {
			return err
		}

		created = stored

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}
