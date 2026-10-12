package middleware

import (
	"maps"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"retrotide/internal/authctx"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type (
	RateLimits struct {
		RateLimitCredentials gin.HandlerFunc
		RateLimitMail        gin.HandlerFunc
		BoardWriteLimit      gin.HandlerFunc
		BoardJoinLimit       gin.HandlerFunc

		credentialsByAccount *bucketSet
		mailByAccount        *bucketSet
	}

	keyedLimit struct {
		buckets *bucketSet
		key     func(c *gin.Context) string
	}

	layeredLimit []keyedLimit

	bucketSet struct {
		every   rate.Limit
		burst   int
		mu      sync.Mutex
		buckets map[string]*rate.Limiter
		swept   time.Time
	}

	admission struct {
		allowed    bool
		remaining  int
		retryAfter time.Duration
	}
)

const (
	CredentialAttemptsPerMinute = 10
	MailAttemptsPerHour         = 5
	BoardWritesPerSecond        = 10
	BoardWriteBurst             = 30
	BoardJoinsPerMinute         = 10
	BoardJoinsPerMinuteFromIP   = 600

	credentialRefill    = time.Minute / CredentialAttemptsPerMinute
	mailRefill          = time.Hour / MailAttemptsPerHour
	boardWriteRefill    = time.Second / BoardWritesPerSecond
	boardJoinRefill     = time.Minute / BoardJoinsPerMinute
	boardJoinIPRefill   = time.Minute / BoardJoinsPerMinuteFromIP
	slugParam           = "slug"
	bucketSweepInterval = time.Minute

	rateLimitHeader          = "X-RateLimit-Limit"
	rateLimitRemainingHeader = "X-RateLimit-Remaining"
	retryAfterHeader         = "Retry-After"
	accountFallbackPrefix    = "ip:"
	rateLimitedMessage       = "too many requests, please try again later"
)

func NewRateLimits() *RateLimits {
	return &RateLimits{
		RateLimitCredentials: keyedLimit{buckets: newBucketSet(credentialRefill, CredentialAttemptsPerMinute), key: (*gin.Context).ClientIP}.handle,
		RateLimitMail:        keyedLimit{buckets: newBucketSet(mailRefill, MailAttemptsPerHour), key: (*gin.Context).ClientIP}.handle,
		BoardWriteLimit:      keyedLimit{buckets: newBucketSet(boardWriteRefill, BoardWriteBurst), key: principalKey}.handle,
		BoardJoinLimit: layeredLimit{
			{buckets: newBucketSet(boardJoinIPRefill, BoardJoinsPerMinuteFromIP), key: (*gin.Context).ClientIP},
			{buckets: newBucketSet(boardJoinRefill, BoardJoinsPerMinute), key: joinKey},
		}.handle,
		credentialsByAccount: newBucketSet(credentialRefill, CredentialAttemptsPerMinute),
		mailByAccount:        newBucketSet(mailRefill, MailAttemptsPerHour),
	}
}

func (r *RateLimits) RateLimitCredentialsByAccount(field string) gin.HandlerFunc {
	return keyedLimit{buckets: r.credentialsByAccount, key: accountKey(field)}.handle
}

func (r *RateLimits) RateLimitMailByAccount(field string) gin.HandlerFunc {
	return keyedLimit{buckets: r.mailByAccount, key: accountKey(field)}.handle
}

func (r *RateLimits) ChargeCredentialsByAccount(c *gin.Context, field string, value string) bool {
	limit := keyedLimit{buckets: r.credentialsByAccount, key: func(c *gin.Context) string {
		return accountBucket(c, field, value)
	}}
	limit.handle(c)

	return !c.IsAborted()
}

func (l keyedLimit) handle(c *gin.Context) {
	result := l.buckets.admit(l.key(c), time.Now())

	header := c.Writer.Header()
	header.Set(rateLimitHeader, strconv.Itoa(l.buckets.burst))
	header.Set(rateLimitRemainingHeader, strconv.Itoa(result.remaining))

	if result.allowed {
		return
	}

	header.Set(retryAfterHeader, strconv.Itoa(max(1, int(math.Ceil(result.retryAfter.Seconds())))))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{Error: rateLimitedMessage})
}

func (l layeredLimit) handle(c *gin.Context) {
	for _, limit := range l {
		if limit.key(c) == "" {
			continue
		}

		limit.handle(c)
		if c.IsAborted() {
			return
		}
	}
}

func joinKey(c *gin.Context) string {
	principal, ok := authctx.Principal(c.Request.Context())
	if !ok {
		return ""
	}

	return c.Param(slugParam) + "|" + string(principal.Kind) + ":" + principal.ID.String()
}

func accountKey(field string) func(c *gin.Context) string {
	return func(c *gin.Context) string {
		var body map[string]any
		if err := c.ShouldBindBodyWithJSON(&body); err != nil {
			return accountFallbackPrefix + c.ClientIP()
		}

		value, _ := body[field].(string)

		return accountBucket(c, field, value)
	}
}

func principalKey(c *gin.Context) string {
	principal, ok := authctx.Principal(c.Request.Context())
	if !ok {
		return accountFallbackPrefix + c.ClientIP()
	}

	return string(principal.Kind) + ":" + principal.ID.String()
}

func accountBucket(c *gin.Context, field string, value string) string {
	account := strings.ToLower(strings.TrimSpace(value))
	if account == "" {
		return accountFallbackPrefix + c.ClientIP()
	}

	return field + ":" + account
}

func newBucketSet(refill time.Duration, burst int) *bucketSet {
	return &bucketSet{
		every:   rate.Every(refill),
		burst:   burst,
		buckets: make(map[string]*rate.Limiter),
	}
}

func (b *bucketSet) admit(key string, now time.Time) admission {
	b.mu.Lock()
	defer b.mu.Unlock()

	if now.Sub(b.swept) >= bucketSweepInterval {
		b.sweep(now)
	}

	limiter, ok := b.buckets[key]
	if !ok {
		limiter = rate.NewLimiter(b.every, b.burst)
		b.buckets[key] = limiter
	}

	if limiter.AllowN(now, 1) {
		return admission{allowed: true, remaining: int(limiter.TokensAt(now))}
	}

	missing := 1 - limiter.TokensAt(now)

	return admission{retryAfter: time.Duration(missing / float64(b.every) * float64(time.Second))}
}

func (b *bucketSet) sweep(now time.Time) {
	full := float64(b.burst)

	maps.DeleteFunc(b.buckets, func(_ string, limiter *rate.Limiter) bool {
		return limiter.TokensAt(now) >= full
	})

	b.swept = now
}
