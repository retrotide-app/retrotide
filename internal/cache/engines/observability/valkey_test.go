package observability

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

func newMockClient(t *testing.T) *mock.Client {
	t.Helper()

	return mock.NewClient(gomock.NewController(t))
}

func TestRegisterStatsReportsADuplicateRegistration(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()
	unconfigured := func() valkey.Client {
		return nil
	}

	require.NoError(t, RegisterStats(reg, unconfigured))

	// when
	err := RegisterStats(reg, unconfigured)

	// then
	_, isDuplicate := errors.AsType[prometheus.AlreadyRegisteredError](err)
	assert.True(t, isDuplicate)
	assert.ErrorContains(t, err, "register valkey stats collector")
}

func TestStatsCollectorReportsDownWithoutAClient(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()
	unconfigured := func() valkey.Client {
		return nil
	}

	require.NoError(t, RegisterStats(reg, unconfigured))

	// when
	err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP cache_up Whether the Valkey cache is configured and reachable (1) or not (0).
# TYPE cache_up gauge
cache_up 0
`))

	// then
	require.NoError(t, err)
}

func TestStatsCollectorReportsDownWhenDbsizeFails(t *testing.T) {
	// given
	client := newMockClient(t)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("DBSIZE")).
		Return(mock.ErrorResult(errors.New("connection refused"))).
		Times(1)

	reg := prometheus.NewRegistry()
	require.NoError(t, RegisterStats(reg, func() valkey.Client {
		return client
	}))

	// when
	err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP cache_up Whether the Valkey cache is configured and reachable (1) or not (0).
# TYPE cache_up gauge
cache_up 0
`))

	// then
	require.NoError(t, err)
}

func TestStatsCollectorReadsServerInfo(t *testing.T) {
	// given
	client := newMockClient(t)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("DBSIZE")).
		Return(mock.Result(mock.ValkeyInt64(42))).
		Times(1)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("INFO", "memory", "stats", "clients")).
		Return(mock.Result(mock.ValkeyBlobString("# Memory\r\nused_memory:1024\r\nmaxmemory:268435456\r\n# Clients\r\nconnected_clients:3\r\n"))).
		Times(1)

	reg := prometheus.NewRegistry()
	require.NoError(t, RegisterStats(reg, func() valkey.Client {
		return client
	}))

	// when
	err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP cache_up Whether the Valkey cache is configured and reachable (1) or not (0).
# TYPE cache_up gauge
cache_up 1
# HELP cache_server_keys Number of keys stored in the Valkey cache database.
# TYPE cache_server_keys gauge
cache_server_keys 42
# HELP cache_server_memory_used_bytes Bytes of memory used by the Valkey server.
# TYPE cache_server_memory_used_bytes gauge
cache_server_memory_used_bytes 1024
# HELP cache_server_memory_max_bytes Configured Valkey maxmemory limit in bytes (0 means unlimited).
# TYPE cache_server_memory_max_bytes gauge
cache_server_memory_max_bytes 2.68435456e+08
# HELP cache_server_connected_clients Number of client connections currently open on the Valkey server.
# TYPE cache_server_connected_clients gauge
cache_server_connected_clients 3
`))

	// then
	require.NoError(t, err)
}

func TestStatsCollectorReportsAFailedInfoAsAnInvalidMetric(t *testing.T) {
	// given
	wantErr := errors.New("info timed out")

	client := newMockClient(t)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("DBSIZE")).
		Return(mock.Result(mock.ValkeyInt64(42))).
		Times(1)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("INFO", "memory", "stats", "clients")).
		Return(mock.ErrorResult(wantErr)).
		Times(1)

	reg := prometheus.NewRegistry()
	require.NoError(t, RegisterStats(reg, func() valkey.Client {
		return client
	}))

	// when
	_, err := reg.Gather()

	// then
	require.ErrorIs(t, err, wantErr)
	assert.ErrorContains(t, err, "read valkey info")
}

func TestHookCountsFailedCommands(t *testing.T) {
	// given
	client := newMockClient(t)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("GET", "k")).
		Return(mock.ErrorResult(errors.New("connection refused"))).
		Times(1)

	before := testutil.ToFloat64(commandErrors.WithLabelValues("GET"))

	// when
	resp := NewHook().Do(client, t.Context(), client.B().Get().Key("k").Build())

	// then
	require.Error(t, resp.Error())
	assert.Equal(t, before+1, testutil.ToFloat64(commandErrors.WithLabelValues("GET")))
}

func TestHookDoesNotCountAMissingKeyAsAFailure(t *testing.T) {
	// given
	client := newMockClient(t)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("GET", "k")).
		Return(mock.Result(mock.ValkeyNil())).
		Times(1)

	before := testutil.ToFloat64(commandErrors.WithLabelValues("GET"))

	// when
	resp := NewHook().Do(client, t.Context(), client.B().Get().Key("k").Build())

	// then
	assert.True(t, valkey.IsValkeyNil(resp.Error()))
	assert.Equal(t, before, testutil.ToFloat64(commandErrors.WithLabelValues("GET")))
}

func TestFirstErrorSkipsMissingKeys(t *testing.T) {
	wantErr := errors.New("connection refused")

	tests := []struct {
		name  string
		resps []valkey.ValkeyResult
		want  error
	}{
		{name: "no responses", resps: nil, want: nil},
		{name: "only successes and misses", resps: []valkey.ValkeyResult{mock.Result(mock.ValkeyString("OK")), mock.Result(mock.ValkeyNil())}, want: nil},
		{name: "a failure after a miss", resps: []valkey.ValkeyResult{mock.Result(mock.ValkeyNil()), mock.ErrorResult(wantErr)}, want: wantErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			resps := tt.resps

			// when
			got := firstError(resps)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInfoIntReadsNamedFields(t *testing.T) {
	info := "# Memory\r\nused_memory:1024\r\nused_memory_human:1.00K\r\nmaxmemory:bogus\r\n"

	tests := []struct {
		name    string
		field   string
		want    float64
		wantOK  bool
		wantErr error
	}{
		{name: "present field", field: "used_memory", want: 1024, wantOK: true},
		{name: "absent field", field: "evicted_keys", want: 0, wantOK: false},
		{name: "unparsable field", field: "maxmemory", want: 0, wantOK: false, wantErr: strconv.ErrSyntax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			field := tt.field

			// when
			got, ok, err := infoInt(info, field)

			// then
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantOK, ok)
			assert.InDelta(t, tt.want, got, 0)
		})
	}
}

func TestStatsCollectorReportsAnUnparsableInfoFieldAsAnInvalidMetric(t *testing.T) {
	// given
	client := newMockClient(t)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("DBSIZE")).
		Return(mock.Result(mock.ValkeyInt64(42))).
		Times(1)
	client.EXPECT().
		Do(gomock.Any(), mock.Match("INFO", "memory", "stats", "clients")).
		Return(mock.Result(mock.ValkeyBlobString("# Memory\r\nused_memory:1024\r\nmaxmemory:bogus\r\n"))).
		Times(1)

	reg := prometheus.NewRegistry()
	require.NoError(t, RegisterStats(reg, func() valkey.Client {
		return client
	}))

	// when
	_, err := reg.Gather()

	// then
	require.ErrorIs(t, err, strconv.ErrSyntax)
	assert.ErrorContains(t, err, "parse valkey info maxmemory")
}
