package engines

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/cache/engine"
	"retrotide/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

func newMockValkey(t *testing.T) (*Valkey, *mock.Client) {
	t.Helper()

	client := mock.NewClient(gomock.NewController(t))

	return NewValkeyWithClient(client), client
}

func TestValkeyDisabledWithoutClient(t *testing.T) {
	tests := []struct {
		name   string
		engine *Valkey
	}{
		{name: "nil engine", engine: nil},
		{name: "unconfigured engine", engine: NewValkey()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			v := tt.engine

			// when
			enabled := v.Enabled()

			// then
			assert.False(t, enabled)
		})
	}
}

func TestValkeyEnabledWhenHealthy(t *testing.T) {
	// given
	v, _ := newMockValkey(t)

	// when
	enabled := v.Enabled()

	// then
	assert.Equal(t, engine.KindValkey, v.Kind())
	assert.True(t, enabled)
}

func TestValkeyUnconfiguredOperationsAreNoOps(t *testing.T) {
	// given
	v := NewValkey()
	ctx := t.Context()

	// when
	_, err := v.Get(ctx, "k")

	// then
	require.ErrorIs(t, err, engine.ErrMiss)
	require.NoError(t, v.Set(ctx, "k", []byte("v"), 0))
	require.NoError(t, v.SetMany(ctx, map[string][]byte{"k": []byte("v")}, 0))
	require.NoError(t, v.Del(ctx, "k"))
	require.NoError(t, v.Ping(ctx))
	require.NoError(t, v.Close())
}

func TestValkeyGetReturnsStoredValue(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	client.EXPECT().
		Do(gomock.Any(), mock.Match("GET", "k")).
		Return(mock.Result(mock.ValkeyString("tide"))).
		Times(1)

	// when
	got, err := v.Get(t.Context(), "k")

	// then
	require.NoError(t, err)
	assert.Equal(t, []byte("tide"), got)
}

func TestValkeySetIfAbsentSendsNX(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		want []string
	}{
		{name: "without expiry", ttl: 0, want: []string{"SET", "k", "v", "NX"}},
		{name: "with expiry", ttl: time.Minute, want: []string{"SET", "k", "v", "NX", "PX", "60000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			v, client := newMockValkey(t)

			client.EXPECT().
				Do(gomock.Any(), mock.Match(tt.want...)).
				Return(mock.Result(mock.ValkeyString("OK"))).
				Times(1)

			// when
			err := v.SetIfAbsent(t.Context(), "k", []byte("v"), tt.ttl)

			// then
			require.NoError(t, err)
		})
	}
}

func TestValkeySetIfAbsentTreatsAKeyAlreadyThereAsSuccess(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	client.EXPECT().
		Do(gomock.Any(), mock.Match("SET", "k", "v", "NX")).
		Return(mock.Result(mock.ValkeyNil())).
		Times(1)

	// when
	err := v.SetIfAbsent(t.Context(), "k", []byte("v"), 0)

	// then
	require.NoError(t, err)
	assert.True(t, v.Enabled())
}

func TestValkeyFlushSendsFlushdb(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	client.EXPECT().
		Do(gomock.Any(), mock.Match("FLUSHDB")).
		Return(mock.Result(mock.ValkeyString("OK"))).
		Times(1)

	// when
	err := v.Flush(t.Context())

	// then
	require.NoError(t, err)
}

func TestValkeyStartsANewGenerationWhenItLosesACommand(t *testing.T) {
	// given
	v, client := newMockValkey(t)
	before := v.Generation()

	client.EXPECT().
		Do(gomock.Any(), mock.Match("SET", "k", "v")).
		Return(mock.ErrorResult(errors.New("connection reset"))).
		Times(1)

	// when
	err := v.Set(t.Context(), "k", []byte("v"), 0)

	// then the manager will flush it before trusting it again
	require.Error(t, err)
	assert.Equal(t, before+1, v.Generation())
}

