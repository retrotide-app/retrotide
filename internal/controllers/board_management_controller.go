package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"retrotide/internal/board/management"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/orgauthz"

	"github.com/gin-gonic/gin"
)

const (
	bulkFailedCode    = "failed"
	bulkFailedMessage = "failed to update the board"
)

var (
	managementErrorAnswers = slices.Concat([]errorAnswer{
		{err: management.ErrSameTeam, status: http.StatusConflict, code: "same_team"},
		{err: management.ErrMoveNotAllowed, status: http.StatusForbidden, code: "move_not_allowed"},
		{err: management.ErrTeamNotFound, status: http.StatusNotFound},
		{err: management.ErrTeamArchived, status: http.StatusConflict, code: "team_archived"},
		{err: management.ErrCSVEmpty, status: http.StatusBadRequest, code: "csv_empty"},
		{err: management.ErrCSVInvalid, status: http.StatusBadRequest, code: "csv_invalid"},
		{err: management.ErrCSVTooManyCards, status: http.StatusBadRequest, code: "csv_too_many_cards"},
		{err: management.ErrCSVTooManyColumns, status: http.StatusBadRequest, code: "csv_too_many_columns"},
		{err: management.ErrRestoreNotAllowed, status: http.StatusForbidden, code: "restore_not_allowed"},
		{err: management.ErrBoardNotFound, status: http.StatusNotFound},
		{err: management.ErrTooManyBoards, status: http.StatusBadRequest, code: "too_many_boards"},
		{err: management.ErrBoardsRequired, status: http.StatusBadRequest, code: "boards_required"},
		{err: management.ErrInvalidBulkAction, status: http.StatusBadRequest, code: "invalid_bulk_action"},
	}, boardErrorAnswers)
)

func (s *Service) getAllBoardManagementRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/archive", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.archiveBoard},
		{Method: http.MethodPost, Path: "/boards/:boardID/unarchive", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.unarchiveBoard},
		{Method: http.MethodDelete, Path: "/boards/:boardID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.deleteBoard},
		{Method: http.MethodPut, Path: "/boards/:boardID/team", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.moveBoard},
		{Method: http.MethodPost, Path: "/boards/:boardID/import", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.importBoardCards},
		{Method: http.MethodPut, Path: "/boards/:boardID/pin", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.boardPinner(true)},
		{Method: http.MethodDelete, Path: "/boards/:boardID/pin", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.boardPinner(false)},
		{Method: http.MethodGet, Path: "/orgs/:orgID/boards/deleted", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.listDeletedBoards},
		{Method: http.MethodPost, Path: "/orgs/:orgID/boards/:boardID/restore", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.restoreBoard},
		{Method: http.MethodPost, Path: "/orgs/:orgID/boards/bulk", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.bulkBoards},
	}
}

func (s *Service) archiveBoard(c *gin.Context) {
	command, ok := boardOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardManagement.Archive(c.Request.Context(), command)
	answerManaged(c, result, err, "failed to archive the board")
}

func (s *Service) unarchiveBoard(c *gin.Context) {
	command, ok := boardOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardManagement.Unarchive(c.Request.Context(), command)
	answerManaged(c, result, err, "failed to unarchive the board")
}

func (s *Service) deleteBoard(c *gin.Context) {
	command, ok := boardOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardManagement.Delete(c.Request.Context(), command)
	answerManaged(c, result, err, "failed to delete the board")
}

func (s *Service) moveBoard(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.MoveBoardRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardManagement.Move(c.Request.Context(), management.MoveCommand{Command: command, TeamID: req.TeamID})
	answerManaged(c, result, err, "failed to move the board")
}

func (s *Service) importBoardCards(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CSVImportRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardManagement.ImportCSV(c.Request.Context(), management.ImportCommand{Command: command, CSV: req.CSV})
	if err != nil {
		answerError(c, managementErrorAnswers, err, "failed to import the cards")
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) boardPinner(pinned bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		command, ok := boardCommand(c)
		if !ok {
			return
		}

		pin := management.PinCommand{Principal: command.Principal, BoardID: command.BoardID, Pinned: pinned}

		if err := s.BoardManagement.Pin(c.Request.Context(), pin); err != nil {
			answerError(c, managementErrorAnswers, err, "failed to pin the board")
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func (s *Service) listDeletedBoards(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	listing, err := s.BoardManagement.ListDeleted(c.Request.Context(), viewer)
	if err != nil {
		answerError(c, managementErrorAnswers, err, "failed to list the deleted boards")
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) restoreBoard(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	boardID, ok := utils.ParseIDParam(c, boardIDParam)
	if !ok {
		return
	}

	err := s.BoardManagement.Restore(c.Request.Context(), management.RestoreCommand{Principal: principal, OrgID: orgID, BoardID: boardID})
	if errors.Is(err, management.ErrNotAudited) {
		c.Error(err)
		noContent(c)
		return
	}

	if err != nil {
		answerError(c, managementErrorAnswers, err, "failed to restore the board")
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) bulkBoards(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BulkBoardRequest](c)
	if !ok {
		return
	}

	command := management.BulkCommand{Principal: principal, OrgID: orgID, Action: req.Action, BoardIDs: req.BoardIDs}

	outcomes, err := s.BoardManagement.Bulk(c.Request.Context(), command)
	if err != nil {
		answerError(c, managementErrorAnswers, err, "failed to update the boards")
		return
	}

	results := make([]dto.BulkBoardResult, 0, len(outcomes))
	for _, outcome := range outcomes {
		results = append(results, bulkResult(c, outcome))
	}

	c.JSON(http.StatusOK, dto.BulkBoardResponse{Results: results})
}

func bulkResult(c *gin.Context, outcome management.BulkOutcome) dto.BulkBoardResult {
	result := dto.BulkBoardResult{BoardID: outcome.BoardID, OK: true}

	if outcome.Err == nil {
		return result
	}

	if errors.Is(outcome.Err, management.ErrNotAudited) {
		c.Error(outcome.Err)
		return result
	}

	result.OK = false

	for _, answer := range managementErrorAnswers {
		if !errors.Is(outcome.Err, answer.err) {
			continue
		}

		if answer.recordCause {
			c.Error(outcome.Err)
		}

		result.Code = answer.code
		result.Error = answer.err.Error()

		return result
	}

	c.Error(fmt.Errorf("bulk action on board %s: %w", outcome.BoardID, outcome.Err))
	result.Code = bulkFailedCode
	result.Error = bulkFailedMessage

	return result
}

func answerManaged(c *gin.Context, result *dto.BoardCommandResponse, err error, msg string) {
	if errors.Is(err, management.ErrNotAudited) && result != nil {
		c.Error(err)
		c.JSON(http.StatusOK, result)
		return
	}

	if err != nil {
		answerError(c, managementErrorAnswers, err, msg)
		return
	}

	c.JSON(http.StatusOK, result)
}
