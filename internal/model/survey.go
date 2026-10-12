package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	QuestionKind string

	SurveyStatus string

	Survey struct {
		ID          uuid.UUID
		OrgID       uuid.UUID
		BoardID     uuid.UUID
		Title       string
		TemplateKey string
		Status      SurveyStatus
		CreatedAt   time.Time
		ClosedAt    *time.Time
		Questions   []SurveyQuestion
	}

	SurveyQuestion struct {
		ID       uuid.UUID
		SurveyID uuid.UUID
		Position int
		Kind     QuestionKind
		Prompt   string
		Options  []string
		ScaleMin *int
		ScaleMax *int
	}

	SurveyResponse struct {
		ID            uuid.UUID
		SurveyID      uuid.UUID
		QuestionID    uuid.UUID
		ParticipantID *uuid.UUID
		Answer        []int
	}

	SurveyState struct {
		Survey    Survey
		Responses []SurveyResponse
	}
)

const (
	QuestionSingleChoice   QuestionKind = "single_choice"
	QuestionMultipleChoice QuestionKind = "multiple_choice"
	QuestionScale          QuestionKind = "scale"
)

const (
	SurveyOpen   SurveyStatus = "open"
	SurveyClosed SurveyStatus = "closed"
)

const (
	SurveyTitleMaxRunes  = 120
	SurveyPromptMaxRunes = 300
	SurveyOptionMaxRunes = 100
	SurveyMaxQuestions   = 20
	SurveyMinOptions     = 2
	SurveyMaxOptions     = 10
	SurveyScaleFloor     = 0
	SurveyScaleCeiling   = 10
	BoardMaxSurveys      = 20
)
