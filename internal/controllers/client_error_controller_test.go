package controllers

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/middleware"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	clientErrorHarness struct {
		*testutil.Harness
		log *bytes.Buffer
	}
)

const (
	clientErrorsPath = "/api/v1/client-errors"
	clientErrorPeer  = "198.51.100.23:4444"
)

func newClientErrorHarness(t *testing.T) *clientErrorHarness {
	t.Helper()

	h := testutil.NewHarness(t)

	ctrl := &Service{Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), ctrl.getAllClientErrorRoutes())

	previousLogger := logger.Log
	previousLevel := zerolog.GlobalLevel()
	t.Cleanup(func() {
		logger.Log = previousLogger
		zerolog.SetGlobalLevel(previousLevel)
	})

	buf := &bytes.Buffer{}
	logger.Log = zerolog.New(buf)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	return &clientErrorHarness{Harness: h, log: buf}
}

func (h *clientErrorHarness) lines(t *testing.T) []map[string]any {
	t.Helper()

	var lines []map[string]any
	for line := range bytes.Lines(h.log.Bytes()) {
		entry := testutil.UnmarshalJSON[map[string]any](t, line)
		if entry["message"] == clientErrorLogMessage {
			lines = append(lines, entry)
		}
	}

	return lines
}

func boardCrash() dto.ClientErrorReportRequest {
	return dto.ClientErrorReportRequest{
		Name:           "TypeError",
		Message:        "cards is undefined",
		Stack:          "TypeError: cards is undefined\n    at Board (index-a1b2c3.js:1:2)",
		ComponentStack: "\n    at RootErrorBoundary",
		Source:         "caught",
		Route:          "/b/:slug",
		Version:        "7.3.2",
		Sequence:       3,
	}
}

func TestClientErrorController_LogsAReportOnceAndAnswersNoContent(t *testing.T) {
	// given no session, because a signed-out visitor's crash is reported too
	h := newClientErrorHarness(t)

	// when
	rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(boardCrash()).WithRemoteAddr(clientErrorPeer).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())

	lines := h.lines(t)
	require.Len(t, lines, 1)
	assert.Equal(t, map[string]any{
		"level":                        "error",
		"message":                      "client error",
		"client_error_name":            "TypeError",
		"client_error_message":         "cards is undefined",
		"client_error_stack":           "TypeError: cards is undefined\n    at Board (index-a1b2c3.js:1:2)",
		"client_error_component_stack": "\n    at RootErrorBoundary",
		"client_error_source":          "caught",
		"client_error_route":           "/b/:slug",
		"client_error_version":         "7.3.2",
		"client_error_sequence":        float64(3),
	}, lines[0])
}

func TestClientErrorController_AcceptsAReportWithoutAStack(t *testing.T) {
	// given the browser sends null for a stack it does not have
	h := newClientErrorHarness(t)
	body := `{"name":"NonError","message":"a bare string was thrown","stack":null,"component_stack":null,"source":"unhandled-rejection","route":"/","version":"unknown","sequence":1}`

	// when
	rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithRawBody(body, "application/json").WithRemoteAddr(clientErrorPeer).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)

	lines := h.lines(t)
	require.Len(t, lines, 1)
	assert.Equal(t, "", lines[0]["client_error_stack"])
	assert.Equal(t, "", lines[0]["client_error_component_stack"])
}

func TestClientErrorController_RefusesABodyOverFourKilobytes(t *testing.T) {
	// given
	h := newClientErrorHarness(t)
	report := boardCrash()
	report.Message = strings.Repeat("a", maxClientErrorBodyBytes)

	// when
	rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(report).WithRemoteAddr(clientErrorPeer).Record()

	// then nothing of the oversized report reaches the log
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.JSONEq(t, `{"error":"request body too large"}`, rec.Body.String())
	assert.Empty(t, h.lines(t))
}

func TestClientErrorController_RefusesABodyThatIsNotAReport(t *testing.T) {
	// given
	h := newClientErrorHarness(t)

	// when
	rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithRawBody(`{"message":`, "application/json").WithRemoteAddr(clientErrorPeer).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid request body"}`, rec.Body.String())
	assert.Empty(t, h.lines(t))
}

