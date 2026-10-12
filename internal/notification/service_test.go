package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	serviceMocks struct {
		notifications *repository.MockNotificationRepository
		users         *repository.MockUserRepository
		email         *email.MockService
		settings      *settings.MockService
	}
)

func newTestService(t *testing.T) (Service, serviceMocks) {
	t.Helper()

	mocks := serviceMocks{
		notifications: repository.NewMockNotificationRepository(t),
		users:         repository.NewMockUserRepository(t),
		email:         email.NewMockService(t),
		settings:      settings.NewMockService(t),
	}

	return NewService(mocks.notifications, mocks.users, mocks.email, mocks.settings), mocks
}

func notifyParams(notificationType model.NotificationType) dto.NotifyParams {
	return dto.NotifyParams{
		RecipientID:   uuid.New(),
		OrgID:         uuid.New(),
		Type:          notificationType,
		ReferenceType: "action_item",
		ReferenceID:   uuid.New(),
		ActorID:       uuid.New(),
		Message:       "You were assigned Fix the deploy script",
		Link:          "/actions",
	}
}

func expectSaved(mocks serviceMocks, params dto.NotifyParams) {
	saved := spec.NewNotification{
		UserID:        params.RecipientID,
		OrgID:         params.OrgID,
		Type:          params.Type,
		ReferenceType: params.ReferenceType,
		ReferenceID:   params.ReferenceID,
		ActorID:       params.ActorID,
		Message:       params.Message,
	}

	mocks.notifications.EXPECT().Create(mock.Anything, saved).Return(&model.Notification{ID: 7, UserID: params.RecipientID, Type: params.Type}, nil)
}

func expectRecipient(mocks serviceMocks, params dto.NotifyParams, emailNotifications bool) {
	recipient := &model.User{ID: params.RecipientID, Email: "sam@example.com", EmailNotifications: emailNotifications}

	mocks.users.EXPECT().GetByID(mock.Anything, params.RecipientID).Return(recipient, nil)
}

func expectSiteSettings(mocks serviceMocks) {
	mocks.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return("RetroTide", nil)
	mocks.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("https://retrotide.test/", nil)
}

func TestNotify_SkipsANotificationToTheActorThemself(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	params := notifyParams(model.NotificationActionItemAssigned)
	params.ActorID = params.RecipientID

	// when
	err := svc.Notify(t.Context(), params)

	// then nothing is written or sent
	require.NoError(t, err)
}

func TestNotify_SavesTheRowWithoutEmailWhileNoProviderIsConfigured(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	params := notifyParams(model.NotificationOrgInvite)
	expectSaved(mocks, params)
	mocks.email.EXPECT().Enabled(mock.Anything).Return(false, nil)

	// when
	err := svc.Notify(t.Context(), params)

	// then no recipient lookup and no send
	require.NoError(t, err)
}

func TestNotify_EmailsAServiceMessageWhateverThePreference(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	params := notifyParams(model.NotificationPaymentFailed)
	expectSaved(mocks, params)
	mocks.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	expectRecipient(mocks, params, false)
	expectSiteSettings(mocks)

	var sent email.Message
	mocks.email.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, message email.Message) error {
		sent = message

		return nil
	})

	// when
	err := svc.Notify(t.Context(), params)

	// then
	require.NoError(t, err)
	assert.Equal(t, "sam@example.com", sent.To)
	assert.Equal(t, "You have a new notification on RetroTide", sent.Subject)
	assert.Contains(t, sent.HTML, "You were assigned Fix the deploy script")
	assert.Contains(t, sent.HTML, `href="https://retrotide.test/actions"`)
}

func TestNotify_FollowsThePreferenceForAnAssignedActionItem(t *testing.T) {
	// given
	cases := []struct {
		name               string
		emailNotifications bool
		wantEmail          bool
	}{
		{name: "email on sends", emailNotifications: true, wantEmail: true},
		{name: "email off does not send", emailNotifications: false, wantEmail: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mocks := newTestService(t)
			params := notifyParams(model.NotificationActionItemAssigned)
			expectSaved(mocks, params)
			mocks.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
			expectRecipient(mocks, params, tc.emailNotifications)

			if tc.wantEmail {
				expectSiteSettings(mocks)
				mocks.email.EXPECT().Send(mock.Anything, mock.MatchedBy(func(message email.Message) bool {
					return message.To == "sam@example.com"
				})).Return(nil)
			}

			// when
			err := svc.Notify(t.Context(), params)

			// then
			require.NoError(t, err)
		})
	}
}

func TestNotify_ReturnsTheSaveErrorAndSendsNothing(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	params := notifyParams(model.NotificationOrgInvite)
	saveErr := errors.New("database unavailable")
	mocks.notifications.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, saveErr)

	// when
	err := svc.Notify(t.Context(), params)

	// then
	require.ErrorIs(t, err, saveErr)
}

func TestNotify_ReturnsAnEmailFailureAfterSavingTheRow(t *testing.T) {
	// given
	cases := []struct {
		name   string
		expect func(mocks serviceMocks, params dto.NotifyParams, failure error)
	}{
		{name: "the provider check fails", expect: func(mocks serviceMocks, _ dto.NotifyParams, failure error) {
			mocks.email.EXPECT().Enabled(mock.Anything).Return(false, failure)
		}},
		{name: "the recipient lookup fails", expect: func(mocks serviceMocks, params dto.NotifyParams, failure error) {
			mocks.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
			mocks.users.EXPECT().GetByID(mock.Anything, params.RecipientID).Return(nil, failure)
		}},
		{name: "the send fails", expect: func(mocks serviceMocks, params dto.NotifyParams, failure error) {
			mocks.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
			expectRecipient(mocks, params, true)
			expectSiteSettings(mocks)
			mocks.email.EXPECT().Send(mock.Anything, mock.Anything).Return(failure)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mocks := newTestService(t)
			params := notifyParams(model.NotificationOrgInvite)
			failure := errors.New("provider unavailable")
			expectSaved(mocks, params)
			tc.expect(mocks, params, failure)

			// when
			err := svc.Notify(t.Context(), params)

			// then
			require.ErrorIs(t, err, failure)
			assert.Contains(t, err.Error(), "the notification itself is saved")
		})
	}
}

