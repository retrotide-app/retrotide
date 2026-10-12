package controllers

import (
	"net/http"
	"slices"

	"retrotide/internal/board/images"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

var (
	boardImageErrorAnswers = slices.Concat([]errorAnswer{
		{err: images.ErrMediaNotFound, status: http.StatusNotFound},
		{err: images.ErrWrongKind, status: http.StatusConflict, code: "wrong_media_kind"},
	}, boardErrorAnswers)
)

func (s *Service) getAllBoardImageRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPut, Path: "/boards/:boardID/cards/:cardID/image", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.setCardImage},
		{Method: http.MethodPut, Path: "/boards/:boardID/background", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.setBoardBackground},
	}
}

func (s *Service) setCardImage(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	cardID, ok := utils.ParseIDParam(c, cardIDParam)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CardImageRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	illustrate := images.CardImageCommand{Command: command, CardID: cardID, MediaID: req.ImageID}

	result, err := s.BoardImages.SetCardImage(c.Request.Context(), illustrate)
	if err != nil {
		answerError(c, boardImageErrorAnswers, err, "failed to change the card's image")
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) setBoardBackground(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardBackgroundRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	decorate := images.BackgroundCommand{Command: command, MediaID: req.ImageID}

	result, err := s.BoardImages.SetBackground(c.Request.Context(), decorate)
	if err != nil {
		answerError(c, boardImageErrorAnswers, err, "failed to change the board's background")
		return
	}

	c.JSON(http.StatusOK, result)
}
