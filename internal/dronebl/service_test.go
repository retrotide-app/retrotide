package dronebl

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	checkerSettings struct {
		enabled   bool
		allowlist string
		ignored   string
	}
)

const (
	listedIP    = "190.114.41.165"
	listedQuery = "165.41.114.190.dnsbl.dronebl.org"
)

func newTestService(t *testing.T, cfg checkerSettings) (*service, *MockResolver) {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingDroneBLEnabled).Return(cfg.enabled, nil).Maybe()
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingDroneBLAllowlist).Return(cfg.allowlist, nil).Maybe()
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingDroneBLIgnoredClasses).Return(cfg.ignored, nil).Maybe()

	resolver := NewMockResolver(t)

	svc, ok := NewService(settingsSvc, resolver).(*service)
	require.True(t, ok)

	return svc, resolver
}

func listedAs(class byte) []netip.Addr {
	return []netip.Addr{netip.AddrFrom4([4]byte{127, 0, 0, class})}
}

func notFound() error {
	return &net.DNSError{Err: "no such host", Name: listedQuery, IsNotFound: true}
}

func TestCheck_BlocksAListedAddress(t *testing.T) {
	// given an address DroneBL answers for
	svc, resolver := newTestService(t, checkerSettings{enabled: true})
	resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(listedAs(13), nil).Once()

	// when
	verdict, err := svc.Check(t.Context(), listedIP)

	// then
	require.NoError(t, err)
	assert.Equal(t, Verdict{Blocked: true, Classes: []int{13}}, verdict)
}

func TestCheck_LetsACleanAddressThrough(t *testing.T) {
	// given DroneBL answers NXDOMAIN, which is how it reports a clean address
	svc, resolver := newTestService(t, checkerSettings{enabled: true})
	resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", "8.8.8.8.dnsbl.dronebl.org").Return(nil, notFound()).Once()

	// when
	verdict, err := svc.Check(t.Context(), "8.8.8.8")

	// then
	require.NoError(t, err)
	assert.Equal(t, Verdict{}, verdict)
}

func TestCheck_QueriesIPv6InNibbleFormat(t *testing.T) {
	// given a listed ipv6 member address
	svc, resolver := newTestService(t, checkerSettings{enabled: true})
	resolver.EXPECT().
		LookupNetIP(mock.Anything, "ip4", "4.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.5.1.0.4.5.1.0.0.7.8.3.0.0.0.6.2.dnsbl.dronebl.org").
		Return(listedAs(17), nil).
		Once()

	// when
	verdict, err := svc.Check(t.Context(), "2600:387:15:4015::4")

	// then
	require.NoError(t, err)
	assert.Equal(t, Verdict{Blocked: true, Classes: []int{17}}, verdict)
}

func TestCheck_DoesNothingWhileDisabled(t *testing.T) {
	// given the default settings, and no expectation for the allowlist or the resolver
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingDroneBLEnabled).Return(false, nil).Once()
	svc := NewService(settingsSvc, NewMockResolver(t))

	// when
	verdict, err := svc.Check(t.Context(), listedIP)

	// then nothing else was read or asked, which the strict mocks enforce
	require.NoError(t, err)
	assert.Equal(t, Verdict{}, verdict)
}

func TestCheck_NeverSendsAnUnroutableAddressToDroneBL(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{name: "private", ip: "10.0.0.4"},
		{name: "loopback", ip: "127.0.0.1"},
		{name: "link local", ip: "fe80::1"},
		{name: "junk", ip: "not-an-ip"},
		{name: "empty", ip: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a resolver with no expectation, so any lookup fails the test
			svc, _ := newTestService(t, checkerSettings{enabled: true})

			// when
			verdict, err := svc.Check(t.Context(), tc.ip)

			// then
			require.NoError(t, err)
			assert.Equal(t, Verdict{}, verdict)
		})
	}
}

