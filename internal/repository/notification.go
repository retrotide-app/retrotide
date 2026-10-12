package repository

import (
	"retrotide/internal/dao"
)

type (
	NotificationRepository interface {
		dao.NotificationDAO
	}

	notificationRepository struct {
		dao.NotificationDAO
	}
)

func NewNotificationRepo(notificationDAO dao.NotificationDAO) NotificationRepository {
	return &notificationRepository{NotificationDAO: notificationDAO}
}
