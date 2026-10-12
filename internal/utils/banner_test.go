package utils

import (
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	failingWriter struct{}
)

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriteFailed
}

func TestWriteBanner_ShowsTheAddressVersionHandlersAndProcess(t *testing.T) {
	// given
	var out strings.Builder
	info := BannerInfo{
		Version: "v1.2.3",
		Addr:    &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4333},
		Routes:  21,
		PID:     4242,
		Startup: 187654 * time.Microsecond,
	}

	// when
	err := WriteBanner(&out, info)

	// then
	require.NoError(t, err)
	banner := out.String()
	assert.Contains(t, banner, "Started in 188ms\n")
	assert.Contains(t, banner, bannerArt[0])
	assert.Contains(t, banner, bannerArt[len(bannerArt)-1])
	assert.Contains(t, banner, "v1.2.3")
	assert.Contains(t, banner, "Listening  http://localhost:4333\n")
	assert.Contains(t, banner, "Handlers   21\n")
	assert.Contains(t, banner, "PID        4242\n")
	assert.Contains(t, banner, "Go         "+runtime.Version()+"\n")
}

func TestWriteBanner_NamesLocalhostForAnUnspecifiedAddress(t *testing.T) {
	// given
	var out strings.Builder
	info := BannerInfo{
		Version: "dev",
		Addr:    &net.TCPAddr{IP: net.IPv6unspecified, Port: 4333},
	}

	// when
	err := WriteBanner(&out, info)

	// then
	require.NoError(t, err)
	assert.Contains(t, out.String(), "http://localhost:4333 (all interfaces)")
}

func TestStartupLabel(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"seconds keep millisecond precision", 1234567 * time.Microsecond, "1.235s"},
		{"milliseconds round to the millisecond", 187654 * time.Microsecond, "188ms"},
		{"under a millisecond shows microseconds", 842345 * time.Nanosecond, "842µs"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := StartupLabel(tc.in)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWriteBanner_ShowsARoutableAddressAsIs(t *testing.T) {
	// given
	var out strings.Builder
	info := BannerInfo{
		Version: "dev",
		Addr:    &net.TCPAddr{IP: net.ParseIP("192.168.1.20"), Port: 4333},
	}

	// when
	err := WriteBanner(&out, info)

	// then
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Listening  http://192.168.1.20:4333\n")
}

func TestWriteBanner_ReturnsTheWriteError(t *testing.T) {
	// given
	info := BannerInfo{
		Version: "dev",
		Addr:    &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4333},
	}

	// when
	err := WriteBanner(failingWriter{}, info)

	// then
	require.ErrorIs(t, err, errWriteFailed)
}
