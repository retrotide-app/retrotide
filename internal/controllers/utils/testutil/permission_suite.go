package testutil

import (
	"net/http"
	"slices"
	"testing"

	"retrotide/internal/authz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type (
	PermissionRoute struct {
		AuthRoute

		Permission authz.Permission
	}
)

func RunPermissionFailureSuite(t *testing.T, newHarness func(t *testing.T) *Harness, route PermissionRoute) {
	t.Helper()

	failures := slices.Concat(authFailures, []authFailure{
		{
			name:    "a user without the permission",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, userID uuid.UUID) *http.Cookie {
				cookie := h.signInPastTheGates(route.AuthRoute, userID)
				h.Authz.EXPECT().Can(mock.Anything, userID, route.Permission).Return(false, nil)

				return cookie
			},
			status: http.StatusForbidden,
			body:   `{"error":"insufficient permissions"}`,
		},
		{
			name:    "a permission store that cannot answer",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, userID uuid.UUID) *http.Cookie {
				cookie := h.signInPastTheGates(route.AuthRoute, userID)
				h.Authz.EXPECT().Can(mock.Anything, userID, route.Permission).Return(false, errRoleStoreDown)

				return cookie
			},
			status: http.StatusServiceUnavailable,
			body:   `{"error":"service unavailable"}`,
		},
	})

	runFailures(t, newHarness, route.AuthRoute, failures)
}

func (h *Harness) signInPastTheGates(route AuthRoute, userID uuid.UUID) *http.Cookie {
	cookie := h.SignIn(userID)

	if isWrite(route.Method) && !route.LockExempt {
		h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	}

	return cookie
}
