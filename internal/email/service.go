package email

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/logger"
	"retrotide/internal/settings"

	"github.com/wneessen/go-mail"
)

type (
	Service interface {
		Send(ctx context.Context, message Message) error
		SendTest(ctx context.Context, message Message) error
		Enabled(ctx context.Context) (bool, error)
		Status(ctx context.Context) (Status, error)
		OnSettingChanged(key config.SiteSettingKey, value string)
	}

	Status string

	Message struct {
		To      string
		Subject string
		HTML    string
	}

	service struct {
		settings   settings.Service
		httpClient *http.Client
		mu         sync.RWMutex
		smtp       smtpState
	}

	smtpState struct {
		client *mail.Client
		err    error
	}

	cloudflareCredentials struct {
		accountID string
		apiToken  string
		from      string
	}

	cloudflareAddress struct {
		Address string `json:"address"`
		Name    string `json:"name,omitempty"`
	}

	cloudflareSendRequest struct {
		To      string            `json:"to"`
		From    cloudflareAddress `json:"from"`
		Subject string            `json:"subject"`
		HTML    string            `json:"html"`
		Text    string            `json:"text"`
	}

	cloudflareSendResponse struct {
		Success bool `json:"success"`
	}
)

const (
	cloudflareSendURL  = "https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send"
	cloudflareTimeout  = 30 * time.Second
	smtpSettingsPrefix = "smtp_"
)

const (
	StatusOff     Status = "off"
	StatusReady   Status = "ready"
	StatusFailing Status = "failing"
)

var (
	htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

	_ settings.Listener = (*service)(nil)
)

func NewService(settingsSvc settings.Service) (Service, error) {
	svc := &service{
		settings:   settingsSvc,
		httpClient: &http.Client{Timeout: cloudflareTimeout},
	}

	ctx := context.Background()

	state, err := svc.buildSMTP(ctx)
	if err != nil {
		return nil, err
	}

	if state.err != nil {
		logger.Ctx(ctx).Error().Err(state.err).Msg("failed to build the SMTP client; email counts as unavailable until the settings are fixed")
	}

	svc.smtp = state

	return svc, nil
}

func (s *service) Send(ctx context.Context, message Message) error {
	if err := s.deliver(ctx, message); err != nil {
		return fmt.Errorf("send the email %q: %w", message.Subject, err)
	}

	return nil
}

func (s *service) SendTest(ctx context.Context, message Message) error {
	if err := s.deliver(ctx, message); err != nil {
		return fmt.Errorf("send the test email: %w", err)
	}

	return nil
}

func (s *service) Enabled(ctx context.Context) (bool, error) {
	status, err := s.Status(ctx)
	if err != nil {
		return false, err
	}

	return status == StatusReady, nil
}

