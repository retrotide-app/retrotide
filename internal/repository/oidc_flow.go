package repository

import (
	"retrotide/internal/dao"
)

type (
	OIDCFlowRepository interface {
		dao.OIDCFlowDAO
	}

	oidcFlowRepository struct {
		dao.OIDCFlowDAO
	}
)

func NewOIDCFlowRepo(flowDAO dao.OIDCFlowDAO) OIDCFlowRepository {
	return &oidcFlowRepository{OIDCFlowDAO: flowDAO}
}
