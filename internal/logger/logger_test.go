package logger

import (
	"bytes"
	"strings"
	"testing"

	"retrotide/internal/config"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	savedLog := Log
	savedLevel := zerolog.GlobalLevel()
	t.Cleanup(func() {
		Log = savedLog
		zerolog.SetGlobalLevel(savedLevel)
	})

	var buf bytes.Buffer
	Log = zerolog.New(&buf)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	return &buf
}

func TestSetLevel_RefusesUnknownAndEmptyLevels(t *testing.T) {
	cases := []struct {
		name    string
		level   string
		wantErr string
	}{
		{"an unknown level", "loud", `invalid log level "loud"`},
		{"the empty string", "", "log level must not be empty"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			captureLog(t)
			zerolog.SetGlobalLevel(zerolog.WarnLevel)

			// when
			err := SetLevel(tc.level)

			// then
			require.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, zerolog.WarnLevel, zerolog.GlobalLevel())
		})
	}
}

func TestSetLevel_AppliesAKnownLevel(t *testing.T) {
	// given
	captureLog(t)

	// when
	err := SetLevel("error")

	// then
	require.NoError(t, err)
	assert.Equal(t, zerolog.ErrorLevel, zerolog.GlobalLevel())
}

func TestValidateLevel(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"debug is accepted", "debug", false},
		{"an unknown level is refused", "loud", true},
		{"the empty string is refused", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ctx := t.Context()

			// when
			err := ValidateLevel(ctx, tc.value)

			// then
			if !tc.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
		})
	}
}

func TestNewLevelWriter_WritesAWarnLine(t *testing.T) {
	// given
	buf := captureLog(t)
	w := NewLevelWriter(zerolog.WarnLevel)
	line := []byte("http: TLS handshake error from 10.0.0.2:4444: EOF\n")

	// when
	n, err := w.Write(line)

	// then
	require.NoError(t, err)
	assert.Equal(t, len(line), n)
	assert.JSONEq(t, `{"level":"warn","message":"http: TLS handshake error from 10.0.0.2:4444: EOF"}`, buf.String())
}

func TestCtx_StampsTraceAndSpanIDs(t *testing.T) {
	// given
	buf := captureLog(t)
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithSpanContext(t.Context(), sc)

	// when
	Ctx(ctx).Info().Msg("traced")

	// then
	assert.JSONEq(t, `{"level":"info","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7","message":"traced"}`, buf.String())
}

func TestCtx_WithoutASpanReturnsLog(t *testing.T) {
	// given
	ctx := t.Context()

	// when
	got := Ctx(ctx)

	// then
	assert.Same(t, &Log, got)
}

func TestSettingsListener_AppliesTheLogLevel(t *testing.T) {
	// given
	buf := captureLog(t)
	listener := NewSettingsListener()

	// when
	listener.OnSettingChanged(config.SettingLogLevel.Key, "debug")

	// then
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	assert.JSONEq(t, `{"level":"info","log_level":"debug","message":"log level changed"}`, buf.String())
}

func TestSettingsListener_LogsARefusedLevel(t *testing.T) {
	// given
	buf := captureLog(t)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	listener := NewSettingsListener()

	// when
	listener.OnSettingChanged(config.SettingLogLevel.Key, "loud")

	// then
	out := buf.String()
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
	assert.Equal(t, 1, strings.Count(out, `"level":`))
	assert.Contains(t, out, `"level":"error"`)
	assert.Contains(t, out, `"log_level":"loud"`)
	assert.Contains(t, out, `"message":"failed to change log level"`)
}

func TestSettingsListener_IgnoresOtherSettings(t *testing.T) {
	// given
	buf := captureLog(t)
	listener := NewSettingsListener()

	// when
	listener.OnSettingChanged(config.SettingSiteName.Key, "warn")

	// then
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	assert.Empty(t, buf.String())
}
