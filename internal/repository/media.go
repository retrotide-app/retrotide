package repository

import "retrotide/internal/dao"

type (
	MediaRepository interface {
		dao.MediaDAO
	}

	mediaRepository struct {
		dao.MediaDAO
	}
)

func NewMediaRepo(mediaDAO dao.MediaDAO) MediaRepository {
	return &mediaRepository{MediaDAO: mediaDAO}
}