func (s *service) Status(ctx context.Context) (Status, error) {
	provider, err := s.settings.Get(ctx, config.SettingEmailProvider)
	if err != nil {
		return "", fmt.Errorf("read the email provider: %w", err)
	}

	if config.EmailProvider(provider) == config.EmailProviderCloudflare {
		_, configured, err := s.cloudflareSettings(ctx)
		if err != nil {
			return "", err
		}

		if !configured {
			return StatusOff, nil
		}

		return StatusReady, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	switch {
	case s.smtp.err != nil:
		return StatusFailing, nil
	case s.smtp.client == nil:
		return StatusOff, nil
	default:
		return StatusReady, nil
	}
}

func (s *service) OnSettingChanged(key config.SiteSettingKey, _ string) {
	if !strings.HasPrefix(string(key), smtpSettingsPrefix) {
		return
	}

	ctx := context.Background()

	state, err := s.buildSMTP(ctx)
	if err != nil {
		state = smtpState{err: err}
	}

	s.mu.Lock()
	s.smtp = state
	s.mu.Unlock()

	if state.err != nil {
		logger.Ctx(ctx).Error().Err(state.err).Str("key", string(key)).Msg("failed to rebuild the SMTP client; email counts as unavailable until the settings are fixed")

		return
	}

	logger.Ctx(ctx).Info().Str("key", string(key)).Bool("configured", state.client != nil).Msg("SMTP client rebuilt")
}

func (s *service) deliver(ctx context.Context, message Message) error {
	provider, err := s.settings.Get(ctx, config.SettingEmailProvider)
	if err != nil {
		return fmt.Errorf("read the email provider: %w", err)
	}

	if config.EmailProvider(provider) == config.EmailProviderCloudflare {
		return s.sendCloudflare(ctx, message)
	}

	return s.sendSMTP(ctx, message)
}

func (s *service) buildSMTP(ctx context.Context) (smtpState, error) {
	host, err := s.settings.Get(ctx, config.SettingSMTPHost)
	if err != nil {
		return smtpState{}, fmt.Errorf("read the SMTP host: %w", err)
	}

	if host == "" {
		return smtpState{}, nil
	}

	port, err := s.settings.GetInt(ctx, config.SettingSMTPPort)
	if err != nil {
		return smtpState{}, fmt.Errorf("read the SMTP port: %w", err)
	}

	username, err := s.settings.Get(ctx, config.SettingSMTPUsername)
	if err != nil {
		return smtpState{}, fmt.Errorf("read the SMTP username: %w", err)
	}

	password, err := s.settings.Get(ctx, config.SettingSMTPPassword)
	if err != nil {
		return smtpState{}, fmt.Errorf("read the SMTP password: %w", err)
	}

	options := []mail.Option{mail.WithPort(port)}

	switch {
	case username != "" && password != "":
		options = append(options, mail.WithSMTPAuth(mail.SMTPAuthPlain), mail.WithUsername(username), mail.WithPassword(password))
	case isLoopbackHost(host):
		options = append(options, mail.WithTLSPolicy(mail.NoTLS))
	default:
		options = append(options, mail.WithTLSPolicy(mail.TLSOpportunistic))
		logger.Ctx(ctx).Warn().Str("host", host).Msg("SMTP has no credentials, so it uses opportunistic TLS; mail leaves this host in cleartext if the server does not offer STARTTLS")
	}

	client, err := mail.NewClient(host, options...)
	if err != nil {
		return smtpState{err: fmt.Errorf("build the SMTP client for %s:%d: %w", host, port, err)}, nil
	}

	return smtpState{client: client}, nil
}

func (s *service) sendSMTP(ctx context.Context, message Message) error {
	s.mu.RLock()
	state := s.smtp
	s.mu.RUnlock()

	if state.err != nil {
		return fmt.Errorf("%w: %w", ErrFailing, state.err)
	}

	if state.client == nil {
		return ErrNotConfigured
	}

	from, err := s.settings.Get(ctx, config.SettingSMTPFrom)
	if err != nil {
		return fmt.Errorf("read the SMTP from address: %w", err)
	}

	msg := mail.NewMsg()
	if err := msg.From(from); err != nil {
		return fmt.Errorf("set the from address: %w", err)
	}

	if err := msg.To(message.To); err != nil {
		return fmt.Errorf("set the to address: %w", err)
	}

	msg.Subject(message.Subject)
	msg.SetBodyString(mail.TypeTextHTML, message.HTML)

	if err := state.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("send the email over SMTP: %w", err)
	}

	return nil
}

func (s *service) cloudflareSettings(ctx context.Context) (cloudflareCredentials, bool, error) {
	var creds cloudflareCredentials

	targets := []struct {
		def   *config.SiteSettingDef
		value *string
	}{
		{def: config.SettingCloudflareAccountID, value: &creds.accountID},
		{def: config.SettingCloudflareAPIToken, value: &creds.apiToken},
		{def: config.SettingCloudflareEmailFrom, value: &creds.from},
	}

	for _, target := range targets {
		value, err := s.settings.Get(ctx, target.def)
		if err != nil {
			return cloudflareCredentials{}, false, fmt.Errorf("read %s: %w", target.def.Key, err)
		}

		*target.value = value
	}

	configured := creds.accountID != "" && creds.apiToken != "" && creds.from != ""

	return creds, configured, nil
}

func (s *service) sendCloudflare(ctx context.Context, message Message) error {
	creds, configured, err := s.cloudflareSettings(ctx)
	if err != nil {
		return err
	}

	if !configured {
		return ErrNotConfigured
	}

	siteName, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return fmt.Errorf("read the site name: %w", err)
	}

	payload, err := json.Marshal(cloudflareSendRequest{
		To:      message.To,
		From:    cloudflareAddress{Address: creds.from, Name: siteName},
		Subject: message.Subject,
		HTML:    message.HTML,
		Text:    htmlToText(message.HTML),
	})
	if err != nil {
		return fmt.Errorf("encode the cloudflare email request: %w", err)
	}

	endpoint := fmt.Sprintf(cloudflareSendURL, url.PathEscape(creds.accountID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build the cloudflare email request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send the email through cloudflare: %w", err)
	}

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read the cloudflare email response: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close the cloudflare email response: %w", closeErr)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cloudflare refused the email with status %d: %s", resp.StatusCode, body)
	}

	var parsed cloudflareSendResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("decode the cloudflare email response: %w", err)
	}

	if !parsed.Success {
		return fmt.Errorf("cloudflare rejected the email: %s", body)
	}

	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

func htmlToText(markup string) string {
	stripped := html.UnescapeString(htmlTagPattern.ReplaceAllString(markup, ""))

	var lines []string
	for line := range strings.Lines(stripped) {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed == "" {
			continue
		}

		lines = append(lines, trimmed)
	}

	return strings.Join(lines, "\n")
}
