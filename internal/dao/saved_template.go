package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	SavedTemplateDAO interface {
		Create(ctx context.Context, s spec.NewSavedTemplate, tx ...*sql.Tx) (*model.SavedTemplate, error)
		Get(ctx context.Context, s spec.SavedTemplateKey, tx ...*sql.Tx) (*model.SavedTemplate, error)
		List(ctx context.Context, s spec.SavedTemplateViewer, tx ...*sql.Tx) ([]model.SavedTemplate, error)
		Count(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (int, error)
		Delete(ctx context.Context, s spec.SavedTemplateKey, tx ...*sql.Tx) (*model.SavedTemplate, error)
	}

	savedTemplateDAO struct {
		db *sql.DB
	}

	savedTemplateColumnJSON struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Colour      string `json:"colour"`
	}

	savedTemplateRow struct {
		ID          uuid.UUID
		OrgID       uuid.UUID
		TeamID      *uuid.UUID
		OwnerID     *uuid.UUID
		CreatedBy   *uuid.UUID
		Name        string
		Description string
		Columns     json.RawMessage
		CreatedAt   time.Time
		TeamName    string
	}
)

func encodeSavedTemplateColumns(columns []model.SavedTemplateColumn) (json.RawMessage, error) {
	encoded := make([]savedTemplateColumnJSON, 0, len(columns))
	for _, column := range columns {
		encoded = append(encoded, savedTemplateColumnJSON{Title: column.Title, Description: column.Description, Colour: string(column.Colour)})
	}

	raw, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode saved template columns: %w", err)
	}

	return raw, nil
}

func toSavedTemplate(row savedTemplateRow) (*model.SavedTemplate, error) {
	var decoded []savedTemplateColumnJSON
	if err := json.Unmarshal(row.Columns, &decoded); err != nil {
		return nil, fmt.Errorf("decode columns of saved template %s: %w", row.ID, err)
	}

	columns := make([]model.SavedTemplateColumn, 0, len(decoded))
	for _, column := range decoded {
		columns = append(columns, model.SavedTemplateColumn{Title: column.Title, Description: column.Description, Colour: model.Colour(column.Colour)})
	}

	return &model.SavedTemplate{
		ID:          row.ID,
		OrgID:       row.OrgID,
		TeamID:      row.TeamID,
		OwnerID:     row.OwnerID,
		CreatedBy:   row.CreatedBy,
		Name:        row.Name,
		Description: row.Description,
		Columns:     columns,
		TeamName:    row.TeamName,
		CreatedAt:   row.CreatedAt,
	}, nil
}

func savedTemplateRowFromTable(row sqlcgen.SavedTemplate) savedTemplateRow {
	return savedTemplateRow{
		ID:          row.ID,
		OrgID:       row.OrgID,
		TeamID:      row.TeamID,
		OwnerID:     row.OwnerID,
		CreatedBy:   row.CreatedBy,
		Name:        row.Name,
		Description: row.Description,
		Columns:     row.Columns,
		CreatedAt:   row.CreatedAt,
	}
}

func (r *savedTemplateDAO) Create(ctx context.Context, s spec.NewSavedTemplate, tx ...*sql.Tx) (*model.SavedTemplate, error) {
	columns, err := encodeSavedTemplateColumns(s.Columns)
	if err != nil {
		return nil, fmt.Errorf("create saved template %q in organisation %s: %w", s.Name, s.OrgID, err)
	}

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateSavedTemplateWithinQuota(ctx, sqlcgen.CreateSavedTemplateWithinQuotaParams{
		OrgID:        s.OrgID,
		TeamID:       s.TeamID,
		OwnerID:      s.OwnerID,
		CreatedBy:    s.CreatedBy,
		Name:         s.Name,
		Description:  s.Description,
		Columns:      columns,
		MaxTemplates: clampInt32(s.MaxTemplates),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("create saved template %q in organisation %s: %w", s.Name, s.OrgID, ErrQuotaExceeded)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create saved template %q in organisation %s: %w: %w", s.Name, s.OrgID, ErrNotFound, err)
	}
	if isCheckViolation(err) {
		return nil, fmt.Errorf("create saved template %q in organisation %s: %w: %w", s.Name, s.OrgID, ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create saved template %q in organisation %s: %w", s.Name, s.OrgID, err)
	}

	return toSavedTemplate(savedTemplateRowFromTable(row))
}

func (r *savedTemplateDAO) Get(ctx context.Context, s spec.SavedTemplateKey, tx ...*sql.Tx) (*model.SavedTemplate, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetSavedTemplate(ctx, sqlcgen.GetSavedTemplateParams{OrgID: s.OrgID, ID: s.ID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get saved template %s of organisation %s: %w", s.ID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get saved template %s of organisation %s: %w", s.ID, s.OrgID, err)
	}

	return toSavedTemplate(savedTemplateRow(row))
}

func (r *savedTemplateDAO) List(ctx context.Context, s spec.SavedTemplateViewer, tx ...*sql.Tx) ([]model.SavedTemplate, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListSavedTemplatesForViewer(ctx, sqlcgen.ListSavedTemplatesForViewerParams{OrgID: s.OrgID, OwnerID: new(s.UserID)})
	if err != nil {
		return nil, fmt.Errorf("list saved templates of organisation %s for user %s: %w", s.OrgID, s.UserID, err)
	}

	templates := make([]model.SavedTemplate, 0, len(rows))
	for _, row := range rows {
		template, err := toSavedTemplate(savedTemplateRow(row))
		if err != nil {
			return nil, err
		}

		templates = append(templates, *template)
	}

	return templates, nil
}

func (r *savedTemplateDAO) Count(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountSavedTemplates(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("count saved templates of organisation %s: %w", orgID, err)
	}

	return int(count), nil
}

func (r *savedTemplateDAO) Delete(ctx context.Context, s spec.SavedTemplateKey, tx ...*sql.Tx) (*model.SavedTemplate, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteSavedTemplate(ctx, sqlcgen.DeleteSavedTemplateParams{OrgID: s.OrgID, ID: s.ID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("delete saved template %s of organisation %s: %w", s.ID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("delete saved template %s of organisation %s: %w", s.ID, s.OrgID, err)
	}

	return toSavedTemplate(savedTemplateRowFromTable(row))
}
