package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/notification"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	notificationHarness struct {
		*testutil.Harness
		notifications *notification.MockService
	}
)

func newNotificationHarness(t *testing.T) *notificationHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	n := &notificationHarness{Harness: h, notifications: notification.NewMockService(t)}

	s := &Service{NotificationService: n.notifications, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllNotificationRoutes())

	return n
}

func TestNotificationController_ListsTheSignedInUsersPage(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	listing := &dto.NotificationListResponse{Notifications: []dto.NotificationResponse{}, Total: 0, Limit: 10, Offset: 20}
	h.notifications.EXPECT().List(mock.Anything, spec.NotificationListing{UserID: userID, Limit: 10, Offset: 20}).Return(listing, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/notifications?limit=10&offset=20").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"notifications":[],"total":0,"limit":10,"offset":20}`, rec.Body.String())
}

func TestNotificationController_LeavesAMissingPageToTheService(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.notifications.EXPECT().List(mock.Anything, spec.NotificationListing{UserID: userID}).Return(&dto.NotificationListResponse{Notifications: []dto.NotificationResponse{}, Limit: 20}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/notifications").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNotificationController_RefusesAMalformedPage(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	cookie := h.SignIn(uuid.New())

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/notifications?limit=ten").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid limit"}`, rec.Body.String())
}

func TestNotificationController_ReportsAFailedList(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	cookie := h.SignIn(uuid.New())
	h.notifications.EXPECT().List(mock.Anything, mock.Anything).Return(nil, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/notifications").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to list the notifications"}`, rec.Body.String())
}

func TestNotificationController_CountsTheUnread(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.notifications.EXPECT().UnreadCount(mock.Anything, userID).Return(3, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"count":3}`, rec.Body.String())
}

func TestNotificationController_AnUnverifiedUserMayMarkNotificationsRead(t *testing.T) {
	tests := []struct {
		path   string
		expect func(h *notificationHarness, userID uuid.UUID)
	}{
		{
			path: "/api/v1/notifications/read",
			expect: func(h *notificationHarness, userID uuid.UUID) {
				h.notifications.EXPECT().MarkAllRead(mock.Anything, userID).Return(nil)
			},
		},
		{
			path: "/api/v1/notifications/42/read",
			expect: func(h *notificationHarness, userID uuid.UUID) {
				h.notifications.EXPECT().MarkRead(mock.Anything, spec.NotificationLookup{UserID: userID, ID: 42}).Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given a signed-in user whose lock and verification state are never read on these routes
			h := newNotificationHarness(t)
			userID := uuid.New()
			cookie := h.SignIn(userID)
			tc.expect(h, userID)

			// when
			rec := h.NewRequest(http.MethodPost, tc.path).WithCookie(cookie).Record()

			// then neither write gate is consulted, which the strict account mock would report
			assert.Equal(t, http.StatusNoContent, rec.Code)
		})
	}
}

func TestNotificationController_AnotherUsersNotificationIsNotFound(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.notifications.EXPECT().MarkRead(mock.Anything, spec.NotificationLookup{UserID: userID, ID: 7}).Return(fmt.Errorf("mark the notification read: %w", dao.ErrNotFound))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/notifications/7/read").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"notification not found"}`, rec.Body.String())
}

func TestNotificationController_RefusesAMalformedID(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	cookie := h.SignIn(uuid.New())

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/notifications/abc/read").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid id"}`, rec.Body.String())
}

func TestNotificationController_ReportsAFailedMark(t *testing.T) {
	// given
	h := newNotificationHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.notifications.EXPECT().MarkAllRead(mock.Anything, userID).Return(errServiceDown)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/notifications/read").WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to mark the notifications read"}`, rec.Body.String())
}

func TestNotificationController_RoutesRunTheAuthFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodGet, Path: "/api/v1/notifications"},
		{Method: http.MethodGet, Path: "/api/v1/notifications/unread-count"},
		{Method: http.MethodPost, Path: "/api/v1/notifications/read", LockExempt: true},
		{Method: http.MethodPost, Path: "/api/v1/notifications/42/read", LockExempt: true},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newNotificationHarness(t).Harness
			}, route)
		})
	}
}
