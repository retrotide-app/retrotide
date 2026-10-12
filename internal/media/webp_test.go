package media

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func webpFile(chunk string, payload []byte) []byte {
	data := []byte(riffTag)
	data = binary.LittleEndian.AppendUint32(data, 0)
	data = append(data, webpTag...)
	data = append(data, chunk...)
	data = binary.LittleEndian.AppendUint32(data, uint32(len(payload)))
	data = append(data, payload...)

	for len(data) < webpHeaderSize {
		data = append(data, 0)
	}

	return data
}

func extendedWebP(width, height int) []byte {
	payload := []byte{0, 0, 0, 0}
	payload = append(payload, byte(width-1), byte((width-1)>>8), byte((width-1)>>16))
	payload = append(payload, byte(height-1), byte((height-1)>>8), byte((height-1)>>16))

	return webpFile(webpChunkExtended, payload)
}

func losslessWebP(width, height int) []byte {
	payload := []byte{webpLosslessSignature}
	payload = binary.LittleEndian.AppendUint32(payload, uint32(width-1)|uint32(height-1)<<webpLosslessShift)

	return webpFile(webpChunkLossless, payload)
}

func lossyWebP(width, height int) []byte {
	payload := []byte{0, 0, 0}
	payload = append(payload, webpLossyStartCode[:]...)
	payload = binary.LittleEndian.AppendUint16(payload, uint16(width))
	payload = binary.LittleEndian.AppendUint16(payload, uint16(height))

	return webpFile(webpChunkLossy, payload)
}

func TestWebPSize_ReadsEveryKindOfWebPHeader(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		width  int
		height int
	}{
		{name: "extended", data: extendedWebP(800, 600), width: 800, height: 600},
		{name: "extended at the largest canvas", data: extendedWebP(16777216, 1), width: 16777216, height: 1},
		{name: "lossless", data: losslessWebP(640, 480), width: 640, height: 480},
		{name: "lossless at the largest edge", data: losslessWebP(16384, 16384), width: 16384, height: 16384},
		{name: "lossy", data: lossyWebP(320, 240), width: 320, height: 240},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			data := tc.data

			// when
			width, height, err := webpSize(data)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.width, width)
			assert.Equal(t, tc.height, height)
		})
	}
}

func TestWebPSize_RefusesAnythingThatIsNotAReadableWebPHeader(t *testing.T) {
	broken := lossyWebP(320, 240)
	broken[23] = 0
	unsigned := losslessWebP(640, 480)
	unsigned[20] = 0
	notRIFF := extendedWebP(800, 600)
	copy(notRIFF[0:4], "RIFX")
	wave := extendedWebP(800, 600)
	copy(wave[8:12], "WAVE")

	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "too short", data: extendedWebP(800, 600)[:29]},
		{name: "not riff", data: notRIFF},
		{name: "riff but not webp", data: wave},
		{name: "an unknown chunk", data: webpFile("ALPH", nil)},
		{name: "a lossy frame without its start code", data: broken},
		{name: "a lossless image without its signature", data: unsigned},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			data := tc.data

			// when
			_, _, err := webpSize(data)

			// then
			assert.ErrorIs(t, err, ErrUnsupportedImage)
		})
	}
}

func TestResizeArgs_ShrinksOnlyAnImageOverTheCapAlongItsLongestEdge(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		want   []string
	}{
		{name: "within the cap", width: 1600, height: 900, want: nil},
		{name: "wide", width: 4000, height: 3000, want: []string{"-resize", "1600", "0"}},
		{name: "square", width: 2000, height: 2000, want: []string{"-resize", "1600", "0"}},
		{name: "tall", width: 1000, height: 3000, want: []string{"-resize", "0", "1600"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			maxEdge := 1600

			// when
			got := resizeArgs(tc.width, tc.height, maxEdge)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