func TestClientErrorController_TruncatesLongFields(t *testing.T) {
	tests := []struct {
		name  string
		field string
		limit int
		set   func(report *dto.ClientErrorReportRequest, value string)
	}{
		{name: "the message", field: "client_error_message", limit: maxClientErrorMessageLength, set: func(report *dto.ClientErrorReportRequest, value string) { report.Message = value }},
		{name: "the stack", field: "client_error_stack", limit: maxClientErrorStackLength, set: func(report *dto.ClientErrorReportRequest, value string) { report.Stack = value }},
		{name: "the component stack", field: "client_error_component_stack", limit: maxClientErrorComponentStackLength, set: func(report *dto.ClientErrorReportRequest, value string) { report.ComponentStack = value }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newClientErrorHarness(t)
			report := boardCrash()
			tc.set(&report, strings.Repeat("x", tc.limit+100))

			// when
			rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(report).WithRemoteAddr(clientErrorPeer).Record()

			// then
			assert.Equal(t, http.StatusNoContent, rec.Code)

			lines := h.lines(t)
			require.Len(t, lines, 1)
			assert.Equal(t, strings.Repeat("x", tc.limit), lines[0][tc.field])
		})
	}
}

func TestClientErrorController_TruncatesOnACharacterBoundary(t *testing.T) {
	// given a message of three-byte characters whose cap falls inside one
	h := newClientErrorHarness(t)
	report := boardCrash()
	report.Message = strings.Repeat("€", maxClientErrorMessageLength/3+50)

	// when
	rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(report).WithRemoteAddr(clientErrorPeer).Record()

	// then the split character is dropped rather than logged as broken bytes
	assert.Equal(t, http.StatusNoContent, rec.Code)

	lines := h.lines(t)
	require.Len(t, lines, 1)
	message, ok := lines[0]["client_error_message"].(string)
	require.True(t, ok)
	assert.True(t, utf8.ValidString(message))
	assert.Equal(t, strings.Repeat("€", maxClientErrorMessageLength/3), message)
}

func TestClientErrorController_NeverLogsAConcretePathOrQuery(t *testing.T) {
	tests := []struct {
		name  string
		route string
		leak  string
	}{
		{name: "a path without a leading slash", route: "b/sprint-review", leak: "sprint-review"},
		{name: "a full URL", route: "https://retrotide.test/b/sprint-review", leak: "sprint-review"},
		{name: "a query", route: "/b/:slug?code=s3cret", leak: "s3cret"},
		{name: "a fragment", route: "/b/:slug#token=s3cret", leak: "s3cret"},
		{name: "a route past its cap", route: "/" + strings.Repeat("s3cret", maxClientErrorRouteLength), leak: "s3cret"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newClientErrorHarness(t)
			report := boardCrash()
			report.Route = tc.route

			// when
			rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(report).WithRemoteAddr(clientErrorPeer).Record()

			// then the report still arrives, with the route blanked
			assert.Equal(t, http.StatusNoContent, rec.Code)

			lines := h.lines(t)
			require.Len(t, lines, 1)
			assert.Equal(t, "unknown", lines[0]["client_error_route"])
			assert.NotContains(t, h.log.String(), tc.leak)
		})
	}
}

func TestClientErrorController_IsRateLimitedPerIP(t *testing.T) {
	// given
	h := newClientErrorHarness(t)

	for i := range middleware.ClientErrorReportsPerMinute {
		rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(boardCrash()).WithRemoteAddr(clientErrorPeer).Record()
		require.Equal(t, http.StatusNoContent, rec.Code, "report %d", i+1)
		assert.Equal(t, strconv.Itoa(middleware.ClientErrorReportsPerMinute), rec.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, strconv.Itoa(middleware.ClientErrorReportsPerMinute-1-i), rec.Header().Get("X-RateLimit-Remaining"))
	}

	// when
	limited := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(boardCrash()).WithRemoteAddr(clientErrorPeer).Record()
	elsewhere := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(boardCrash()).WithRemoteAddr(clientErrorPeer).WithRemoteAddr("203.0.113.9:4444").Record()

	// then the refused report is never logged, and another address keeps its own budget
	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.NotEmpty(t, limited.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusNoContent, elsewhere.Code)
	assert.Len(t, h.lines(t), middleware.ClientErrorReportsPerMinute+1)
}

func TestClientErrorController_ChargesTheRateLimitForAnOversizedBody(t *testing.T) {
	// given
	h := newClientErrorHarness(t)
	report := boardCrash()
	report.Message = strings.Repeat("a", maxClientErrorBodyBytes)

	// when
	rec := h.NewRequest(http.MethodPost, clientErrorsPath).WithJSONBody(report).WithRemoteAddr(clientErrorPeer).Record()

	// then a flood of oversized bodies spends the same budget
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, strconv.Itoa(middleware.ClientErrorReportsPerMinute-1), rec.Header().Get("X-RateLimit-Remaining"))
}