func TestCheck_NeverLooksUpAnAllowlistedAddress(t *testing.T) {
	// given an operator who rescued their own range, and a resolver with no expectation
	svc, _ := newTestService(t, checkerSettings{enabled: true, allowlist: "190.114.41.0/24"})

	// when
	verdict, err := svc.Check(t.Context(), listedIP)

	// then
	require.NoError(t, err)
	assert.Equal(t, Verdict{}, verdict)
}

func TestCheck_HonoursIgnoredClasses(t *testing.T) {
	tests := []struct {
		name    string
		answers []netip.Addr
		ignored string
		want    bool
	}{
		{name: "no ignore list blocks any listing", answers: listedAs(13), ignored: "", want: true},
		{name: "the ignored class does not block", answers: listedAs(13), ignored: "13", want: false},
		{name: "another class still blocks", answers: listedAs(9), ignored: "13", want: true},
		{name: "several ignored classes", answers: listedAs(18), ignored: "13, 18", want: false},
		{name: "one class that is not ignored is enough to block", answers: append(listedAs(13), listedAs(9)...), ignored: "13", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, resolver := newTestService(t, checkerSettings{enabled: true, ignored: tc.ignored})
			resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(tc.answers, nil).Once()

			// when
			verdict, err := svc.Check(t.Context(), listedIP)

			// then
			require.NoError(t, err)
			assert.NotEmpty(t, verdict.Classes)
			assert.Equal(t, tc.want, verdict.Blocked)
		})
	}
}

func TestCheck_SkipsJunkEntriesButKeepsTheRest(t *testing.T) {
	tests := []struct {
		name      string
		allowlist string
		ignored   string
		lookedUp  bool
	}{
		{name: "an allowlist with junk still allowlists the sane entry", allowlist: "not-a-cidr, 190.114.41.165", lookedUp: false},
		{name: "an ignore list with junk still ignores the sane class", ignored: "nonsense, 13", lookedUp: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, resolver := newTestService(t, checkerSettings{enabled: true, allowlist: tc.allowlist, ignored: tc.ignored})
			if tc.lookedUp {
				resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(listedAs(13), nil).Once()
			}

			// when
			verdict, err := svc.Check(t.Context(), listedIP)

			// then
			require.NoError(t, err)
			assert.False(t, verdict.Blocked)
		})
	}
}

func TestCheck_ReportsASettingItCannotRead(t *testing.T) {
	tests := []struct {
		name    string
		failing *config.SiteSettingDef
	}{
		{name: "the switch", failing: config.SettingDroneBLEnabled},
		{name: "the allowlist", failing: config.SettingDroneBLAllowlist},
		{name: "the ignored classes", failing: config.SettingDroneBLIgnoredClasses},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			settingsSvc := settings.NewMockService(t)
			if tc.failing == config.SettingDroneBLEnabled {
				settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingDroneBLEnabled).Return(false, errSettingsDown).Once()
			} else {
				settingsSvc.EXPECT().GetBool(mock.Anything, config.SettingDroneBLEnabled).Return(true, nil).Once()
			}

			for _, def := range []*config.SiteSettingDef{config.SettingDroneBLAllowlist, config.SettingDroneBLIgnoredClasses} {
				if def == tc.failing {
					settingsSvc.EXPECT().Get(mock.Anything, def).Return("", errSettingsDown).Maybe()
				} else {
					settingsSvc.EXPECT().Get(mock.Anything, def).Return("", nil).Maybe()
				}
			}

			resolver := NewMockResolver(t)
			resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(listedAs(13), nil).Maybe()

			svc := NewService(settingsSvc, resolver)

			// when
			_, err := svc.Check(t.Context(), listedIP)

			// then a failed read is never taken to mean the address is clean
			assert.ErrorIs(t, err, errSettingsDown)
		})
	}
}

