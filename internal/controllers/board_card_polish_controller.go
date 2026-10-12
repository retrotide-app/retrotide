package controllers

import (
	"net/http"
	"slices"

	"retrotide/internal/board/cardpolish"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/gin-gonic/gin"
)

const (
	reactionParam = "reaction"
)

var (
	cardPolishErrorAnswers = slices.Concat([]errorAnswer{
		{err: cardpolish.ErrUnknownReaction, status: http.StatusBadRequest, code: "unknown_reaction"},
		{err: policy.ErrCardNotPublic, status: http.StatusConflict, code: "card_not_public"},
	}, boardErrorAnswers)
)

func (s *Service) getAllBoardCardPolishRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPut, Path: "/boards/:boardID/cards/:cardID/reactions/:reaction", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.addReaction},
		{Method: http.MethodDelete, Path: "/boards/:boardID/cards/:cardID/reactions/:reaction", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.removeReaction},
		{Method: http.MethodPut, Path: "/boards/:boardID/cards/:cardID/pin", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.pinCard},
	}
}

func (s *Service) addReaction(c *gin.Context) {
	command, ok := reactCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardCardPolish.React(c.Request.Context(), command)
	if err != nil {
		answerError(c, cardPolishErrorAnswers, err, "failed to add the reaction")
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) removeReaction(c *gin.Context) {
	command, ok := reactCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardCardPolish.Unreact(c.Request.Context(), command)
	if err != nil {
		answerError(c, cardPolishErrorAnswers, err, "failed to remove the reaction")
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) pinCard(c *gin.Context) {
	target, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CardPinRequest](c)
	if !ok {
		return
	}

	target.OpID = req.OpID

	pin := cardpolish.PinCommand{Command: target.Command, CardID: target.CardID, Pinned: req.Pinned}

	result, err := s.BoardCardPolish.Pin(c.Request.Context(), pin)
	if err != nil {
		answerError(c, cardPolishErrorAnswers, err, "failed to pin the card")
		return
	}

	c.JSON(http.StatusOK, result)
}

func reactCommand(c *gin.Context) (cardpolish.ReactCommand, bool) {
	target, ok := cardCommand(c)
	if !ok {
		return cardpolish.ReactCommand{}, false
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return cardpolish.ReactCommand{}, false
	}

	target.OpID = req.OpID

	return cardpolish.ReactCommand{Command: target.Command, CardID: target.CardID, Reaction: model.Reaction(c.Param(reactionParam))}, true
}
