package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"

	"retrotide/internal/config"
	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
)

type (
	SettingsDAO interface {
		Get(ctx context.Context, key config.SiteSettingKey, tx ...*sql.Tx) (string, error)
		GetAll(ctx context.Context, tx ...*sql.Tx) (map[config.SiteSettingKey]string, error)
		Set(ctx context.Context, s spec.SettingsUpdate, tx ...*sql.Tx) error
		SetMultiple(ctx context.Context, s spec.SettingsBulkUpdate, tx ...*sql.Tx) error
		Delete(ctx context.Context, key config.SiteSettingKey, tx ...*sql.Tx) error
	}

	settingsDAO struct {
		db *sql.DB
	}
)

func (r *settingsDAO) Get(ctx context.Context, key config.SiteSettingKey, tx ...*sql.Tx) (string, error) {
	value, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetSiteSetting(ctx, string(key))
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("get setting %q: %w", key, ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}

	return value, nil
}

func (r *settingsDAO) GetAll(ctx context.Context, tx ...*sql.Tx) (map[config.SiteSettingKey]string, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetAllSiteSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}

	values := make(map[config.SiteSettingKey]string, len(rows))
	for _, row := range rows {
		values[config.SiteSettingKey(row.Key)] = row.Value
	}

	return values, nil
}

func (r *settingsDAO) Set(ctx context.Context, s spec.SettingsUpdate, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).UpsertSiteSetting(ctx, sqlcgen.UpsertSiteSettingParams{
		Key:       string(s.Key),
		Value:     s.Value,
		UpdatedBy: nullableUUID(s.UpdatedBy),
	})
	if err != nil {
		return fmt.Errorf("set setting %q: %w", s.Key, err)
	}

	return nil
}

func (r *settingsDAO) SetMultiple(ctx context.Context, s spec.SettingsBulkUpdate, tx ...*sql.Tx) error {
	actor := nullableUUID(s.UpdatedBy)
	keys := slices.Sorted(maps.Keys(s.Values))

	return db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		queries := sqlcgen.New(tx)

		for _, key := range keys {
			err := queries.UpsertSiteSetting(ctx, sqlcgen.UpsertSiteSettingParams{
				Key:       string(key),
				Value:     s.Values[key],
				UpdatedBy: actor,
			})
			if err != nil {
				return fmt.Errorf("set setting %q: %w", key, err)
			}
		}

		return nil
	})
}

func (r *settingsDAO) Delete(ctx context.Context, key config.SiteSettingKey, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteSiteSetting(ctx, string(key)); err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}

	return nil
}
