package surveys

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"retrotide/internal/board/policy"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) Respond(ctx context.Context, command RespondCommand) (*dto.BoardCommandResponse, error) {
	if len(command.Answers) == 0 {
		return nil, ErrAnswersRequired
	}

	respond := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		survey, err := s.boardSurvey(ctx, access, command.SurveyID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanRespondToSurvey(access, *survey); err != nil {
			return nil, err
		}

		answers, err := checkAnswers(*survey, command.Answers)
		if err != nil {
			return nil, err
		}

		written, err := s.surveys.Respond(ctx, spec.SurveyResponseWrite{
			BoardWrite:    target,
			SurveyID:      survey.ID,
			ParticipantID: *access.ParticipantID,
			Answers:       answers,
		})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", policy.ErrSurveyNotFound, err)
		}
		if err != nil {
			return nil, fmt.Errorf("answer survey %s of board %s: %w", survey.ID, target.BoardID, err)
		}

		return s.announceSurvey(access, written.Value, written.Version), nil
	}

	return s.Run(ctx, command.Command, s.surveyReplay(ctx, command.SurveyID), respond)
}

func checkAnswers(survey model.Survey, answers []spec.SurveyAnswer) ([]spec.SurveyAnswer, error) {
	if len(answers) == 0 {
		return nil, ErrAnswersRequired
	}

	questions := make(map[uuid.UUID]model.SurveyQuestion, len(survey.Questions))
	for _, question := range survey.Questions {
		questions[question.ID] = question
	}

	answered := make(map[uuid.UUID]struct{}, len(answers))
	checked := make([]spec.SurveyAnswer, 0, len(answers))
	for _, answer := range answers {
		question, ok := questions[answer.QuestionID]
		if !ok {
			return nil, fmt.Errorf("%w: question %s is not part of survey %s", ErrUnknownQuestion, answer.QuestionID, survey.ID)
		}

		if _, twice := answered[answer.QuestionID]; twice {
			return nil, fmt.Errorf("%w: question %s is answered twice", ErrUnknownQuestion, answer.QuestionID)
		}

		answered[answer.QuestionID] = struct{}{}

		picked, err := checkAnswer(question, answer.Answer)
		if err != nil {
			return nil, fmt.Errorf("question %s: %w", question.ID, err)
		}

		checked = append(checked, spec.SurveyAnswer{QuestionID: question.ID, Answer: picked})
	}

	return checked, nil
}

func checkAnswer(question model.SurveyQuestion, answer []int) ([]int, error) {
	switch question.Kind {
	case model.QuestionSingleChoice:
		if len(answer) != 1 || !optionIndex(question, answer[0]) {
			return nil, ErrInvalidAnswer
		}

		return []int{answer[0]}, nil
	case model.QuestionMultipleChoice:
		if len(answer) == 0 || len(answer) > len(question.Options) {
			return nil, ErrInvalidAnswer
		}

		picked := slices.Sorted(slices.Values(answer))
		if len(slices.Compact(slices.Clone(picked))) != len(picked) {
			return nil, ErrInvalidAnswer
		}

		for _, index := range picked {
			if !optionIndex(question, index) {
				return nil, ErrInvalidAnswer
			}
		}

		return picked, nil
	case model.QuestionScale:
		if len(answer) != 1 || question.ScaleMin == nil || question.ScaleMax == nil {
			return nil, ErrInvalidAnswer
		}

		if answer[0] < *question.ScaleMin || answer[0] > *question.ScaleMax {
			return nil, ErrInvalidAnswer
		}

		return []int{answer[0]}, nil
	default:
		return nil, ErrInvalidAnswer
	}
}

func optionIndex(question model.SurveyQuestion, index int) bool {
	return index >= 0 && index < len(question.Options)
}
