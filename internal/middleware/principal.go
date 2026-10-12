package middleware

import (
	"errors"
	"net/http"

	"retrotide/internal/authctx"
	"retrotide/internal/dto"
	"retrotide/internal/guest"
	"retrotide/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type (
	principalGuard struct {
		users  userGuard
		guests *guest.Manager
	}

	guestState int
)

const (
	guestUnchecked guestState = iota
	guestAbsent
	guestInvalid
	guestLive

	authenticationRequiredMessage = "authentication required"
	invalidCredentialsMessage     = "invalid or expired session"
	bannedMessage                 = "your account has been banned"
)

func (g principalGuard) require() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, state := g.users.resolve(c)
		switch state {
		case sessionUnchecked:
			return
		case sessionBanned:
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: bannedMessage})
			return
		case sessionLive:
			if !g.users.passesLockGate(c, userID) {
				return
			}

			withPrincipal(c, model.Principal{Kind: model.PrincipalUser, ID: userID})
			return
		}

		deviceID, presence := g.resolveGuest(c)
		switch presence {
		case guestUnchecked:
			return
		case guestLive:
			withPrincipal(c, model.Principal{Kind: model.PrincipalGuest, ID: deviceID})
			return
		}

		if state == sessionInvalid || presence == guestInvalid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: invalidCredentialsMessage})
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: authenticationRequiredMessage})
	}
}

func (g principalGuard) optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, state := g.users.resolve(c)
		if state == sessionUnchecked {
			return
		}

		if state == sessionLive {
			if !g.users.passesLockGate(c, userID) {
				return
			}

			withPrincipal(c, model.Principal{Kind: model.PrincipalUser, ID: userID})
			return
		}

		deviceID, presence := g.resolveGuest(c)
		if presence != guestLive {
			return
		}

		withPrincipal(c, model.Principal{Kind: model.PrincipalGuest, ID: deviceID})
	}
}

func (g principalGuard) resolveGuest(c *gin.Context) (uuid.UUID, guestState) {
	token, err := g.guests.Token(c.Request)
	if err != nil {
		abortUnavailable(c, err)
		return uuid.Nil, guestUnchecked
	}

	if token == "" {
		return uuid.Nil, guestAbsent
	}

	deviceID, err := g.guests.Validate(c.Request.Context(), token)
	if errors.Is(err, guest.ErrInvalidGuest) {
		return uuid.Nil, guestInvalid
	}
	if err != nil {
		abortUnavailable(c, err)
		return uuid.Nil, guestUnchecked
	}

	return deviceID, guestLive
}

func withPrincipal(c *gin.Context, principal model.Principal) {
	c.Request = c.Request.WithContext(authctx.WithPrincipal(c.Request.Context(), principal))
}
