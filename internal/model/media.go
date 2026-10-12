package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	MediaKind string

	Media struct {
		ID          uuid.UUID
		OrgID       uuid.UUID
		BoardID     uuid.UUID
		Kind        MediaKind
		ContentType string
		Width       int
		Height      int
		ByteSize    int
		CreatedAt   time.Time
	}

	MediaFile struct {
		Media

		Data []byte
	}
)

const (
	MediaKindCard       MediaKind = "card"
	MediaKindBackground MediaKind = "background"
)

const (
	MediaContentType     = "image/webp"
	CardImageMaxEdge     = 1600
	BackgroundMaxEdge    = 2560
	MediaMaxPixels       = 40_000_000
	MediaUnattachedLimit = 24 * time.Hour
	MediaPathFormat      = "/uploads/%s/%s.webp"
)
