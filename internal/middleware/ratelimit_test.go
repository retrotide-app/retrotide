package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"retrotide/internal/authctx"
	"retrotide/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	limitedRoute struct {
		path  string
		limit gin.HandlerFunc
	}
)

const (
	limitedPeer      = "198.51.100.7:4444"
	otherLimitedPeer = "198.51.100.8:4444"
	loginBody        = `{"email":"beatrice@example.com","password":"x"}`
)

func limitedEngine(t *testing.T, routes ...limitedRoute) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	for _, route := range routes {
		engine.POST(route.path, route.limit, func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
	}

	return engine
}

func postFrom(t *testing.T, engine *gin.Engine, target string, peer string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(body))
	req.RemoteAddr = peer
	req.Header.Set("Content-Type", "application/json")

	return serve(engine, req)
}

func TestRateLimitByClientIP_AllowsTheBudgetThenRefuses(t *testing.T) {
	tests := []struct {
		name       string
		limit      func(*RateLimits) gin.HandlerFunc
		allowed    int
		retryAfter string
	}{
		{name: "credentials", limit: func(r *RateLimits) gin.HandlerFunc { return r.RateLimitCredentials }, allowed: CredentialAttemptsPerMinute, retryAfter: "6"},
		{name: "mail", limit: func(r *RateLimits) gin.HandlerFunc { return r.RateLimitMail }, allowed: MailAttemptsPerHour, retryAfter: "720"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// given
				engine := limitedEngine(t, limitedRoute{path: "/limited", limit: tc.limit(NewRateLimits())})

				// when
				allowed := make([]*httptest.ResponseRecorder, 0, tc.allowed)
				for range tc.allowed {
					allowed = append(allowed, postFrom(t, engine, "/limited", limitedPeer, ""))
				}
				refused := postFrom(t, engine, "/limited", limitedPeer, "")

				// then every allowed request carries its remaining budget, and the refusal says when to come back
				for i, rec := range allowed {
					assert.Equal(t, http.StatusOK, rec.Code)
					assert.Equal(t, strconv.Itoa(tc.allowed), rec.Header().Get("X-RateLimit-Limit"))
					assert.Equal(t, strconv.Itoa(tc.allowed-1-i), rec.Header().Get("X-RateLimit-Remaining"))
				}

				assert.Equal(t, http.StatusTooManyRequests, refused.Code)
				assert.JSONEq(t, `{"error":"too many requests, please try again later"}`, refused.Body.String())
				assert.Equal(t, tc.retryAfter, refused.Header().Get("Retry-After"))
				assert.Equal(t, "0", refused.Header().Get("X-RateLimit-Remaining"))
			})
		})
	}
}

func TestRateLimitCredentials_RefillsOneAttemptEverySixSeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a spent budget
		engine := limitedEngine(t, limitedRoute{path: "/login", limit: NewRateLimits().RateLimitCredentials})
		for range CredentialAttemptsPerMinute {
			require.Equal(t, http.StatusOK, postFrom(t, engine, "/login", limitedPeer, "").Code)
		}

		// when six seconds pass
		time.Sleep(6 * time.Second)
		refilled := postFrom(t, engine, "/login", limitedPeer, "")
		spentAgain := postFrom(t, engine, "/login", limitedPeer, "")

		// then exactly one attempt has come back
		assert.Equal(t, http.StatusOK, refilled.Code)
		assert.Equal(t, http.StatusTooManyRequests, spentAgain.Code)
	})
}

