package controllers

import (
	"fmt"
	"mime"
	"net/http"
	"slices"
	"time"

	"retrotide/internal/board/management"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/export"
	"retrotide/internal/export/engine"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	formatQuery          = "format"
	boardIDQuery         = "board_id"
	attachmentType       = "attachment"
	filenameParam        = "filename"
	contentType          = "Content-Type"
	contentDisposition   = "Content-Disposition"
	exportWriteAllowance = 15 * time.Minute
)

var (
	exportErrorAnswers = []errorAnswer{
		{err: export.ErrUnknownFormat, status: http.StatusBadRequest, code: "invalid_format"},
		{err: export.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: export.ErrOrganisationNotAllowed, status: http.StatusForbidden, code: "not_allowed"},
		{err: export.ErrAccountNotFound, status: http.StatusNotFound},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, code: "not_allowed"},
	}

	boardExportErrorAnswers = slices.Concat(exportErrorAnswers, boardErrorAnswers)
)

func (s *Service) getAllExportRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/boards/:boardID/export", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.exportBoard},
		{Method: http.MethodGet, Path: "/orgs/:orgID/export", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgExport)}, Handler: s.exportOrganisation},
		{Method: http.MethodGet, Path: "/orgs/:orgID/boards/export", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.exportBoards},
		{Method: http.MethodGet, Path: "/auth/export", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.exportSelf},
	}
}

func (s *Service) exportBoard(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	boardID, ok := utils.ParseIDParam(c, boardIDParam)
	if !ok {
		return
	}

	query := export.BoardQuery{Access: spec.BoardAccessQuery{Principal: principal, BoardID: boardID}, Format: engine.Format(c.Query(formatQuery))}

	download, err := s.ExportService.Board(c.Request.Context(), query)
	if err != nil {
		answerError(c, boardExportErrorAnswers, err, "failed to export the board")
		return
	}

	sendDownload(c, download)
}

func (s *Service) exportOrganisation(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	download, err := s.ExportService.Organisation(c.Request.Context(), viewer)
	if err != nil {
		answerError(c, exportErrorAnswers, err, "failed to export the organisation")
		return
	}

	sendDownload(c, download)
}

func (s *Service) exportBoards(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	boardIDs, ok := boardIDQueries(c)
	if !ok {
		return
	}

	if err := management.CheckBulkBoards(boardIDs); err != nil {
		answerError(c, managementErrorAnswers, err, "failed to export the boards")
		return
	}

	download, err := s.ExportService.Boards(c.Request.Context(), export.BoardsQuery{Viewer: viewer, BoardIDs: boardIDs})
	if err != nil {
		answerError(c, exportErrorAnswers, err, "failed to export the boards")
		return
	}

	sendDownload(c, download)
}

func (s *Service) exportSelf(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	download, err := s.ExportService.Self(c.Request.Context(), userID)
	if err != nil {
		answerError(c, exportErrorAnswers, err, "failed to export your data")
		return
	}

	sendDownload(c, download)
}

func sendDownload(c *gin.Context, download *export.Download) {
	name := export.FileName(download.Title, time.Now(), download.Engine.Extension())

	disposition := mime.FormatMediaType(attachmentType, map[string]string{filenameParam: name})
	if disposition == "" {
		utils.InternalError(c, "failed to name the download", fmt.Errorf("%w: %q", errUnnamedDownload, name))
		return
	}

	c.Header(contentType, download.Engine.ContentType())
	c.Header(contentDisposition, disposition)

	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(exportWriteAllowance)); err != nil {
		withdrawDownload(c.Writer.Header())
		utils.InternalError(c, "failed to allow the download time to stream", err)
		return
	}

	c.Status(http.StatusOK)

	err := download.Write(c.Writer)
	if err == nil {
		return
	}

	if !c.Writer.Written() {
		withdrawDownload(c.Writer.Header())
		utils.InternalError(c, "failed to write the download", err)
		return
	}

	c.Error(fmt.Errorf("stream the download %q: %w", name, err))
	panic(http.ErrAbortHandler)
}

func boardIDQueries(c *gin.Context) ([]uuid.UUID, bool) {
	raw := c.QueryArray(boardIDQuery)

	boardIDs := make([]uuid.UUID, 0, len(raw))
	for _, value := range raw {
		boardID, err := uuid.Parse(value)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + boardIDQuery})
			return nil, false
		}

		boardIDs = append(boardIDs, boardID)
	}

	return boardIDs, true
}

func withdrawDownload(header http.Header) {
	header.Del(contentType)
	header.Del(contentDisposition)
}
