package dronebl

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/logger"
	"retrotide/internal/settings"

	"golang.org/x/sync/singleflight"
)

type (
	Resolver interface {
		LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
	}

	Service interface {
		Check(ctx context.Context, ip string) (Verdict, error)
	}

	Verdict struct {
		Blocked bool
		Classes []int
	}

	service struct {
		settings settings.Service
		resolver Resolver
		lookups  singleflight.Group
		mu       sync.Mutex
		listings map[netip.Addr]cachedListing
	}

	cachedListing struct {
		classes []int
		expires time.Time
	}
)

const (
	lookupTimeout = 2 * time.Second
	listingTTL    = 24 * time.Hour
	failureTTL    = 5 * time.Minute
	maxListings   = 50_000
)

var (
	_ Resolver = net.DefaultResolver
)

func NewService(settingsSvc settings.Service, resolver Resolver) Service {
	return &service{
		settings: settingsSvc,
		resolver: resolver,
		listings: make(map[netip.Addr]cachedListing),
	}
}

func (s *service) Check(ctx context.Context, ip string) (Verdict, error) {
	enabled, err := s.settings.GetBool(ctx, config.SettingDroneBLEnabled)
	if err != nil {
		return Verdict{}, err
	}

	if !enabled {
		return Verdict{}, nil
	}

	addr, ok := routable(ip)
	if !ok {
		return Verdict{}, nil
	}

	rawAllowlist, err := s.settings.Get(ctx, config.SettingDroneBLAllowlist)
	if err != nil {
		return Verdict{}, err
	}

	allowlist, err := parseAllowlist(rawAllowlist)
	if err != nil {
		logger.Ctx(ctx).Error().Err(err).Msg("dronebl_allowlist has entries that were skipped")
	}

	if allowlisted(allowlist, addr) {
		return Verdict{}, nil
	}

	classes, err := s.listing(ctx, addr)
	if err != nil {
		return Verdict{}, err
	}

	if len(classes) == 0 {
		return Verdict{}, nil
	}

	rawIgnored, err := s.settings.Get(ctx, config.SettingDroneBLIgnoredClasses)
	if err != nil {
		return Verdict{}, err
	}

	ignored, err := parseIgnoredClasses(rawIgnored)
	if err != nil {
		logger.Ctx(ctx).Error().Err(err).Msg("dronebl_ignored_classes has entries that were skipped")
	}

	blocked := slices.ContainsFunc(classes, func(class int) bool {
		_, skip := ignored[class]

		return !skip
	})

	return Verdict{Blocked: blocked, Classes: classes}, nil
}

func (s *service) listing(ctx context.Context, addr netip.Addr) ([]int, error) {
	if classes, ok := s.cached(addr, time.Now()); ok {
		return classes, nil
	}

	found, err, _ := s.lookups.Do(addr.String(), func() (any, error) {
		return s.lookup(context.WithoutCancel(ctx), addr), nil
	})
	if err != nil {
		return nil, fmt.Errorf("dronebl lookup of %s: %w", addr, err)
	}

	classes, _ := found.([]int)

	return classes, nil
}

func (s *service) lookup(ctx context.Context, addr netip.Addr) []int {
	lookupCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()

	answers, err := s.resolver.LookupNetIP(lookupCtx, "ip4", queryName(addr))
	if dnsErr, isDNSErr := errors.AsType[*net.DNSError](err); isDNSErr && dnsErr.IsNotFound {
		s.remember(addr, nil, listingTTL)

		return nil
	}

	if err != nil {
		logger.Ctx(ctx).Warn().Err(err).Str("ip", addr.String()).Dur("retry_after", failureTTL).Msg("dronebl lookup failed, so the address counts as clean until the lookup is retried")
		s.remember(addr, nil, failureTTL)

		return nil
	}

	classes := classesFrom(answers)
	s.remember(addr, classes, listingTTL)

	return classes
}

func (s *service) cached(addr netip.Addr, now time.Time) ([]int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.listings[addr]
	if !ok || !now.Before(entry.expires) {
		return nil, false
	}

	return entry.classes, true
}

func (s *service) remember(addr netip.Addr, classes []int, ttl time.Duration) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.listings) >= maxListings {
		maps.DeleteFunc(s.listings, func(_ netip.Addr, entry cachedListing) bool {
			return !now.Before(entry.expires)
		})
	}

	if len(s.listings) >= maxListings {
		clear(s.listings)
	}

	s.listings[addr] = cachedListing{classes: classes, expires: now.Add(ttl)}
}
