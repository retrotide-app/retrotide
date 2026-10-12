package repository

import (
	"retrotide/internal/dao"
)

type (
	GuestDeviceRepository interface {
		dao.GuestDeviceDAO
	}

	guestDeviceRepository struct {
		dao.GuestDeviceDAO
	}
)

func NewGuestDeviceRepo(guestDeviceDAO dao.GuestDeviceDAO) GuestDeviceRepository {
	return &guestDeviceRepository{GuestDeviceDAO: guestDeviceDAO}
}
