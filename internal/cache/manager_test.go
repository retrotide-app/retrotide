package cache

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"retrotide/internal/cache/engine"
	"retrotide/internal/cache/engines"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
	valkeymock "github.com/valkey-io/valkey-go/mock"
	"go.uber.org/mock/gomock"
)

func newMockManager(t *testing.T) (*Manager, *valkeymock.Client) {
	t.Helper()

	client := valkeymock.NewClient(gomock.NewController(t))
	client.EXPECT().
		Do(gomock.Any(), valkeymock.Match("FLUSHDB")).
		Return(valkeymock.Result(valkeymock.ValkeyString("OK"))).
		AnyTimes()

	m := NewManager(engines.NewValkeyWithClient(client))

	return m, client
}

func expectActivation(e *engine.MockEngine) {
	e.EXPECT().Generation().Return(uint64(0))
	e.EXPECT().Flush(mock.Anything).Return(nil).Once()
}

func okResults(n int) []valkey.ValkeyResult {
	resps := make([]valkey.ValkeyResult, n)
	for i := range resps {
		resps[i] = valkeymock.Result(valkeymock.ValkeyString("OK"))
	}

	return resps
}

func TestSetManySendsEveryKeyInOneBatch(t *testing.T) {
	// given
	m, client := newMockManager(t)

	var captured [][]string

	client.EXPECT().
		DoMulti(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmds ...valkey.Completed) []valkey.ValkeyResult {
			for _, cmd := range cmds {
				captured = append(captured, cmd.Commands())
			}

			return okResults(len(cmds))
		}).
		Times(1)

	// when
	err := m.SetMany(t.Context(), map[string]string{"a": "1", "b": "2", "c": "3"}, 0)

	// then
	require.NoError(t, err)

	slices.SortFunc(captured, slices.Compare)

	assert.Equal(t, [][]string{
		{"SET", "a", "1"},
		{"SET", "b", "2"},
		{"SET", "c", "3"},
	}, captured)
}

func TestSetManyAttachesTTLAsMilliseconds(t *testing.T) {
	// given
	m, client := newMockManager(t)

	var captured []string

	client.EXPECT().
		DoMulti(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmds ...valkey.Completed) []valkey.ValkeyResult {
			captured = cmds[0].Commands()

			return okResults(len(cmds))
		}).
		Times(1)

	// when
	err := m.SetMany(t.Context(), map[string]string{"k": "v"}, time.Minute)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"SET", "k", "v", "PX", "60000"}, captured)
}

func TestSetManyOmitsTTLWhenZero(t *testing.T) {
	// given
	m, client := newMockManager(t)

	var captured []string

	client.EXPECT().
		DoMulti(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmds ...valkey.Completed) []valkey.ValkeyResult {
			captured = cmds[0].Commands()

			return okResults(len(cmds))
		}).
		Times(1)

	// when
	err := m.SetMany(t.Context(), map[string]string{"k": "v"}, 0)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"SET", "k", "v"}, captured)
	assert.NotContains(t, captured, "PX")
}

func TestSetManyEncodesStructsAsJSON(t *testing.T) {
	// given
	m, client := newMockManager(t)

	var captured []string

	client.EXPECT().
		DoMulti(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmds ...valkey.Completed) []valkey.ValkeyResult {
			captured = cmds[0].Commands()

			return okResults(len(cmds))
		}).
		Times(1)

	// when
	err := m.SetMany(t.Context(), map[string]sample{"s": {Name: "went well", N: 1}}, 0)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"SET", "s", `{"name":"went well","n":1}`}, captured)
}

func TestSetManyReturnsCommandError(t *testing.T) {
	// given
	m, client := newMockManager(t)

	wantErr := errors.New("valkey unavailable")

	client.EXPECT().
		DoMulti(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmds ...valkey.Completed) []valkey.ValkeyResult {
			resps := okResults(len(cmds))
			resps[len(resps)-1] = valkeymock.ErrorResult(wantErr)

			return resps
		}).
		Times(1)

	// when
	err := m.SetMany(t.Context(), map[string]string{"a": "1", "b": "2"}, 0)

	// then
	require.ErrorIs(t, err, wantErr)
}

