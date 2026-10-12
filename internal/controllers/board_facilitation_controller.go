package controllers

import (
	"net/http"

	"retrotide/internal/board"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

const (
	participantIDParam = "participantID"
)

func (s *Service) getAllBoardFacilitationRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/reveal", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.revealCards},
		{Method: http.MethodPost, Path: "/boards/:boardID/hide", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.hideCards},
		{Method: http.MethodPost, Path: "/boards/:boardID/timer", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.controlTimer},
		{Method: http.MethodPost, Path: "/boards/:boardID/stage", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.changeStage},
		{Method: http.MethodPost, Path: "/boards/:boardID/close", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.closeBoard},
		{Method: http.MethodPut, Path: "/boards/:boardID/participants/:participantID/role", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.changeParticipantRole},
		{Method: http.MethodDelete, Path: "/boards/:boardID/participants/:participantID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.removeParticipant},
	}
}

func (s *Service) revealCards(c *gin.Context) {
	command, ok := boardOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.RevealCards(c.Request.Context(), command)
	if boardFailed(c, err, "failed to reveal the cards") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) hideCards(c *gin.Context) {
	command, ok := boardOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.HideCards(c.Request.Context(), command)
	if boardFailed(c, err, "failed to hide the cards") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) controlTimer(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.TimerRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	timer := board.TimerCommand{Command: command, Action: req.Action, DurationMS: req.DurationMS}

	result, err := s.BoardService.ControlTimer(c.Request.Context(), timer)
	if boardFailed(c, err, "failed to change the timer") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) changeStage(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.StageRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	stage := board.StageCommand{Command: command, Stage: req.Stage}

	result, err := s.BoardService.ChangeStage(c.Request.Context(), stage)
	answerShared(c, result, err, "failed to move the board to its next stage")
}

func (s *Service) closeBoard(c *gin.Context) {
	command, ok := boardOpCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardService.Close(c.Request.Context(), command)
	answerShared(c, result, err, "failed to end the retro")
}

func (s *Service) changeParticipantRole(c *gin.Context) {
	command, ok := participantCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ParticipantRoleRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	assign := board.ParticipantRoleCommand{Command: command.Command, ParticipantID: command.ParticipantID, Role: req.Role}

	result, err := s.BoardService.ChangeParticipantRole(c.Request.Context(), assign)
	if boardFailed(c, err, "failed to change the participant's role") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) removeParticipant(c *gin.Context) {
	command, ok := participantCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardService.RemoveParticipant(c.Request.Context(), command)
	if boardFailed(c, err, "failed to remove the participant") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func boardOpCommand(c *gin.Context) (board.Command, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return board.Command{}, false
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return board.Command{}, false
	}

	command.OpID = req.OpID

	return command, true
}

func participantCommand(c *gin.Context) (board.ParticipantCommand, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return board.ParticipantCommand{}, false
	}

	participantID, ok := utils.ParseIDParam(c, participantIDParam)
	if !ok {
		return board.ParticipantCommand{}, false
	}

	return board.ParticipantCommand{Command: command, ParticipantID: participantID}, true
}
