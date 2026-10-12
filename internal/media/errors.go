package media

import (
	"errors"
)

var (
	ErrUnsupportedImage   = errors.New("only PNG, JPEG and WebP images can be uploaded")
	ErrInvalidKind        = errors.New("an image is either a card image or a board background")
	ErrImageDimensions    = errors.New("the image is too large: it can have at most 40 megapixels")
	ErrEncoderUnavailable = errors.New("images cannot be processed right now, so try again later")
	ErrMediaNotFound      = errors.New("image not found")
)
