package controllers

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/board/management"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/export"
	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	exportHarness struct {
		*testutil.Harness
		exports *export.MockService
	}
)

const (
	serverWriteTimeout = 60 * time.Second
	bubbleDate         = "2000-01-01"
)

var (
	errExportStoreDown = errors.New("the database is unreachable")

	boardExportQuery = spec.BoardAccessQuery{Principal: boardPrincipal, BoardID: boardID}
	exportViewer     = spec.OrganisationMemberKey{OrgID: boardOrg, UserID: boardUser}
)

func newExportHarness(t *testing.T) *exportHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	eh := &exportHarness{Harness: h, exports: export.NewMockService(t)}

	s := &Service{ExportService: eh.exports, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllExportRoutes())

	return eh
}

func downloadOf(title string, format engine.Engine, body string) *export.Download {
	write := func(w io.Writer) error {
		_, err := io.WriteString(w, body)

		return err
	}

	return &export.Download{Title: title, Engine: format, Write: write}
}

func attachmentName(t *testing.T, header http.Header) string {
	t.Helper()

	disposition, params, err := mime.ParseMediaType(header.Get("Content-Disposition"))
	require.NoError(t, err)
	assert.Equal(t, "attachment", disposition)

	return params["filename"]
}

func TestExportController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{ExportService: export.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllExportRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/boards/:boardID/export",
		"HEAD /api/v1/boards/:boardID/export",
		"GET /api/v1/orgs/:orgID/export",
		"HEAD /api/v1/orgs/:orgID/export",
		"GET /api/v1/orgs/:orgID/boards/export",
		"HEAD /api/v1/orgs/:orgID/boards/export",
		"GET /api/v1/auth/export",
		"HEAD /api/v1/auth/export",
	}, registeredRoutes(h.Engine))
}

func TestExportController_TheBoardExportRunsThePrincipalFailureSuite(t *testing.T) {
	testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newExportHarness(t).Harness
	}, testutil.AuthRoute{Method: http.MethodGet, Path: boardPath("/export?format=csv")})
}

func TestExportController_TheOrganisationExportRunsTheOrgPermissionFailureSuite(t *testing.T) {
	testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newExportHarness(t).Harness
	}, orgRoute(http.MethodGet, boardOrg, "/export", orgauthz.PermOrgExport, nil))
}

func TestExportController_TheSelfExportRunsTheAuthFailureSuite(t *testing.T) {
	testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newExportHarness(t).Harness
	}, testutil.AuthRoute{Method: http.MethodGet, Path: "/api/v1/auth/export"})
}

func TestExportController_TheBoardExportStreamsANamedAttachmentInEachFormat(t *testing.T) {
	tests := []struct {
		query       string
		format      engine.Engine
		contentType string
		fileName    string
	}{
		{query: "csv", format: engines.CSV{}, contentType: "text/csv; charset=utf-8", fileName: "RetroTide - Sprint 42 - " + bubbleDate + ".csv"},
		{query: "markdown", format: engines.Markdown{}, contentType: "text/markdown; charset=utf-8", fileName: "RetroTide - Sprint 42 - " + bubbleDate + ".md"},
		{query: "json", format: engines.JSON{}, contentType: "application/json", fileName: "RetroTide - Sprint 42 - " + bubbleDate + ".json"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// given
				eh := newExportHarness(t)
				cookie := eh.SignIn(boardUser)
				query := export.BoardQuery{Access: boardExportQuery, Format: tc.format.Format()}
				eh.exports.EXPECT().Board(mock.Anything, query).Return(downloadOf("Sprint 42", tc.format, "the board"), nil)
				started := time.Now()

				// when
				rec := eh.NewRequest(http.MethodGet, boardPath("/export?format="+tc.query)).WithCookie(cookie).RecordStream()

				// then
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "the board", rec.Body.String())
				assert.Equal(t, tc.contentType, rec.Header().Get("Content-Type"))
				assert.Equal(t, tc.fileName, attachmentName(t, rec.Header()))
				assert.Greater(t, rec.WriteDeadline.Sub(started), serverWriteTimeout, "the stream outlives the server's write timeout")
			})
		})
	}
}

func TestExportController_TheFileNameCarriesATitleBeyondASCII(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		eh := newExportHarness(t)
		cookie := eh.SignIn(boardUser)
		eh.exports.EXPECT().Board(mock.Anything, mock.Anything).Return(downloadOf(`Rétro été: "Q3"`, engines.CSV{}, "cells"), nil)

		// when
		rec := eh.NewRequest(http.MethodGet, boardPath("/export?format=csv")).WithCookie(cookie).RecordStream()

		// then
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "RetroTide - Rétro été- -Q3- - "+bubbleDate+".csv", attachmentName(t, rec.Header()))
	})
}

