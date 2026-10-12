package controllers

import (
	"net/http"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllBoardVoteRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/cards/:cardID/merge", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.mergeCard},
		{Method: http.MethodPost, Path: "/boards/:boardID/cards/:cardID/unmerge", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.unmergeCard},
		{Method: http.MethodPost, Path: "/boards/:boardID/cards/:cardID/votes", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.addVote},
		{Method: http.MethodDelete, Path: "/boards/:boardID/cards/:cardID/votes", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.removeVote},
		{Method: http.MethodPost, Path: "/boards/:boardID/votes/reset", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.resetVotes},
		{Method: http.MethodPut, Path: "/boards/:boardID/settings", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.changeSettings},
	}
}

func (s *Service) mergeCard(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.MergeCardRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	merge := board.MergeCardCommand{Command: command.Command, CardID: command.CardID, TargetCardID: req.TargetCardID}

	result, err := s.BoardService.MergeCard(c.Request.Context(), merge)
	if boardFailed(c, err, "failed to merge the card") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) unmergeCard(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.UnmergeCard(c.Request.Context(), command)
	if boardFailed(c, err, "failed to unmerge the card") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) addVote(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.AddVote(c.Request.Context(), command)
	if boardFailed(c, err, "failed to add the vote") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) removeVote(c *gin.Context) {
	command, ok := cardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.RemoveVote(c.Request.Context(), command)
	if boardFailed(c, err, "failed to take back the vote") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) resetVotes(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.ResetVotes(c.Request.Context(), command)
	if boardFailed(c, err, "failed to reset the votes") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) changeSettings(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardSettingsRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	settings := spec.BoardSettingsPatch{
		VotingOpen:       req.VotingOpen,
		VoteCountsHidden: req.VoteCountsHidden,
		MaxVotes:         req.MaxVotes,
		MaxVotesPerCard:  req.MaxVotesPerCard,
		CommentsEnabled:  req.CommentsEnabled,
		MergingEnabled:   req.MergingEnabled,
		SortMode:         req.SortMode,
		CardsLocked:      req.CardsLocked,
		MovesLocked:      req.MovesLocked,
		BoardLocked:      req.BoardLocked,
		TimerPlaySound:   req.TimerPlaySound,
		TimerAutoLock:    req.TimerAutoLock,
		SortEnforced:     req.SortEnforced,
		AnyoneCanEdit:    req.AnyoneCanEdit,
		ShowCardDates:    req.ShowCardDates,
	}

	result, err := s.BoardService.ChangeSettings(c.Request.Context(), board.SettingsCommand{Command: command, Settings: settings})
	if boardFailed(c, err, "failed to change the board's settings") {
		return
	}

	c.JSON(http.StatusOK, result)
}
