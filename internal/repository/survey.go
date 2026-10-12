package repository

import (
	"context"
	"database/sql"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	SurveyRepository interface {
		dao.SurveyDAO

		CreateSurvey(ctx context.Context, s spec.SurveyCreateWrite, tx ...*sql.Tx) (*model.Versioned[model.Survey], error)
		CloseSurvey(ctx context.Context, s spec.SurveyWrite, tx ...*sql.Tx) (*model.Versioned[model.SurveyState], error)
		DeleteSurvey(ctx context.Context, s spec.SurveyWrite, tx ...*sql.Tx) (*model.Versioned[model.Survey], error)
		Respond(ctx context.Context, s spec.SurveyResponseWrite, tx ...*sql.Tx) (*model.Versioned[model.SurveyState], error)
	}

	surveyRepository struct {
		dao.SurveyDAO

		db     *sql.DB
		boards dao.BoardDAO
	}
)

func NewSurveyRepo(database *sql.DB, surveyDAO dao.SurveyDAO, boards dao.BoardDAO) SurveyRepository {
	return &surveyRepository{SurveyDAO: surveyDAO, db: database, boards: boards}
}

func (r *surveyRepository) CreateSurvey(ctx context.Context, s spec.SurveyCreateWrite, tx ...*sql.Tx) (*model.Versioned[model.Survey], error) {
	create := func(tx *sql.Tx) (*model.Survey, error) {
		if _, err := r.SurveyDAO.Create(ctx, s.Survey, tx); err != nil {
			return nil, err
		}

		for _, question := range s.Questions {
			if _, err := r.SurveyDAO.AddQuestion(ctx, question, tx); err != nil {
				return nil, err
			}
		}

		return r.SurveyDAO.Get(ctx, spec.SurveyKey{BoardID: s.BoardID, SurveyID: s.Survey.ID}, tx)
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, create)
}

func (r *surveyRepository) CloseSurvey(ctx context.Context, s spec.SurveyWrite, tx ...*sql.Tx) (*model.Versioned[model.SurveyState], error) {
	closeSurvey := func(tx *sql.Tx) (*model.SurveyState, error) {
		key := spec.SurveyKey{BoardID: s.BoardID, SurveyID: s.SurveyID}

		survey, err := r.SurveyDAO.Close(ctx, key, tx)
		if err != nil {
			return nil, err
		}

		responses, err := r.SurveyDAO.ListResponsesForSurvey(ctx, key, tx)
		if err != nil {
			return nil, err
		}

		return &model.SurveyState{Survey: *survey, Responses: responses}, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, closeSurvey)
}

func (r *surveyRepository) DeleteSurvey(ctx context.Context, s spec.SurveyWrite, tx ...*sql.Tx) (*model.Versioned[model.Survey], error) {
	remove := func(tx *sql.Tx) (*model.Survey, error) {
		key := spec.SurveyKey{BoardID: s.BoardID, SurveyID: s.SurveyID}

		survey, err := r.SurveyDAO.Get(ctx, key, tx)
		if err != nil {
			return nil, err
		}

		if err := r.SurveyDAO.Delete(ctx, key, tx); err != nil {
			return nil, err
		}

		return survey, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, remove)
}

func (r *surveyRepository) Respond(ctx context.Context, s spec.SurveyResponseWrite, tx ...*sql.Tx) (*model.Versioned[model.SurveyState], error) {
	respond := func(tx *sql.Tx) (*model.SurveyState, error) {
		key := spec.SurveyKey{BoardID: s.BoardID, SurveyID: s.SurveyID}

		if _, err := r.SurveyDAO.DeleteResponses(ctx, spec.SurveyParticipantKey{BoardID: s.BoardID, SurveyID: s.SurveyID, ParticipantID: s.ParticipantID}, tx); err != nil {
			return nil, err
		}

		for _, answer := range s.Answers {
			err := r.SurveyDAO.AddResponse(ctx, spec.NewSurveyResponse{
				OrgID:         s.OrgID,
				BoardID:       s.BoardID,
				SurveyID:      s.SurveyID,
				QuestionID:    answer.QuestionID,
				ParticipantID: s.ParticipantID,
				Answer:        answer.Answer,
			}, tx)
			if err != nil {
				return nil, err
			}
		}

		survey, err := r.SurveyDAO.Get(ctx, key, tx)
		if err != nil {
			return nil, err
		}

		responses, err := r.SurveyDAO.ListResponsesForSurvey(ctx, key, tx)
		if err != nil {
			return nil, err
		}

		return &model.SurveyState{Survey: *survey, Responses: responses}, nil
	}

	return writeBoard(ctx, r.db, r.boards, s.BoardWrite, tx, respond)
}