func TestRateLimit_ARefusedRequestSpendsNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a spent budget
		engine := limitedEngine(t, limitedRoute{path: "/login", limit: NewRateLimits().RateLimitCredentials})
		for range CredentialAttemptsPerMinute {
			require.Equal(t, http.StatusOK, postFrom(t, engine, "/login", limitedPeer, "").Code)
		}

		// when a request is refused halfway through the refill
		time.Sleep(3 * time.Second)
		refused := postFrom(t, engine, "/login", limitedPeer, "")
		time.Sleep(3 * time.Second)
		afterRefill := postFrom(t, engine, "/login", limitedPeer, "")

		// then the refusal took nothing from the bucket, so the refill still lands on time
		assert.Equal(t, http.StatusTooManyRequests, refused.Code)
		assert.Equal(t, "3", refused.Header().Get("Retry-After"))
		assert.Equal(t, http.StatusOK, afterRefill.Code)
	})
}

func TestRateLimit_RoutesThatNameALimiterShareItsBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given register and login on the credential limiter, and forgotten password on the mail limiter
		limits := NewRateLimits()
		engine := limitedEngine(t,
			limitedRoute{path: "/login", limit: limits.RateLimitCredentials},
			limitedRoute{path: "/register", limit: limits.RateLimitCredentials},
			limitedRoute{path: "/forgot", limit: limits.RateLimitMail},
		)

		// when one address spends half its budget on each credential route
		var statuses []int
		for range CredentialAttemptsPerMinute / 2 {
			statuses = append(statuses, postFrom(t, engine, "/login", limitedPeer, "").Code)
			statuses = append(statuses, postFrom(t, engine, "/register", limitedPeer, "").Code)
		}
		overflow := postFrom(t, engine, "/register", limitedPeer, "")
		mail := postFrom(t, engine, "/forgot", limitedPeer, "")

		// then both routes drew on one bucket, and the mail limiter kept its own
		for _, status := range statuses {
			assert.Equal(t, http.StatusOK, status)
		}
		assert.Equal(t, http.StatusTooManyRequests, overflow.Code)
		assert.Equal(t, http.StatusOK, mail.Code)
	})
}

func TestRateLimitByClientIP_KeysOnTheClientIP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given one address that has spent its budget
		engine := limitedEngine(t, limitedRoute{path: "/forgot", limit: NewRateLimits().RateLimitMail})
		for range MailAttemptsPerHour {
			require.Equal(t, http.StatusOK, postFrom(t, engine, "/forgot", limitedPeer, "").Code)
		}

		// when
		samePeerNewPort := postFrom(t, engine, "/forgot", "198.51.100.7:5555", "")
		otherPeer := postFrom(t, engine, "/forgot", otherLimitedPeer, "")

		// then a new source port is the same client, and another client keeps its own budget
		assert.Equal(t, http.StatusTooManyRequests, samePeerNewPort.Code)
		assert.Equal(t, http.StatusOK, otherPeer.Code)
	})
}

func boardWriteEngine(t *testing.T, limits *RateLimits) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.POST("/boards/:principalID/cards", func(c *gin.Context) {
		principalID := c.Param("principalID")
		if principalID == "anonymous" {
			return
		}

		principal := model.Principal{Kind: model.PrincipalUser, ID: uuid.MustParse(principalID)}
		c.Request = c.Request.WithContext(authctx.WithPrincipal(c.Request.Context(), principal))
	}, limits.BoardWriteLimit, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return engine
}

func TestBoardWriteLimit_AllowsABurstOfThirtyPerPrincipalThenTenASecond(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given one principal that has spent its burst
		alice, bob := uuid.New(), uuid.New()
		engine := boardWriteEngine(t, NewRateLimits())
		for i := range BoardWriteBurst {
			rec := postFrom(t, engine, "/boards/"+alice.String()+"/cards", limitedPeer, "")
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, strconv.Itoa(BoardWriteBurst-1-i), rec.Header().Get("X-RateLimit-Remaining"))
		}

		// when
		refused := postFrom(t, engine, "/boards/"+alice.String()+"/cards", limitedPeer, "")
		samePeerOtherPrincipal := postFrom(t, engine, "/boards/"+bob.String()+"/cards", limitedPeer, "")
		time.Sleep(100 * time.Millisecond)
		refilled := postFrom(t, engine, "/boards/"+alice.String()+"/cards", otherLimitedPeer, "")
		spentAgain := postFrom(t, engine, "/boards/"+alice.String()+"/cards", otherLimitedPeer, "")

		// then the bucket follows the principal, not the address, and one write comes back every tenth of a second
		assert.Equal(t, http.StatusTooManyRequests, refused.Code)
		assert.Equal(t, strconv.Itoa(BoardWriteBurst), refused.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "1", refused.Header().Get("Retry-After"))
		assert.JSONEq(t, `{"error":"too many requests, please try again later"}`, refused.Body.String())
		assert.Equal(t, http.StatusOK, samePeerOtherPrincipal.Code)
		assert.Equal(t, http.StatusOK, refilled.Code)
		assert.Equal(t, http.StatusTooManyRequests, spentAgain.Code)
	})
}

