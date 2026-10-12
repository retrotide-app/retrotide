package surveys

import (
	"fmt"
	"slices"

	"retrotide/internal/board"
	"retrotide/internal/dto"
	"retrotide/internal/surveytemplate"

	"github.com/google/uuid"
)

func (s *service) Templates() *dto.SurveyTemplateListResponse {
	catalogue := surveytemplate.All()

	templates := make([]dto.SurveyTemplateView, 0, len(catalogue))
	for _, template := range catalogue {
		templates = append(templates, templateView(template))
	}

	return &dto.SurveyTemplateListResponse{Templates: templates}
}

func templateView(template surveytemplate.Template) dto.SurveyTemplateView {
	questions := make([]dto.SurveyTemplateQuestionView, 0, len(template.Questions))
	for _, question := range template.Questions {
		questions = append(questions, dto.SurveyTemplateQuestionView{
			Kind:     question.Kind,
			Prompt:   question.Prompt,
			Options:  append([]string{}, question.Options...),
			ScaleMin: question.ScaleMin,
			ScaleMax: question.ScaleMax,
		})
	}

	return dto.SurveyTemplateView{
		Key:         template.Key,
		Title:       template.Title,
		Description: template.Description,
		Category:    template.Category,
		Questions:   questions,
	}
}

func questionDrafts(command CreateCommand) ([]QuestionDraft, error) {
	if command.TemplateKey == "" {
		for _, question := range command.Questions {
			if err := board.ClientID(question.ID); err != nil {
				return nil, err
			}
		}

		return command.Questions, nil
	}

	if len(command.Questions) > 0 {
		return nil, ErrTemplateWithQuestions
	}

	template, ok := surveytemplate.Lookup(command.TemplateKey)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTemplate, command.TemplateKey)
	}

	drafts := make([]QuestionDraft, 0, len(template.Questions))
	for _, question := range template.Questions {
		drafts = append(drafts, QuestionDraft{
			ID:       uuid.New(),
			Kind:     question.Kind,
			Prompt:   question.Prompt,
			Options:  slices.Clone(question.Options),
			ScaleMin: question.ScaleMin,
			ScaleMax: question.ScaleMax,
		})
	}

	return drafts, nil
}
