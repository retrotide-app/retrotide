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
)

type (
	OIDCFlowDAO interface {
		Create(ctx context.Context, s spec.NewOIDCFlow, tx ...*sql.Tx) error
		Consume(ctx context.Context, s spec.OIDCFlowKey, tx ...*sql.Tx) (*model.OIDCFlow, error)
		CleanExpired(ctx context.Context, tx ...*sql.Tx) (int64, error)
	}

	oidcFlowDAO struct {
		db *sql.DB
	}
)

func (r *oidcFlowDAO) Create(ctx context.Context, s spec.NewOIDCFlow, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateOIDCFlow(ctx, sqlcgen.CreateOIDCFlowParams{
		TokenHash:        s.TokenHash,
		Provider:         string(s.Provider),
		Purpose:          string(s.Purpose),
		State:            s.State,
		Nonce:            s.Nonce,
		CodeVerifier:     s.CodeVerifier,
		UserID:           nullableUUID(s.UserID),
		SessionTokenHash: textOrNull(s.SessionTokenHash),
		ReturnTo:         textOrNull(s.ReturnTo),
		ExpiresAt:        s.ExpiresAt,
		ThenLink:         textOrNull(string(s.ThenLink)),
	})
	if isUniqueViolation(err) {
		return fmt.Errorf("start %s %s flow: %w: %w", s.Provider, s.Purpose, ErrConflict, err)
	}
	if err != nil {
		return fmt.Errorf("start %s %s flow: %w", s.Provider, s.Purpose, err)
	}

	return nil
}

func (r *oidcFlowDAO) Consume(ctx context.Context, s spec.OIDCFlowKey, tx ...*sql.Tx) (*model.OIDCFlow, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ConsumeOIDCFlow(ctx, sqlcgen.ConsumeOIDCFlowParams{
		TokenHash: s.TokenHash,
		Provider:  string(s.Provider),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("consume %s flow: %w", s.Provider, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consume %s flow: %w", s.Provider, err)
	}

	return &model.OIDCFlow{
		TokenHash:        row.TokenHash,
		Provider:         model.IdentityProvider(row.Provider),
		Purpose:          model.OIDCFlowPurpose(row.Purpose),
		State:            row.State,
		Nonce:            row.Nonce,
		CodeVerifier:     row.CodeVerifier,
		UserID:           row.UserID,
		SessionTokenHash: nullableString(row.SessionTokenHash),
		ReturnTo:         nullableString(row.ReturnTo),
		ThenLink:         nullableProvider(row.ThenLink),
		CreatedAt:        row.CreatedAt,
		ExpiresAt:        row.ExpiresAt,
	}, nil
}

func nullableProvider(value sql.NullString) *model.IdentityProvider {
	if !value.Valid {
		return nil
	}

	return new(model.IdentityProvider(value.String))
}

func (r *oidcFlowDAO) CleanExpired(ctx context.Context, tx ...*sql.Tx) (int64, error) {
	removed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteExpiredOIDCFlows(ctx)
	if err != nil {
		return 0, fmt.Errorf("clean expired sign-in flows: %w", err)
	}

	return removed, nil
}
