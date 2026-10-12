package controllers

import (
	"net/http"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllBoardSpotlightRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/presentation", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.presentCard},
		{Method: http.MethodPost, Path: "/boards/:boardID/highlight", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.highlightCard},
		{Method: http.MethodPost, Path: "/boards/:boardID/focus", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.focusColumn},
	}
}

func (s *Service) presentCard(c *gin.Context) {
	command, ok := cardSpotlightCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.Present(c.Request.Context(), command)
	if boardFailed(c, err, "failed to change the presentation") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) highlightCard(c *gin.Context) {
	command, ok := cardSpotlightCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.Highlight(c.Request.Context(), command)
	if boardFailed(c, err, "failed to change the highlight") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) focusColumn(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ColumnFocusRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	focus := board.ColumnFocusCommand{Command: command, ColumnID: req.ColumnID}

	result, err := s.BoardService.Focus(c.Request.Context(), focus)
	if boardFailed(c, err, "failed to change the focused column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func cardSpotlightCommand(c *gin.Context) (board.CardSpotlightCommand, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return board.CardSpotlightCommand{}, false
	}

	req, ok := utils.BindJSON[dto.CardSpotlightRequest](c)
	if !ok {
		return board.CardSpotlightCommand{}, false
	}

	command.OpID = req.OpID

	return board.CardSpotlightCommand{Command: command, CardID: req.CardID}, true
}
