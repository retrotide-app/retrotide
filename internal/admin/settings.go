package admin

import (
	"context"
	"errors"
	"fmt"
	"html"
	"maps"
	"slices"
	"strings"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/identity"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
)

func (s *service) GetSettings(ctx context.Context) (*dto.AdminSettingsResponse, error) {
	all, err := s.settings.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the site settings: %w", err)
	}

	masked := make(map[string]string, len(all))
	for key, value := range all {
		def, known := config.SettingByKey(key)
		if known && def.Secret && value != "" {
			masked[string(key)] = config.SecretMask
			continue
		}

		masked[string(key)] = value
	}

	return &dto.AdminSettingsResponse{Settings: masked}, nil
}

func (s *service) UpdateSettings(ctx context.Context, update spec.StaffSettingsUpdate) error {
	current, err := s.settings.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("read the site settings: %w", err)
	}

	changed, err := changedSettings(update.Values, current)
	if err != nil {
		return err
	}

	if len(changed) == 0 {
		return nil
	}

	merged := maps.Clone(current)
	maps.Copy(merged, changed)

	if err := config.ValidateSettings(merged); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSetting, err)
	}

	if !s.signInMethodRemains(merged) {
		return ErrNoSignInMethod
	}

	writeErr := s.settings.SetMultiple(ctx, changed, update.ActorID)
	if errors.Is(writeErr, ErrInvalidSetting) {
		return writeErr
	}

	if writeErr != nil && !errors.Is(writeErr, repository.ErrStaleSettingsCache) {
		return fmt.Errorf("update the site settings: %w", writeErr)
	}

	keys := make([]string, 0, len(changed))
	for _, key := range slices.Sorted(maps.Keys(changed)) {
		keys = append(keys, string(key))
	}

	auditErr := s.record(ctx, staffEntry(update.StaffAction, audit.ActionUpdateSettings, audit.TargetSettings, "", map[string]string{"keys": strings.Join(keys, ",")}))

	if writeErr != nil {
		return fmt.Errorf("%w: %w", ErrSettingsCacheStale, errors.Join(writeErr, auditErr))
	}

	return auditErr
}

func (s *service) SendTestEmail(ctx context.Context, action spec.StaffAction) error {
	actor, err := s.lookUp(ctx, action.ActorID)
	if err != nil {
		return err
	}

	siteName, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return fmt.Errorf("read the site name: %w", err)
	}

	message := email.Message{
		To:      actor.Email,
		Subject: "Test email from " + siteName,
		HTML:    fmt.Sprintf("<p>This is a test email from <bdi>%s</bdi>, confirming that its email settings work.</p>", html.EscapeString(siteName)),
	}

	err = s.email.SendTest(ctx, message)
	if errors.Is(err, email.ErrNotConfigured) {
		return ErrEmailUnavailable
	}
	if errors.Is(err, email.ErrFailing) {
		return fmt.Errorf("%w: %w", ErrEmailUnavailable, err)
	}
	if err != nil {
		return fmt.Errorf("send a test email to user %s: %w", action.ActorID, err)
	}

	return s.record(ctx, staffEntry(action, audit.ActionSendTestEmail, audit.TargetSettings, "", nil))
}

func (s *service) signInMethodRemains(merged map[config.SiteSettingKey]string) bool {
	if merged[config.SettingAllowPasswordSignIn.Key] != "false" {
		return true
	}

	if merged[config.SettingSiteMode.Key] != string(config.SiteModePublic) {
		return false
	}

	return slices.ContainsFunc(s.signInProviders, func(provider identity.Provider) bool {
		checker, ok := provider.(identity.ConfigurationChecker)

		return ok && checker.ConfiguredWith(merged)
	})
}

func changedSettings(submitted map[string]string, current map[config.SiteSettingKey]string) (map[config.SiteSettingKey]string, error) {
	changed := make(map[config.SiteSettingKey]string)

	for _, name := range slices.Sorted(maps.Keys(submitted)) {
		value := submitted[name]
		key := config.SiteSettingKey(name)

		def, known := config.SettingByKey(key)
		if !known {
			return nil, fmt.Errorf("%w: %s", ErrUnknownSetting, name)
		}

		if def.Secret && value == config.SecretMask {
			continue
		}

		if err := config.ValidateSettingValue(def, value); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidSetting, err)
		}

		if current[key] == value {
			continue
		}

		changed[key] = value
	}

	return changed, nil
}
