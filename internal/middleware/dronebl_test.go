package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dronebl"
	"retrotide/internal/model"
	"retrotide/internal/repository"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	listedPeer         = "198.51.100.7:4444"
	listedClientIP     = "198.51.100.7"
	listedSessionToken = "a-session-token"
	htmlAccept         = "text/html,application/xhtml+xml"
)

var (
	listedVerdict = dronebl.Verdict{Blocked: true, Classes: []int{13}}
)

func droneblEngine(t *testing.T, checker dronebl.Service, sessions repository.SessionRepository) *gin.Engine {
	t.Helper()

	settingsSvc := settings.NewMockService(t)
	settingsSvc.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("http://"+testHost, nil).Maybe()

	engine := newTestEngine(t)
	engine.Use(RequireCleanIP(checker, session.NewManager(sessions, settingsSvc)))
	engine.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "handler reached")
	})

	return engine
}

func getFromListedPeer(t *testing.T, engine *gin.Engine, target string, accept string, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	req.RemoteAddr = listedPeer
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	if token != "" {
		req.AddCookie(&http.Cookie{Name: "rt_session", Value: token})
	}

	return serve(engine, req)
}

func listedChecker(t *testing.T) *dronebl.MockService {
	t.Helper()

	checker := dronebl.NewMockService(t)
	checker.EXPECT().Check(mock.Anything, listedClientIP).Return(listedVerdict, nil).Once()

	return checker
}

func TestRequireCleanIP_AnswersABlockedNavigationWithAPage(t *testing.T) {
	// given a listed visitor with no session
	engine := droneblEngine(t, listedChecker(t), repository.NewMockSessionRepository(t))

	// when they ask for a page
	rec := getFromListedPeer(t, engine, "/dashboard", htmlAccept, "")

	// then they get the blocked page, not the site
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "<title>Access blocked</title>")
	assert.Contains(t, rec.Body.String(), `href="/login"`)
	assert.NotContains(t, rec.Body.String(), "handler reached")
}

func TestRequireCleanIP_AnswersEverythingButANavigationWithJSON(t *testing.T) {
	tests := []struct {
		name   string
		target string
		accept string
	}{
		{name: "an api call, even one that accepts html", target: "/api/v1/boards", accept: htmlAccept},
		{name: "a page asked for without html", target: "/dashboard", accept: "application/json"},
		{name: "a page asked for with no accept header", target: "/dashboard", accept: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := droneblEngine(t, listedChecker(t), repository.NewMockSessionRepository(t))

			// when
			rec := getFromListedPeer(t, engine, tc.target, tc.accept, "")

			// then an API caller is never handed a page to parse
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.JSONEq(t, `{"error":"your network is listed on a public abuse blocklist"}`, rec.Body.String())
		})
	}
}

func TestRequireCleanIP_LetsACleanAddressThrough(t *testing.T) {
	// given a clean address, and a session store that must not be asked
	checker := dronebl.NewMockService(t)
	checker.EXPECT().Check(mock.Anything, listedClientIP).Return(dronebl.Verdict{}, nil).Once()
	engine := droneblEngine(t, checker, repository.NewMockSessionRepository(t))

	// when
	rec := getFromListedPeer(t, engine, "/dashboard", htmlAccept, listedSessionToken)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "handler reached", rec.Body.String())
}

func TestRequireCleanIP_LetsASignedInMemberThrough(t *testing.T) {
	// given a listed address whose visitor holds a live session
	sessions := repository.NewMockSessionRepository(t)
	sessions.EXPECT().Get(mock.Anything, listedSessionToken).Return(&model.Session{UserID: uuid.New()}, nil).Once()
	engine := droneblEngine(t, listedChecker(t), sessions)

	// when
	rec := getFromListedPeer(t, engine, "/api/v1/boards", "", listedSessionToken)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCleanIP_BlocksASessionThatIsNoLongerValid(t *testing.T) {
	// given a listed address presenting an unknown or expired session
	sessions := repository.NewMockSessionRepository(t)
	sessions.EXPECT().Get(mock.Anything, listedSessionToken).Return(nil, fmt.Errorf("session: %w", dao.ErrNotFound)).Once()
	engine := droneblEngine(t, listedChecker(t), sessions)

	// when
	rec := getFromListedPeer(t, engine, "/api/v1/boards", "", listedSessionToken)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCleanIP_AnswersUnavailableWhenItCannotDecide(t *testing.T) {
	tests := []struct {
		name     string
		checker  func(t *testing.T) *dronebl.MockService
		sessions func(t *testing.T) *repository.MockSessionRepository
	}{
		{
			name: "the check fails",
			checker: func(t *testing.T) *dronebl.MockService {
				checker := dronebl.NewMockService(t)
				checker.EXPECT().Check(mock.Anything, listedClientIP).Return(dronebl.Verdict{}, errors.New("settings down")).Once()

				return checker
			},
			sessions: func(t *testing.T) *repository.MockSessionRepository {
				return repository.NewMockSessionRepository(t)
			},
		},
		{
			name:    "the session cannot be read",
			checker: listedChecker,
			sessions: func(t *testing.T) *repository.MockSessionRepository {
				sessions := repository.NewMockSessionRepository(t)
				sessions.EXPECT().Get(mock.Anything, listedSessionToken).Return(nil, errors.New("db down")).Once()

				return sessions
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := droneblEngine(t, tc.checker(t), tc.sessions(t))

			// when
			rec := getFromListedPeer(t, engine, "/api/v1/boards", "", listedSessionToken)

			// then a failure is never taken to mean clean or signed in
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.JSONEq(t, `{"error":"service unavailable"}`, rec.Body.String())
		})
	}
}

func TestRequireCleanIP_LeavesAWayToSignIn(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "the login page", target: "/login"},
		{name: "the login page in another case", target: "/Login"},
		{name: "the login endpoint", target: "/api/v1/auth/login"},
		{name: "the session endpoint", target: "/api/v1/auth/session"},
		{name: "site info, which the shell needs to boot", target: "/api/v1/site-info"},
		{name: "a script bundle", target: "/assets/index-a1b2c3.js"},
		{name: "a stylesheet", target: "/assets/index-a1b2c3.css"},
		{name: "a file at the root", target: "/favicon.ico"},
		{name: "the liveness probe", target: "/livez"},
		{name: "the health check", target: "/health"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a checker with no expectation, so consulting it fails the test
			engine := droneblEngine(t, dronebl.NewMockService(t), repository.NewMockSessionRepository(t))

			// when a listed visitor whose session has expired asks for it
			rec := getFromListedPeer(t, engine, tc.target, htmlAccept, "")

			// then
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestRequireCleanIP_StillBlocksTheRest(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "registration, which is how a raid starts", target: "/api/v1/auth/register"},
		{name: "the registration page", target: "/register"},
		{name: "forgotten password, which sends mail", target: "/api/v1/auth/forgot-password"},
		{name: "a board link", target: "/b/ABCDEFGHIJKLMNOP"},
		{name: "an api path with a dot in it", target: "/api/v1/exports/board.csv"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := droneblEngine(t, listedChecker(t), repository.NewMockSessionRepository(t))

			// when
			rec := getFromListedPeer(t, engine, tc.target, htmlAccept, "")

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}