func TestValkeyStartsANewGenerationWhenItsURLChanges(t *testing.T) {
	// given
	v := NewValkey()
	v.dial = func(valkey.ClientOption) (valkey.Client, error) {
		return nil, errors.New("unreachable")
	}
	before := v.Generation()

	// when
	err := v.Reconfigure("redis://127.0.0.1:6391")

	// then
	require.Error(t, err)
	assert.Equal(t, before+1, v.Generation())
}

func TestValkeyTreatsMissingKeyAsHealthyMiss(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	client.EXPECT().
		Do(gomock.Any(), mock.Match("GET", "k")).
		Return(mock.Result(mock.ValkeyNil())).
		Times(1)

	// when
	_, err := v.Get(t.Context(), "k")

	// then
	require.ErrorIs(t, err, engine.ErrMiss)
	assert.True(t, v.Enabled())
}

func TestValkeyDisablesItselfAfterCommandFailure(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	wantErr := errors.New("connection refused")

	client.EXPECT().
		Do(gomock.Any(), mock.Match("GET", "k")).
		Return(mock.ErrorResult(wantErr)).
		Times(1)

	// when
	_, err := v.Get(t.Context(), "k")

	// then
	require.ErrorIs(t, err, wantErr)
	assert.False(t, v.Enabled())
}

func TestValkeyObserveTracksHealth(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantHealthy bool
	}{
		{name: "success keeps it enabled", err: nil, wantHealthy: true},
		{name: "missing key keeps it enabled", err: mock.Result(mock.ValkeyNil()).Error(), wantHealthy: true},
		{name: "command failure disables it", err: errors.New("connection refused"), wantHealthy: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			v, _ := newMockValkey(t)

			// when
			got := v.observe(t.Context(), tt.err)

			// then
			assert.Equal(t, tt.err, got)
			assert.Equal(t, tt.wantHealthy, v.Enabled())
		})
	}
}

func TestValkeyStaysDisabledDuringCooldown(t *testing.T) {
	// given
	v, _ := newMockValkey(t)

	// when
	require.Error(t, v.observe(t.Context(), errors.New("connection refused")))

	// then
	assert.False(t, v.Enabled())
	assert.False(t, v.Enabled())
}

func TestValkeyStaysHealthyWhenTheCallerGoesAway(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "a cancelled request", err: context.Canceled},
		{name: "a request past its deadline", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given a caller whose own context has ended
			v, _ := newMockValkey(t)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			// when
			err := v.observe(ctx, fmt.Errorf("valkey get: %w", tt.err))

			// then one aborted request never pushes the whole process onto the in-memory engine
			require.ErrorIs(t, err, tt.err)
			assert.True(t, v.Enabled())
		})
	}
}

func TestValkeyTreatsItsOwnTimeoutAsAFailure(t *testing.T) {
	// given a caller whose context is still live
	v, _ := newMockValkey(t)

	// when
	err := v.observe(t.Context(), context.DeadlineExceeded)

	// then a timeout the caller did not cause is valkey being slow, so it fails over
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, v.Enabled())
}

func TestValkeyAllowsOneProbeAfterCooldown(t *testing.T) {
	// given
	v, _ := newMockValkey(t)

	require.Error(t, v.observe(t.Context(), errors.New("connection refused")))

	// when
	v.downAt.Store(time.Now().Add(-recoveryInterval - time.Second).UnixNano())

	// then
	assert.True(t, v.Enabled())
	assert.False(t, v.Enabled())
}

func TestValkeyRecoversAfterSuccessfulProbe(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	require.Error(t, v.observe(t.Context(), errors.New("connection refused")))
	require.False(t, v.Enabled())

	v.downAt.Store(time.Now().Add(-recoveryInterval - time.Second).UnixNano())
	require.True(t, v.Enabled())

	client.EXPECT().
		Do(gomock.Any(), mock.Match("GET", "k")).
		Return(mock.Result(mock.ValkeyString("v"))).
		Times(1)

	// when
	got, err := v.Get(t.Context(), "k")

	// then
	require.NoError(t, err)
	assert.Equal(t, []byte("v"), got)

	assert.True(t, v.Enabled())
	assert.True(t, v.Enabled())
}

