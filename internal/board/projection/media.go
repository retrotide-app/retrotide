package projection

import (
	"fmt"

	"retrotide/internal/dto"
	"retrotide/internal/model"
)

func MediaResponse(media model.Media) dto.MediaView {
	return dto.MediaView{
		ID:     media.ID,
		Path:   fmt.Sprintf(model.MediaPathFormat, media.BoardID, media.ID),
		Width:  media.Width,
		Height: media.Height,
	}
}
