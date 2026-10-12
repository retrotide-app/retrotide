package dto

import "github.com/google/uuid"

type (
	CardPinRequest struct {
		BoardCommandRequest

		Pinned bool `json:"pinned"`
	}

	CardImageRequest struct {
		BoardCommandRequest

		ImageID *uuid.UUID `json:"image_id"`
	}

	BoardBackgroundRequest struct {
		BoardCommandRequest

		ImageID *uuid.UUID `json:"image_id"`
	}

	MediaView struct {
		ID     uuid.UUID `json:"id"`
		Path   string    `json:"path"`
		Width  int       `json:"width"`
		Height int       `json:"height"`
	}
)