func TestBoardWriteLimit_KeysARequestWithNoPrincipalOnItsAddress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given an address that has spent a burst without a principal
		engine := boardWriteEngine(t, NewRateLimits())
		for range BoardWriteBurst {
			require.Equal(t, http.StatusOK, postFrom(t, engine, "/boards/anonymous/cards", limitedPeer, "").Code)
		}

		// when
		samePeer := postFrom(t, engine, "/boards/anonymous/cards", limitedPeer, "")
		otherPeer := postFrom(t, engine, "/boards/anonymous/cards", otherLimitedPeer, "")

		// then
		assert.Equal(t, http.StatusTooManyRequests, samePeer.Code)
		assert.Equal(t, http.StatusOK, otherPeer.Code)
	})
}

func TestRateLimitByAccount_KeysOnTheBodyField(t *testing.T) {
	tests := []struct {
		name       string
		bodyFor    func(call int) string
		peerFor    func(call int) string
		wantStatus int
	}{
		{
			name:       "rotating addresses cannot escape one account's budget",
			bodyFor:    func(int) string { return loginBody },
			peerFor:    func(call int) string { return fmt.Sprintf("203.0.113.%d:4444", call+1) },
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name: "case and whitespace do not make a new key",
			bodyFor: func(call int) string {
				if call%2 == 0 {
					return `{"email":" Beatrice@Example.com ","password":"x"}`
				}

				return `{"email":"BEATRICE@EXAMPLE.COM","password":"x"}`
			},
			peerFor:    func(call int) string { return fmt.Sprintf("203.0.113.%d:4444", call+1) },
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "different accounts keep separate budgets",
			bodyFor:    func(call int) string { return fmt.Sprintf(`{"email":"user%d@example.com","password":"x"}`, call) },
			peerFor:    func(int) string { return limitedPeer },
			wantStatus: http.StatusOK,
		},
		{
			name:       "a body that does not decode falls back to the client IP",
			bodyFor:    func(int) string { return "not json" },
			peerFor:    func(int) string { return limitedPeer },
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "the fallback is per client IP",
			bodyFor:    func(int) string { return "not json" },
			peerFor:    func(call int) string { return fmt.Sprintf("203.0.113.%d:4444", call+1) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "a missing field falls back to the client IP",
			bodyFor:    func(int) string { return `{"password":"x"}` },
			peerFor:    func(int) string { return limitedPeer },
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "a field that is not a string falls back to the client IP",
			bodyFor:    func(call int) string { return fmt.Sprintf(`{"email":%d}`, call) },
			peerFor:    func(int) string { return limitedPeer },
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "an empty body falls back to the client IP",
			bodyFor:    func(int) string { return "" },
			peerFor:    func(int) string { return limitedPeer },
			wantStatus: http.StatusTooManyRequests,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// given
				engine := limitedEngine(t, limitedRoute{path: "/login", limit: NewRateLimits().RateLimitCredentialsByAccount("email")})

				// when one more request than the budget allows is sent
				var last int
				for call := range CredentialAttemptsPerMinute + 1 {
					last = postFrom(t, engine, "/login", tc.peerFor(call), tc.bodyFor(call)).Code
				}

				// then
				assert.Equal(t, tc.wantStatus, last)
			})
		})
	}
}

