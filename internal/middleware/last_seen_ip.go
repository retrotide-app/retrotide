package middleware

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"retrotide/internal/authctx"
	"retrotide/internal/dao"
	"retrotide/internal/logger"
	"retrotide/internal/model/spec"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type (
	LastSeenIPWriter interface {
		UpdateLastSeenIP(ctx context.Context, s spec.UserLastSeenIPUpdate, tx ...*sql.Tx) error
	}

	LastSeenIP struct {
		writer LastSeenIPWriter
		seen   sync.Map
	}

	lastSeenEntry struct {
		ip      string
		wroteAt time.Time
	}
)

const (
	lastSeenIPInterval = time.Hour
)

func NewLastSeenIP(writer LastSeenIPWriter) *LastSeenIP {
	return &LastSeenIP{writer: writer}
}

func (l *LastSeenIP) Handle(c *gin.Context) {
	c.Next()

	if c.FullPath() == socketRoute {
		return
	}

	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		return
	}

	ip := c.ClientIP()
	if ip == "" {
		return
	}

	entry, due := l.claim(userID, ip, time.Now())
	if !due {
		return
	}

	ctx := context.WithoutCancel(c.Request.Context())
	go l.write(ctx, userID, entry)
}

func (l *LastSeenIP) claim(userID uuid.UUID, ip string, now time.Time) (lastSeenEntry, bool) {
	fresh := lastSeenEntry{ip: ip, wroteAt: now}

	previous, loaded := l.seen.LoadOrStore(userID, fresh)
	if !loaded {
		return fresh, true
	}

	last := previous.(lastSeenEntry)
	if last.ip == ip && now.Sub(last.wroteAt) < lastSeenIPInterval {
		return lastSeenEntry{}, false
	}

	return fresh, l.seen.CompareAndSwap(userID, previous, fresh)
}

func (l *LastSeenIP) write(ctx context.Context, userID uuid.UUID, entry lastSeenEntry) {
	err := l.writer.UpdateLastSeenIP(ctx, spec.UserLastSeenIPUpdate{UserID: userID, IP: entry.ip})
	if errors.Is(err, dao.ErrNotFound) {
		l.seen.Delete(userID)
		logger.Ctx(ctx).Info().Err(err).Str("user_id", userID.String()).Msg("the account no longer exists, so its last seen ip was not recorded")
		return
	}
	if err != nil {
		l.seen.CompareAndDelete(userID, entry)
		logger.Ctx(ctx).Error().Err(err).Str("user_id", userID.String()).Msg("failed to record the last seen ip")
	}
}
