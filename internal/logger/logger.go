package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"retrotide/internal/config"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

type (
	SettingsListener struct{}

	levelWriter struct {
		level zerolog.Level
	}
)

var (
	Log = zerolog.New(os.Stdout).With().Timestamp().Logger()

	jsonOutput bool

	initWriter = sync.OnceFunc(func() {
		jsonOutput = strings.EqualFold(os.Getenv("LOG_FORMAT"), "json")

		Log = zerolog.New(newWriter()).With().Timestamp().Logger()
	})
)

func Init(level string) error {
	initWriter()

	return SetLevel(level)
}

func SetLevel(level string) error {
	parsed, err := parseLevel(level)
	if err != nil {
		return err
	}

	zerolog.SetGlobalLevel(parsed)

	return nil
}

func ValidateLevel(_ context.Context, value string) error {
	_, err := parseLevel(value)

	return err
}

func parseLevel(level string) (zerolog.Level, error) {
	if level == "" {
		return zerolog.NoLevel, errEmptyLevel
	}

	parsed, err := zerolog.ParseLevel(level)
	if err != nil {
		return zerolog.NoLevel, fmt.Errorf("invalid log level %q: %w", level, err)
	}

	return parsed, nil
}

func NewLevelWriter(level zerolog.Level) io.Writer {
	return &levelWriter{level: level}
}

func (w *levelWriter) Write(p []byte) (int, error) {
	Log.WithLevel(w.level).Msg(strings.TrimRight(string(p), "\r\n"))

	return len(p), nil
}

func Ctx(ctx context.Context) *zerolog.Logger {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return &Log
	}

	traced := Log.With().
		Str("trace_id", sc.TraceID().String()).
		Str("span_id", sc.SpanID().String()).
		Logger()

	return &traced
}

func newWriter() io.Writer {
	if jsonOutput {
		return os.Stdout
	}

	return zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.DateTime}
}

func JSONOutput() bool {
	return jsonOutput
}

func NewSettingsListener() *SettingsListener {
	return &SettingsListener{}
}

func (l *SettingsListener) OnSettingChanged(key config.SiteSettingKey, value string) {
	if key != config.SettingLogLevel.Key {
		return
	}

	if err := SetLevel(value); err != nil {
		Log.Error().Err(err).Str("log_level", value).Msg("failed to change log level")
		return
	}

	Log.Info().Str("log_level", value).Msg("log level changed")
}