func TestDelSkipsRoundTripWhenNoKeys(t *testing.T) {
	// given
	m, _ := newMockManager(t)

	// when
	err := m.Del(t.Context())

	// then
	require.NoError(t, err)
}

func TestManagerPicksFirstEnabledEngine(t *testing.T) {
	tests := []struct {
		name   string
		expect func(first *engine.MockEngine, second *engine.MockEngine)
	}{
		{
			name: "first enabled wins",
			expect: func(first *engine.MockEngine, _ *engine.MockEngine) {
				first.EXPECT().Enabled().Return(true).Once()
				expectActivation(first)
				first.EXPECT().Set(mock.Anything, "k", []byte("v"), time.Duration(0)).Return(nil).Once()
			},
		},
		{
			name: "falls through to second",
			expect: func(first *engine.MockEngine, second *engine.MockEngine) {
				first.EXPECT().Enabled().Return(false).Once()
				second.EXPECT().Enabled().Return(true).Once()
				expectActivation(second)
				second.EXPECT().Set(mock.Anything, "k", []byte("v"), time.Duration(0)).Return(nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			first := engine.NewMockEngine(t)
			second := engine.NewMockEngine(t)
			tt.expect(first, second)

			m := NewManager(first, second)

			// when
			err := m.Set(t.Context(), "k", "v", 0)

			// then the write reaches only the engine the expectations name
			require.NoError(t, err)
		})
	}
}

func TestManagerFailsOverWhenEngineGoesDown(t *testing.T) {
	// given
	primary := engine.NewMockEngine(t)
	fallback := engine.NewMockEngine(t)

	m := NewManager(primary, fallback)
	ctx := t.Context()

	primary.EXPECT().Enabled().Return(true).Once()
	expectActivation(primary)
	primary.EXPECT().Set(mock.Anything, "k", []byte("primary value"), time.Duration(0)).Return(nil).Once()
	require.NoError(t, m.Set(ctx, "k", "primary value", 0))

	primary.EXPECT().Enabled().Return(false).Twice()
	fallback.EXPECT().Enabled().Return(true).Twice()
	expectActivation(fallback)
	fallback.EXPECT().Set(mock.Anything, "k", []byte("fallback value"), time.Duration(0)).Return(nil).Once()
	fallback.EXPECT().Get(mock.Anything, "k").Return([]byte("fallback value"), nil).Once()

	// when the primary goes down
	require.NoError(t, m.Set(ctx, "k", "fallback value", 0))
	got, err := m.Get[string](ctx, "k")

	// then
	require.NoError(t, err)
	assert.Equal(t, "fallback value", got)
}

func TestManagerWithoutAnyEnabledEngine(t *testing.T) {
	// given
	only := engine.NewMockEngine(t)
	only.EXPECT().Enabled().Return(false).Times(3)
	only.EXPECT().Ping(mock.Anything).Return(nil).Once()
	only.EXPECT().Del(mock.Anything, []string{"k"}).Return(nil).Once()

	m := NewManager(only)
	ctx := t.Context()

	// when
	_, getErr := m.Get[string](ctx, "k")
	setErr := m.Set(ctx, "k", "v", 0)
	setManyErr := m.SetMany(ctx, map[string]string{"k": "v"}, 0)
	pingErr := m.Ping(ctx)
	delErr := m.Del(ctx, "k")

	// then reads miss and writes are dropped, while ping and del still reach the engine
	require.ErrorIs(t, getErr, ErrMiss)
	require.NoError(t, setErr)
	require.NoError(t, setManyErr)
	require.NoError(t, pingErr)
	require.NoError(t, delErr)
}

func TestManagerDelReachesEveryEngineIncludingDisabledOnes(t *testing.T) {
	// given
	primary := engine.NewMockEngine(t)
	primary.EXPECT().Del(mock.Anything, []string{"role:u1"}).Return(nil).Once()

	fallback := engine.NewMockEngine(t)
	fallback.EXPECT().Del(mock.Anything, []string{"role:u1"}).Return(nil).Once()

	m := NewManager(primary, fallback)

	// when
	err := m.Del(t.Context(), "role:u1")

	// then
	require.NoError(t, err)
}

func TestManagerDelSurfacesEveryEngineFailure(t *testing.T) {
	// given
	primaryErr := errors.New("valkey unreachable")
	primary := engine.NewMockEngine(t)
	primary.EXPECT().Del(mock.Anything, []string{"role:u1"}).Return(primaryErr).Once()
	primary.EXPECT().Kind().Return(engine.Kind("primary")).Once()

	fallbackErr := errors.New("in-memory refused")
	fallback := engine.NewMockEngine(t)
	fallback.EXPECT().Del(mock.Anything, []string{"role:u1"}).Return(fallbackErr).Once()
	fallback.EXPECT().Kind().Return(engine.Kind("fallback")).Once()

	healthy := engine.NewMockEngine(t)
	healthy.EXPECT().Del(mock.Anything, []string{"role:u1"}).Return(nil).Once()

	// when
	err := NewManager(primary, fallback, healthy).Del(t.Context(), "role:u1")

	// then
	require.ErrorIs(t, err, primaryErr)
	require.ErrorIs(t, err, fallbackErr)
	assert.ErrorContains(t, err, "cache engine primary")
	assert.ErrorContains(t, err, "cache engine fallback")
}

func TestManagerPingJoinsEveryEngineError(t *testing.T) {
	// given
	disabledErr := errors.New("valkey unreachable")
	disabled := engine.NewMockEngine(t)
	disabled.EXPECT().Ping(mock.Anything).Return(disabledErr).Once()
	disabled.EXPECT().Kind().Return(engine.Kind("disabled")).Once()

	activeErr := errors.New("active refused")
	active := engine.NewMockEngine(t)
	active.EXPECT().Ping(mock.Anything).Return(activeErr).Once()
	active.EXPECT().Kind().Return(engine.Kind("active")).Once()

	healthy := engine.NewMockEngine(t)
	healthy.EXPECT().Ping(mock.Anything).Return(nil).Once()

	// when
	err := NewManager(disabled, active, healthy).Ping(t.Context())

	// then
	require.ErrorIs(t, err, disabledErr)
	require.ErrorIs(t, err, activeErr)
}

func TestManagerPingReportsAConfiguredButUnreachableValkey(t *testing.T) {
	// given
	client := valkeymock.NewClient(gomock.NewController(t))
	valkeyEngine := engines.NewValkeyWithClient(client)

	wantErr := errors.New("connection refused")

	client.EXPECT().
		Do(gomock.Any(), valkeymock.Match("PING")).
		Return(valkeymock.ErrorResult(wantErr)).
		Times(2)

	m := NewManager(valkeyEngine, engines.NewInMemory(0))
	ctx := t.Context()

	require.ErrorIs(t, m.Ping(ctx), wantErr)
	require.False(t, valkeyEngine.Enabled())

	// when
	err := m.Ping(ctx)

	// then
	require.ErrorIs(t, err, wantErr)
}

func TestManagerPingIsHealthyWithoutValkeyConfigured(t *testing.T) {
	// given
	m := NewManager(engines.NewValkey(), engines.NewInMemory(0))

	// when
	err := m.Ping(t.Context())

	// then
	require.NoError(t, err)
}

func TestManagerNilReceiverIsSafe(t *testing.T) {
	// given
	var m *Manager

	ctx := t.Context()

	// when
	_, getErr := m.Get[string](ctx, "k")

	// then
	require.ErrorIs(t, getErr, ErrMiss)
	assert.Nil(t, m.Engines())
	require.NoError(t, m.Del(ctx, "k"))
	require.NoError(t, m.Ping(ctx))
	require.NoError(t, m.Close())
}

func TestManagerExposesEnginesInPriorityOrder(t *testing.T) {
	// given
	first := engine.NewMockEngine(t)
	first.EXPECT().Kind().Return(engine.Kind("first")).Once()

	second := engine.NewMockEngine(t)
	second.EXPECT().Kind().Return(engine.Kind("second")).Once()

	// when
	kinds := engineKinds(NewManager(first, second))

	// then
	assert.Equal(t, []engine.Kind{"first", "second"}, kinds)
}

func TestManagerCloseClosesEveryEngineAndJoinsEveryError(t *testing.T) {
	// given
	firstErr := errors.New("first close failed")
	first := engine.NewMockEngine(t)
	first.EXPECT().Close().Return(firstErr).Once()
	first.EXPECT().Kind().Return(engine.Kind("first")).Once()

	secondErr := errors.New("second close failed")
	second := engine.NewMockEngine(t)
	second.EXPECT().Close().Return(secondErr).Once()
	second.EXPECT().Kind().Return(engine.Kind("second")).Once()

	disabled := engine.NewMockEngine(t)
	disabled.EXPECT().Close().Return(nil).Once()

	// when
	err := NewManager(first, second, disabled).Close()

	// then
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
}

func TestManagerCountsHitsAndMisses(t *testing.T) {
	// given
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Twice()
	expectActivation(stub)
	stub.EXPECT().Get(mock.Anything, "present").Return([]byte("v"), nil).Once()
	stub.EXPECT().Get(mock.Anything, "absent").Return(nil, engine.ErrMiss).Once()

	m := NewManager(stub)
	ctx := t.Context()

	hitsBefore := testutil.ToFloat64(cacheHits)
	missesBefore := testutil.ToFloat64(cacheMisses)

	// when
	got, hitErr := m.Get[string](ctx, "present")
	_, missErr := m.Get[string](ctx, "absent")

	// then
	require.NoError(t, hitErr)
	assert.Equal(t, "v", got)
	require.ErrorIs(t, missErr, ErrMiss)

	assert.Equal(t, hitsBefore+1, testutil.ToFloat64(cacheHits))
	assert.Equal(t, missesBefore+1, testutil.ToFloat64(cacheMisses))
}

func TestManagerFlushesAnEngineOnceWhenItBecomesActive(t *testing.T) {
	// given
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Twice()
	expectActivation(stub)
	stub.EXPECT().Get(mock.Anything, "k").Return([]byte("v"), nil).Twice()

	m := NewManager(stub)
	ctx := t.Context()

	// when
	_, firstErr := m.Get[string](ctx, "k")
	_, secondErr := m.Get[string](ctx, "k")

	// then the second read uses the engine without flushing it again
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
}

func TestManagerFlushesAgainWhenTheEngineStartsANewGeneration(t *testing.T) {
	// given
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Twice()
	stub.EXPECT().Generation().Return(uint64(0)).Once()
	stub.EXPECT().Generation().Return(uint64(1)).Once()
	stub.EXPECT().Flush(mock.Anything).Return(nil).Twice()
	stub.EXPECT().Get(mock.Anything, "k").Return([]byte("v"), nil).Twice()

	m := NewManager(stub)
	ctx := t.Context()

	// when the engine lost writes between the two reads
	_, firstErr := m.Get[string](ctx, "k")
	_, secondErr := m.Get[string](ctx, "k")

	// then it is flushed before it serves the second read
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
}

func TestManagerSkipsAnEngineItCannotFlush(t *testing.T) {
	// given
	flushErr := errors.New("valkey unreachable")
	primary := engine.NewMockEngine(t)
	primary.EXPECT().Enabled().Return(true).Once()
	primary.EXPECT().Generation().Return(uint64(0))
	primary.EXPECT().Flush(mock.Anything).Return(flushErr).Once()
	primary.EXPECT().Kind().Return(engine.Kind("primary")).Once()

	fallback := engine.NewMockEngine(t)
	fallback.EXPECT().Enabled().Return(true).Once()
	expectActivation(fallback)
	fallback.EXPECT().Get(mock.Anything, "k").Return([]byte("fallback value"), nil).Once()

	m := NewManager(primary, fallback)

	// when
	got, err := m.Get[string](t.Context(), "k")

	// then the engine that may hold stale values serves nothing
	require.NoError(t, err)
	assert.Equal(t, "fallback value", got)
}

func TestLoadFillsTheCacheOnlyWhereTheKeyIsAbsent(t *testing.T) {
	// given
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Twice()
	expectActivation(stub)
	stub.EXPECT().Get(mock.Anything, "setting:site_name").Return(nil, engine.ErrMiss).Once()
	stub.EXPECT().SetIfAbsent(mock.Anything, "setting:site_name", []byte("RetroTide"), time.Duration(0)).Return(nil).Once()

	m := NewManager(stub)
	load := func(context.Context) (string, error) {
		return "RetroTide", nil
	}

	// when
	got, err := m.Load(t.Context(), Setting, load, "site_name")

	// then a value written by a concurrent settings write is never overwritten by this fill
	require.NoError(t, err)
	assert.Equal(t, "RetroTide", got)
}

func TestManagerDoesNotCountAFailedReadAsAMiss(t *testing.T) {
	// given
	getErr := errors.New("valkey unreachable")
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Once()
	expectActivation(stub)
	stub.EXPECT().Get(mock.Anything, "k").Return(nil, getErr).Once()

	m := NewManager(stub)

	missesBefore := testutil.ToFloat64(cacheMisses)

	// when
	_, err := m.Get[string](t.Context(), "k")

	// then
	require.ErrorIs(t, err, getErr)
	assert.Equal(t, missesBefore, testutil.ToFloat64(cacheMisses))
}

func engineKinds(m *Manager) []engine.Kind {
	list := m.Engines()

	kinds := make([]engine.Kind, 0, len(list))
	for _, candidate := range list {
		kinds = append(kinds, candidate.Kind())
	}

	return kinds
}

func TestNewAsksTheFactoryForValkeyAheadOfInMemory(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()

	// when
	m, err := New(reg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []engine.Kind{engine.KindValkey, engine.KindInMemory}, engineKinds(m))
}

func TestNewReportsASecondRegistrationOnTheSameRegistry(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()

	_, err := New(reg)
	require.NoError(t, err)

	// when
	m, err := New(reg)

	// then
	require.Error(t, err)
	assert.Nil(t, m)

	_, isDuplicate := errors.AsType[prometheus.AlreadyRegisteredError](err)
	assert.True(t, isDuplicate)
}

func TestNewLeavesValkeyDisabledUntilConfigured(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()

	// when
	m, err := New(reg)

	// then
	require.NoError(t, err)

	list := m.Engines()
	require.Len(t, list, 2)
	assert.False(t, list[0].Enabled())
	assert.True(t, list[1].Enabled())
}

func TestNewFallsBackToInMemoryWithoutValkey(t *testing.T) {
	// given
	m, err := New(prometheus.NewRegistry())
	require.NoError(t, err)

	ctx := t.Context()
	want := sample{Name: "went well", N: 1}

	// when
	require.NoError(t, m.Set(ctx, "retro", want, 0))

	got, err := m.Get[sample](ctx, "retro")

	// then
	require.NoError(t, err)
	assert.Equal(t, want, got)

	require.NoError(t, m.Del(ctx, "retro"))

	_, err = m.Get[sample](ctx, "retro")
	require.ErrorIs(t, err, ErrMiss)
}