func TestRateLimitByAccount_EveryCallSharesOneBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given two routes that each build their own account limiter from one set of limits
		limits := NewRateLimits()
		engine := limitedEngine(t,
			limitedRoute{path: "/login", limit: limits.RateLimitCredentialsByAccount("email")},
			limitedRoute{path: "/google/link", limit: limits.RateLimitCredentialsByAccount("email")},
			limitedRoute{path: "/forgot", limit: limits.RateLimitMailByAccount("email")},
		)

		// when one account spends its budget across both credential routes
		for call := range CredentialAttemptsPerMinute {
			target := "/login"
			if call%2 == 1 {
				target = "/google/link"
			}

			require.Equal(t, http.StatusOK, postFrom(t, engine, target, limitedPeer, loginBody).Code)
		}
		overflow := postFrom(t, engine, "/google/link", otherLimitedPeer, loginBody)
		mail := postFrom(t, engine, "/forgot", limitedPeer, loginBody)

		// then the account's credential bucket is one bucket, and its mail bucket is another
		assert.Equal(t, http.StatusTooManyRequests, overflow.Code)
		assert.Equal(t, http.StatusOK, mail.Code)
	})
}

func chargingEngine(t *testing.T, limits *RateLimits, account string) (*gin.Engine, *int) {
	t.Helper()

	handled := new(int)

	engine := limitedEngine(t, limitedRoute{path: "/login", limit: limits.RateLimitCredentialsByAccount("email")})
	engine.POST("/google/start", func(c *gin.Context) {
		if !limits.ChargeCredentialsByAccount(c, "email", account) {
			return
		}

		*handled++
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return engine, handled
}

func TestChargeCredentialsByAccount_DrawsOnTheLoginBucketOfTheSameAddress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given a link start charged for the signed-in address, whose body names no address
		limits := NewRateLimits()
		engine, _ := chargingEngine(t, limits, " Beatrice@Example.COM ")

		// when the account spends its budget across login and the link start, from rotating addresses
		for call := range CredentialAttemptsPerMinute {
			target, body := "/login", loginBody
			if call%2 == 1 {
				target, body = "/google/start", `{"purpose":"link","current_password":"x"}`
			}

			require.Equal(t, http.StatusOK, postFrom(t, engine, target, fmt.Sprintf("203.0.113.%d:4444", call+1), body).Code)
		}
		loginOverflow := postFrom(t, engine, "/login", otherLimitedPeer, loginBody)
		startOverflow := postFrom(t, engine, "/google/start", otherLimitedPeer, `{"purpose":"link"}`)

		// then both draw on the one bucket that login keys on the same address
		assert.Equal(t, http.StatusTooManyRequests, loginOverflow.Code)
		assert.Equal(t, http.StatusTooManyRequests, startOverflow.Code)
	})
}

func TestChargeCredentialsByAccount_RefusesAndStopsTheHandlerOnceTheBudgetIsSpent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		engine, handled := chargingEngine(t, NewRateLimits(), "beatrice@example.com")

		// when one more start than the budget allows is sent
		allowed := make([]*httptest.ResponseRecorder, 0, CredentialAttemptsPerMinute)
		for range CredentialAttemptsPerMinute {
			allowed = append(allowed, postFrom(t, engine, "/google/start", limitedPeer, ""))
		}
		refused := postFrom(t, engine, "/google/start", limitedPeer, "")

		// then each allowed start carries its remaining budget, and the refusal answers alone, saying when to come back
		for i, rec := range allowed {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, strconv.Itoa(CredentialAttemptsPerMinute), rec.Header().Get("X-RateLimit-Limit"))
			assert.Equal(t, strconv.Itoa(CredentialAttemptsPerMinute-1-i), rec.Header().Get("X-RateLimit-Remaining"))
		}

		assert.Equal(t, CredentialAttemptsPerMinute, *handled)
		assert.Equal(t, http.StatusTooManyRequests, refused.Code)
		assert.JSONEq(t, `{"error":"too many requests, please try again later"}`, refused.Body.String())
		assert.Equal(t, "6", refused.Header().Get("Retry-After"))
	})
}