func TestValkeyFailedProbeRestartsCooldown(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	require.Error(t, v.observe(t.Context(), errors.New("connection refused")))
	v.downAt.Store(time.Now().Add(-recoveryInterval - time.Second).UnixNano())
	require.True(t, v.Enabled())

	client.EXPECT().
		Do(gomock.Any(), mock.Match("GET", "k")).
		Return(mock.ErrorResult(errors.New("still refused"))).
		Times(1)

	// when
	_, err := v.Get(t.Context(), "k")

	// then
	require.Error(t, err)
	assert.False(t, v.Enabled())
}

func TestValkeySetAttachesTTLAsMilliseconds(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
		want []string
	}{
		{name: "with ttl", ttl: time.Minute, want: []string{"SET", "k", "v", "PX", "60000"}},
		{name: "without ttl", ttl: 0, want: []string{"SET", "k", "v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			v, client := newMockValkey(t)

			var captured []string

			client.EXPECT().
				Do(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, cmd valkey.Completed) valkey.ValkeyResult {
					captured = cmd.Commands()

					return mock.Result(mock.ValkeyString("OK"))
				}).
				Times(1)

			// when
			err := v.Set(t.Context(), "k", []byte("v"), tt.ttl)

			// then
			require.NoError(t, err)
			assert.Equal(t, tt.want, captured)
		})
	}
}

func TestValkeySkipsRoundTripWhenNothingToDo(t *testing.T) {
	// given
	v, _ := newMockValkey(t)
	ctx := t.Context()

	// when
	delErr := v.Del(ctx)
	setManyErr := v.SetMany(ctx, nil, 0)

	// then
	require.NoError(t, delErr)
	require.NoError(t, setManyErr)
}

func TestValkeyPingSurfacesAnUnreachableServer(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	wantErr := errors.New("connection refused")

	client.EXPECT().
		Do(gomock.Any(), mock.Match("PING")).
		Return(mock.ErrorResult(wantErr)).
		Times(1)

	// when
	err := v.Ping(t.Context())

	// then
	require.ErrorIs(t, err, wantErr)
	assert.False(t, v.Enabled())
}

func TestValkeyCloseReleasesClientOnce(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	client.EXPECT().Close().Times(1)

	// when
	require.NoError(t, v.Close())

	// then
	assert.False(t, v.Enabled())
	require.NoError(t, v.Close())
}

func TestValkeyReconfigureRejectsInvalidURL(t *testing.T) {
	// given
	v := NewValkey()

	// when
	err := v.Reconfigure("://")

	// then
	require.Error(t, err)
	assert.False(t, v.Enabled())
}

func TestValkeyReconfigureWithEmptyURLStaysDisabled(t *testing.T) {
	// given
	v := NewValkey()

	// when
	err := v.Reconfigure("")

	// then
	require.NoError(t, err)
	assert.False(t, v.Enabled())
}

func TestValkeyIgnoresUnrelatedSettingChanges(t *testing.T) {
	// given
	v, _ := newMockValkey(t)

	// when
	v.OnSettingChanged(config.SettingLogLevel.Key, "debug")

	// then
	assert.True(t, v.Enabled())
}

func TestValkeyClearingURLDisablesEngine(t *testing.T) {
	// given
	v, client := newMockValkey(t)
	v.url = "valkey://localhost:6379"

	client.EXPECT().Close().Times(1)

	// when
	v.OnSettingChanged(config.SettingValkeyURL.Key, "")

	// then
	assert.False(t, v.Enabled())
	assert.Nil(t, v.Client())
}

