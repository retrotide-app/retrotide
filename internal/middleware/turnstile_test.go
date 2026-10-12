package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	siteverifyFake struct {
		t      *testing.T
		status int
		answer string
		mu     sync.Mutex
		forms  []url.Values
	}

	turnstileSettings struct {
		enabled bool
		secret  string
	}
)

const (
	testTurnstileSecret = "s3cret"
	verifiedBody        = `{"email":"beatrice@example.com","password":"x","turnstile_token":"tok"}`
	successAnswer       = `{"success":true,"error-codes":[],"hostname":"retrotide.test"}`
)

func newSiteverify(t *testing.T, status int, answer string) (*siteverifyFake, string) {
	t.Helper()

	fake := &siteverifyFake{t: t, status: status, answer: answer}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(srv.Close)

	return fake, srv.URL
}

func (f *siteverifyFake) serve(w http.ResponseWriter, r *http.Request) {
	parseErr := r.ParseForm()

	f.mu.Lock()
	f.forms = append(f.forms, r.PostForm)
	f.mu.Unlock()

	if parseErr != nil {
		f.t.Errorf("siteverify received a form it could not parse: %v", parseErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.status)
	if _, err := io.WriteString(w, f.answer); err != nil {
		f.t.Errorf("siteverify could not answer: %v", err)
	}
}

func (f *siteverifyFake) received() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.forms)
}

func turnstileEngine(t *testing.T, cfg turnstileSettings, verifyURL string) (*gin.Engine, *string) {
	t.Helper()

	svc := settings.NewMockService(t)
	svc.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(cfg.enabled, nil).Maybe()
	svc.EXPECT().Get(mock.Anything, config.SettingTurnstileSecretKey).Return(cfg.secret, nil).Maybe()

	email := new(string)

	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit * 16).Handle)
	engine.POST("/login", RequireTurnstile(svc, verifyURL), func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
		}
		require.NoError(t, c.ShouldBindBodyWithJSON(&req))
		*email = req.Email
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return engine, email
}

func postJSON(t *testing.T, engine *gin.Engine, target string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	return serve(engine, req)
}

func TestRequireTurnstile_DoesNothingWhileSwitchedOff(t *testing.T) {
	// given the default settings, and a secret that must not be read
	fake, verifyURL := newSiteverify(t, http.StatusOK, successAnswer)
	svc := settings.NewMockService(t)
	svc.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(false, nil).Once()

	engine := newTestEngine(t)
	engine.POST("/login", RequireTurnstile(svc, verifyURL), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// when
	rec := postJSON(t, engine, "/login", `{"email":"beatrice@example.com"}`)

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, fake.received())
}

func TestRequireTurnstile_LetsAVerifiedRequestThroughWithItsBody(t *testing.T) {
	// given
	fake, verifyURL := newSiteverify(t, http.StatusOK, successAnswer)
	engine, email := turnstileEngine(t, turnstileSettings{enabled: true, secret: testTurnstileSecret}, verifyURL)

	// when
	rec := postJSON(t, engine, "/login", verifiedBody)

	// then the handler still reads the body the guard peeked at, and Cloudflare was sent the secret and the token
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "beatrice@example.com", *email)

	forms := fake.received()
	require.Len(t, forms, 1)
	assert.Equal(t, testTurnstileSecret, forms[0].Get("secret"))
	assert.Equal(t, "tok", forms[0].Get("response"))
}

