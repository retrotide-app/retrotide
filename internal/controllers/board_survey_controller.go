package controllers

import (
	"errors"
	"net/http"
	"slices"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/surveys"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"

	"github.com/gin-gonic/gin"
)

const (
	surveyIDParam = "surveyID"
)

var (
	surveyAnswers = []errorAnswer{
		{err: surveys.ErrTitleRequired, status: http.StatusBadRequest, code: "survey_title_required"},
		{err: surveys.ErrTitleTooLong, status: http.StatusBadRequest, code: "survey_title_too_long"},
		{err: surveys.ErrQuestionsRequired, status: http.StatusBadRequest, code: "survey_questions_required"},
		{err: surveys.ErrTooManyQuestions, status: http.StatusBadRequest, code: "too_many_questions"},
		{err: surveys.ErrInvalidQuestionKind, status: http.StatusBadRequest, code: "invalid_question_kind"},
		{err: surveys.ErrPromptRequired, status: http.StatusBadRequest, code: "question_prompt_required"},
		{err: surveys.ErrPromptTooLong, status: http.StatusBadRequest, code: "question_prompt_too_long"},
		{err: surveys.ErrInvalidOptions, status: http.StatusBadRequest, code: "invalid_options"},
		{err: surveys.ErrInvalidScale, status: http.StatusBadRequest, code: "invalid_scale"},
		{err: surveys.ErrUnknownTemplate, status: http.StatusBadRequest, code: "unknown_survey_template"},
		{err: surveys.ErrTemplateWithQuestions, status: http.StatusBadRequest, code: "template_with_questions"},
		{err: surveys.ErrAnswersRequired, status: http.StatusBadRequest, code: "answers_required"},
		{err: surveys.ErrInvalidAnswer, status: http.StatusBadRequest, code: "invalid_answer"},
		{err: surveys.ErrUnknownQuestion, status: http.StatusBadRequest, code: "unknown_question"},
		{err: policy.ErrSurveyNotFound, status: http.StatusNotFound},
		{err: policy.ErrSurveyClosed, status: http.StatusConflict, code: "survey_closed"},
		{err: surveys.ErrTooManySurveys, status: http.StatusConflict, code: "too_many_surveys"},
	}

	boardSurveyErrorAnswers = slices.Concat(surveyAnswers, boardErrorAnswers)
)

func (s *Service) getAllSurveyTemplateRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/survey-templates", Handler: s.listSurveyTemplates},
	}
}

func (s *Service) getAllBoardSurveyRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/boards/:boardID/surveys", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.createSurvey},
		{Method: http.MethodPost, Path: "/boards/:boardID/surveys/:surveyID/close", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.closeSurvey},
		{Method: http.MethodDelete, Path: "/boards/:boardID/surveys/:surveyID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.deleteSurvey},
		{Method: http.MethodPut, Path: "/boards/:boardID/surveys/:surveyID/responses", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.respondToSurvey},
	}
}

func (s *Service) listSurveyTemplates(c *gin.Context) {
	listing := s.BoardSurveys.Templates()

	c.JSON(http.StatusOK, listing)
}

func (s *Service) createSurvey(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateSurveyRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	questions := make([]surveys.QuestionDraft, 0, len(req.Questions))
	for _, question := range req.Questions {
		questions = append(questions, surveys.QuestionDraft{
			ID:       question.ID,
			Kind:     question.Kind,
			Prompt:   question.Prompt,
			Options:  question.Options,
			ScaleMin: question.ScaleMin,
			ScaleMax: question.ScaleMax,
		})
	}

	create := surveys.CreateCommand{Command: command, SurveyID: req.ID, Title: req.Title, TemplateKey: req.TemplateKey, Questions: questions}

	result, err := s.BoardSurveys.Create(c.Request.Context(), create)
	if surveyFailed(c, err, "failed to create the survey") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) closeSurvey(c *gin.Context) {
	command, ok := surveyCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardSurveys.Close(c.Request.Context(), command)
	if surveyFailed(c, err, "failed to close the survey") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) deleteSurvey(c *gin.Context) {
	command, ok := surveyCommand(c)
	if !ok {
		return
	}

	result, err := s.BoardSurveys.Delete(c.Request.Context(), command)
	if surveyFailed(c, err, "failed to delete the survey") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) respondToSurvey(c *gin.Context) {
	command, ok := surveyTarget(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.SurveyResponseRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	answers := make([]spec.SurveyAnswer, 0, len(req.Answers))
	for _, answer := range req.Answers {
		answers = append(answers, spec.SurveyAnswer{QuestionID: answer.QuestionID, Answer: answer.Answer})
	}

	respond := surveys.RespondCommand{Command: command.Command, SurveyID: command.SurveyID, Answers: answers}

	result, err := s.BoardSurveys.Respond(c.Request.Context(), respond)
	if surveyFailed(c, err, "failed to answer the survey") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func surveyTarget(c *gin.Context) (surveys.SurveyCommand, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return surveys.SurveyCommand{}, false
	}

	surveyID, ok := utils.ParseIDParam(c, surveyIDParam)
	if !ok {
		return surveys.SurveyCommand{}, false
	}

	return surveys.SurveyCommand{Command: command, SurveyID: surveyID}, true
}

func surveyCommand(c *gin.Context) (surveys.SurveyCommand, bool) {
	command, ok := surveyTarget(c)
	if !ok {
		return surveys.SurveyCommand{}, false
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return surveys.SurveyCommand{}, false
	}

	command.OpID = req.OpID

	return command, true
}

func surveyFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	if _, retry := errors.AsType[*live.RetryError](err); retry {
		return boardFailed(c, err, msg)
	}

	answerError(c, boardSurveyErrorAnswers, err, msg)

	return true
}
