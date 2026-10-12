package cache

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"retrotide/internal/cache/engine"
	"retrotide/internal/cache/engines"
	"retrotide/internal/logger"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	sample struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
)

func roundTrip[T any](t *testing.T, value T) T {
	t.Helper()

	data, err := encode(value)
	require.NoError(t, err)

	got, err := decode[T](data)
	require.NoError(t, err)

	return got
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	original := logger.Log
	logger.Log = zerolog.New(&buf)
	t.Cleanup(func() {
		logger.Log = original
	})

	return &buf
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		// given
		value := "hello retrotide"

		// when
		got := roundTrip(t, value)

		// then
		assert.Equal(t, value, got)
	})

	t.Run("bytes", func(t *testing.T) {
		// given
		value := []byte{0x00, 0x01, 0xff, 0x10, 0x7f}

		// when
		got := roundTrip(t, value)

		// then
		assert.Equal(t, value, got)
	})

	t.Run("struct", func(t *testing.T) {
		// given
		value := sample{Name: "went well", N: 7}

		// when
		got := roundTrip(t, value)

		// then
		assert.Equal(t, value, got)
	})

	t.Run("pointer", func(t *testing.T) {
		// given
		value := &sample{Name: "to improve", N: 3}

		// when
		got := roundTrip(t, value)

		// then
		assert.Equal(t, value, got)
	})

	t.Run("int", func(t *testing.T) {
		// given
		value := 1998

		// when
		got := roundTrip(t, value)

		// then
		assert.Equal(t, value, got)
	})
}

func TestEncodeStoresBytesAndStringsRaw(t *testing.T) {
	// given
	raw := []byte{0x00, 0x10, 0xff}

	// when
	encodedBytes, bytesErr := encode(raw)
	encodedString, stringErr := encode("hi")

	// then
	require.NoError(t, bytesErr)
	assert.Equal(t, raw, encodedBytes)

	require.NoError(t, stringErr)
	assert.Equal(t, []byte("hi"), encodedString)
}

func TestEncodeStructUsesJSON(t *testing.T) {
	// given
	value := sample{Name: "action item", N: 12}

	// when
	encoded, err := encode(value)

	// then
	require.NoError(t, err)
	assert.Equal(t, `{"name":"action item","n":12}`, string(encoded))
}

func TestSetManyWithoutClient(t *testing.T) {
	tests := []struct {
		name    string
		manager *Manager
		values  map[string]string
	}{
		{name: "nil manager", manager: nil, values: map[string]string{"k": "v"}},
		{name: "nil values", manager: NewManager(), values: nil},
		{name: "empty values", manager: NewManager(), values: map[string]string{}},
		{name: "cache disabled", manager: NewManager(), values: map[string]string{"k": "v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			m := tt.manager

			// when
			err := m.SetMany(t.Context(), tt.values, 0)

			// then
			require.NoError(t, err)
		})
	}
}

func TestSetManyPropagatesEncodeError(t *testing.T) {
	// given
	values := map[string]chan int{"unserialisable": make(chan int)}

	// when
	err := NewManager().SetMany(t.Context(), values, 0)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chan int")
}

func TestSetManyEncodesEveryValue(t *testing.T) {
	// given
	values := map[string]sample{
		"a": {Name: "went well", N: 1},
		"b": {Name: "to improve", N: 2},
	}

	// when
	entries := make(map[string][]byte, len(values))
	for key, value := range values {
		data, err := encode(value)
		require.NoError(t, err)

		entries[key] = data
	}

	// then
	assert.Equal(t, `{"name":"went well","n":1}`, string(entries["a"]))
	assert.Equal(t, `{"name":"to improve","n":2}`, string(entries["b"]))
}

func TestEncodeDecodeNilPointer(t *testing.T) {
	// given
	var original *sample

	// when
	encoded, err := encode(original)
	require.NoError(t, err)

	got, decodeErr := decode[*sample](encoded)

	// then
	assert.Equal(t, "null", string(encoded))
	require.NoError(t, decodeErr)
	assert.Nil(t, got)
}

func newLoadManager() (*Manager, Namespace) {
	return NewManager(engines.NewInMemory(0)), Namespace{Prefix: "retro:", TTL: time.Minute}
}

func TestLoadReturnsCachedValueWithoutCallingLoader(t *testing.T) {
	// given
	m, ns := newLoadManager()
	ctx := t.Context()

	require.NoError(t, m.Set(ctx, ns.Key("board"), sample{Name: "went well", N: 1}, ns.TTL))

	calls := 0
	load := func(context.Context) (sample, error) {
		calls++

		return sample{Name: "to improve", N: 2}, nil
	}

	// when
	got, err := m.Load(ctx, ns, load, "board")

	// then
	require.NoError(t, err)
	assert.Equal(t, sample{Name: "went well", N: 1}, got)
	assert.Zero(t, calls)
}

