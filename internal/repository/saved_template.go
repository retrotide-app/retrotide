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
	SavedTemplateRepository interface {
		dao.SavedTemplateDAO
	}

	savedTemplateRepository struct {
		dao.SavedTemplateDAO

		db            *sql.DB
		organisations dao.OrganisationDAO
	}
)

func NewSavedTemplateRepo(database *sql.DB, savedTemplateDAO dao.SavedTemplateDAO, organisations dao.OrganisationDAO) SavedTemplateRepository {
	return &savedTemplateRepository{SavedTemplateDAO: savedTemplateDAO, db: database, organisations: organisations}
}

func (r *savedTemplateRepository) Create(ctx context.Context, s spec.NewSavedTemplate, tx ...*sql.Tx) (*model.SavedTemplate, error) {
	var created *model.SavedTemplate

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if err := r.organisations.Lock(ctx, s.OrgID, tx); err != nil {
			return err
		}

		template, err := r.SavedTemplateDAO.Create(ctx, s, tx)
		if err != nil {
			return err
		}

		created = template

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}
