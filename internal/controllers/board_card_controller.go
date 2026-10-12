package controllers

import (
	"net/http"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllBoardCardRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/cards", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.createCard},
		{Method: http.MethodPut, Path: "/boards/:boardID/cards/:cardID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.editCard},
		{Method: http.MethodPut, Path: "/boards/:boardID/cards/:cardID/colour", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.recolourCard},
		{Method: http.MethodPost, Path: "/boards/:boardID/cards/:cardID/move", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.moveCard},
		{Method: http.MethodDelete, Path: "/boards/:boardID/cards/:cardID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.deleteCard},
	}
}

func (s *Service) createCard(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateCardRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	add := board.AddCardCommand{Command: command, CardID: req.ID, ColumnID: req.ColumnID, Body: req.Body, Colour: req.Colour, Mentions: req.Mentions}

	result, err := s.BoardService.AddCard(c.Request.Context(), add)
	if boardFailed(c, err, "failed to add the card") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) editCard(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.EditCardRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	edit := board.EditCardCommand{Command: command.Command, CardID: command.CardID, Body: req.Body, Mentions: req.Mentions}

	result, err := s.BoardService.EditCard(c.Request.Context(), edit)
	if boardFailed(c, err, "failed to edit the card") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) recolourCard(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.RecolourCardRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	recolour := board.RecolourCardCommand{Command: command.Command, CardID: command.CardID, Colour: req.Colour}

	result, err := s.BoardService.RecolourCard(c.Request.Context(), recolour)
	if boardFailed(c, err, "failed to recolour the card") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) moveCard(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.MoveCardRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	move := board.MoveCardCommand{Command: command.Command, CardID: command.CardID, ColumnID: req.ColumnID, AfterCardID: req.AfterCardID}

	result, err := s.BoardService.MoveCard(c.Request.Context(), move)
	if boardFailed(c, err, "failed to move the card") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) deleteCard(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.DeleteCard(c.Request.Context(), command)
	if boardFailed(c, err, "failed to delete the card") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func cardCommand(c *gin.Context) (board.CardCommand, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return board.CardCommand{}, false
	}

	cardID, ok := utils.ParseIDParam(c, cardIDParam)
	if !ok {
		return board.CardCommand{}, false
	}

	return board.CardCommand{Command: command, CardID: cardID}, true
}
