package email

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"
)

type (
	settingValues map[*config.SiteSettingDef]string

	recordingTransport struct {
		status  int
		body    string
		err     error
		request *http.Request
		payload []byte
	}

	fakeSMTPServer struct {
		listener net.Listener
		received chan string
		done     chan struct{}
	}
)

var (
	testMessage = Message{To: "sam@example.com", Subject: "Welcome", HTML: "<p>Hi &amp; bye</p>"}
)

func newSettings(t *testing.T, values settingValues) *settings.MockService {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, def *config.SiteSettingDef) (string, error) {
		if value, ok := values[def]; ok {
			return value, nil
		}

		return def.Default, nil
	}).Maybe()
	settingsSvc.EXPECT().GetInt(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, def *config.SiteSettingDef) (int, error) {
		value, ok := values[def]
		if !ok {
			value = def.Default
		}

		return strconv.Atoi(value)
	}).Maybe()

	return settingsSvc
}

func newTestService(t *testing.T, values settingValues) *service {
	t.Helper()

	svc, err := NewService(newSettings(t, values))
	require.NoError(t, err)

	concrete, ok := svc.(*service)
	require.True(t, ok)

	return concrete
}

func cloudflareValues() settingValues {
	return settingValues{
		config.SettingEmailProvider:       string(config.EmailProviderCloudflare),
		config.SettingCloudflareAccountID: "acct-123",
		config.SettingCloudflareAPIToken:  "secret-token",
		config.SettingCloudflareEmailFrom: "noreply@retrotide.test",
		config.SettingSiteName:            "RetroTide",
	}
}

func newCloudflareService(t *testing.T, transport *recordingTransport) *service {
	t.Helper()

	svc := newTestService(t, cloudflareValues())
	svc.httpClient = &http.Client{Transport: transport}

	return svc
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.request = req

	payload, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	if err := req.Body.Close(); err != nil {
		return nil, err
	}

	r.payload = payload

	if r.err != nil {
		return nil, r.err
	}

	return &http.Response{StatusCode: r.status, Body: io.NopCloser(strings.NewReader(r.body)), Header: make(http.Header)}, nil
}

func startFakeSMTP(t *testing.T) *fakeSMTPServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &fakeSMTPServer{listener: listener, received: make(chan string, 1), done: make(chan struct{})}
	go server.serve(t)

	t.Cleanup(func() {
		assert.NoError(t, listener.Close())
		<-server.done
	})

	return server
}

func (s *fakeSMTPServer) port() string {
	_, port, _ := strings.Cut(s.listener.Addr().String(), ":")

	return port
}

func (s *fakeSMTPServer) serve(t *testing.T) {
	defer close(s.done)

	for {
		conn, err := s.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if !assert.NoError(t, err) {
			return
		}

		s.handle(t, conn)
	}
}

func (s *fakeSMTPServer) handle(t *testing.T, conn net.Conn) {
	defer func() {
		assert.NoError(t, conn.Close())
	}()

	reader := bufio.NewReader(conn)
	reply := func(line string) bool {
		_, err := fmt.Fprintf(conn, "%s\r\n", line)

		return assert.NoError(t, err)
	}

	if !reply("220 fake.test ESMTP") {
		return
	}

	for {
		line, err := reader.ReadString('\n')
		if !assert.NoError(t, err) {
			return
		}

		command := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
			if !reply("250 fake.test") {
				return
			}
		case command == "DATA":
			if !reply("354 end with a dot") {
				return
			}

			var data strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if !assert.NoError(t, err) {
					return
				}

				if dataLine == ".\r\n" {
					break
				}

				data.WriteString(dataLine)
			}

			s.received <- data.String()

			if !reply("250 queued") {
				return
			}
		case command == "QUIT":
			reply("221 bye")

			return
		default:
			if !reply("250 ok") {
				return
			}
		}
	}
}

