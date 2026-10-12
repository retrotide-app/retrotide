package controllers

import (
	"net/http"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllBoardColumnFacilitationRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/columns/:columnID/reveal", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.revealColumn},
		{Method: http.MethodPost, Path: "/boards/:boardID/columns/:columnID/hide", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.hideColumn},
		{Method: http.MethodPost, Path: "/boards/:boardID/columns/:columnID/show", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.showColumn},
		{Method: http.MethodPut, Path: "/boards/:boardID/columns/:columnID/vote-limit", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.setColumnVoteLimit},
		{Method: http.MethodPost, Path: "/boards/:boardID/columns/:columnID/votes/reset", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.resetColumnVotes},
	}
}

func (s *Service) revealColumn(c *gin.Context) {
	command, ok := columnOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.RevealColumn(c.Request.Context(), command)
	if boardFailed(c, err, "failed to reveal the column's cards") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) hideColumn(c *gin.Context) {
	command, ok := columnOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.HideColumn(c.Request.Context(), command)
	if boardFailed(c, err, "failed to hide the column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) showColumn(c *gin.Context) {
	command, ok := columnOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.ShowColumn(c.Request.Context(), command)
	if boardFailed(c, err, "failed to show the column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) setColumnVoteLimit(c *gin.Context) {
	command, ok := columnCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ColumnVoteLimitRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	limit := board.ColumnVoteLimitCommand{Command: command.Command, ColumnID: command.ColumnID, MaxVotes: req.MaxVotes}

	result, err := s.BoardService.SetColumnVoteLimit(c.Request.Context(), limit)
	if boardFailed(c, err, "failed to set the column's vote limit") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) resetColumnVotes(c *gin.Context) {
	command, ok := columnOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.ResetColumnVotes(c.Request.Context(), command)
	if boardFailed(c, err, "failed to reset the column's votes") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func columnOpCommand(c *gin.Context) (board.ColumnCommand, bool) {
	command, ok := columnCommand(c)
	if !ok {
		return board.ColumnCommand{}, false
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return board.ColumnCommand{}, false
	}

	command.OpID = req.OpID

	return command, true
}
