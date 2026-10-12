package dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	NotificationDAO interface {
		Create(ctx context.Context, s spec.NewNotification, tx ...*sql.Tx) (*model.Notification, error)
		ListByUser(ctx context.Context, s spec.NotificationListing, tx ...*sql.Tx) ([]model.Notification, error)
		CountByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int, error)
		CountUnread(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int, error)
		MarkRead(ctx context.Context, s spec.NotificationLookup, tx ...*sql.Tx) error
		MarkAllRead(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int64, error)
		DeleteCreatedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error)
	}

	notificationDAO struct {
		db *sql.DB
	}
)

func toNotification(row sqlcgen.Notification) model.Notification {
	return model.Notification{
		ID:            row.ID,
		UserID:        row.UserID,
		OrgID:         row.OrgID,
		Type:          model.NotificationType(row.Type),
		ReferenceType: row.ReferenceType,
		ReferenceID:   row.ReferenceID,
		ActorID:       row.ActorID,
		Message:       row.Message,
		Read:          row.Read,
		CreatedAt:     row.CreatedAt,
	}
}

func (r *notificationDAO) Create(ctx context.Context, s spec.NewNotification, tx ...*sql.Tx) (*model.Notification, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateNotification(ctx, sqlcgen.CreateNotificationParams{
		UserID:        s.UserID,
		OrgID:         nullableUUID(s.OrgID),
		Type:          string(s.Type),
		ReferenceType: s.ReferenceType,
		ReferenceID:   nullableUUID(s.ReferenceID),
		ActorID:       nullableUUID(s.ActorID),
		Message:       s.Message,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s notification for user %s: %w", s.Type, s.UserID, err)
	}

	return new(toNotification(row)), nil
}

func (r *notificationDAO) ListByUser(ctx context.Context, s spec.NotificationListing, tx ...*sql.Tx) ([]model.Notification, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListNotificationsForUser(ctx, sqlcgen.ListNotificationsForUserParams{
		UserID: s.UserID,
		Limit:  int32(s.Limit),
		Offset: int32(s.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications for user %s: %w", s.UserID, err)
	}

	notifications := make([]model.Notification, 0, len(rows))
	for _, row := range rows {
		notifications = append(notifications, toNotification(row))
	}

	return notifications, nil
}

func (r *notificationDAO) CountByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountNotificationsForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count notifications for user %s: %w", userID, err)
	}

	return int(count), nil
}

func (r *notificationDAO) CountUnread(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountUnreadNotificationsForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications for user %s: %w", userID, err)
	}

	return int(count), nil
}

func (r *notificationDAO) MarkRead(ctx context.Context, s spec.NotificationLookup, tx ...*sql.Tx) error {
	affected, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MarkNotificationRead(ctx, sqlcgen.MarkNotificationReadParams{
		ID:     s.ID,
		UserID: s.UserID,
	})
	if err != nil {
		return fmt.Errorf("mark notification %d read for user %s: %w", s.ID, s.UserID, err)
	}

	if affected == 0 {
		return fmt.Errorf("mark notification %d read for user %s: %w", s.ID, s.UserID, ErrNotFound)
	}

	return nil
}

func (r *notificationDAO) MarkAllRead(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	marked, err := sqlcgen.New(db.TxOrDB(r.db, tx)).MarkAllNotificationsRead(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read for user %s: %w", userID, err)
	}

	return marked, nil
}

func (r *notificationDAO) DeleteCreatedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteNotificationsCreatedBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete notifications created before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return deleted, nil
}
