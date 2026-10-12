package middleware

import (
	"net/http"
	"strings"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/guest"
	"retrotide/internal/session"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
)

const (
	privateSiteMessage = "this site is private, sign in to continue"
	privateSiteCode    = "sign_in_required"
)

var (
	privateSiteOpenRoutes = map[guardedRoute]struct{}{
		{method: http.MethodGet, route: "/api/v1/site-info"}:                 {},
		{method: http.MethodHead, route: "/api/v1/site-info"}:                {},
		{method: http.MethodGet, route: "/api/v1/auth/session"}:              {},
		{method: http.MethodHead, route: "/api/v1/auth/session"}:             {},
		{method: http.MethodPost, route: "/api/v1/auth/login"}:               {},
		{method: http.MethodPost, route: "/api/v1/auth/logout"}:              {},
		{method: http.MethodPost, route: "/api/v1/auth/register"}:            {},
		{method: http.MethodPost, route: "/api/v1/auth/verify-email"}:        {},
		{method: http.MethodPost, route: "/api/v1/auth/resend-verification"}: {},
		{method: http.MethodPost, route: "/api/v1/auth/forgot-password"}:     {},
		{method: http.MethodPost, route: "/api/v1/auth/reset-password"}:      {},
		{method: http.MethodPost, route: "/api/v1/auth/invitation"}:          {},
		{method: http.MethodPost, route: "/api/v1/auth/invitation/accept"}:   {},
		{method: http.MethodPost, route: "/api/v1/auth/two-factor/verify"}:   {},
		{method: http.MethodPost, route: "/api/v1/invites/lookup"}:           {},
		{method: http.MethodPost, route: "/api/v1/invites/register"}:         {},
		{method: http.MethodGet, route: "/api/v1/b/:slug"}:                   {},
		{method: http.MethodHead, route: "/api/v1/b/:slug"}:                  {},
		{method: http.MethodPost, route: "/api/v1/b/:slug/join"}:             {},
	}
)

func PrivateSite(settingsSvc settings.Service, sessions *session.Manager, guests *guest.Manager, authzSvc authz.Service) gin.HandlerFunc {
	members := userGuard{sessions: sessions, authz: authzSvc}
	visitors := principalGuard{users: members, guests: guests}

	return func(c *gin.Context) {
		if !strings.HasPrefix(strings.ToLower(c.Request.URL.Path), "/api/") {
			return
		}

		if _, open := privateSiteOpenRoutes[guardedRoute{method: c.Request.Method, route: c.FullPath()}]; open {
			return
		}

		mode, err := settings.ReadSiteMode(c.Request.Context(), settingsSvc)
		if err != nil {
			abortUnreadableSetting(c, err)
			return
		}

		if mode == config.SiteModePublic {
			return
		}

		_, state := members.resolve(c)
		if state == sessionUnchecked || state == sessionLive {
			return
		}

		if guestRoute(c.FullPath()) {
			_, presence := visitors.resolveGuest(c)
			if presence == guestUnchecked || presence == guestLive {
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: privateSiteMessage, Code: privateSiteCode})
	}
}

func guestRoute(template string) bool {
	return template == socketRoute || template == boardRoute || strings.HasPrefix(template, boardRoute+"/")
}
