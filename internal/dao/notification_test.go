package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationDAO_CreateAndList(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)
	actor := daotest.CreateUser(t, repos)
	orgID := uuid.New()
	referenceID := uuid.New()

	// when
	created, err := repos.Notification.Create(t.Context(), spec.NewNotification{
		UserID:        recipient.ID,
		OrgID:         orgID,
		Type:          model.NotificationOrgInvite,
		ReferenceType: "organisation_invite",
		ReferenceID:   referenceID,
		ActorID:       actor.ID,
		Message:       "invited you to Harbour",
	})

	// then
	require.NoError(t, err)
	listed, err := repos.Notification.ListByUser(t.Context(), spec.NotificationListing{UserID: recipient.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	got := listed[0]
	assert.Equal(t, *created, got)
	assert.Equal(t, model.NotificationOrgInvite, got.Type)
	assert.Equal(t, "organisation_invite", got.ReferenceType)
	assert.Equal(t, "invited you to Harbour", got.Message)
	assert.False(t, got.Read)
	require.NotNil(t, got.OrgID)
	assert.Equal(t, orgID, *got.OrgID)
	require.NotNil(t, got.ReferenceID)
	assert.Equal(t, referenceID, *got.ReferenceID)
	require.NotNil(t, got.ActorID)
	assert.Equal(t, actor.ID, *got.ActorID)
}

func TestNotificationDAO_Create_LeavesUnsetReferencesEmpty(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)

	// when
	created, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationPaymentFailed})

	// then
	require.NoError(t, err)
	assert.Nil(t, created.OrgID)
	assert.Nil(t, created.ReferenceID)
	assert.Nil(t, created.ActorID)
	assert.Empty(t, created.ReferenceType)
}

func TestNotificationDAO_ListByUser_NewestFirstAndPaged(t *testing.T) {
	// given three notifications, each a minute newer than the last
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	var ids []int64
	for age := range 3 {
		created, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationOrgInvite})
		require.NoError(t, err)
		_, err = repos.DB().ExecContext(t.Context(), `UPDATE notifications SET created_at = NOW() - make_interval(mins => $2) WHERE id = $1`, created.ID, 3-age)
		require.NoError(t, err)
		ids = append(ids, created.ID)
	}
	_, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: other.ID, Type: model.NotificationOrgInvite})
	require.NoError(t, err)

	// when
	firstPage, firstErr := repos.Notification.ListByUser(t.Context(), spec.NotificationListing{UserID: recipient.ID, Limit: 2})
	secondPage, secondErr := repos.Notification.ListByUser(t.Context(), spec.NotificationListing{UserID: recipient.ID, Limit: 2, Offset: 2})

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Len(t, firstPage, 2)
	require.Len(t, secondPage, 1)
	assert.Equal(t, []int64{ids[2], ids[1], ids[0]}, []int64{firstPage[0].ID, firstPage[1].ID, secondPage[0].ID})
}

func TestNotificationDAO_ListByUser_EmptyIsAnEmptySlice(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)

	// when
	listed, err := repos.Notification.ListByUser(t.Context(), spec.NotificationListing{UserID: recipient.ID, Limit: 10})

	// then
	require.NoError(t, err)
	assert.NotNil(t, listed)
	assert.Empty(t, listed)
}

func TestNotificationDAO_CountsAndMarkRead(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)
	first, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationOrgInvite})
	require.NoError(t, err)
	_, err = repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationActionItemAssigned})
	require.NoError(t, err)

	// when
	err = repos.Notification.MarkRead(t.Context(), spec.NotificationLookup{UserID: recipient.ID, ID: first.ID})

	// then
	require.NoError(t, err)
	total, err := repos.Notification.CountByUser(t.Context(), recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	unread, err := repos.Notification.CountUnread(t.Context(), recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)
}

func TestNotificationDAO_MarkRead_AnotherUsersNotificationIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)
	intruder := daotest.CreateUser(t, repos)
	created, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationOrgInvite})
	require.NoError(t, err)

	// when
	err = repos.Notification.MarkRead(t.Context(), spec.NotificationLookup{UserID: intruder.ID, ID: created.ID})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	unread, err := repos.Notification.CountUnread(t.Context(), recipient.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)
}

func TestNotificationDAO_MarkAllRead_OnlyTouchesTheUsersUnreadNotifications(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	for range 2 {
		_, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationOrgInvite})
		require.NoError(t, err)
	}
	_, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: other.ID, Type: model.NotificationOrgInvite})
	require.NoError(t, err)

	// when
	marked, err := repos.Notification.MarkAllRead(t.Context(), recipient.ID)
	again, againErr := repos.Notification.MarkAllRead(t.Context(), recipient.ID)

	// then
	require.NoError(t, err)
	require.NoError(t, againErr)
	assert.Equal(t, int64(2), marked)
	assert.Zero(t, again)
	otherUnread, err := repos.Notification.CountUnread(t.Context(), other.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, otherUnread)
}

func TestNotificationDAO_DeleteCreatedBefore_PrunesOnlyOldRows(t *testing.T) {
	// given one notification from 100 days ago and one from now
	repos := daotest.NewRepos(t)
	recipient := daotest.CreateUser(t, repos)
	old, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationOrgInvite})
	require.NoError(t, err)
	_, err = repos.DB().ExecContext(t.Context(), `UPDATE notifications SET created_at = NOW() - INTERVAL '100 days' WHERE id = $1`, old.ID)
	require.NoError(t, err)
	recent, err := repos.Notification.Create(t.Context(), spec.NewNotification{UserID: recipient.ID, Type: model.NotificationOrgInvite})
	require.NoError(t, err)

	// when
	deleted, err := repos.Notification.DeleteCreatedBefore(t.Context(), time.Now().Add(-90*24*time.Hour))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	listed, err := repos.Notification.ListByUser(t.Context(), spec.NotificationListing{UserID: recipient.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, recent.ID, listed[0].ID)
}