func TestNewValkeyStartsUnconfigured(t *testing.T) {
	// given
	v := NewValkey()

	// when
	client := v.Client()

	// then
	assert.Nil(t, client)
	assert.False(t, v.Enabled())
}

func TestValkeySetManyDisablesEngineOnFailure(t *testing.T) {
	// given
	v, client := newMockValkey(t)

	wantErr := errors.New("connection refused")

	client.EXPECT().
		DoMulti(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ ...valkey.Completed) []valkey.ValkeyResult {
			return []valkey.ValkeyResult{mock.ErrorResult(wantErr)}
		}).
		Times(1)

	// when
	err := v.SetMany(t.Context(), map[string][]byte{"k": []byte("v")}, 0)

	// then
	require.ErrorIs(t, err, wantErr)
	assert.False(t, v.Enabled())
}

func TestValkeyKeepsAnUnreachableURLAndSaysSoOnPing(t *testing.T) {
	// given valkey is configured but not up yet when the app boots
	v := NewValkey()
	dialErr := errors.New("dial tcp 10.0.0.9:6379: connection refused")
	v.dial = func(valkey.ClientOption) (valkey.Client, error) {
		return nil, dialErr
	}

	// when
	err := v.Reconfigure("redis://10.0.0.9:6379")

	// then the failure is reported, the url is kept for the retry, and the health check says the cache is down
	require.ErrorIs(t, err, dialErr)
	assert.False(t, v.Enabled())

	pingErr := v.Ping(t.Context())
	require.ErrorIs(t, pingErr, errNotConnected)
	assert.ErrorContains(t, pingErr, "10.0.0.9:6379")
}

func TestValkeyReconnectsInTheBackgroundAfterTheCooldown(t *testing.T) {
	// given a boot-time connection failure
	v := NewValkey()
	v.dial = func(valkey.ClientOption) (valkey.Client, error) {
		return nil, errors.New("connection refused")
	}
	require.Error(t, v.Reconfigure("redis://10.0.0.9:6379"))

	client := mock.NewClient(gomock.NewController(t))
	v.dial = func(valkey.ClientOption) (valkey.Client, error) {
		return client, nil
	}
	v.downAt.Store(time.Now().Add(-recoveryInterval - time.Second).UnixNano())

	// when
	enabled := v.Enabled()

	// then no request waits on the dial, and the engine comes back once it lands
	assert.False(t, enabled)
	assert.Eventually(t, v.Enabled, time.Second, 10*time.Millisecond)
	assert.Equal(t, client, v.Client())
}

func TestValkeyWaitsOutTheCooldownBeforeRetryingTheConnection(t *testing.T) {
	// given a boot-time connection failure
	v := NewValkey()
	dials := 0
	v.dial = func(valkey.ClientOption) (valkey.Client, error) {
		dials++

		return nil, errors.New("connection refused")
	}
	require.Error(t, v.Reconfigure("redis://10.0.0.9:6379"))

	// when
	enabled := v.Enabled()

	// then
	assert.False(t, enabled)
	assert.Equal(t, 1, dials)
}

func TestValkeyDropsAReconnectThatLostARaceWithReconfigure(t *testing.T) {
	// given the url changed while an old reconnect was dialling
	v := NewValkey()
	v.url = "redis://new:6379"

	stale := mock.NewClient(gomock.NewController(t))
	stale.EXPECT().Close().Times(1)
	v.dial = func(valkey.ClientOption) (valkey.Client, error) {
		return stale, nil
	}

	// when
	v.reconnect("redis://old:6379", valkey.ClientOption{})

	// then the stale connection is closed rather than adopted
	assert.Nil(t, v.Client())
}

func TestProbeURLValidatesInput(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "empty url is accepted", url: "", wantErr: false},
		{name: "blank url is accepted", url: "   ", wantErr: false},
		{name: "malformed url is rejected", url: "://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			rawURL := tt.url

			// when
			err := ProbeURL(t.Context(), rawURL)

			// then
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}