func TestExportController_TheBoardExportAnswersAMissingOrUnknownFormatAsABadRequest(t *testing.T) {
	tests := []struct {
		path   string
		format string
	}{
		{path: "/export", format: ""},
		{path: "/export?format=", format: ""},
		{path: "/export?format=pdf", format: "pdf"},
		{path: "/export?format=CSV", format: "CSV"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// given no engine is registered under the requested name
			eh := newExportHarness(t)
			cookie := eh.SignIn(boardUser)
			refusal := fmt.Errorf("%w: got %q", export.ErrUnknownFormat, tc.format)
			eh.exports.EXPECT().Board(mock.Anything, export.BoardQuery{Access: boardExportQuery, Format: engine.Format(tc.format)}).Return(nil, refusal)

			// when
			rec := eh.NewRequest(http.MethodGet, boardPath(tc.path)).WithCookie(cookie).RecordStream()

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"unknown export format","code":"invalid_format"}`, rec.Body.String())
			assert.Empty(t, rec.Header().Get("Content-Disposition"))
		})
	}
}

func TestExportController_TheBoardExportMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "no board.export", err: fmt.Errorf("export: %w", policy.ErrNotAllowed), status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that","code":"not_allowed"}`},
		{name: "a board the requester cannot open", err: fmt.Errorf("resolve: %w", live.ErrNoAccess), status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{name: "a removed participant", err: live.ErrRemoved, status: http.StatusForbidden, body: `{"error":"you were removed from this board","code":"removed"}`},
		{name: "database", err: errExportStoreDown, status: http.StatusInternalServerError, body: `{"error":"failed to export the board"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			eh := newExportHarness(t)
			cookie := eh.SignIn(boardUser)
			eh.exports.EXPECT().Board(mock.Anything, export.BoardQuery{Access: boardExportQuery, Format: engine.FormatCSV}).Return(nil, tc.err)

			// when
			rec := eh.NewRequest(http.MethodGet, boardPath("/export?format=csv")).WithCookie(cookie).RecordStream()

			// then the refusal is a normal JSON error, because nothing was written yet
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Header().Get("Content-Disposition"))
		})
	}
}

func TestExportController_AWriterWithoutADeadlineIsAnErrorNotADownload(t *testing.T) {
	// given a response writer that cannot extend its write deadline
	eh := newExportHarness(t)
	cookie := eh.SignIn(boardUser)
	eh.exports.EXPECT().Board(mock.Anything, mock.Anything).Return(downloadOf("Sprint 42", engines.CSV{}, "cells"), nil)

	// when
	rec := eh.NewRequest(http.MethodGet, boardPath("/export?format=csv")).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to allow the download time to stream"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Content-Disposition"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestExportController_AFailureBeforeTheFirstByteIsAJSONError(t *testing.T) {
	// given
	eh := newExportHarness(t)
	cookie := eh.SignIn(boardUser)
	failing := &export.Download{Title: "Sprint 42", Engine: engines.CSV{}, Write: func(io.Writer) error {
		return errExportStoreDown
	}}
	eh.exports.EXPECT().Board(mock.Anything, mock.Anything).Return(failing, nil)

	// when
	rec := eh.NewRequest(http.MethodGet, boardPath("/export?format=csv")).WithCookie(cookie).RecordStream()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to write the download"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Content-Disposition"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestExportController_AFailureAfterTheFirstByteAbortsTheConnection(t *testing.T) {
	// given a stream that fails part way through
	eh := newExportHarness(t)
	cookie := eh.SignIn(boardUser)
	partial := &export.Download{Title: "Sprint 42", Engine: engines.CSV{}, Write: func(w io.Writer) error {
		if _, err := io.WriteString(w, "Went well\n"); err != nil {
			return err
		}

		return errExportStoreDown
	}}
	eh.exports.EXPECT().Board(mock.Anything, mock.Anything).Return(partial, nil)
	request := eh.NewRequest(http.MethodGet, boardPath("/export?format=csv")).WithCookie(cookie)

	// when
	stream := func() {
		request.RecordStream()
	}

	// then the handler aborts, so net/http drops the connection rather than end a truncated file cleanly
	assert.PanicsWithValue(t, http.ErrAbortHandler, stream)
}

func TestExportController_TheOrganisationExportStreamsAJSONAttachment(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		eh := newExportHarness(t)
		route := orgRoute(http.MethodGet, boardOrg, "/export", orgauthz.PermOrgExport, nil)
		cookie := eh.SignInToOrg(route, boardUser)
		eh.exports.EXPECT().Organisation(mock.Anything, exportViewer).Return(downloadOf("Acme", engines.JSON{}, `{"boards":[]}`), nil)

		// when
		rec := eh.NewRequest(route.Method, route.Path).WithCookie(cookie).RecordStream()

		// then
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"boards":[]}`, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "RetroTide - Acme - "+bubbleDate+".json", attachmentName(t, rec.Header()))
	})
}

