package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"retrotide/internal/authctx"
	"retrotide/internal/authz"
	"retrotide/internal/dto"
	"retrotide/internal/guest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type (
	Guards struct {
		*RateLimits

		MetricsToken      gin.HandlerFunc
		Auth              gin.HandlerFunc
		OptionalAuth      gin.HandlerFunc
		Principal         gin.HandlerFunc
		OptionalPrincipal gin.HandlerFunc
		Turnstile         gin.HandlerFunc

		users    userGuard
		orgAuthz orgauthz.Service
	}

	GuardDeps struct {
		Settings settings.Service
		Sessions *session.Manager
		Guests   *guest.Manager
		Authz    authz.Service
		OrgAuthz orgauthz.Service
	}

	userGuard struct {
		sessions *session.Manager
		authz    authz.Service
	}

	sessionState int

	guardedRoute struct {
		method string
		route  string
	}
)

const (
	sessionUnchecked sessionState = iota
	sessionAbsent
	sessionInvalid
	sessionBanned
	sessionLive

	insufficientPermissionsMessage = "insufficient permissions"
	organisationNotFoundMessage    = "organisation not found"
	teamNotFoundMessage            = "team not found"

	orgIDParam  = "orgID"
	teamIDParam = "teamID"
)

var (
	writeMethods = map[string]struct{}{
		http.MethodPost:   {},
		http.MethodPut:    {},
		http.MethodPatch:  {},
		http.MethodDelete: {},
	}

	lockExemptRoutes = map[guardedRoute]struct{}{
		{method: http.MethodPost, route: "/api/v1/notifications/read"}:     {},
		{method: http.MethodPost, route: "/api/v1/notifications/:id/read"}: {},
	}
)

func NewGuards(deps GuardDeps) *Guards {
	users := userGuard{sessions: deps.Sessions, authz: deps.Authz}
	principals := principalGuard{users: users, guests: deps.Guests}

	return &Guards{
		RateLimits:        NewRateLimits(),
		MetricsToken:      metricsToken(deps.Settings),
		Auth:              users.require(),
		OptionalAuth:      users.optional(),
		Principal:         principals.require(),
		OptionalPrincipal: principals.optional(),
		Turnstile:         RequireTurnstile(deps.Settings, TurnstileVerifyURL),
		users:             users,
		orgAuthz:          deps.OrgAuthz,
	}
}

func (g *Guards) Permission(perm authz.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := g.users.authenticate(c)
		if !ok {
			return
		}

		allowed, err := g.users.authz.Can(c.Request.Context(), userID, perm)
		if err != nil {
			abortUnavailable(c, fmt.Errorf("check permission %s for user %s: %w", perm, userID, err))
			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: insufficientPermissionsMessage})
			return
		}

		withUser(c, userID)
	}
}

func (g *Guards) OrgPermission(perm orgauthz.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := g.users.authenticate(c)
		if !ok {
			return
		}

		orgID, ok := parsePathID(c, orgIDParam)
		if !ok {
			return
		}

		decision, err := g.orgAuthz.CanInOrg(c.Request.Context(), spec.OrgPermissionCheck{OrgID: orgID, UserID: userID, Permission: perm})
		if err != nil {
			abortUnavailable(c, fmt.Errorf("check permission %s in organisation %s for user %s: %w", perm, orgID, userID, err))
			return
		}

		if !admits(c, decision, organisationNotFoundMessage) {
			return
		}

		withUser(c, userID)
	}
}

func (g *Guards) TeamPermission(perm orgauthz.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := g.users.authenticate(c)
		if !ok {
			return
		}

		teamID, ok := parsePathID(c, teamIDParam)
		if !ok {
			return
		}

		decision, err := g.orgAuthz.CanInTeam(c.Request.Context(), spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: perm})
		if err != nil {
			abortUnavailable(c, fmt.Errorf("check permission %s in team %s for user %s: %w", perm, teamID, userID, err))
			return
		}

		if !admits(c, decision, teamNotFoundMessage) {
			return
		}

		withUser(c, userID)
	}
}

func parsePathID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + param})
		return uuid.Nil, false
	}

	return id, true
}

func admits(c *gin.Context, decision orgauthz.Decision, notFoundMessage string) bool {
	if !decision.Member {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return false
	}

	if !decision.Allowed {
		c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: insufficientPermissionsMessage})
		return false
	}

	return true
}

func (g userGuard) require() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := g.authenticate(c)
		if !ok {
			return
		}

		withUser(c, userID)
	}
}

func (g userGuard) authenticate(c *gin.Context) (uuid.UUID, bool) {
	userID, state := g.resolve(c)
	switch state {
	case sessionUnchecked:
		return uuid.Nil, false

	case sessionAbsent:
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication required"})
		return uuid.Nil, false

	case sessionInvalid:
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid or expired session"})
		return uuid.Nil, false

	case sessionBanned:
		c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "your account has been banned"})
		return uuid.Nil, false
	}

	if !g.passesLockGate(c, userID) {
		return uuid.Nil, false
	}

	return userID, true
}

func (g userGuard) optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, state := g.resolve(c)
		if state != sessionLive {
			return
		}

		withUser(c, userID)
	}
}

func (g userGuard) resolve(c *gin.Context) (uuid.UUID, sessionState) {
	ctx := c.Request.Context()

	token, err := g.sessions.Token(c.Request)
	if err != nil {
		abortUnavailable(c, err)
		return uuid.Nil, sessionUnchecked
	}

	if token == "" {
		return uuid.Nil, sessionAbsent
	}

	userID, err := g.sessions.Validate(ctx, token)
	if errors.Is(err, session.ErrInvalidSession) {
		return uuid.Nil, sessionInvalid
	}
	if err != nil {
		abortUnavailable(c, err)
		return uuid.Nil, sessionUnchecked
	}

	banned, err := g.authz.IsBanned(ctx, userID)
	if err != nil {
		abortUnavailable(c, err)
		return uuid.Nil, sessionUnchecked
	}

	if banned {
		if err := g.sessions.Delete(ctx, token); err != nil {
			c.Error(fmt.Errorf("revoke the session of banned user %s: %w", userID, err))
		}

		return uuid.Nil, sessionBanned
	}

	return userID, sessionLive
}

func (g userGuard) passesLockGate(c *gin.Context, userID uuid.UUID) bool {
	if _, write := writeMethods[c.Request.Method]; !write {
		return true
	}

	if _, exempt := lockExemptRoutes[guardedRoute{method: c.Request.Method, route: c.FullPath()}]; exempt {
		return true
	}

	locked, err := g.authz.IsLocked(c.Request.Context(), userID)
	if err != nil {
		abortUnavailable(c, err)
		return false
	}

	if locked {
		c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "your account is locked"})
		return false
	}

	return true
}

func withUser(c *gin.Context, userID uuid.UUID) {
	c.Request = c.Request.WithContext(authctx.WithPrincipal(c.Request.Context(), model.Principal{Kind: model.PrincipalUser, ID: userID}))
}

func abortUnavailable(c *gin.Context, err error) {
	c.Error(err)
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: unavailableMessage})
}