func TestSend_ReportsThatNoProviderIsConfigured(t *testing.T) {
	cases := []struct {
		name   string
		values settingValues
	}{
		{name: "smtp without a host", values: nil},
		{name: "cloudflare without its credentials", values: settingValues{config.SettingEmailProvider: string(config.EmailProviderCloudflare)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc := newTestService(t, tc.values)

			// when
			err := svc.Send(t.Context(), testMessage)

			// then an unsent email is never reported as sent
			require.ErrorIs(t, err, ErrNotConfigured)
		})
	}
}

func TestSendTest_ReportsThatSMTPIsUnconfigured(t *testing.T) {
	// given
	svc := newTestService(t, nil)

	// when
	err := svc.SendTest(t.Context(), testMessage)

	// then
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestSendTest_ReportsThatCloudflareIsUnconfigured(t *testing.T) {
	// given
	svc := newTestService(t, settingValues{config.SettingEmailProvider: string(config.EmailProviderCloudflare)})

	// when
	err := svc.SendTest(t.Context(), testMessage)

	// then
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestSend_DeliversOverSMTP(t *testing.T) {
	// given
	server := startFakeSMTP(t)
	svc := newTestService(t, settingValues{
		config.SettingSMTPHost: "127.0.0.1",
		config.SettingSMTPPort: server.port(),
		config.SettingSMTPFrom: "RetroTide <noreply@retrotide.test>",
	})

	// when
	err := svc.Send(t.Context(), testMessage)

	// then
	require.NoError(t, err)

	select {
	case data := <-server.received:
		assert.Contains(t, data, "Subject: Welcome")
		assert.Contains(t, data, "noreply@retrotide.test")
		assert.Contains(t, data, "sam@example.com")
		assert.Contains(t, data, "text/html")
	case <-time.After(5 * time.Second):
		t.Fatal("the fake SMTP server received no message")
	}
}

func TestSend_ReturnsTheSMTPFailure(t *testing.T) {
	// given a port nothing listens on
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, _ := strings.Cut(listener.Addr().String(), ":")
	require.NoError(t, listener.Close())

	svc := newTestService(t, settingValues{
		config.SettingSMTPHost: "127.0.0.1",
		config.SettingSMTPPort: port,
		config.SettingSMTPFrom: "noreply@retrotide.test",
	})

	// when
	err = svc.Send(t.Context(), testMessage)

	// then the failure is not treated as unconfigured
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotConfigured)
}

func TestSend_CloudflareBuildsTheRequest(t *testing.T) {
	// given
	transport := &recordingTransport{status: http.StatusOK, body: `{"success":true,"errors":[],"messages":[],"result":{"delivered":["sam@example.com"]}}`}
	svc := newCloudflareService(t, transport)

	// when
	err := svc.Send(t.Context(), testMessage)

	// then
	require.NoError(t, err)
	require.NotNil(t, transport.request)
	assert.Equal(t, http.MethodPost, transport.request.Method)
	assert.Equal(t, "https://api.cloudflare.com/client/v4/accounts/acct-123/email/sending/send", transport.request.URL.String())
	assert.Equal(t, "Bearer secret-token", transport.request.Header.Get("Authorization"))
	assert.Equal(t, "application/json", transport.request.Header.Get("Content-Type"))

	var payload cloudflareSendRequest
	require.NoError(t, json.Unmarshal(transport.payload, &payload))
	assert.Equal(t, cloudflareSendRequest{
		To:      "sam@example.com",
		From:    cloudflareAddress{Address: "noreply@retrotide.test", Name: "RetroTide"},
		Subject: "Welcome",
		HTML:    "<p>Hi &amp; bye</p>",
		Text:    "Hi & bye",
	}, payload)
}

func TestSend_CloudflareFailures(t *testing.T) {
	// given
	cases := []struct {
		name      string
		transport *recordingTransport
		want      string
	}{
		{name: "an error status", transport: &recordingTransport{status: http.StatusUnauthorized, body: `{"success":false,"errors":[{"code":10000,"message":"bad token"}]}`}, want: "401"},
		{name: "a refusal inside a 200", transport: &recordingTransport{status: http.StatusOK, body: `{"success":false,"errors":[{"code":1000,"message":"domain not verified"}]}`}, want: "rejected"},
		{name: "a body that does not decode", transport: &recordingTransport{status: http.StatusOK, body: `not json`}, want: "decode"},
		{name: "a transport failure", transport: &recordingTransport{err: errors.New("connection refused")}, want: "connection refused"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newCloudflareService(t, tc.transport)

			// when
			err := svc.Send(t.Context(), testMessage)

			// then
			require.Error(t, err)
			assert.NotErrorIs(t, err, ErrNotConfigured)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestEnabled(t *testing.T) {
	// given
	cases := []struct {
		name   string
		values settingValues
		want   bool
	}{
		{name: "smtp with a host", values: settingValues{config.SettingSMTPHost: "127.0.0.1"}, want: true},
		{name: "smtp without a host", values: nil, want: false},
		{name: "cloudflare with every value", values: cloudflareValues(), want: true},
		{name: "cloudflare without a token", values: settingValues{
			config.SettingEmailProvider:       string(config.EmailProviderCloudflare),
			config.SettingCloudflareAccountID: "acct-123",
			config.SettingCloudflareEmailFrom: "noreply@retrotide.test",
		}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(t, tc.values)

			// when
			enabled, err := svc.Enabled(t.Context())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, enabled)
		})
	}
}

func TestBuildSMTP_ChoosesTheTLSPolicy(t *testing.T) {
	// given
	cases := []struct {
		name   string
		values settingValues
		want   string
	}{
		{name: "a loopback ip may stay in cleartext", values: settingValues{config.SettingSMTPHost: "127.0.0.1"}, want: mail.NoTLS.String()},
		{name: "an ipv6 loopback may stay in cleartext", values: settingValues{config.SettingSMTPHost: "::1"}, want: mail.NoTLS.String()},
		{name: "localhost may stay in cleartext", values: settingValues{config.SettingSMTPHost: "localhost"}, want: mail.NoTLS.String()},
		{name: "a remote host without credentials uses tls when offered", values: settingValues{config.SettingSMTPHost: "smtp.example.com"}, want: mail.TLSOpportunistic.String()},
		{name: "credentials keep tls mandatory", values: settingValues{
			config.SettingSMTPHost:     "smtp.example.com",
			config.SettingSMTPUsername: "retrotide",
			config.SettingSMTPPassword: "secret",
		}, want: mail.TLSMandatory.String()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			svc := newTestService(t, tc.values)

			// then
			require.NotNil(t, svc.smtp.client)
			assert.Equal(t, tc.want, svc.smtp.client.TLSPolicy())
		})
	}
}

func TestOnSettingChanged_RebuildsTheClientForAnSMTPKey(t *testing.T) {
	// given an unconfigured service whose host is then set
	values := settingValues{}
	svc := newTestService(t, values)
	values[config.SettingSMTPHost] = "127.0.0.1"

	// when
	svc.OnSettingChanged(config.SettingSMTPHost.Key, "127.0.0.1")

	// then
	enabled, err := svc.Enabled(t.Context())
	require.NoError(t, err)
	assert.True(t, enabled)
}

func TestOnSettingChanged_IgnoresOtherKeys(t *testing.T) {
	// given
	values := settingValues{}
	svc := newTestService(t, values)
	values[config.SettingSMTPHost] = "127.0.0.1"

	// when
	svc.OnSettingChanged(config.SettingSiteName.Key, "Renamed")

	// then the client was not rebuilt
	enabled, err := svc.Enabled(t.Context())
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestOnSettingChanged_AnInvalidPortFailsEverySendUntilFixed(t *testing.T) {
	// given
	values := settingValues{config.SettingSMTPHost: "127.0.0.1"}
	svc := newTestService(t, values)
	values[config.SettingSMTPPort] = "0"

	// when
	svc.OnSettingChanged(config.SettingSMTPPort.Key, "0")

	// then email counts as unavailable, the status says it is failing, and a send carries the cause
	enabled, enabledErr := svc.Enabled(t.Context())
	require.NoError(t, enabledErr)
	assert.False(t, enabled)

	status, statusErr := svc.Status(t.Context())
	require.NoError(t, statusErr)
	assert.Equal(t, StatusFailing, status)

	sendErr := svc.Send(t.Context(), testMessage)
	require.ErrorIs(t, sendErr, ErrFailing)
	require.ErrorIs(t, sendErr, mail.ErrInvalidPort)
}

func TestNewService_AnInvalidSMTPConfigBootsWithEmailUnavailable(t *testing.T) {
	// given a host with a port the client refuses
	values := settingValues{config.SettingSMTPHost: "127.0.0.1", config.SettingSMTPPort: "0"}

	// when
	svc := newTestService(t, values)

	// then the service still starts, and reports the failing state rather than an error
	enabled, enabledErr := svc.Enabled(t.Context())
	require.NoError(t, enabledErr)
	assert.False(t, enabled)

	status, statusErr := svc.Status(t.Context())
	require.NoError(t, statusErr)
	assert.Equal(t, StatusFailing, status)
	require.ErrorIs(t, svc.smtp.err, mail.ErrInvalidPort)

	testErr := svc.SendTest(t.Context(), testMessage)
	require.ErrorIs(t, testErr, ErrFailing)
}

func TestOnSettingChanged_AFixedPortMakesEmailReadyAgain(t *testing.T) {
	// given a failing SMTP config
	values := settingValues{config.SettingSMTPHost: "127.0.0.1", config.SettingSMTPPort: "0"}
	svc := newTestService(t, values)
	values[config.SettingSMTPPort] = "2525"

	// when
	svc.OnSettingChanged(config.SettingSMTPPort.Key, "2525")

	// then
	status, err := svc.Status(t.Context())
	require.NoError(t, err)
	assert.Equal(t, StatusReady, status)
}

func TestStatus(t *testing.T) {
	// given
	cases := []struct {
		name   string
		values settingValues
		want   Status
	}{
		{name: "smtp with a host", values: settingValues{config.SettingSMTPHost: "127.0.0.1"}, want: StatusReady},
		{name: "smtp without a host", values: nil, want: StatusOff},
		{name: "smtp with a host and a port the client refuses", values: settingValues{config.SettingSMTPHost: "127.0.0.1", config.SettingSMTPPort: "0"}, want: StatusFailing},
		{name: "cloudflare with every value", values: cloudflareValues(), want: StatusReady},
		{name: "cloudflare without a token", values: settingValues{
			config.SettingEmailProvider:       string(config.EmailProviderCloudflare),
			config.SettingCloudflareAccountID: "acct-123",
			config.SettingCloudflareEmailFrom: "noreply@retrotide.test",
		}, want: StatusOff},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(t, tc.values)

			// when
			status, err := svc.Status(t.Context())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, status)
		})
	}
}

