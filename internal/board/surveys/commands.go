package surveys

import (
	"retrotide/internal/board"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	QuestionDraft struct {
		ID       uuid.UUID
		Kind     model.QuestionKind
		Prompt   string
		Options  []string
		ScaleMin *int
		ScaleMax *int
	}

	CreateCommand struct {
		board.Command

		SurveyID    uuid.UUID
		Title       string
		TemplateKey string
		Questions   []QuestionDraft
	}

	SurveyCommand struct {
		board.Command

		SurveyID uuid.UUID
	}

	RespondCommand struct {
		board.Command

		SurveyID uuid.UUID
		Answers  []spec.SurveyAnswer
	}
)
