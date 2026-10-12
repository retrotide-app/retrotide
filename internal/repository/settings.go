package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"

	"retrotide/internal/cache"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"
)

type (
	SettingsRepository interface {
		dao.SettingsDAO

		Reconcile(ctx context.Context, s spec.SettingsReconcile, tx ...*sql.Tx) error
	}

	settingsRepository struct {
		dao.SettingsDAO
		cache *cache.Manager
	}
)

const (
	publishAttempts = 3
)

func NewSettingsRepo(settingsDAO dao.SettingsDAO, c *cache.Manager) SettingsRepository {
	return &settingsRepository{SettingsDAO: settingsDAO, cache: c}
}

func (r *settingsRepository) Reconcile(ctx context.Context, s spec.SettingsReconcile, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("reconcile settings: %w", db.ErrInboundTx)
	}

	if len(s.Missing) == 0 {
		return nil
	}

	update := spec.SettingsBulkUpdate{Values: s.Missing, UpdatedBy: s.UpdatedBy}

	if err := r.SettingsDAO.SetMultiple(ctx, update); err != nil {
		return err
	}

	return r.invalidate(ctx, slices.Sorted(maps.Keys(s.Missing))...)
}

func (r *settingsRepository) Get(ctx context.Context, key config.SiteSettingKey, tx ...*sql.Tx) (string, error) {
	load := func(ctx context.Context) (string, error) {
		return r.SettingsDAO.Get(ctx, key, tx...)
	}

	return r.cache.Load(ctx, cache.Setting, load, string(key))
}

func (r *settingsRepository) Set(ctx context.Context, s spec.SettingsUpdate, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("set setting %q: %w", s.Key, db.ErrInboundTx)
	}

	if err := r.SettingsDAO.Set(ctx, s); err != nil {
		return err
	}

	return r.publish(ctx, s.Key)
}

func (r *settingsRepository) SetMultiple(ctx context.Context, s spec.SettingsBulkUpdate, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("set settings: %w", db.ErrInboundTx)
	}

	if err := r.SettingsDAO.SetMultiple(ctx, s); err != nil {
		return err
	}

	return r.publish(ctx, slices.Sorted(maps.Keys(s.Values))...)
}

func (r *settingsRepository) Delete(ctx context.Context, key config.SiteSettingKey, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("delete setting %q: %w", key, db.ErrInboundTx)
	}

	if err := r.SettingsDAO.Delete(ctx, key); err != nil {
		return err
	}

	return r.invalidate(ctx, key)
}

func (r *settingsRepository) publish(ctx context.Context, keys ...config.SiteSettingKey) error {
	ctx = context.WithoutCancel(ctx)

	for _, key := range keys {
		err := r.publishOne(ctx, key)
		if err == nil {
			continue
		}

		delErr := r.cache.Del(ctx, cache.Setting.Key(string(key)))

		return fmt.Errorf("%w: %s: %w", ErrStaleSettingsCache, key, errors.Join(err, delErr))
	}

	return nil
}

func (r *settingsRepository) publishOne(ctx context.Context, key config.SiteSettingKey) error {
	cacheKey := cache.Setting.Key(string(key))

	for range publishAttempts {
		committed, err := r.SettingsDAO.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("read the committed value: %w", err)
		}

		if err := r.cache.Set(ctx, cacheKey, committed, cache.Setting.TTL); err != nil {
			return fmt.Errorf("write the cached copy: %w", err)
		}

		settled, err := r.SettingsDAO.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("confirm the committed value: %w", err)
		}

		if settled == committed {
			return nil
		}
	}

	return errSettingKeptChanging
}

func (r *settingsRepository) invalidate(ctx context.Context, keys ...config.SiteSettingKey) error {
	if len(keys) == 0 {
		return nil
	}

	cacheKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		cacheKeys = append(cacheKeys, cache.Setting.Key(string(key)))
	}

	if err := r.cache.Del(ctx, cacheKeys...); err != nil {
		return fmt.Errorf("%w: %w", ErrStaleSettingsCache, err)
	}

	return nil
}