func TestLoadStoresLoadedValueOnMiss(t *testing.T) {
	// given
	m, ns := newLoadManager()
	ctx := t.Context()
	logs := captureLog(t)

	want := sample{Name: "went well", N: 7}
	load := func(context.Context) (sample, error) {
		return want, nil
	}

	// when
	got, err := m.Load(ctx, ns, load, "board")

	// then
	require.NoError(t, err)
	assert.Equal(t, want, got)

	cached, err := m.Get[sample](ctx, ns.Key("board"))
	require.NoError(t, err)
	assert.Equal(t, want, cached)
	assert.Empty(t, logs.String())
}

func TestLoadPropagatesLoaderErrorAndCachesNothing(t *testing.T) {
	// given
	m, ns := newLoadManager()
	ctx := t.Context()

	wantErr := errors.New("the board is closed")
	load := func(context.Context) (sample, error) {
		return sample{Name: "to improve", N: 2}, wantErr
	}

	// when
	got, err := m.Load(ctx, ns, load, "board")

	// then
	require.ErrorIs(t, err, wantErr)
	assert.Zero(t, got)

	_, err = m.Get[sample](ctx, ns.Key("board"))
	assert.ErrorIs(t, err, ErrMiss)
}

func TestLoadFallsBackToLoaderWithoutManager(t *testing.T) {
	// given
	var m *Manager

	want := sample{Name: "went well", N: 3}
	load := func(context.Context) (sample, error) {
		return want, nil
	}

	// when
	got, err := m.Load(t.Context(), Namespace{Prefix: "retro:"}, load, "board")

	// then
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLoadLogsAFailedReadAndServesTheLoadedValue(t *testing.T) {
	// given
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Twice()
	expectActivation(stub)
	stub.EXPECT().Get(mock.Anything, "retro:board").Return(nil, errors.New("valkey unreachable")).Once()
	stub.EXPECT().SetIfAbsent(mock.Anything, "retro:board", mock.Anything, time.Minute).Return(nil).Once()

	m := NewManager(stub)
	ns := Namespace{Prefix: "retro:", TTL: time.Minute}
	logs := captureLog(t)

	want := sample{Name: "went well", N: 4}
	load := func(context.Context) (sample, error) {
		return want, nil
	}

	// when
	got, err := m.Load(t.Context(), ns, load, "board")

	// then the loaded value is served and written back once
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Contains(t, logs.String(), `"level":"warn"`)
	assert.Contains(t, logs.String(), `"key":"retro:board"`)
	assert.Contains(t, logs.String(), "valkey unreachable")
}

func TestLoadStaysQuietWhenTheCallerHasGone(t *testing.T) {
	// given a caller that went away before the read
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Once()
	expectActivation(stub)
	stub.EXPECT().Get(mock.Anything, "retro:board").Return(nil, context.Canceled).Once()

	m := NewManager(stub)
	ns := Namespace{Prefix: "retro:", TTL: time.Minute}
	logs := captureLog(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	load := func(ctx context.Context) (sample, error) {
		return sample{}, ctx.Err()
	}

	// when
	_, err := m.Load(ctx, ns, load, "board")

	// then the cancellation reaches the caller, and no warning claims the cache failed
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, logs.String())
}

func TestLoadLogsAFailedWriteAndServesTheLoadedValue(t *testing.T) {
	// given
	stub := engine.NewMockEngine(t)
	stub.EXPECT().Enabled().Return(true).Twice()
	expectActivation(stub)
	stub.EXPECT().Get(mock.Anything, "retro:board").Return(nil, engine.ErrMiss).Once()
	stub.EXPECT().SetIfAbsent(mock.Anything, "retro:board", mock.Anything, time.Minute).Return(errors.New("valkey out of memory")).Once()

	m := NewManager(stub)
	ns := Namespace{Prefix: "retro:", TTL: time.Minute}
	logs := captureLog(t)

	want := sample{Name: "went well", N: 5}
	load := func(context.Context) (sample, error) {
		return want, nil
	}

	// when
	got, err := m.Load(t.Context(), ns, load, "board")

	// then
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Contains(t, logs.String(), `"level":"warn"`)
	assert.Contains(t, logs.String(), `"key":"retro:board"`)
	assert.Contains(t, logs.String(), "valkey out of memory")
}
