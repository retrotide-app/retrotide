package engines

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"retrotide/internal/cache/engine"
	"retrotide/internal/cache/engines/observability"
	"retrotide/internal/config"
	"retrotide/internal/logger"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeyhook"
)

const (
	probeTimeout = 5 * time.Second

	recoveryInterval = 30 * time.Second
)

type (
	dialFunc func(option valkey.ClientOption) (valkey.Client, error)

	Valkey struct {
		mu     sync.RWMutex
		client valkey.Client
		url    string
		option valkey.ClientOption
		dial   dialFunc

		healthy    atomic.Bool
		downAt     atomic.Int64
		generation atomic.Uint64
	}
)

func NewValkey() *Valkey {
	return &Valkey{dial: connect}
}

func NewValkeyWithClient(client valkey.Client) *Valkey {
	v := &Valkey{client: client, dial: connect}
	v.healthy.Store(true)

	return v
}

func ProbeURL(ctx context.Context, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}

	opt, err := valkey.ParseURL(rawURL)
	if err != nil {
		return fmt.Errorf("cache: invalid valkey url: %w", err)
	}

	client, err := valkey.NewClient(opt)
	if err != nil {
		return fmt.Errorf("cache: cannot reach valkey: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	err = client.Do(ctx, client.B().Ping().Build()).Error()
	if err != nil {
		return fmt.Errorf("cache: cannot reach valkey: %w", err)
	}

	return nil
}

func connect(option valkey.ClientOption) (valkey.Client, error) {
	client, err := valkey.NewClient(option)
	if err != nil {
		return nil, fmt.Errorf("cache: cannot reach valkey: %w", err)
	}

	return valkeyhook.WithHook(client, observability.NewHook()), nil
}

func (v *Valkey) Kind() engine.Kind {
	return engine.KindValkey
}

func (v *Valkey) Enabled() bool {
	client, rawURL, option := v.connection()
	if client == nil && rawURL == "" {
		return false
	}

	if client != nil && v.healthy.Load() {
		return true
	}

	downAt := v.downAt.Load()
	if time.Since(time.Unix(0, downAt)) < recoveryInterval {
		return false
	}

	if !v.downAt.CompareAndSwap(downAt, time.Now().UnixNano()) {
		return false
	}

	if client != nil {
		return true
	}

	go v.reconnect(rawURL, option)

	return false
}

func (v *Valkey) Generation() uint64 {
	if v == nil {
		return 0
	}

	return v.generation.Load()
}

func (v *Valkey) observe(ctx context.Context, err error) error {
	if err == nil || valkey.IsValkeyNil(err) {
		v.healthy.Store(true)

		return err
	}

	if observability.CallerGone(ctx, err) {
		return err
	}

	if v.healthy.Swap(false) {
		v.downAt.Store(time.Now().UnixNano())
		v.generation.Add(1)
		logger.Log.Warn().Err(err).Msg("valkey cache unhealthy, falling back to next engine")
	}

	return err
}

func (v *Valkey) Reconfigure(rawURL string) error {
	client, err := v.swapClient(rawURL)
	if err != nil {
		return err
	}

	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	err = client.Do(ctx, client.B().Ping().Build()).Error()
	if err != nil {
		return fmt.Errorf("cache: ping failed: %w", err)
	}

	v.healthy.Store(true)

	logger.Ctx(ctx).Info().Msg("valkey cache enabled")

	return nil
}

func (v *Valkey) OnSettingChanged(key config.SiteSettingKey, value string) {
	if key != config.SettingValkeyURL.Key {
		return
	}

	err := v.Reconfigure(value)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("valkey cache reconfigure failed")
	}
}

func (v *Valkey) Get(ctx context.Context, key string) ([]byte, error) {
	client := v.Client()
	if client == nil {
		return nil, engine.ErrMiss
	}

	data, err := client.Do(ctx, client.B().Get().Key(key).Build()).AsBytes()
	if err := v.observe(ctx, err); err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, engine.ErrMiss
		}

		return nil, err
	}

	return data, nil
}

func (v *Valkey) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	client := v.Client()
	if client == nil {
		return nil
	}

	value := client.B().Set().Key(key).Value(valkey.BinaryString(data))

	if ttl > 0 {
		return v.observe(ctx, client.Do(ctx, value.Px(ttl).Build()).Error())
	}

	return v.observe(ctx, client.Do(ctx, value.Build()).Error())
}