func TestChargeCredentialsByAccount_KeepsOtherAccountsApart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given one account that has spent its budget on login
		limits := NewRateLimits()
		engine, _ := chargingEngine(t, limits, "someone-else@example.com")
		for range CredentialAttemptsPerMinute {
			require.Equal(t, http.StatusOK, postFrom(t, engine, "/login", limitedPeer, loginBody).Code)
		}

		// when another signed-in account starts a link from the same address
		rec := postFrom(t, engine, "/google/start", limitedPeer, "")

		// then
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRateLimitMailByAccount_AllowsFiveAnHourPerAccount(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		engine := limitedEngine(t, limitedRoute{path: "/forgot", limit: NewRateLimits().RateLimitMailByAccount("email")})

		// when one account asks from a new address every time
		var statuses []int
		for call := range MailAttemptsPerHour + 1 {
			statuses = append(statuses, postFrom(t, engine, "/forgot", fmt.Sprintf("203.0.113.%d:4444", call+1), loginBody).Code)
		}

		// then
		for _, status := range statuses[:MailAttemptsPerHour] {
			assert.Equal(t, http.StatusOK, status)
		}
		assert.Equal(t, http.StatusTooManyRequests, statuses[MailAttemptsPerHour])
	})
}

func TestRateLimitByAccount_LeavesTheBodyForTheHandler(t *testing.T) {
	// given a handler that binds the body the limiter has already read
	var email string

	engine := newTestEngine(t)
	engine.POST("/login", NewRateLimits().RateLimitCredentialsByAccount("email"), func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
		}
		require.NoError(t, c.ShouldBindBodyWithJSON(&req))
		email = req.Email
		c.Status(http.StatusNoContent)
	})

	// when
	rec := postFrom(t, engine, "/login", limitedPeer, loginBody)

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "beatrice@example.com", email)
}

func TestRateLimitByAccount_LeavesAnOversizedBodyToTheHandler(t *testing.T) {
	// given a body limit smaller than the body, whose length is not declared
	var bindErr error

	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit).Handle)
	engine.POST("/login", NewRateLimits().RateLimitCredentialsByAccount("email"), func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
		}
		bindErr = c.ShouldBindBodyWithJSON(&req)
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", io.MultiReader(strings.NewReader(loginBody)))
	req.RemoteAddr = limitedPeer
	req.Header.Set("Content-Type", "application/json")

	// when
	rec := serve(engine, req)

	// then the limiter keyed it on the address and let the handler answer 413
	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, tooLarge := errors.AsType[*http.MaxBytesError](bindErr)
	assert.True(t, tooLarge, "got %v", bindErr)
}

func TestBucketSet_SweepsOnlyFullBuckets(t *testing.T) {
	// given one bucket that is used once and one that is drained
	start := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	credentials := newBucketSet(credentialRefill, CredentialAttemptsPerMinute)
	mail := newBucketSet(mailRefill, MailAttemptsPerHour)

	credentials.admit("idle", start)
	for range MailAttemptsPerHour {
		mail.admit("drained", start)
	}

	// when a later request runs the sweep
	credentials.admit("fresh", start.Add(bucketSweepInterval+time.Second))
	mail.admit("fresh", start.Add(bucketSweepInterval+time.Second))

	// then the refilled bucket is gone, and the one still refilling keeps its debt
	assert.NotContains(t, credentials.buckets, "idle")
	assert.Contains(t, credentials.buckets, "fresh")
	assert.Contains(t, mail.buckets, "drained")
	assert.False(t, mail.admit("drained", start.Add(bucketSweepInterval+2*time.Second)).allowed)
}
