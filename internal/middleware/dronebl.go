package middleware

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"retrotide/internal/dronebl"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/session"

	"github.com/gin-gonic/gin"
)

const (
	blockedMessage  = "your network is listed on a public abuse blocklist"
	htmlContentType = "text/html; charset=utf-8"

	blockedPage = `<!doctype html>
<html lang="en-GB">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Access blocked</title>
<style>
:root { color-scheme: light dark; }
body {
  margin: 0;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  box-sizing: border-box;
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  background: Canvas;
  color: CanvasText;
}
main {
  max-width: 32rem;
  padding: 2rem;
  border: 1px solid GrayText;
  border-radius: 0.5rem;
  text-align: center;
}
h1 { margin: 0 0 1rem; font-size: 1.5rem; }
p { margin: 0 0 1rem; line-height: 1.5; }
a.sign-in {
  display: inline-block;
  margin: 0.5rem 0 1rem;
  padding: 0.6rem 1.4rem;
  border: 1px solid currentColor;
  border-radius: 0.375rem;
  color: LinkText;
  text-decoration: none;
}
.muted { margin-bottom: 0; font-size: 0.875rem; color: GrayText; }
</style>
</head>
<body>
<main>
<h1>Access blocked</h1>
<p>Your connection comes from an address listed on a public abuse blocklist, so this site will not answer it.</p>
<p>This is usually not about you. Addresses are recycled, and whoever held yours before may have run an open proxy or a compromised machine.</p>
<a class="sign-in" href="/login">Sign in</a>
<p class="muted">If you have an account, you can still sign in. Otherwise, check whether your address is listed at dronebl.org/lookup, or reconnect to get a new one.</p>
</main>
</body>
</html>`
)

var (
	reachableWhenListed = map[string]struct{}{
		"/livez":               {},
		"/health":              {},
		"/login":               {},
		"/api/v1/auth/login":   {},
		"/api/v1/auth/session": {},
		"/api/v1/site-info":    {},
	}
)

func RequireCleanIP(checker dronebl.Service, sessions *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		requested := strings.ToLower(c.Request.URL.Path)
		if _, reachable := reachableWhenListed[requested]; reachable || isStaticAsset(requested) {
			return
		}

		ip := c.ClientIP()

		verdict, err := checker.Check(c.Request.Context(), ip)
		if err != nil {
			abortUnreadableSetting(c, err)
			return
		}

		if !verdict.Blocked {
			return
		}

		signedIn, err := hasSession(c, sessions)
		if err != nil {
			abortUnavailable(c, err)
			return
		}

		if signedIn {
			return
		}

		route, keepIP := accessLogRoute(c.FullPath(), c.Request.URL.Path)
		event := logger.Ctx(c.Request.Context()).Warn().Ints("classes", verdict.Classes).Str("route", route)
		if keepIP {
			event = event.Str("ip", ip)
		}
		event.Msg("dronebl blocked a request")

		if isAPIPath(requested) || !strings.Contains(c.GetHeader("Accept"), "text/html") {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: blockedMessage})
			return
		}

		c.Abort()
		c.Data(http.StatusForbidden, htmlContentType, []byte(blockedPage))
	}
}

func hasSession(c *gin.Context, sessions *session.Manager) (bool, error) {
	token, err := sessions.Token(c.Request)
	if err != nil {
		return false, err
	}

	if token == "" {
		return false, nil
	}

	_, err = sessions.Validate(c.Request.Context(), token)
	if errors.Is(err, session.ErrInvalidSession) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func isStaticAsset(requested string) bool {
	return !isAPIPath(requested) && path.Ext(requested) != ""
}

func isAPIPath(requested string) bool {
	return requested == "/api" || strings.HasPrefix(requested, "/api/")
}