func TestNotify_KeepsAnAbsoluteLink(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	params := notifyParams(model.NotificationOrgInvite)
	params.Link = "https://billing.example.com/portal"
	expectSaved(mocks, params)
	mocks.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	expectRecipient(mocks, params, true)
	mocks.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return("RetroTide", nil)
	mocks.email.EXPECT().Send(mock.Anything, mock.MatchedBy(func(message email.Message) bool {
		return strings.Contains(message.HTML, `href="https://billing.example.com/portal"`)
	})).Return(nil)

	// when
	err := svc.Notify(t.Context(), params)

	// then
	require.NoError(t, err)
}

func TestList_ClampsThePage(t *testing.T) {
	// given
	cases := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{name: "a missing limit takes the default", limit: 0, offset: 0, wantLimit: defaultPageSize, wantOffset: 0},
		{name: "a large limit is capped", limit: 500, offset: 40, wantLimit: maxPageSize, wantOffset: 40},
		{name: "a negative offset starts at the top", limit: 10, offset: -5, wantLimit: 10, wantOffset: 0},
		{name: "a deep offset is capped", limit: 10, offset: 5000, wantLimit: 10, wantOffset: maxOffset},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mocks := newTestService(t)
			userID := uuid.New()
			clamped := spec.NotificationListing{UserID: userID, Limit: tc.wantLimit, Offset: tc.wantOffset}
			mocks.notifications.EXPECT().ListByUser(mock.Anything, clamped).Return([]model.Notification{}, nil)
			mocks.notifications.EXPECT().CountByUser(mock.Anything, userID).Return(0, nil)

			// when
			got, err := svc.List(t.Context(), spec.NotificationListing{UserID: userID, Limit: tc.limit, Offset: tc.offset})

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.wantLimit, got.Limit)
			assert.Equal(t, tc.wantOffset, got.Offset)
		})
	}
}

func TestList_MapsTheRowsAndTheTotal(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	rows := []model.Notification{
		{ID: 2, UserID: userID, Type: model.NotificationOrgInvite, Message: "Join Acme", CreatedAt: time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)},
		{ID: 1, UserID: userID, Type: model.NotificationTrialEnding, Read: true, CreatedAt: time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)},
	}
	mocks.notifications.EXPECT().ListByUser(mock.Anything, spec.NotificationListing{UserID: userID, Limit: defaultPageSize}).Return(rows, nil)
	mocks.notifications.EXPECT().CountByUser(mock.Anything, userID).Return(12, nil)

	// when
	got, err := svc.List(t.Context(), spec.NotificationListing{UserID: userID})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.NotificationListResponse{
		Notifications: []dto.NotificationResponse{dto.NewNotificationResponse(rows[0]), dto.NewNotificationResponse(rows[1])},
		Total:         12,
		Limit:         defaultPageSize,
		Offset:        0,
	}, got)
}

func TestList_AnEmptyPageIsAnEmptyList(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.notifications.EXPECT().ListByUser(mock.Anything, mock.Anything).Return([]model.Notification{}, nil)
	mocks.notifications.EXPECT().CountByUser(mock.Anything, userID).Return(0, nil)

	// when
	got, err := svc.List(t.Context(), spec.NotificationListing{UserID: userID})

	// then the list marshals as [] rather than null
	require.NoError(t, err)
	assert.NotNil(t, got.Notifications)
	assert.Empty(t, got.Notifications)
}

func TestList_ReturnsTheReadError(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	readErr := errors.New("database unavailable")
	mocks.notifications.EXPECT().ListByUser(mock.Anything, mock.Anything).Return(nil, readErr)

	// when
	_, err := svc.List(t.Context(), spec.NotificationListing{UserID: uuid.New()})

	// then
	require.ErrorIs(t, err, readErr)
}

func TestMarkRead_KeepsTheNotFoundSentinel(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	lookup := spec.NotificationLookup{UserID: uuid.New(), ID: 42}
	mocks.notifications.EXPECT().MarkRead(mock.Anything, lookup).Return(fmt.Errorf("mark notification 42 read: %w", dao.ErrNotFound))

	// when
	err := svc.MarkRead(t.Context(), lookup)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestMarkAllRead(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.notifications.EXPECT().MarkAllRead(mock.Anything, userID).Return(3, nil)

	// when
	err := svc.MarkAllRead(t.Context(), userID)

	// then
	require.NoError(t, err)
}

func TestUnreadCount(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.notifications.EXPECT().CountUnread(mock.Anything, userID).Return(4, nil)

	// when
	count, err := svc.UnreadCount(t.Context(), userID)

	// then
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestPruneOld_DeletesRowsOlderThanTheRetention(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	before := time.Now().Add(-retention)
	mocks.notifications.EXPECT().DeleteCreatedBefore(mock.Anything, mock.MatchedBy(func(cutoff time.Time) bool {
		return !cutoff.Before(before) && cutoff.Before(time.Now().Add(-retention+time.Minute))
	})).Return(9, nil)

	// when
	deleted, err := svc.PruneOld(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(9), deleted)
}
