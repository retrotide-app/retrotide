package utils

import (
	"net/http"
	"strings"
	"testing"

	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type (
	bindRequest struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	bindPeek struct {
		Name string `json:"name"`
	}
)

const (
	bindPath         = "/bind"
	bindLimitBytes   = 8
	peekedNameHeader = "X-Peeked-Name"
)

func bindEngine(before ...gin.HandlerFunc) *gin.Engine {
	engine := gin.New()
	engine.Use(before...)
	engine.POST(bindPath, func(c *gin.Context) {
		req, ok := BindJSON[bindRequest](c)
		if !ok {
			return
		}

		c.JSON(http.StatusOK, req)
	})

	return engine
}

func TestBindJSON_DecodesAValidBody(t *testing.T) {
	// given
	engine := bindEngine()

	// when
	rec := perform(t, engine, http.MethodPost, bindPath, strings.NewReader(`{"name":"retro","count":3}`))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"name":"retro","count":3}`, rec.Body.String())
}

func TestBindJSON_AnswersBadRequestWhenTheBodyDoesNotDecode(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"malformed JSON", `{"name":`},
		{"a field of the wrong type", `{"count":"three"}`},
		{"an empty body", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := bindEngine()

			// when
			rec := perform(t, engine, http.MethodPost, bindPath, strings.NewReader(tc.body))

			// then
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.JSONEq(t, `{"error":"invalid request body"}`, rec.Body.String())
		})
	}
}

func TestBindJSON_AnswersRequestEntityTooLargeOverTheBodyLimit(t *testing.T) {
	// given
	engine := bindEngine(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bindLimitBytes)
	})

	// when
	rec := perform(t, engine, http.MethodPost, bindPath, strings.NewReader(`{"name":"longer than the limit"}`))

	// then
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.JSONEq(t, `{"error":"request body too large"}`, rec.Body.String())
}

func TestBindJSON_DecodesTheCachedBodyAgainAfterAnEarlierBind(t *testing.T) {
	// given
	engine := bindEngine(func(c *gin.Context) {
		peek, ok := BindJSON[bindPeek](c)
		if !ok {
			return
		}

		c.Header(peekedNameHeader, peek.Name)
	})

	// when
	rec := perform(t, engine, http.MethodPost, bindPath, strings.NewReader(`{"name":"retro","count":3}`))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "retro", rec.Header().Get(peekedNameHeader))
	require.JSONEq(t, `{"name":"retro","count":3}`, rec.Body.String())
}

func clientIDEngine() *gin.Engine {
	engine := gin.New()
	engine.POST(bindPath, func(c *gin.Context) {
		req, ok := BindJSON[dto.CreateCardRequest](c)
		if !ok {
			return
		}

		c.JSON(http.StatusOK, gin.H{"id": req.ID})
	})

	return engine
}

func TestBindJSON_AcceptsAVersion4ClientID(t *testing.T) {
	// given
	engine := clientIDEngine()

	// when
	rec := perform(t, engine, http.MethodPost, bindPath, strings.NewReader(`{"id":"0b5c8e2a-6f1d-4a3b-9c7e-2d4f6a8b0c1e"}`))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"id":"0b5c8e2a-6f1d-4a3b-9c7e-2d4f6a8b0c1e"}`, rec.Body.String())
}

func TestBindJSON_AnswersBadRequestForAClientIDThatIsNotVersion4(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"a version 7 ID, which embeds its creation time", `{"id":"019a6b2c-3d4e-7f50-8a1b-2c3d4e5f6a7b"}`},
		{"a version 1 ID", `{"id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8"}`},
		{"a version 4 nibble with the wrong variant", `{"id":"0b5c8e2a-6f1d-4a3b-1c7e-2d4f6a8b0c1e"}`},
		{"the nil ID", `{"id":"00000000-0000-0000-0000-000000000000"}`},
		{"no ID at all", `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := clientIDEngine()

			// when
			rec := perform(t, engine, http.MethodPost, bindPath, strings.NewReader(tc.body))

			// then
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.JSONEq(t, `{"error":"a board content ID must be a version 4 UUID","code":"invalid_id"}`, rec.Body.String())
		})
	}
}
