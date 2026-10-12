package surveys

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) Create(ctx context.Context, command CreateCommand) (*dto.BoardCommandResponse, error) {
	if err := board.ClientID(command.SurveyID); err != nil {
		return nil, err
	}

	drafts, err := questionDrafts(command)
	if err != nil {
		return nil, err
	}

	title, err := CleanTitle(command.Title)
	if err != nil {
		return nil, err
	}

	questions, err := CleanQuestions(drafts)
	if err != nil {
		return nil, err
	}

	create := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageSurveys(access); err != nil {
			return nil, err
		}

		count, err := s.surveys.Count(ctx, access.Header.ID)
		if err != nil {
			return nil, fmt.Errorf("count the surveys of board %s: %w", access.Header.ID, err)
		}

		if count >= model.BoardMaxSurveys {
			return nil, fmt.Errorf("%w: board %s holds %d", ErrTooManySurveys, access.Header.ID, count)
		}

		written, err := s.surveys.CreateSurvey(ctx, spec.SurveyCreateWrite{
			BoardWrite: target,
			Survey:     spec.NewSurvey{OrgID: target.OrgID, BoardID: target.BoardID, ID: command.SurveyID, Title: title, TemplateKey: command.TemplateKey},
			Questions:  newQuestions(target, command.SurveyID, questions),
		})
		if errors.Is(err, dao.ErrConflict) {
			return nil, fmt.Errorf("%w: survey %s: %w", board.ErrIDConflict, command.SurveyID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("create survey %s on board %s: %w", command.SurveyID, target.BoardID, err)
		}

		return s.announceSurvey(access, model.SurveyState{Survey: written.Value}, written.Version), nil
	}

	return s.Run(ctx, command.Command, s.surveyReplay(ctx, command.SurveyID), create)
}

func (s *service) Close(ctx context.Context, command SurveyCommand) (*dto.BoardCommandResponse, error) {
	closeSurvey := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageSurveys(access); err != nil {
			return nil, err
		}

		current, err := s.boardSurvey(ctx, access, command.SurveyID)
		if err != nil {
			return nil, err
		}

		if current.Status == model.SurveyClosed {
			return s.surveyAt(ctx, access, *current, access.Header.Version)
		}

		written, err := s.surveys.CloseSurvey(ctx, spec.SurveyWrite{BoardWrite: target, SurveyID: current.ID})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", policy.ErrSurveyNotFound, err)
		}
		if err != nil {
			return nil, fmt.Errorf("close survey %s of board %s: %w", current.ID, target.BoardID, err)
		}

		return s.announceSurvey(access, written.Value, written.Version), nil
	}

	return s.Run(ctx, command.Command, s.surveyReplay(ctx, command.SurveyID), closeSurvey)
}

func (s *service) Delete(ctx context.Context, command SurveyCommand) (*dto.BoardCommandResponse, error) {
	remove := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageSurveys(access); err != nil {
			return nil, err
		}

		current, err := s.boardSurvey(ctx, access, command.SurveyID)
		if err != nil {
			return nil, err
		}

		written, err := s.surveys.DeleteSurvey(ctx, spec.SurveyWrite{BoardWrite: target, SurveyID: current.ID})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", policy.ErrSurveyNotFound, err)
		}
		if err != nil {
			return nil, fmt.Errorf("delete survey %s of board %s: %w", current.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.SurveyRemoved(current.ID, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, remove)
}

func (s *service) announceSurvey(access model.BoardAccess, state model.SurveyState, version int64) *dto.BoardCommandResponse {
	s.events.Publish(access, projection.SurveyUpserted(access.Header.Board, state, version))

	response := projection.SurveyResponse(access.Header.Board, state, projection.ViewerFor(access), version)

	return &response
}

func (s *service) surveyAt(ctx context.Context, access model.BoardAccess, survey model.Survey, version int64) (*dto.BoardCommandResponse, error) {
	responses, err := s.surveys.ListResponsesForSurvey(ctx, spec.SurveyKey{BoardID: access.Header.ID, SurveyID: survey.ID})
	if err != nil {
		return nil, fmt.Errorf("list the answers to survey %s of board %s: %w", survey.ID, access.Header.ID, err)
	}

	response := projection.SurveyResponse(access.Header.Board, model.SurveyState{Survey: survey, Responses: responses}, projection.ViewerFor(access), version)

	return &response, nil
}

func (s *service) surveyReplay(ctx context.Context, surveyID uuid.UUID) board.Replay {
	return func(access model.BoardAccess, version int64) (*dto.BoardCommandResponse, error) {
		replayed := projection.ReplayResponse(version, &surveyID)

		survey, err := s.surveys.Get(ctx, spec.SurveyKey{BoardID: access.Header.ID, SurveyID: surveyID})
		if errors.Is(err, dao.ErrNotFound) {
			return &replayed, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read the replayed survey %s of board %s: %w", surveyID, access.Header.ID, err)
		}

		response, err := s.surveyAt(ctx, access, *survey, version)
		if err != nil {
			return nil, err
		}

		response.Replayed = true
		response.ID = &surveyID

		return response, nil
	}
}

func (s *service) boardSurvey(ctx context.Context, access model.BoardAccess, surveyID uuid.UUID) (*model.Survey, error) {
	survey, err := s.surveys.Get(ctx, spec.SurveyKey{BoardID: access.Header.ID, SurveyID: surveyID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", policy.ErrSurveyNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read survey %s of board %s: %w", surveyID, access.Header.ID, err)
	}

	return survey, nil
}

func newQuestions(target spec.BoardWrite, surveyID uuid.UUID, questions []QuestionDraft) []spec.NewSurveyQuestion {
	written := make([]spec.NewSurveyQuestion, 0, len(questions))
	for position, question := range questions {
		written = append(written, spec.NewSurveyQuestion{
			OrgID:    target.OrgID,
			BoardID:  target.BoardID,
			SurveyID: surveyID,
			ID:       question.ID,
			Position: position,
			Kind:     question.Kind,
			Prompt:   question.Prompt,
			Options:  question.Options,
			ScaleMin: question.ScaleMin,
			ScaleMax: question.ScaleMax,
		})
	}

	return written
}
