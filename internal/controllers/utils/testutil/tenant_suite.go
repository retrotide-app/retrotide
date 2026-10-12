package testutil

import (
	"net/http"
	"slices"
	"testing"

	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type (
	OrgPermissionRoute struct {
		AuthRoute

		OrgID      uuid.UUID
		Permission orgauthz.Permission
	}

	TeamPermissionRoute struct {
		AuthRoute

		TeamID     uuid.UUID
		Permission orgauthz.Permission
	}

	tenantDecision func(h *Harness, userID uuid.UUID, decision orgauthz.Decision, err error)
)

const (
	organisationNotFoundBody = `{"error":"organisation not found"}`
	teamNotFoundBody         = `{"error":"team not found"}`
)

func RunOrgPermissionFailureSuite(t *testing.T, newHarness func(t *testing.T) *Harness, route OrgPermissionRoute) {
	t.Helper()

	runFailures(t, newHarness, route.AuthRoute, tenantFailures(route.AuthRoute, organisationNotFoundBody, route.expect))
}

func RunTeamPermissionFailureSuite(t *testing.T, newHarness func(t *testing.T) *Harness, route TeamPermissionRoute) {
	t.Helper()

	runFailures(t, newHarness, route.AuthRoute, tenantFailures(route.AuthRoute, teamNotFoundBody, route.expect))
}

func (h *Harness) SignInToOrg(route OrgPermissionRoute, userID uuid.UUID) *http.Cookie {
	cookie := h.signInPastTheGates(route.AuthRoute, userID)
	route.expect(h, userID, orgauthz.Decision{Member: true, Allowed: true}, nil)

	return cookie
}

func (h *Harness) SignInToTeam(route TeamPermissionRoute, userID uuid.UUID) *http.Cookie {
	cookie := h.signInPastTheGates(route.AuthRoute, userID)
	route.expect(h, userID, orgauthz.Decision{Member: true, Allowed: true}, nil)

	return cookie
}

func (route OrgPermissionRoute) expect(h *Harness, userID uuid.UUID, decision orgauthz.Decision, err error) {
	check := spec.OrgPermissionCheck{OrgID: route.OrgID, UserID: userID, Permission: route.Permission}

	h.OrgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(decision, err)
}

func (route TeamPermissionRoute) expect(h *Harness, userID uuid.UUID, decision orgauthz.Decision, err error) {
	check := spec.TeamPermissionCheck{TeamID: route.TeamID, UserID: userID, Permission: route.Permission}

	h.OrgAuthz.EXPECT().CanInTeam(mock.Anything, check).Return(decision, err)
}

func tenantFailures(route AuthRoute, notFoundBody string, decide tenantDecision) []authFailure {
	return slices.Concat(authFailures, []authFailure{
		{
			name:    "someone who is not a member",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, userID uuid.UUID) *http.Cookie {
				cookie := h.signInPastTheGates(route, userID)
				decide(h, userID, orgauthz.Decision{}, nil)

				return cookie
			},
			status: http.StatusNotFound,
			body:   notFoundBody,
		},
		{
			name:    "a member without the permission",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, userID uuid.UUID) *http.Cookie {
				cookie := h.signInPastTheGates(route, userID)
				decide(h, userID, orgauthz.Decision{Member: true}, nil)

				return cookie
			},
			status: http.StatusForbidden,
			body:   `{"error":"insufficient permissions"}`,
		},
		{
			name:    "a membership store that cannot answer",
			applies: func(AuthRoute) bool { return true },
			arrange: func(h *Harness, userID uuid.UUID) *http.Cookie {
				cookie := h.signInPastTheGates(route, userID)
				decide(h, userID, orgauthz.Decision{}, errMembershipStoreDown)

				return cookie
			},
			status: http.StatusServiceUnavailable,
			body:   `{"error":"service unavailable"}`,
		},
	})
}
