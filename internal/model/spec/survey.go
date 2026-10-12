package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewSurvey struct {
		OrgID       uuid.UUID
		BoardID     uuid.UUID
		ID          uuid.UUID
		Title       string
		TemplateKey string
	}

	NewSurveyQuestion struct {
		OrgID    uuid.UUID
		BoardID  uuid.UUID
		SurveyID uuid.UUID
		ID       uuid.UUID
		Position int
		Kind     model.QuestionKind
		Prompt   string
		Options  []string
		ScaleMin *int
		ScaleMax *int
	}

	SurveyKey struct {
		BoardID  uuid.UUID
		SurveyID uuid.UUID
	}

	SurveyParticipantKey struct {
		BoardID       uuid.UUID
		SurveyID      uuid.UUID
		ParticipantID uuid.UUID
	}

	NewSurveyResponse struct {
		OrgID         uuid.UUID
		BoardID       uuid.UUID
		SurveyID      uuid.UUID
		QuestionID    uuid.UUID
		ParticipantID uuid.UUID
		Answer        []int
	}

	SurveyCreateWrite struct {
		BoardWrite

		Survey    NewSurvey
		Questions []NewSurveyQuestion
	}

	SurveyWrite struct {
		BoardWrite

		SurveyID uuid.UUID
	}

	SurveyAnswer struct {
		QuestionID uuid.UUID
		Answer     []int
	}

	SurveyResponseWrite struct {
		BoardWrite

		SurveyID      uuid.UUID
		ParticipantID uuid.UUID
		Answers       []SurveyAnswer
	}
)
