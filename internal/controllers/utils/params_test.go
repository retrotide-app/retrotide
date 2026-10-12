package utils

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	boardIDParam  = "boardID"
	boardPath     = "/boards/:" + boardIDParam
	limitQuery    = "limit"
	limitFallback = 20
	queryPath     = "/query"
)

func idParamEngine() *gin.Engine {
	engine := gin.New()
	engine.GET(boardPath, func(c *gin.Context) {
		id, ok := ParseIDParam(c, boardIDParam)
		if !ok {
			return
		}

		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	return engine
}

func queryIntEngine() *gin.Engine {
	engine := gin.New()
	engine.GET(queryPath, func(c *gin.Context) {
		limit, ok := QueryInt(c, limitQuery, limitFallback)
		if !ok {
			return
		}

		c.JSON(http.StatusOK, gin.H{"limit": limit})
	})

	return engine
}

func TestParseIDParam_ReturnsTheIDFromThePath(t *testing.T) {
	// given
	engine := idParamEngine()
	id := uuid.New()

	// when
	rec := perform(t, engine, http.MethodGet, "/boards/"+id.String(), nil)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"id":"`+id.String()+`"}`, rec.Body.String())
}

func TestParseIDParam_AnswersBadRequestForAMalformedID(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"a word", "not-a-uuid"},
		{"a number", "42"},
		{"a truncated UUID", "0f8fad5b-d9cb-469f-a165"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := idParamEngine()

			// when
			rec := perform(t, engine, http.MethodGet, "/boards/"+tc.raw, nil)

			// then
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.JSONEq(t, `{"error":"invalid boardID"}`, rec.Body.String())
		})
	}
}

func TestQueryInt_ReturnsTheFallbackWhenTheParameterIsAbsent(t *testing.T) {
	// given
	engine := queryIntEngine()

	// when
	rec := perform(t, engine, http.MethodGet, queryPath, nil)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"limit":20}`, rec.Body.String())
}

func TestQueryInt_ReturnsTheParsedValue(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantBody string
	}{
		{"a positive number", "5", `{"limit":5}`},
		{"zero", "0", `{"limit":0}`},
		{"a negative number", "-3", `{"limit":-3}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := queryIntEngine()

			// when
			rec := perform(t, engine, http.MethodGet, queryPath+"?"+limitQuery+"="+tc.raw, nil)

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			require.JSONEq(t, tc.wantBody, rec.Body.String())
		})
	}
}

func TestQueryInt_AnswersBadRequestForAValueThatIsNotAnInteger(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"a word", "abc"},
		{"an empty value", ""},
		{"a fraction", "2.5"},
		{"a number that overflows an int", "99999999999999999999999"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			engine := queryIntEngine()

			// when
			rec := perform(t, engine, http.MethodGet, queryPath+"?"+limitQuery+"="+tc.raw, nil)

			// then
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.JSONEq(t, `{"error":"invalid limit"}`, rec.Body.String())
		})
	}
}
