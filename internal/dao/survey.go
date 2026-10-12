package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	SurveyDAO interface {
		Create(ctx context.Context, s spec.NewSurvey, tx ...*sql.Tx) (*model.Survey, error)
		AddQuestion(ctx context.Context, s spec.NewSurveyQuestion, tx ...*sql.Tx) (*model.SurveyQuestion, error)
		Get(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) (*model.Survey, error)
		List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.Survey, error)
		Count(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int, error)
		Close(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) (*model.Survey, error)
		CloseOpen(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
		Delete(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) error
		AddResponse(ctx context.Context, s spec.NewSurveyResponse, tx ...*sql.Tx) error
		DeleteResponses(ctx context.Context, s spec.SurveyParticipantKey, tx ...*sql.Tx) (int64, error)
		ListResponses(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.SurveyResponse, error)
		ListResponsesForSurvey(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) ([]model.SurveyResponse, error)
		HandOverResponses(ctx context.Context, s spec.ParticipantHandover, tx ...*sql.Tx) (int64, error)
		SeverResponses(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error)
	}

	surveyDAO struct {
		db *sql.DB
	}
)

func toSurvey(row sqlcgen.BoardSurvey) *model.Survey {
	return &model.Survey{
		ID:          row.ID,
		OrgID:       row.OrgID,
		BoardID:     row.BoardID,
		Title:       row.Title,
		TemplateKey: row.TemplateKey,
		Status:      model.SurveyStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		ClosedAt:    nullableTime(row.ClosedAt),
	}
}

func toSurveyQuestion(row sqlcgen.SurveyQuestion) (*model.SurveyQuestion, error) {
	var options []string
	if err := json.Unmarshal(row.Options, &options); err != nil {
		return nil, fmt.Errorf("decode the options of survey question %s: %w", row.ID, err)
	}

	if options == nil {
		options = []string{}
	}

	return &model.SurveyQuestion{
		ID:       row.ID,
		SurveyID: row.SurveyID,
		Position: int(row.Position),
		Kind:     model.QuestionKind(row.Kind),
		Prompt:   row.Prompt,
		Options:  options,
		ScaleMin: nullableInt16(row.ScaleMin),
		ScaleMax: nullableInt16(row.ScaleMax),
	}, nil
}

func toSurveyResponse(id, surveyID, questionID uuid.UUID, participantID *uuid.UUID, answer json.RawMessage) (*model.SurveyResponse, error) {
	var picked []int
	if err := json.Unmarshal(answer, &picked); err != nil {
		return nil, fmt.Errorf("decode the answer of survey response %s: %w", id, err)
	}

	return &model.SurveyResponse{ID: id, SurveyID: surveyID, QuestionID: questionID, ParticipantID: participantID, Answer: picked}, nil
}

func toSurveyQuestions(rows []sqlcgen.SurveyQuestion) ([]model.SurveyQuestion, error) {
	questions := make([]model.SurveyQuestion, 0, len(rows))
	for _, row := range rows {
		question, err := toSurveyQuestion(row)
		if err != nil {
			return nil, err
		}

		questions = append(questions, *question)
	}

	return questions, nil
}

func surveyRow(row sqlcgen.BoardSurvey, err error, action string) (*model.Survey, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	return toSurvey(row), nil
}

