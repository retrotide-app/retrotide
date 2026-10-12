package telemetry

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"retrotide/internal/config"
	"retrotide/internal/logger"

	"github.com/grafana/pyroscope-go"
)

type (
	ProfilingSettingsListener struct{}
)

var (
	profilingMu       sync.Mutex
	profiler          *pyroscope.Profiler
	currentPyroscope  string
	profilingService  string
	profilingInstance string
)

func InitProfiling(serviceName, instance, url string) error {
	profilingMu.Lock()
	defer profilingMu.Unlock()

	profilingService = serviceName
	profilingInstance = instance

	return applyProfilingLocked(url)
}

func ApplyProfiling(url string) error {
	profilingMu.Lock()
	defer profilingMu.Unlock()

	if profilingService == "" {
		return nil
	}

	return applyProfilingLocked(url)
}

func applyProfilingLocked(url string) error {
	url = strings.TrimSpace(url)
	if url == currentPyroscope {
		return nil
	}

	var stopErr error
	if profiler != nil {
		if err := profiler.Stop(); err != nil {
			stopErr = fmt.Errorf("stop pyroscope profiler: %w", err)
		}
		profiler = nil
	}

	currentPyroscope = ""

	if url == "" {
		logger.Log.Info().Msg("pyroscope profiling disabled")
		return stopErr
	}

	p, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: profilingService,
		ServerAddress:   url,
		Logger:          nil,
		Tags: map[string]string{
			"instance": profilingInstance,
		},
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		return errors.Join(stopErr, fmt.Errorf("start pyroscope profiler: %w", err))
	}

	profiler = p
	currentPyroscope = url

	logger.Log.Info().Str("url", url).Msg("pyroscope profiling enabled")

	return stopErr
}

func ShutdownProfiling() error {
	profilingMu.Lock()
	defer profilingMu.Unlock()

	if profiler == nil {
		return nil
	}

	running := profiler
	profiler = nil
	currentPyroscope = ""

	if err := running.Stop(); err != nil {
		return fmt.Errorf("stop pyroscope profiler: %w", err)
	}

	return nil
}

func NewProfilingSettingsListener() *ProfilingSettingsListener {
	return &ProfilingSettingsListener{}
}

func (l *ProfilingSettingsListener) OnSettingChanged(key config.SiteSettingKey, value string) {
	if key != config.SettingPyroscopeURL.Key {
		return
	}

	if err := ApplyProfiling(value); err != nil {
		logger.Log.Warn().Err(err).Msg("failed to apply pyroscope url change")
	}
}
