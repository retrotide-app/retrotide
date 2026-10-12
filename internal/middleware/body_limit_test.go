package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"retrotide/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBodyLimit = 16
)

func TestBodyLimit_RefusesADeclaredLengthOverTheLimitBeforeTheHandler(t *testing.T) {
	// given
	reached := false
	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit).Handle)
	engine.POST("/upload", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusNoContent)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", strings.NewReader(strings.Repeat("x", testBodyLimit+1))))

	// then
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.JSONEq(t, `{"error":"request body too large"}`, rec.Body.String())
	assert.False(t, reached)
}

func TestBodyLimit_FailsAChunkedBodyOverTheLimitWhenItIsRead(t *testing.T) {
	// given a body of unknown length
	var readErr error

	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit).Handle)
	engine.POST("/upload", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", io.MultiReader(strings.NewReader(strings.Repeat("x", testBodyLimit*4))))
	require.Equal(t, int64(-1), req.ContentLength)

	// when
	serve(engine, req)

	// then
	tooLarge, isTooLarge := errors.AsType[*http.MaxBytesError](readErr)
	require.True(t, isTooLarge, "got %v", readErr)
	assert.Equal(t, int64(testBodyLimit), tooLarge.Limit)
}

func TestBodyLimit_LetsABodyWithinTheLimitThrough(t *testing.T) {
	// given
	var body []byte

	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit).Handle)
	engine.POST("/upload", func(c *gin.Context) {
		read, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		body = read
		c.Status(http.StatusNoContent)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", strings.NewReader("small")))

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "small", string(body))
}

func TestBodyLimit_LeavesTheImageUploadRouteToItsOwnPlanLimit(t *testing.T) {
	// given an upload larger than the global limit, which the media controller bounds by the plan instead
	var body []byte

	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit).Handle)
	engine.POST(boardMediaRoute, func(c *gin.Context) {
		read, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		body = read
		c.Status(http.StatusCreated)
	})
	image := strings.Repeat("x", testBodyLimit*4)

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/boards/0e1f2a3b-4c5d-4e6f-a071-8c9d0e1f2a3b/media", strings.NewReader(image)))

	// then
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, image, string(body))
}

func TestBodyLimit_StillBoundsEveryOtherBoardRoute(t *testing.T) {
	// given
	reached := false
	engine := newTestEngine(t)
	engine.Use(NewBodyLimit(testBodyLimit).Handle)
	engine.POST("/api/v1/boards/:boardID/cards", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	// when
	rec := serve(engine, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/boards/0e1f2a3b-4c5d-4e6f-a071-8c9d0e1f2a3b/cards", strings.NewReader(strings.Repeat("x", testBodyLimit+1))))

	// then
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.False(t, reached)
}

func TestBodyLimit_FollowsTheSetting(t *testing.T) {
	const initial = 1 << 20

	tests := []struct {
		name  string
		key   config.SiteSettingKey
		value string
		want  int64
	}{
		{name: "raised", key: config.SettingMaxBodySize.Key, value: "4194304", want: 4194304},
		{name: "lowered", key: config.SettingMaxBodySize.Key, value: "65536", want: 65536},
		{name: "another setting changed", key: config.SettingSiteName.Key, value: "123", want: initial},
		{name: "a value that is not a number", key: config.SettingMaxBodySize.Key, value: "lots", want: initial},
		{name: "zero, which would refuse every body", key: config.SettingMaxBodySize.Key, value: "0", want: initial},
		{name: "a negative value, which would refuse every request", key: config.SettingMaxBodySize.Key, value: "-1", want: initial},
		{name: "a value under the floor", key: config.SettingMaxBodySize.Key, value: "65535", want: initial},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			limit := NewBodyLimit(initial)

			// when
			limit.OnSettingChanged(tc.key, tc.value)

			// then
			assert.Equal(t, tc.want, limit.maxBytes.Load())
		})
	}
}

func TestValidateBodyLimit(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{value: "65535", wantErr: true},
		{value: "0", wantErr: true},
		{value: "-1", wantErr: true},
		{value: "abc", wantErr: true},
		{value: "", wantErr: true},
		{value: "65536", wantErr: false},
		{value: "1048576", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			// given
			value := tc.value

			// when
			err := ValidateBodyLimit(t.Context(), value)

			// then
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestParseBodyLimit_ReturnsTheSizeItAccepts(t *testing.T) {
	// given
	value := "1048576"

	// when
	size, err := ParseBodyLimit(value)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1048576), size)
}
