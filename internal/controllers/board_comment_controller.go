package controllers

import (
	"net/http"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

const (
	commentIDParam = "commentID"
)

func (s *Service) getAllBoardCommentRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/cards/:cardID/comments", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.addComment},
		{Method: http.MethodPut, Path: "/boards/:boardID/comments/:commentID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.editComment},
		{Method: http.MethodDelete, Path: "/boards/:boardID/comments/:commentID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.deleteComment},
	}
}

func (s *Service) addComment(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateCommentRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	add := board.AddCommentCommand{Command: command.Command, CardID: command.CardID, CommentID: req.ID, Body: req.Body, Mentions: req.Mentions}

	result, err := s.BoardService.AddComment(c.Request.Context(), add)
	if boardFailed(c, err, "failed to add the comment") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) editComment(c *gin.Context) {
	command, ok := commentCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.EditCommentRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	edit := board.EditCommentCommand{Command: command.Command, CommentID: command.CommentID, Body: req.Body, Mentions: req.Mentions}

	result, err := s.BoardService.EditComment(c.Request.Context(), edit)
	if boardFailed(c, err, "failed to edit the comment") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) deleteComment(c *gin.Context) {
	command, ok := commentCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.DeleteComment(c.Request.Context(), command)
	if boardFailed(c, err, "failed to delete the comment") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func commentCommand(c *gin.Context) (board.CommentCommand, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return board.CommentCommand{}, false
	}

	commentID, ok := utils.ParseIDParam(c, commentIDParam)
	if !ok {
		return board.CommentCommand{}, false
	}

	return board.CommentCommand{Command: command, CommentID: commentID}, true
}