func TestExportController_TheOrganisationExportMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "not an owner or admin", err: fmt.Errorf("export: %w", export.ErrOrganisationNotAllowed), status: http.StatusForbidden, body: `{"error":"only an owner or an admin can export the organisation","code":"not_allowed"}`},
		{name: "organisation gone", err: export.ErrOrganisationNotFound, status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{name: "database", err: errExportStoreDown, status: http.StatusInternalServerError, body: `{"error":"failed to export the organisation"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			eh := newExportHarness(t)
			route := orgRoute(http.MethodGet, boardOrg, "/export", orgauthz.PermOrgExport, nil)
			cookie := eh.SignInToOrg(route, boardUser)
			eh.exports.EXPECT().Organisation(mock.Anything, exportViewer).Return(nil, tc.err)

			// when
			rec := eh.NewRequest(route.Method, route.Path).WithCookie(cookie).RecordStream()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Header().Get("Content-Disposition"))
		})
	}
}

func TestExportController_TheSelfExportStreamsTheSignedInUsersOwnData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		eh := newExportHarness(t)
		cookie := eh.SignIn(boardUser)
		eh.exports.EXPECT().Self(mock.Anything, boardUser).Return(downloadOf("Sam Lee", engines.JSON{}, `{"account":{}}`), nil)

		// when
		rec := eh.NewRequest(http.MethodGet, "/api/v1/auth/export").WithCookie(cookie).RecordStream()

		// then
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"account":{}}`, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "RetroTide - Sam Lee - "+bubbleDate+".json", attachmentName(t, rec.Header()))
	})
}

func TestExportController_TheSelfExportMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "account gone", err: export.ErrAccountNotFound, status: http.StatusNotFound, body: `{"error":"account not found"}`},
		{name: "database", err: errExportStoreDown, status: http.StatusInternalServerError, body: `{"error":"failed to export your data"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			eh := newExportHarness(t)
			cookie := eh.SignIn(boardUser)
			eh.exports.EXPECT().Self(mock.Anything, boardUser).Return(nil, tc.err)

			// when
			rec := eh.NewRequest(http.MethodGet, "/api/v1/auth/export").WithCookie(cookie).RecordStream()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestExportController_TheBoardsExportRunsTheOrgPermissionFailureSuite(t *testing.T) {
	testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newExportHarness(t).Harness
	}, orgRoute(http.MethodGet, boardOrg, "/boards/export?board_id="+boardID.String(), orgauthz.PermOrgView, nil))
}

func TestExportController_TheBoardsExportStreamsTheChosenBoardsAsOneJSONFile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// given
		eh := newExportHarness(t)
		other := uuid.MustParse("c4d5e6f7-a8b9-4acb-bcd3-a4b5c6d7e8f9")
		route := orgRoute(http.MethodGet, boardOrg, "/boards/export", orgauthz.PermOrgView, nil)
		cookie := eh.SignInToOrg(route, boardUser)
		query := export.BoardsQuery{Viewer: exportViewer, BoardIDs: []uuid.UUID{boardID, other}}
		eh.exports.EXPECT().Boards(mock.Anything, query).Return(downloadOf("Acme - boards", engines.JSON{}, `{"boards":[]}`), nil)

		// when
		rec := eh.NewRequest(http.MethodGet, route.Path+"?board_id="+boardID.String()+"&board_id="+other.String()).WithCookie(cookie).RecordStream()

		// then
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"boards":[]}`, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "RetroTide - Acme - boards - "+bubbleDate+".json", attachmentName(t, rec.Header()))
	})
}

func TestExportController_TheBoardsExportRefusesABadChoiceOfBoards(t *testing.T) {
	tooMany := strings.Repeat("&board_id="+boardID.String(), management.MaxBulkBoards+1)

	tests := []struct {
		name  string
		query string
		body  string
	}{
		{name: "no boards", query: "", body: `{"error":"a bulk action needs at least one board","code":"boards_required"}`},
		{name: "a board ID that is not a UUID", query: "?board_id=sprint-12", body: `{"error":"invalid board_id"}`},
		{name: "more than 100 boards", query: "?" + strings.TrimPrefix(tooMany, "&"), body: `{"error":"a bulk action takes at most 100 boards","code":"too_many_boards"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			eh := newExportHarness(t)
			route := orgRoute(http.MethodGet, boardOrg, "/boards/export", orgauthz.PermOrgView, nil)
			cookie := eh.SignInToOrg(route, boardUser)

			// when
			rec := eh.NewRequest(http.MethodGet, route.Path+tc.query).WithCookie(cookie).RecordStream()

			// then the strict mock proves the service was never asked
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestExportController_TheBoardsExportMapsTheServicesRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "organisation gone", err: export.ErrOrganisationNotFound, status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{name: "database", err: errExportStoreDown, status: http.StatusInternalServerError, body: `{"error":"failed to export the boards"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			eh := newExportHarness(t)
			route := orgRoute(http.MethodGet, boardOrg, "/boards/export", orgauthz.PermOrgView, nil)
			cookie := eh.SignInToOrg(route, boardUser)
			eh.exports.EXPECT().Boards(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("export: %w", tc.err))

			// when
			rec := eh.NewRequest(http.MethodGet, route.Path+"?board_id="+boardID.String()).WithCookie(cookie).RecordStream()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
			assert.Empty(t, rec.Header().Get("Content-Disposition"))
		})
	}
}
