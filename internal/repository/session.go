package repository

import (
	"retrotide/internal/dao"
)

type (
	SessionRepository interface {
		dao.SessionDAO
	}

	sessionRepository struct {
		dao.SessionDAO
	}
)

func NewSessionRepo(sessionDAO dao.SessionDAO) SessionRepository {
	return &sessionRepository{SessionDAO: sessionDAO}
}
