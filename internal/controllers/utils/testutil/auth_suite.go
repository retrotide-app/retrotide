package testutil

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	AuthRoute struct {
		Method     string
		Path       string
		Body       any
		LockExempt bool
	}

	authFailure struct {
		name    string
		applies func(route AuthRoute) bool
		arrange func(h *Harness, userID uuid.UUID) *http.Cookie
		status  int
		body    string
	}
)

var (
	authFailures = []authFailure{
		{
			name:    "no session",
			applies: func(AuthRoute) bool { return true },
			arrange: func(*Harness, uuid.UUID) *http.Cookie { return nil },
			status:  http.StatusUnauthorized,
			body:    `{"error":"authentication required"}`,
		},
		{
			name:    "an unknown or expired session",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, _ uuid.UUID) *http.Cookie {
				h.SessionRepo.EXPECT().Get(mock.Anything, TestSessionToken).Return(nil, fmt.Errorf("get session: %w", dao.ErrNotFound))

				return sessionCookie()
			},
			status: http.StatusUnauthorized,
			body:   `{"error":"invalid or expired session"}`,
		},
		{
			name:    "a banned user, whose session is revoked",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, userID uuid.UUID) *http.Cookie {
				h.SessionRepo.EXPECT().Get(mock.Anything, TestSessionToken).Return(&model.Session{UserID: userID}, nil)
				h.Authz.EXPECT().IsBanned(mock.Anything, userID).Return(true, nil)
				h.SessionRepo.EXPECT().Delete(mock.Anything, TestSessionToken).Return(nil)

				return sessionCookie()
			},
			status: http.StatusForbidden,
			body:   `{"error":"your account has been banned"}`,
		},
		{
			name:    "a session store that cannot answer",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, _ uuid.UUID) *http.Cookie {
				h.SessionRepo.EXPECT().Get(mock.Anything, TestSessionToken).Return(nil, errSessionStoreDown)

				return sessionCookie()
			},
			status: http.StatusServiceUnavailable,
			body:   `{"error":"service unavailable"}`,
		},
		{
			name: "a locked user",
			applies: func(route AuthRoute) bool {
				return isWrite(route.Method) && !route.LockExempt
			},
			arrange: func(h *Harness, userID uuid.UUID) *http.Cookie {
				h.SessionRepo.EXPECT().Get(mock.Anything, TestSessionToken).Return(&model.Session{UserID: userID}, nil)
				h.Authz.EXPECT().IsBanned(mock.Anything, userID).Return(false, nil)
				h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(true, nil)

				return sessionCookie()
			},
			status: http.StatusForbidden,
			body:   `{"error":"your account is locked"}`,
		},
	}

	principalFailures = []authFailure{
		{
			name:    "an unknown or expired guest device",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, _ uuid.UUID) *http.Cookie {
				h.GuestRepo.EXPECT().Get(mock.Anything, TestGuestToken).Return(nil, fmt.Errorf("get guest device: %w", dao.ErrNotFound))

				return guestCookie()
			},
			status: http.StatusUnauthorized,
			body:   `{"error":"invalid or expired session"}`,
		},
		{
			name:    "a guest device store that cannot answer",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, _ uuid.UUID) *http.Cookie {
				h.GuestRepo.EXPECT().Get(mock.Anything, TestGuestToken).Return(nil, errSessionStoreDown)

				return guestCookie()
			},
			status: http.StatusServiceUnavailable,
			body:   `{"error":"service unavailable"}`,
		},
	}
)

func RunAuthFailureSuite(t *testing.T, newHarness func(t *testing.T) *Harness, route AuthRoute) {
	t.Helper()

	runFailures(t, newHarness, route, authFailures)
}

func RunPrincipalFailureSuite(t *testing.T, newHarness func(t *testing.T) *Harness, route AuthRoute) {
	t.Helper()

	runFailures(t, newHarness, route, append(slices.Clone(authFailures), principalFailures...))
}

func runFailures(t *testing.T, newHarness func(t *testing.T) *Harness, route AuthRoute, failures []authFailure) {
	t.Helper()

	for _, failure := range failures {
		if !failure.applies(route) {
			continue
		}

		t.Run(failure.name, func(t *testing.T) {
			// given
			h := newHarness(t)
			cookie := failure.arrange(h, uuid.New())

			request := h.NewRequest(route.Method, route.Path)
			if route.Body != nil {
				request = request.WithJSONBody(route.Body)
			}

			if cookie != nil {
				request = request.WithCookie(cookie)
			}

			// when
			rec := request.Record()

			// then
			assert.Equal(t, failure.status, rec.Code)
			assert.JSONEq(t, failure.body, rec.Body.String())
		})
	}
}

func sessionCookie() *http.Cookie {
	return &http.Cookie{Name: SessionCookieName, Value: TestSessionToken}
}

func guestCookie() *http.Cookie {
	return &http.Cookie{Name: GuestCookieName, Value: TestGuestToken}
}

func isWrite(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}