func (r *surveyDAO) Create(ctx context.Context, s spec.NewSurvey, tx ...*sql.Tx) (*model.Survey, error) {
	action := fmt.Sprintf("create survey %s on board %s", s.ID, s.BoardID)

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateSurvey(ctx, sqlcgen.CreateSurveyParams{
		ID:          s.ID,
		OrgID:       s.OrgID,
		BoardID:     s.BoardID,
		Title:       s.Title,
		TemplateKey: s.TemplateKey,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	survey := toSurvey(row)
	survey.Questions = []model.SurveyQuestion{}

	return survey, nil
}

func (r *surveyDAO) AddQuestion(ctx context.Context, s spec.NewSurveyQuestion, tx ...*sql.Tx) (*model.SurveyQuestion, error) {
	action := fmt.Sprintf("add question %s to survey %s of board %s", s.ID, s.SurveyID, s.BoardID)

	options := s.Options
	if options == nil {
		options = []string{}
	}

	encoded, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("%s: encode the options: %w", action, err)
	}

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddSurveyQuestion(ctx, sqlcgen.AddSurveyQuestionParams{
		ID:       s.ID,
		OrgID:    s.OrgID,
		BoardID:  s.BoardID,
		SurveyID: s.SurveyID,
		Position: int16(min(max(s.Position, 0), model.SurveyMaxQuestions)),
		Kind:     string(s.Kind),
		Prompt:   s.Prompt,
		Options:  encoded,
		ScaleMin: toNullInt16(s.ScaleMin),
		ScaleMax: toNullInt16(s.ScaleMax),
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	return toSurveyQuestion(row)
}

func (r *surveyDAO) Get(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) (*model.Survey, error) {
	action := fmt.Sprintf("get survey %s of board %s", s.SurveyID, s.BoardID)
	queries := sqlcgen.New(db.TxOrDB(r.db, tx))

	row, err := queries.GetSurvey(ctx, sqlcgen.GetSurveyParams{BoardID: s.BoardID, ID: s.SurveyID})

	survey, err := surveyRow(row, err, action)
	if err != nil {
		return nil, err
	}

	rows, err := queries.ListSurveyQuestions(ctx, sqlcgen.ListSurveyQuestionsParams{BoardID: s.BoardID, SurveyID: s.SurveyID})
	if err != nil {
		return nil, fmt.Errorf("%s: list its questions: %w", action, err)
	}

	questions, err := toSurveyQuestions(rows)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	survey.Questions = questions

	return survey, nil
}

func (r *surveyDAO) List(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.Survey, error) {
	action := fmt.Sprintf("list the surveys of board %s", boardID)
	queries := sqlcgen.New(db.TxOrDB(r.db, tx))

	rows, err := queries.ListSurveys(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	questionRows, err := queries.ListBoardSurveyQuestions(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("%s: list their questions: %w", action, err)
	}

	questions, err := toSurveyQuestions(questionRows)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	asked := make(map[uuid.UUID][]model.SurveyQuestion, len(rows))
	for _, question := range questions {
		asked[question.SurveyID] = append(asked[question.SurveyID], question)
	}

	surveys := make([]model.Survey, 0, len(rows))
	for _, row := range rows {
		survey := toSurvey(row)

		survey.Questions = asked[row.ID]
		if survey.Questions == nil {
			survey.Questions = []model.SurveyQuestion{}
		}

		surveys = append(surveys, *survey)
	}

	return surveys, nil
}

func (r *surveyDAO) Count(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountSurveys(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("count the surveys of board %s: %w", boardID, err)
	}

	return int(count), nil
}

func (r *surveyDAO) Close(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) (*model.Survey, error) {
	action := fmt.Sprintf("close survey %s of board %s", s.SurveyID, s.BoardID)
	queries := sqlcgen.New(db.TxOrDB(r.db, tx))

	row, err := queries.CloseSurvey(ctx, sqlcgen.CloseSurveyParams{BoardID: s.BoardID, ID: s.SurveyID})

	survey, err := surveyRow(row, err, action)
	if err != nil {
		return nil, err
	}

	rows, err := queries.ListSurveyQuestions(ctx, sqlcgen.ListSurveyQuestionsParams{BoardID: s.BoardID, SurveyID: s.SurveyID})
	if err != nil {
		return nil, fmt.Errorf("%s: list its questions: %w", action, err)
	}

	questions, err := toSurveyQuestions(rows)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	survey.Questions = questions

	return survey, nil
}

func (r *surveyDAO) CloseOpen(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	closed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CloseOpenSurveys(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("close the open surveys of board %s: %w", boardID, err)
	}

	return closed, nil
}

func (r *surveyDAO) Delete(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) error {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteSurvey(ctx, sqlcgen.DeleteSurveyParams{BoardID: s.BoardID, ID: s.SurveyID})
	if err != nil {
		return fmt.Errorf("delete survey %s of board %s: %w", s.SurveyID, s.BoardID, err)
	}

	if deleted == 0 {
		return fmt.Errorf("delete survey %s of board %s: %w", s.SurveyID, s.BoardID, ErrNotFound)
	}

	return nil
}

func (r *surveyDAO) AddResponse(ctx context.Context, s spec.NewSurveyResponse, tx ...*sql.Tx) error {
	action := fmt.Sprintf("answer question %s of survey %s on board %s as participant %s", s.QuestionID, s.SurveyID, s.BoardID, s.ParticipantID)

	answer := s.Answer
	if answer == nil {
		answer = []int{}
	}

	encoded, err := json.Marshal(answer)
	if err != nil {
		return fmt.Errorf("%s: encode the answer: %w", action, err)
	}

	err = sqlcgen.New(db.TxOrDB(r.db, tx)).AddSurveyResponse(ctx, sqlcgen.AddSurveyResponseParams{
		OrgID:         s.OrgID,
		BoardID:       s.BoardID,
		SurveyID:      s.SurveyID,
		QuestionID:    s.QuestionID,
		ParticipantID: s.ParticipantID,
		Answer:        encoded,
	})
	if isUniqueViolation(err) {
		return fmt.Errorf("%s: %w: %w", action, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	return nil
}

func (r *surveyDAO) DeleteResponses(ctx context.Context, s spec.SurveyParticipantKey, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteSurveyResponses(ctx, sqlcgen.DeleteSurveyResponsesParams{BoardID: s.BoardID, SurveyID: s.SurveyID, ParticipantID: s.ParticipantID})
	if err != nil {
		return 0, fmt.Errorf("delete the answers of participant %s to survey %s of board %s: %w", s.ParticipantID, s.SurveyID, s.BoardID, err)
	}

	return deleted, nil
}

func (r *surveyDAO) ListResponses(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.SurveyResponse, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListSurveyResponses(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list the survey answers of board %s: %w", boardID, err)
	}

	responses := make([]model.SurveyResponse, 0, len(rows))
	for _, row := range rows {
		response, err := toSurveyResponse(row.ID, row.SurveyID, row.QuestionID, row.ParticipantID, row.Answer)
		if err != nil {
			return nil, fmt.Errorf("list the survey answers of board %s: %w", boardID, err)
		}

		responses = append(responses, *response)
	}

	return responses, nil
}

func (r *surveyDAO) ListResponsesForSurvey(ctx context.Context, s spec.SurveyKey, tx ...*sql.Tx) ([]model.SurveyResponse, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListSurveyResponsesForSurvey(ctx, sqlcgen.ListSurveyResponsesForSurveyParams{BoardID: s.BoardID, SurveyID: s.SurveyID})
	if err != nil {
		return nil, fmt.Errorf("list the answers to survey %s of board %s: %w", s.SurveyID, s.BoardID, err)
	}

	responses := make([]model.SurveyResponse, 0, len(rows))
	for _, row := range rows {
		response, err := toSurveyResponse(row.ID, row.SurveyID, row.QuestionID, row.ParticipantID, row.Answer)
		if err != nil {
			return nil, fmt.Errorf("list the answers to survey %s of board %s: %w", s.SurveyID, s.BoardID, err)
		}

		responses = append(responses, *response)
	}

	return responses, nil
}

func (r *surveyDAO) HandOverResponses(ctx context.Context, s spec.ParticipantHandover, tx ...*sql.Tx) (int64, error) {
	handed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).HandOverSurveyResponses(ctx, sqlcgen.HandOverSurveyResponsesParams{
		ToParticipantID:   s.ToParticipantID,
		BoardID:           s.BoardID,
		FromParticipantID: s.FromParticipantID,
	})
	if err != nil {
		return 0, fmt.Errorf("hand the survey answers of participant %s to participant %s on board %s: %w", s.FromParticipantID, s.ToParticipantID, s.BoardID, err)
	}

	return handed.Moved, nil
}

func (r *surveyDAO) SeverResponses(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	severed, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SeverSurveyResponses(ctx, boardID)
	if err != nil {
		return 0, fmt.Errorf("sever the survey answers of board %s from their participants: %w", boardID, err)
	}

	return severed, nil
}
