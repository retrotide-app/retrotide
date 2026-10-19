package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/guest"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	Harness struct {
		T               *testing.T
		Engine          *gin.Engine
		SettingsService *settings.MockService
		SessionRepo     *repository.MockSessionRepository
		GuestRepo       *repository.MockGuestDeviceRepository
		Authz           *authz.MockService
		OrgAuthz        *orgauthz.MockService
		Sessions        *session.Manager
		Guests          *guest.Manager
		Guards          *middleware.Guards
	}

	StreamRecorder struct {
		*httptest.ResponseRecorder

		WriteDeadline time.Time
	}

	Request struct {
		h          *Harness
		method     string
		path       string
		body       io.Reader
		headers    map[string]string
		cookies    []*http.Cookie
		remoteAddr string
	}
)

const (
	TestBaseURL         = "https://retrotide.test"
	TestInsecureBaseURL = "http://localhost:4323"
	TestSessionToken    = "4f1c9a7e2b8d6f3a5c0e9b7d1f3a5c7e9b1d3f5a7c9e1b3d5f7a9c1e3b5d7f9a"
	TestSessionDays     = 30
	SessionCookieName   = "__Host-rt_session"
	TestGuestToken      = "7d2e9b4a1c6f8e3d5b0a2c4e6f8a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f5a"
	TestGuestDays       = 90
	GuestCookieName     = "__Host-rt_guest"
)

func NewHarness(t *testing.T) *Harness {
	t.Helper()

	settingsService := settings.NewMockService(t)
	sessionRepo := repository.NewMockSessionRepository(t)
	guestRepo := repository.NewMockGuestDeviceRepository(t)
	authzService := authz.NewMockService(t)
	orgAuthzService := orgauthz.NewMockService(t)
	sessions := session.NewManager(sessionRepo, settingsService)
	guests := guest.NewManager(guestRepo, settingsService)

	engine, err := utils.NewEngine(utils.DefaultTrustedProxies, config.DeploymentHosted)
	require.NoError(t, err)

	return &Harness{
		T:               t,
		Engine:          engine,
		SettingsService: settingsService,
		SessionRepo:     sessionRepo,
		GuestRepo:       guestRepo,
		Authz:           authzService,
		OrgAuthz:        orgAuthzService,
		Sessions:        sessions,
		Guests:          guests,
		Guards:          middleware.NewGuards(middleware.GuardDeps{Settings: settingsService, Sessions: sessions, Guests: guests, Authz: authzService, OrgAuthz: orgAuthzService}),
	}
}

func (h *Harness) UseBaseURL(baseURL string) {
	h.SettingsService.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(baseURL, nil).Maybe()
	h.SettingsService.EXPECT().GetInt(mock.Anything, config.SettingSessionDurationDays).Return(TestSessionDays, nil).Maybe()
	h.SettingsService.EXPECT().GetInt(mock.Anything, config.SettingGuestDeviceDays).Return(TestGuestDays, nil).Maybe()
}

func (h *Harness) SignIn(userID uuid.UUID) *http.Cookie {
	h.SessionRepo.EXPECT().Get(mock.Anything, TestSessionToken).Return(&model.Session{UserID: userID}, nil)
	h.Authz.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)

	return &http.Cookie{Name: SessionCookieName, Value: TestSessionToken}
}

func (h *Harness) SignInAsGuest(deviceID uuid.UUID) *http.Cookie {
	h.GuestRepo.EXPECT().Get(mock.Anything, TestGuestToken).Return(&model.GuestDevice{ID: deviceID, LastSeenAt: time.Now()}, nil)

	return &http.Cookie{Name: GuestCookieName, Value: TestGuestToken}
}

func (h *Harness) NewRequest(method, path string) *Request {
	return &Request{h: h, method: method, path: path, headers: map[string]string{}}
}

func (r *Request) WithJSONBody(v any) *Request {
	data, err := json.Marshal(v)
	require.NoError(r.h.T, err)

	r.body = bytes.NewReader(data)
	r.headers["Content-Type"] = "application/json"

	return r
}

func (r *Request) WithRawBody(s string, contentType string) *Request {
	r.body = strings.NewReader(s)
	if contentType != "" {
		r.headers["Content-Type"] = contentType
	}

	return r
}

func (r *Request) WithHeader(k, v string) *Request {
	r.headers[k] = v

	return r
}

func (r *Request) WithCookie(cookie *http.Cookie) *Request {
	r.cookies = append(r.cookies, cookie)

	return r
}

func (r *Request) WithRemoteAddr(addr string) *Request {
	r.remoteAddr = addr

	return r
}

func (r *Request) Record() *httptest.ResponseRecorder {
	r.h.T.Helper()

	rec := httptest.NewRecorder()
	r.h.Engine.ServeHTTP(rec, r.build())

	return rec
}

func (r *Request) RecordStream() *StreamRecorder {
	r.h.T.Helper()

	rec := &StreamRecorder{ResponseRecorder: httptest.NewRecorder()}
	r.h.Engine.ServeHTTP(rec, r.build())

	return rec
}

func (r *Request) build() *http.Request {
	req := httptest.NewRequestWithContext(r.h.T.Context(), r.method, r.path, r.body)
	for name, value := range r.headers {
		req.Header.Set(name, value)
	}

	for _, cookie := range r.cookies {
		req.AddCookie(cookie)
	}

	if r.remoteAddr != "" {
		req.RemoteAddr = r.remoteAddr
	}

	return req
}

func (r *StreamRecorder) SetWriteDeadline(deadline time.Time) error {
	r.WriteDeadline = deadline

	return nil
}

func (r *Request) Do() (int, []byte) {
	r.h.T.Helper()

	rec := r.Record()

	return rec.Code, rec.Body.Bytes()
}

func ResponseCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	require.Failf(t, "cookie not set", "the response set no %s cookie", name)

	return nil
}

func UnmarshalJSON[T any](t *testing.T, body []byte) T {
	t.Helper()

	var v T
	require.NoError(t, json.Unmarshal(body, &v))

	return v
}