func TestStatus_ReportsAProviderThatCannotBeRead(t *testing.T) {
	// given
	readErr := errors.New("settings unavailable")
	svc := newTestService(t, nil)
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingEmailProvider).Return("", readErr)
	svc.settings = settingsSvc

	// when
	status, err := svc.Status(t.Context())

	// then
	require.ErrorIs(t, err, readErr)
	assert.Empty(t, status)
}

func TestNewService_ReturnsTheSettingsReadError(t *testing.T) {
	// given
	readErr := errors.New("settings unavailable")
	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingSMTPHost).Return("", readErr)

	// when
	svc, err := NewService(settingsSvc)

	// then
	require.ErrorIs(t, err, readErr)
	assert.Nil(t, svc)
}

func TestHTMLToText(t *testing.T) {
	// given
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "tags are stripped", in: "<p>Hello <b>world</b></p>", want: "Hello world"},
		{name: "entities are decoded", in: "<p>Tom &amp; Jerry &lt;3 &#39;hi&#39;</p>", want: "Tom & Jerry <3 'hi'"},
		{name: "a non-breaking space becomes a space", in: "a&nbsp;b", want: "a b"},
		{name: "blank lines and indentation are dropped", in: "<table>\n  <tr>\n    <td>  First  </td>\n\n  </tr>\n  <tr><td>Second</td></tr>\n</table>", want: "First\nSecond"},
		{name: "plain text is unchanged", in: "no tags here", want: "no tags here"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := htmlToText(tc.in)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
