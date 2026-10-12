package middleware

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"retrotide/internal/authctx"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	publicRoute  = "/api/v1/public"
	firstPeer    = "198.51.100.7:4444"
	firstPeerIP  = "198.51.100.7"
	secondPeer   = "203.0.113.9:4444"
	secondPeerIP = "203.0.113.9"
	guestKind    = model.PrincipalKind("guest")
)

func lastSeenEngine(t *testing.T, recorder *LastSeenIP, principal model.Principal) *gin.Engine {
	t.Helper()

	engine := newTestEngine(t)
	engine.Use(recorder.Handle)

	inject := func(c *gin.Context) {
		c.Request = c.Request.WithContext(authctx.WithPrincipal(c.Request.Context(), principal))
	}

	engine.GET(probeRoute, inject, principalProbe)
	engine.GET(socketRoute, inject, principalProbe)
	engine.GET(publicRoute, principalProbe)

	return engine
}

func requestFrom(t *testing.T, engine *gin.Engine, path string, peer string) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	req.RemoteAddr = peer

	rec := serve(engine, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func userPrincipal() (model.Principal, uuid.UUID) {
	userID := uuid.New()

	return model.Principal{Kind: model.PrincipalUser, ID: userID}, userID
}

func TestLastSeenIP_RecordsTheFirstRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		writer := repository.NewMockUserRepository(t)
		principal, userID := userPrincipal()
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, spec.UserLastSeenIPUpdate{UserID: userID, IP: firstPeerIP}).Return(nil).Once()
		engine := lastSeenEngine(t, NewLastSeenIP(writer), principal)

		// when
		requestFrom(t, engine, probeRoute, firstPeer)

		// then
		synctest.Wait()
		writer.AssertExpectations(t)
	})
}

func TestLastSeenIP_SkipsTheSameAddressWithinTheHour(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		writer := repository.NewMockUserRepository(t)
		principal, userID := userPrincipal()
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, spec.UserLastSeenIPUpdate{UserID: userID, IP: firstPeerIP}).Return(nil).Once()
		engine := lastSeenEngine(t, NewLastSeenIP(writer), principal)

		// when
		for range 50 {
			requestFrom(t, engine, probeRoute, firstPeer)
		}
		time.Sleep(lastSeenIPInterval - time.Second)
		requestFrom(t, engine, probeRoute, firstPeer)

		// then
		synctest.Wait()
		writer.AssertNumberOfCalls(t, "UpdateLastSeenIP", 1)
	})
}

func TestLastSeenIP_RecordsAChangedAddressAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		writer := repository.NewMockUserRepository(t)
		principal, userID := userPrincipal()
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, spec.UserLastSeenIPUpdate{UserID: userID, IP: firstPeerIP}).Return(nil).Once()
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, spec.UserLastSeenIPUpdate{UserID: userID, IP: secondPeerIP}).Return(nil).Once()
		engine := lastSeenEngine(t, NewLastSeenIP(writer), principal)
		requestFrom(t, engine, probeRoute, firstPeer)
		synctest.Wait()

		// when
		requestFrom(t, engine, probeRoute, secondPeer)

		// then
		synctest.Wait()
		writer.AssertExpectations(t)
	})
}

func TestLastSeenIP_RecordsTheSameAddressAgainOnceTheHourHasPassed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		writer := repository.NewMockUserRepository(t)
		principal, userID := userPrincipal()
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, spec.UserLastSeenIPUpdate{UserID: userID, IP: firstPeerIP}).Return(nil).Twice()
		engine := lastSeenEngine(t, NewLastSeenIP(writer), principal)
		requestFrom(t, engine, probeRoute, firstPeer)
		synctest.Wait()

		// when
		time.Sleep(lastSeenIPInterval)
		requestFrom(t, engine, probeRoute, firstPeer)

		// then
		synctest.Wait()
		writer.AssertExpectations(t)
	})
}

func TestLastSeenIP_RetriesOnTheNextRequestAfterAFailedWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		writer := repository.NewMockUserRepository(t)
		principal, userID := userPrincipal()
		update := spec.UserLastSeenIPUpdate{UserID: userID, IP: firstPeerIP}
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, update).Return(errDatabaseDown).Once()
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, update).Return(nil).Once()
		engine := lastSeenEngine(t, NewLastSeenIP(writer), principal)
		requestFrom(t, engine, probeRoute, firstPeer)
		synctest.Wait()

		// when
		requestFrom(t, engine, probeRoute, firstPeer)

		// then
		synctest.Wait()
		writer.AssertExpectations(t)
	})
}

func TestLastSeenIP_ForgetsADeletedAccountWithoutAnErrorLog(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given the request that is finishing has just deleted the account
		logs := captureLog(t, zerolog.DebugLevel)
		writer := repository.NewMockUserRepository(t)
		principal, userID := userPrincipal()
		update := spec.UserLastSeenIPUpdate{UserID: userID, IP: firstPeerIP}
		writer.EXPECT().UpdateLastSeenIP(mock.Anything, update).Return(fmt.Errorf("update last seen ip for user %s: %w", userID, dao.ErrNotFound)).Once()
		recorder := NewLastSeenIP(writer)
		engine := lastSeenEngine(t, recorder, principal)

		// when
		requestFrom(t, engine, probeRoute, firstPeer)
		synctest.Wait()

		// then the missing row is an expected outcome, logged at info, and the account is forgotten
		_, remembered := recorder.seen.Load(userID)
		assert.False(t, remembered)

		var levels []string
		for line := range bytes.Lines(logs.Bytes()) {
			var entry map[string]any
			require.NoError(t, json.Unmarshal(line, &entry))
			levels = append(levels, fmt.Sprint(entry["level"]))
		}
		assert.Equal(t, []string{"info"}, levels)
	})
}

func TestLastSeenIP_RecordsNothingWithoutAUserPrincipal(t *testing.T) {
	tests := []struct {
		name      string
		principal model.Principal
		path      string
	}{
		{name: "no principal", principal: model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}, path: publicRoute},
		{name: "a guest", principal: model.Principal{Kind: guestKind, ID: uuid.New()}, path: probeRoute},
		{name: "the socket route", principal: model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}, path: socketRoute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// given a strict writer, so any write fails the test
				writer := repository.NewMockUserRepository(t)
				engine := lastSeenEngine(t, NewLastSeenIP(writer), tc.principal)

				// when
				requestFrom(t, engine, tc.path, firstPeer)

				// then
				synctest.Wait()
				writer.AssertNotCalled(t, "UpdateLastSeenIP", mock.Anything, mock.Anything)
			})
		})
	}
}
