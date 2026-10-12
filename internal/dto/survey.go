package dto

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	SurveyView struct {
		ID          uuid.UUID            `json:"id"`
		Title       string               `json:"title"`
		TemplateKey string               `json:"template_key"`
		Status      model.SurveyStatus   `json:"status"`
		Questions   []SurveyQuestionView `json:"questions"`
		Respondents int                  `json:"respondents"`
		MyAnswers   map[uuid.UUID][]int  `json:"my_answers"`
	}

	SurveyQuestionView struct {
		ID       uuid.UUID          `json:"id"`
		Kind     model.QuestionKind `json:"kind"`
		Prompt   string             `json:"prompt"`
		Options  []string           `json:"options"`
		ScaleMin *int               `json:"scale_min"`
		ScaleMax *int               `json:"scale_max"`
		Results  *SurveyResultsView `json:"results"`
	}

	SurveyResultsView struct {
		Counts    []int    `json:"counts"`
		Responses int      `json:"responses"`
		Average   *float64 `json:"average"`
	}

	CreateSurveyRequest struct {
		BoardCommandRequest

		ID          uuid.UUID               `json:"id"`
		Title       string                  `json:"title"`
		TemplateKey string                  `json:"template_key"`
		Questions   []SurveyQuestionRequest `json:"questions"`
	}

	SurveyQuestionRequest struct {
		ID       uuid.UUID          `json:"id"`
		Kind     model.QuestionKind `json:"kind"`
		Prompt   string             `json:"prompt"`
		Options  []string           `json:"options"`
		ScaleMin *int               `json:"scale_min"`
		ScaleMax *int               `json:"scale_max"`
	}

	SurveyResponseRequest struct {
		BoardCommandRequest

		Answers []SurveyAnswerRequest `json:"answers"`
	}

	SurveyAnswerRequest struct {
		QuestionID uuid.UUID `json:"question_id"`
		Answer     []int     `json:"answer"`
	}

	SurveyTemplateQuestionView struct {
		Kind     model.QuestionKind `json:"kind"`
		Prompt   string             `json:"prompt"`
		Options  []string           `json:"options"`
		ScaleMin *int               `json:"scale_min"`
		ScaleMax *int               `json:"scale_max"`
	}

	SurveyTemplateView struct {
		Key         string                       `json:"key"`
		Title       string                       `json:"title"`
		Description string                       `json:"description"`
		Category    string                       `json:"category"`
		Questions   []SurveyTemplateQuestionView `json:"questions"`
	}

	SurveyTemplateListResponse struct {
		Templates []SurveyTemplateView `json:"templates"`
	}
)

func (r CreateSurveyRequest) ClientIDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(r.Questions)+1)
	ids = append(ids, r.ID)

	for _, question := range r.Questions {
		ids = append(ids, question.ID)
	}

	return ids
}
