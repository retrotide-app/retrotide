package controllers

import (
	"net/http"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllBoardColumnRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/columns", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.createColumn},
		{Method: http.MethodPut, Path: "/boards/:boardID/columns/:columnID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.editColumn},
		{Method: http.MethodPost, Path: "/boards/:boardID/columns/:columnID/move", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.moveColumn},
		{Method: http.MethodPost, Path: "/boards/:boardID/columns/:columnID/clear", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.clearColumn},
		{Method: http.MethodDelete, Path: "/boards/:boardID/columns/:columnID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.deleteColumn},
	}
}

func (s *Service) createColumn(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateColumnRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	add := board.AddColumnCommand{
		Command: command,
		Title:   req.Title, Description: req.Description, Colour: req.Colour,
		ColumnID: req.ID,
	}

	result, err := s.BoardService.AddColumn(c.Request.Context(), add)
	if boardFailed(c, err, "failed to add the column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) editColumn(c *gin.Context) {
	command, ok := columnCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ColumnFieldsRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	edit := board.EditColumnCommand{
		Command: command.Command,
		Title:   req.Title, Description: req.Description, Colour: req.Colour,
		ColumnID: command.ColumnID,
	}

	result, err := s.BoardService.EditColumn(c.Request.Context(), edit)
	if boardFailed(c, err, "failed to edit the column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) moveColumn(c *gin.Context) {
	command, ok := columnCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.MoveColumnRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	move := board.MoveColumnCommand{Command: command.Command, ColumnID: command.ColumnID, AfterColumnID: req.AfterColumnID}

	result, err := s.BoardService.MoveColumn(c.Request.Context(), move)
	if boardFailed(c, err, "failed to move the column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) clearColumn(c *gin.Context) {
	command, ok := columnCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.ClearColumn(c.Request.Context(), command)
	if boardFailed(c, err, "failed to clear the column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) deleteColumn(c *gin.Context) {
	command, ok := columnCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.DeleteColumn(c.Request.Context(), command)
	if boardFailed(c, err, "failed to delete the column") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func columnCommand(c *gin.Context) (board.ColumnCommand, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return board.ColumnCommand{}, false
	}

	columnID, ok := utils.ParseIDParam(c, columnIDParam)
	if !ok {
		return board.ColumnCommand{}, false
	}

	return board.ColumnCommand{Command: command, ColumnID: columnID}, true
}
