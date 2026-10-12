package media

import (
	"encoding/binary"
	"fmt"
)

const (
	webpHeaderSize        = 30
	riffTag               = "RIFF"
	webpTag               = "WEBP"
	webpChunkLossy        = "VP8 "
	webpChunkLossless     = "VP8L"
	webpChunkExtended     = "VP8X"
	webpLosslessSignature = 0x2f
	webpDimensionMask     = 0x3fff
	webpLosslessShift     = 14
)

var (
	webpLossyStartCode = [3]byte{0x9d, 0x01, 0x2a}
)

func webpSize(data []byte) (int, int, error) {
	if len(data) < webpHeaderSize || string(data[0:4]) != riffTag || string(data[8:12]) != webpTag {
		return 0, 0, fmt.Errorf("%w: the file is not a WebP image", ErrUnsupportedImage)
	}

	chunk := string(data[12:16])

	switch chunk {
	case webpChunkExtended:
		width := 1 + (int(data[24]) | int(data[25])<<8 | int(data[26])<<16)
		height := 1 + (int(data[27]) | int(data[28])<<8 | int(data[29])<<16)

		return width, height, nil
	case webpChunkLossless:
		if data[20] != webpLosslessSignature {
			return 0, 0, fmt.Errorf("%w: the lossless WebP image has no signature", ErrUnsupportedImage)
		}

		bits := binary.LittleEndian.Uint32(data[21:25])
		width := int(bits&webpDimensionMask) + 1
		height := int((bits>>webpLosslessShift)&webpDimensionMask) + 1

		return width, height, nil
	case webpChunkLossy:
		if [3]byte(data[23:26]) != webpLossyStartCode {
			return 0, 0, fmt.Errorf("%w: the lossy WebP image has no start code", ErrUnsupportedImage)
		}

		width := int(binary.LittleEndian.Uint16(data[26:28]) & webpDimensionMask)
		height := int(binary.LittleEndian.Uint16(data[28:30]) & webpDimensionMask)

		return width, height, nil
	}

	return 0, 0, fmt.Errorf("%w: the WebP image starts with an unknown %q chunk", ErrUnsupportedImage, chunk)
}
