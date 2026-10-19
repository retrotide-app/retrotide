package media

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"retrotide/internal/logger"
)

type (
	cwebpEncoder struct {
		timeout   time.Duration
		queueWait time.Duration
		slots     chan struct{}
	}
)

const (
	cwebpBinary      = "cwebp"
	cwebpTimeout     = 30 * time.Second
	cwebpQuality     = "80"
	cwebpConcurrency = 2
	cwebpQueueWait   = 10 * time.Second
)

func NewCWebPEncoder() Encoder {
	return &cwebpEncoder{timeout: cwebpTimeout, queueWait: cwebpQueueWait, slots: make(chan struct{}, cwebpConcurrency)}
}

func (e *cwebpEncoder) Encode(ctx context.Context, input []byte, maxEdge int) (*Encoded, error) {
	binary, err := exec.LookPath(cwebpBinary)
	if err != nil {
		return nil, fmt.Errorf("%w: find %s: %w", ErrEncoderUnavailable, cwebpBinary, err)
	}

	width, height, err := imageSize(input)
	if err != nil {
		return nil, err
	}

	if err := e.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() {
		<-e.slots
	}()

	inPath, err := writeTemp(ctx, input)
	if err != nil {
		return nil, err
	}
	defer removeTemp(ctx, inPath)

	outPath, err := reserveTemp(ctx)
	if err != nil {
		return nil, err
	}
	defer removeTemp(ctx, outPath)

	args := []string{"-quiet", "-metadata", "none", "-q", cwebpQuality}
	args = append(args, resizeArgs(width, height, maxEdge)...)
	args = append(args, inPath, "-o", outPath)

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	output, err := exec.CommandContext(runCtx, binary, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: run %s on a %dx%d image: %w: %s", ErrEncoderUnavailable, cwebpBinary, width, height, err, strings.TrimSpace(string(output)))
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("%w: read the output of %s: %w", ErrEncoderUnavailable, cwebpBinary, err)
	}

	encodedWidth, encodedHeight, err := webpSize(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s wrote an unreadable image: %w", ErrEncoderUnavailable, cwebpBinary, err)
	}

	return &Encoded{Data: data, Width: encodedWidth, Height: encodedHeight}, nil
}

func (e *cwebpEncoder) acquire(ctx context.Context) error {
	wait := time.NewTimer(e.queueWait)
	defer wait.Stop()

	select {
	case e.slots <- struct{}{}:
		return nil
	case <-wait.C:
		return fmt.Errorf("%w: %d images are already being processed", ErrEncoderUnavailable, cap(e.slots))
	case <-ctx.Done():
		return fmt.Errorf("%w: waiting to process the image: %w", ErrEncoderUnavailable, ctx.Err())
	}
}

func resizeArgs(width, height, maxEdge int) []string {
	if width <= maxEdge && height <= maxEdge {
		return nil
	}

	if width >= height {
		return []string{"-resize", strconv.Itoa(maxEdge), "0"}
	}

	return []string{"-resize", "0", strconv.Itoa(maxEdge)}
}

func writeTemp(ctx context.Context, input []byte) (string, error) {
	file, err := os.CreateTemp("", "retrotide-upload-*")
	if err != nil {
		return "", fmt.Errorf("%w: create a temporary file for the upload: %w", ErrEncoderUnavailable, err)
	}

	path := file.Name()

	if _, err := file.Write(input); err != nil {
		closeTemp(ctx, file)
		removeTemp(ctx, path)

		return "", fmt.Errorf("%w: write the upload to %s: %w", ErrEncoderUnavailable, path, err)
	}

	if err := file.Close(); err != nil {
		removeTemp(ctx, path)

		return "", fmt.Errorf("%w: close the upload in %s: %w", ErrEncoderUnavailable, path, err)
	}

	return path, nil
}

func reserveTemp(ctx context.Context) (string, error) {
	file, err := os.CreateTemp("", "retrotide-webp-*.webp")
	if err != nil {
		return "", fmt.Errorf("%w: create a temporary file for the encoded image: %w", ErrEncoderUnavailable, err)
	}

	path := file.Name()

	if err := file.Close(); err != nil {
		removeTemp(ctx, path)

		return "", fmt.Errorf("%w: close the encoded image file %s: %w", ErrEncoderUnavailable, path, err)
	}

	return path, nil
}

func closeTemp(ctx context.Context, file *os.File) {
	if err := file.Close(); err != nil {
		logger.Ctx(ctx).Warn().Err(err).Str("path", file.Name()).Msg("failed to close a temporary image file")
	}
}

func removeTemp(ctx context.Context, path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.Ctx(ctx).Warn().Err(err).Str("path", path).Msg("failed to remove a temporary image file")
	}
}