func TestCheck_CachesAVerdictForADay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given an address that is listed now and delisted a day later
		svc, resolver := newTestService(t, checkerSettings{enabled: true})
		resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(listedAs(9), nil).Once()

		// when it is checked twice, then again after a day
		first, firstErr := svc.Check(t.Context(), listedIP)
		time.Sleep(listingTTL - time.Second)
		second, secondErr := svc.Check(t.Context(), listedIP)

		resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(nil, notFound()).Once()
		time.Sleep(time.Second)
		third, thirdErr := svc.Check(t.Context(), listedIP)

		// then the resolver is asked once a day, which Once() enforces
		require.NoError(t, errors.Join(firstErr, secondErr, thirdErr))
		assert.True(t, first.Blocked)
		assert.True(t, second.Blocked)
		assert.False(t, third.Blocked)
	})
}

func TestCheck_FailsOpenForFiveMinutesWhenTheLookupFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given the resolver itself is broken, not the address
		svc, resolver := newTestService(t, checkerSettings{enabled: true})
		resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(nil, errDNSBroken).Once()

		// when it is checked during the outage, then after the retry delay
		during, duringErr := svc.Check(t.Context(), listedIP)
		time.Sleep(failureTTL - time.Second)
		stillDuring, stillDuringErr := svc.Check(t.Context(), listedIP)

		resolver.EXPECT().LookupNetIP(mock.Anything, "ip4", listedQuery).Return(listedAs(13), nil).Once()
		time.Sleep(time.Second)
		after, afterErr := svc.Check(t.Context(), listedIP)

		// then an unreachable blocklist never locks the site down, and is retried after five minutes
		require.NoError(t, errors.Join(duringErr, stillDuringErr, afterErr))
		assert.False(t, during.Blocked)
		assert.False(t, stillDuring.Blocked)
		assert.True(t, after.Blocked)
	})
}

func TestCheck_ConcurrentChecksOfOneAddressShareOneLookup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a lookup that is held until every check has joined it
		const checks = 5

		release := make(chan struct{})
		svc, resolver := newTestService(t, checkerSettings{enabled: true})
		resolver.EXPECT().
			LookupNetIP(mock.Anything, "ip4", listedQuery).
			RunAndReturn(func(context.Context, string, string) ([]netip.Addr, error) {
				<-release

				return listedAs(13), nil
			}).
			Once()

		var (
			wg       sync.WaitGroup
			mu       sync.Mutex
			verdicts []Verdict
			errs     []error
		)

		// when
		for range checks {
			wg.Go(func() {
				verdict, err := svc.Check(t.Context(), listedIP)

				mu.Lock()
				defer mu.Unlock()

				verdicts = append(verdicts, verdict)
				errs = append(errs, err)
			})
		}

		synctest.Wait()
		close(release)
		wg.Wait()

		// then the resolver was asked once, which Once() enforces
		require.NoError(t, errors.Join(errs...))
		require.Len(t, verdicts, checks)
		for _, verdict := range verdicts {
			assert.True(t, verdict.Blocked)
		}
	})
}

func TestRemember_BoundsTheCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name         string
			shortLived   int
			wantAfterAdd int
		}{
			{name: "expired entries are dropped first", shortLived: maxListings / 2, wantAfterAdd: maxListings/2 + 1},
			{name: "a cache full of live entries starts again", shortLived: 0, wantAfterAdd: 1},
		}

		for _, tc := range tests {
			// given a full cache, part of it past its expiry
			svc, _ := newTestService(t, checkerSettings{enabled: true})
			for i := range maxListings {
				ttl := time.Hour
				if i < tc.shortLived {
					ttl = time.Minute
				}

				svc.remember(netip.AddrFrom4([4]byte{198, 18, byte(i >> 8), byte(i)}), nil, ttl)
			}
			require.Len(t, svc.listings, maxListings, tc.name)
			time.Sleep(2 * time.Minute)

			// when one more address is remembered
			svc.remember(netip.MustParseAddr(listedIP), []int{13}, listingTTL)

			// then
			assert.Len(t, svc.listings, tc.wantAfterAdd, tc.name)
			assert.Contains(t, svc.listings, netip.MustParseAddr(listedIP), tc.name)
		}
	})
}
