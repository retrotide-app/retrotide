package repository

import (
	"retrotide/internal/dao"
)

type (
	AuditLogRepository interface {
		dao.AuditLogDAO
	}

	auditLogRepository struct {
		dao.AuditLogDAO
	}
)

func NewAuditLogRepo(auditLogDAO dao.AuditLogDAO) AuditLogRepository {
	return &auditLogRepository{AuditLogDAO: auditLogDAO}
}
