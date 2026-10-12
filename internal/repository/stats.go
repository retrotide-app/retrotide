package repository

import (
	"retrotide/internal/dao"
)

type (
	StatsRepository interface {
		dao.StatsDAO
	}

	statsRepository struct {
		dao.StatsDAO
	}
)

func NewStatsRepo(statsDAO dao.StatsDAO) StatsRepository {
	return &statsRepository{StatsDAO: statsDAO}
}