func TestRequireTurnstile_RefusesARequestWithoutAToken(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "no body", body: ""},
		{name: "no token", body: `{"email":"beatrice@example.com"}`},
		{name: "an empty token", body: `{"email":"beatrice@example.com","turnstile_token":""}`},
		{name: "a token that is not a string", body: `{"turnstile_token":42}`},
		{name: "a body that is not json", body: "not json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake, verifyURL := newSiteverify(t, http.StatusOK, successAnswer)
			engine, _ := turnstileEngine(t, turnstileSettings{enabled: true, secret: testTurnstileSecret}, verifyURL)

			// when
			rec := postJSON(t, engine, "/login", tc.body)

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"verification required"}`, rec.Body.String())
			assert.Empty(t, fake.received())
		})
	}
}

func TestRequireTurnstile_RefusesAFailedChallenge(t *testing.T) {
	tests := []struct {
		name   string
		answer string
	}{
		{name: "a forged token", answer: `{"success":false,"error-codes":["invalid-input-response"]}`},
		{name: "a replayed token", answer: `{"success":false,"error-codes":["timeout-or-duplicate"]}`},
		{name: "no reason given", answer: `{"success":false}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			_, verifyURL := newSiteverify(t, http.StatusOK, tc.answer)
			engine, email := turnstileEngine(t, turnstileSettings{enabled: true, secret: testTurnstileSecret}, verifyURL)

			// when
			rec := postJSON(t, engine, "/login", verifiedBody)

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"verification failed, please try again"}`, rec.Body.String())
			assert.Empty(t, *email)
		})
	}
}

func TestRequireTurnstile_AnswersUnavailableWhenVerificationCannotRun(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		status int
		answer string
	}{
		{name: "switched on without a secret", secret: "", status: http.StatusOK, answer: successAnswer},
		{name: "siteverify fails", secret: testTurnstileSecret, status: http.StatusInternalServerError, answer: `{}`},
		{name: "siteverify answers something that is not json", secret: testTurnstileSecret, status: http.StatusOK, answer: "<html>"},
		{name: "the secret is wrong", secret: testTurnstileSecret, status: http.StatusOK, answer: `{"success":false,"error-codes":["invalid-input-secret"]}`},
		{name: "the secret is missing on Cloudflare's side", secret: testTurnstileSecret, status: http.StatusOK, answer: `{"success":false,"error-codes":["missing-input-secret"]}`},
		{name: "Cloudflare failed internally", secret: testTurnstileSecret, status: http.StatusOK, answer: `{"success":false,"error-codes":["internal-error"]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			_, verifyURL := newSiteverify(t, tc.status, tc.answer)
			engine, email := turnstileEngine(t, turnstileSettings{enabled: true, secret: tc.secret}, verifyURL)

			// when
			rec := postJSON(t, engine, "/login", verifiedBody)

			// then our own failure is never passed off as the visitor's
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, `{"error":"verification is temporarily unavailable, please try again later"}`, rec.Body.String())
			assert.Empty(t, *email)
		})
	}
}

func TestRequireTurnstile_AnswersUnavailableWhenSiteverifyIsUnreachable(t *testing.T) {
	// given a siteverify address that no longer answers
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	engine, email := turnstileEngine(t, turnstileSettings{enabled: true, secret: testTurnstileSecret}, srv.URL)

	// when
	rec := postJSON(t, engine, "/login", verifiedBody)

	// then
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Empty(t, *email)
}

func TestRequireTurnstile_RefusesWhenASettingCannotBeRead(t *testing.T) {
	tests := []struct {
		name      string
		enableErr error
		secretErr error
	}{
		{name: "the switch", enableErr: errors.New("settings down")},
		{name: "the secret", secretErr: errors.New("settings down")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fake, verifyURL := newSiteverify(t, http.StatusOK, successAnswer)
			svc := settings.NewMockService(t)
			svc.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(true, tc.enableErr).Once()
			svc.EXPECT().Get(mock.Anything, config.SettingTurnstileSecretKey).Return(testTurnstileSecret, tc.secretErr).Maybe()

			engine := newTestEngine(t)
			engine.POST("/login", RequireTurnstile(svc, verifyURL), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			// when
			rec := postJSON(t, engine, "/login", verifiedBody)

			// then a failed read is never taken to mean the check is off
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.Empty(t, fake.received())
		})
	}
}

func TestRequireTurnstile_AnswersAnOversizedBodyWith413(t *testing.T) {
	// given a body over the limit, whose length is not declared
	fake, verifyURL := newSiteverify(t, http.StatusOK, successAnswer)
	svc := settings.NewMockService(t)
	svc.EXPECT().GetBool(mock.Anything, config.SettingTurnstileEnabled).Return(true, nil).Once()
	svc.EXPECT().Get(mock.Anything, config.SettingTurnstileSecretKey).Return(testTurnstileSecret, nil).Once()

	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit).Handle)
	engine.POST("/login", RequireTurnstile(svc, verifyURL), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", io.MultiReader(strings.NewReader(verifiedBody)))
	req.Header.Set("Content-Type", "application/json")

	// when
	rec := serve(engine, req)

	// then
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.JSONEq(t, `{"error":"request body too large"}`, rec.Body.String())
	assert.Empty(t, fake.received())
}
