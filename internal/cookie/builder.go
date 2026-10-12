package cookie

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/settings"
)

type (
	Lifetime func(ctx context.Context) (time.Duration, error)

	Builder struct {
		settings settings.Service
		name     string
		lifetime Lifetime
	}
)

const (
	hostPrefix    = "__Host-"
	rootPath      = "/"
	expiredMaxAge = -1
	browserMaxAge = 0
	secureScheme  = "https"
)

func NewBuilder(settingsSvc settings.Service, name string, lifetime Lifetime) *Builder {
	return &Builder{settings: settingsSvc, name: name, lifetime: lifetime}
}

func Fixed(lifetime time.Duration) Lifetime {
	return func(context.Context) (time.Duration, error) {
		return lifetime, nil
	}
}

func (b *Builder) Cookie(ctx context.Context, value string) (*http.Cookie, error) {
	secure, err := b.secure(ctx)
	if err != nil {
		return nil, err
	}

	lifetime, err := b.lifetime(ctx)
	if err != nil {
		return nil, err
	}

	return b.build(value, int(lifetime/time.Second), secure), nil
}

func (b *Builder) BrowserCookie(ctx context.Context, value string) (*http.Cookie, error) {
	secure, err := b.secure(ctx)
	if err != nil {
		return nil, err
	}

	return b.build(value, browserMaxAge, secure), nil
}

func (b *Builder) ExpiredCookie(ctx context.Context) (*http.Cookie, error) {
	secure, err := b.secure(ctx)
	if err != nil {
		return nil, err
	}

	return b.build("", expiredMaxAge, secure), nil
}

func (b *Builder) Token(r *http.Request) (string, error) {
	secure, err := b.secure(r.Context())
	if err != nil {
		return "", err
	}

	found, err := r.Cookie(b.nameFor(secure))
	if errors.Is(err, http.ErrNoCookie) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read the %s cookie: %w", b.name, err)
	}

	return found.Value, nil
}

func (b *Builder) secure(ctx context.Context) (bool, error) {
	raw, err := b.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return false, fmt.Errorf("read base_url: %w", err)
	}

	base, err := url.Parse(raw)
	if err != nil {
		return false, fmt.Errorf("parse base_url: %w", err)
	}

	return base.Scheme == secureScheme, nil
}

func (b *Builder) nameFor(secure bool) string {
	if secure {
		return hostPrefix + b.name
	}

	return b.name
}

func (b *Builder) build(value string, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     b.nameFor(secure),
		Value:    value,
		Path:     rootPath,
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