func (v *Valkey) SetIfAbsent(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	client := v.Client()
	if client == nil {
		return nil
	}

	value := client.B().Set().Key(key).Value(valkey.BinaryString(data)).Nx()

	var err error
	if ttl > 0 {
		err = v.observe(ctx, client.Do(ctx, value.Px(ttl).Build()).Error())
	} else {
		err = v.observe(ctx, client.Do(ctx, value.Build()).Error())
	}

	if valkey.IsValkeyNil(err) {
		return nil
	}

	return err
}

func (v *Valkey) SetMany(ctx context.Context, entries map[string][]byte, ttl time.Duration) error {
	client := v.Client()
	if client == nil || len(entries) == 0 {
		return nil
	}

	cmds := make([]valkey.Completed, 0, len(entries))
	for key, data := range entries {
		value := client.B().Set().Key(key).Value(valkey.BinaryString(data))

		if ttl > 0 {
			cmds = append(cmds, value.Px(ttl).Build())

			continue
		}

		cmds = append(cmds, value.Build())
	}

	for _, resp := range client.DoMulti(ctx, cmds...) {
		err := resp.Error()
		if err != nil {
			return v.observe(ctx, err)
		}
	}

	return v.observe(ctx, nil)
}

func (v *Valkey) Del(ctx context.Context, keys ...string) error {
	client := v.Client()
	if client == nil || len(keys) == 0 {
		return nil
	}

	return v.observe(ctx, client.Do(ctx, client.B().Del().Key(keys...).Build()).Error())
}

func (v *Valkey) Flush(ctx context.Context) error {
	client := v.Client()
	if client == nil {
		return nil
	}

	return v.observe(ctx, client.Do(ctx, client.B().Flushdb().Build()).Error())
}

func (v *Valkey) Ping(ctx context.Context) error {
	client, rawURL, option := v.connection()
	if client == nil && rawURL != "" {
		return fmt.Errorf("%w at %s", errNotConnected, strings.Join(option.InitAddress, ","))
	}

	if client == nil {
		return nil
	}

	return v.observe(ctx, client.Do(ctx, client.B().Ping().Build()).Error())
}

func (v *Valkey) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.url = ""

	if v.client == nil {
		return nil
	}

	v.client.Close()
	v.client = nil

	return nil
}

func (v *Valkey) Client() valkey.Client {
	client, _, _ := v.connection()

	return client
}

func (v *Valkey) connection() (valkey.Client, string, valkey.ClientOption) {
	if v == nil {
		return nil, "", valkey.ClientOption{}
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.client, v.url, v.option
}

func (v *Valkey) reconnect(rawURL string, option valkey.ClientOption) {
	client, err := v.dial(option)
	if err != nil {
		logger.Log.Warn().Err(err).Dur("retry_in", recoveryInterval).Msg("valkey cache still unreachable")
		return
	}

	if !v.adopt(rawURL, client) {
		client.Close()
		return
	}

	logger.Log.Info().Msg("valkey cache connected")
}

func (v *Valkey) adopt(rawURL string, client valkey.Client) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.url != rawURL || v.client != nil {
		return false
	}

	v.client = client
	v.healthy.Store(true)

	return true
}

func (v *Valkey) swapClient(rawURL string) (valkey.Client, error) {
	rawURL = strings.TrimSpace(rawURL)

	v.mu.Lock()
	defer v.mu.Unlock()

	if rawURL == v.url && (rawURL == "" || v.client != nil) {
		return nil, nil
	}

	if v.client != nil {
		v.client.Close()
		v.client = nil
	}

	v.url = rawURL
	v.generation.Add(1)

	if rawURL == "" {
		logger.Log.Info().Msg("valkey cache disabled")

		return nil, nil
	}

	opt, err := valkey.ParseURL(rawURL)
	if err != nil {
		v.url = ""

		return nil, fmt.Errorf("cache: invalid valkey url: %w", err)
	}

	v.option = opt

	client, err := v.dial(opt)
	if err != nil {
		v.healthy.Store(false)
		v.downAt.Store(time.Now().UnixNano())

		return nil, fmt.Errorf("%w; retrying every %s", err, recoveryInterval)
	}

	v.client = client

	return client, nil
}
