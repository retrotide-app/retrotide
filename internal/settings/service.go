package settings

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"sync"

	"retrotide/internal/config"
	"retrotide/internal/logger"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Listener interface {
		OnSettingChanged(key config.SiteSettingKey, value string)
	}

	BatchListener interface {
		OnSettingsBatchChanged(keys []config.SiteSettingKey)
	}

	Validator func(ctx context.Context, value string) error

	Service interface {
		Get(ctx context.Context, def *config.SiteSettingDef) (string, error)
		GetInt(ctx context.Context, def *config.SiteSettingDef) (int, error)
		GetBool(ctx context.Context, def *config.SiteSettingDef) (bool, error)
		GetAll(ctx context.Context) (map[config.SiteSettingKey]string, error)
		Set(ctx context.Context, setting *config.SiteSettingDef, value string, updatedBy uuid.UUID) error
		SetMultiple(ctx context.Context, values map[config.SiteSettingKey]string, updatedBy uuid.UUID) error
		Subscribe(listener Listener)
		SubscribeBatch(listener BatchListener)
		RegisterValidator(setting *config.SiteSettingDef, validate Validator)
		Refresh(ctx context.Context) error
	}

	service struct {
		repo           repository.SettingsRepository
		listeners      []Listener
		batchListeners []BatchListener
		listenerMu     sync.RWMutex
		validators     map[config.SiteSettingKey]Validator
		validatorMu    sync.RWMutex
	}
)

func NewService(repo repository.SettingsRepository) Service {
	return &service{repo: repo}
}

func (s *service) Subscribe(listener Listener) {
	s.listenerMu.Lock()
	defer s.listenerMu.Unlock()

	s.listeners = append(s.listeners, listener)
}

func (s *service) SubscribeBatch(listener BatchListener) {
	s.listenerMu.Lock()
	defer s.listenerMu.Unlock()

	s.batchListeners = append(s.batchListeners, listener)
}

func (s *service) RegisterValidator(setting *config.SiteSettingDef, validate Validator) {
	s.validatorMu.Lock()
	defer s.validatorMu.Unlock()

	if s.validators == nil {
		s.validators = make(map[config.SiteSettingKey]Validator)
	}

	s.validators[setting.Key] = validate
}

func (s *service) validatorFor(key config.SiteSettingKey) Validator {
	s.validatorMu.RLock()
	defer s.validatorMu.RUnlock()

	return s.validators[key]
}

func (s *service) validateChanged(ctx context.Context, changed map[config.SiteSettingKey]string) error {
	for key, value := range changed {
		def, ok := config.SettingByKey(key)
		if !ok {
			return fmt.Errorf("unknown setting: %s", key)
		}

		if err := config.ValidateSettingValue(def, value); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidValue, err)
		}

		validate := s.validatorFor(key)
		if validate == nil {
			continue
		}

		if err := validate(ctx, value); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrInvalidValue, key, err)
		}
	}

	return nil
}

func (s *service) notify(key config.SiteSettingKey, value string) {
	s.listenerMu.RLock()
	defer s.listenerMu.RUnlock()

	for _, l := range s.listeners {
		l.OnSettingChanged(key, value)
	}
}

func (s *service) notifyBatch(keys []config.SiteSettingKey) {
	s.listenerMu.RLock()
	defer s.listenerMu.RUnlock()

	for _, l := range s.batchListeners {
		l.OnSettingsBatchChanged(keys)
	}
}

func (s *service) Refresh(ctx context.Context) error {
	existing, err := s.repo.GetAll(ctx)
	if err != nil {
		return err
	}

	missing := make(map[config.SiteSettingKey]string)
	for _, def := range config.AllSiteSettings {
		if _, ok := existing[def.Key]; !ok {
			missing[def.Key] = def.Default
		}
	}

	if len(missing) > 0 {
		if err := s.repo.Reconcile(ctx, spec.SettingsReconcile{Missing: missing, UpdatedBy: uuid.Nil}); err != nil {
			return err
		}

		logger.Ctx(ctx).Info().Int("count", len(missing)).Msg("seeded missing settings with defaults")
	}

	logger.Ctx(ctx).Debug().Msg("settings reconciled")

	return nil
}

func (s *service) Get(ctx context.Context, def *config.SiteSettingDef) (string, error) {
	value, err := s.repo.Get(ctx, def.Key)
	if err != nil {
		return "", fmt.Errorf("read setting %s: %w", def.Key, err)
	}

	return value, nil
}

func (s *service) GetInt(ctx context.Context, def *config.SiteSettingDef) (int, error) {
	value, err := s.Get(ctx, def)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("setting %s is not a whole number: %w", def.Key, err)
	}

	return parsed, nil
}

func (s *service) GetBool(ctx context.Context, def *config.SiteSettingDef) (bool, error) {
	value, err := s.Get(ctx, def)
	if err != nil {
		return false, err
	}

	return value == "true", nil
}

func (s *service) GetAll(ctx context.Context) (map[config.SiteSettingKey]string, error) {
	persisted, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("read stored settings: %w", err)
	}

	all := make(map[config.SiteSettingKey]string, len(config.AllSiteSettings))
	for _, def := range config.AllSiteSettings {
		value, ok := persisted[def.Key]
		if !ok {
			value = def.Default
		}

		all[def.Key] = value
	}

	return all, nil
}

func (s *service) Set(ctx context.Context, setting *config.SiteSettingDef, value string, updatedBy uuid.UUID) error {
	merged, err := s.GetAll(ctx)
	if err != nil {
		return err
	}

	changed := merged[setting.Key] != value
	merged[setting.Key] = value

	if err := config.ValidateSettings(merged); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidValue, err)
	}

	if changed {
		if err := s.validateChanged(ctx, map[config.SiteSettingKey]string{setting.Key: value}); err != nil {
			return err
		}
	}

	writeErr := s.repo.Set(ctx, spec.SettingsUpdate{Key: setting.Key, Value: value, UpdatedBy: updatedBy})
	if writeErr != nil && !errors.Is(writeErr, repository.ErrStaleSettingsCache) {
		return writeErr
	}

	s.notify(setting.Key, value)

	logger.Ctx(ctx).Info().Str("key", string(setting.Key)).Str("updated_by", updatedBy.String()).Msg("setting updated")

	return writeErr
}

func (s *service) SetMultiple(ctx context.Context, values map[config.SiteSettingKey]string, updatedBy uuid.UUID) error {
	merged, err := s.GetAll(ctx)
	if err != nil {
		return err
	}

	keys := make([]config.SiteSettingKey, 0, len(values))
	changed := make(map[config.SiteSettingKey]string)

	for k, v := range values {
		if _, ok := config.SettingByKey(k); !ok {
			return fmt.Errorf("unknown setting: %s", k)
		}

		keys = append(keys, k)

		if merged[k] != v {
			changed[k] = v
		}
	}

	maps.Copy(merged, values)

	if err := config.ValidateSettings(merged); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidValue, err)
	}

	if err := s.validateChanged(ctx, changed); err != nil {
		return err
	}

	writeErr := s.repo.SetMultiple(ctx, spec.SettingsBulkUpdate{Values: values, UpdatedBy: updatedBy})
	if writeErr != nil && !errors.Is(writeErr, repository.ErrStaleSettingsCache) {
		return writeErr
	}

	for k, v := range values {
		s.notify(k, v)
	}

	s.notifyBatch(keys)

	logger.Ctx(ctx).Info().Int("count", len(values)).Str("updated_by", updatedBy.String()).Msg("settings updated")

	return writeErr
}
