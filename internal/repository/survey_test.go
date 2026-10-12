package repository

import (
	"context"
	"database/sql"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRespond_ReplacesTheParticipantsAnswersThenReadsTheSurvey(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	surveys := dao.NewMockSurveyDAO(t)
	repo := NewSurveyRepo(nil, surveys, boards)
	write, inTx := boardWriteFixture()
	surveyID := uuid.New()
	participantID := uuid.New()
	first := spec.SurveyAnswer{QuestionID: uuid.New(), Answer: []int{1}}
	second := spec.SurveyAnswer{QuestionID: uuid.New(), Answer: []int{0, 2}}
	key := spec.SurveyKey{BoardID: write.BoardID, SurveyID: surveyID}
	responses := []model.SurveyResponse{{ID: uuid.New(), SurveyID: surveyID, QuestionID: first.QuestionID, ParticipantID: new(participantID), Answer: []int{1}}}
	var order []string
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	surveys.EXPECT().DeleteResponses(mock.Anything, spec.SurveyParticipantKey{BoardID: write.BoardID, SurveyID: surveyID, ParticipantID: participantID}, inTx).RunAndReturn(func(context.Context, spec.SurveyParticipantKey, ...*sql.Tx) (int64, error) {
		order = append(order, "clear")
		return 2, nil
	}).Once()
	for _, answer := range []spec.SurveyAnswer{first, second} {
		surveys.EXPECT().AddResponse(mock.Anything, spec.NewSurveyResponse{
			OrgID:         write.OrgID,
			BoardID:       write.BoardID,
			SurveyID:      surveyID,
			QuestionID:    answer.QuestionID,
			ParticipantID: participantID,
			Answer:        answer.Answer,
		}, inTx).RunAndReturn(func(context.Context, spec.NewSurveyResponse, ...*sql.Tx) error {
			order = append(order, "answer")
			return nil
		}).Once()
	}
	surveys.EXPECT().Get(mock.Anything, key, inTx).Return(&model.Survey{ID: surveyID, Status: model.SurveyOpen}, nil).Once()
	surveys.EXPECT().ListResponsesForSurvey(mock.Anything, key, inTx).Return(responses, nil).Once()

	// when
	written, err := repo.Respond(t.Context(), spec.SurveyResponseWrite{BoardWrite: write, SurveyID: surveyID, ParticipantID: participantID, Answers: []spec.SurveyAnswer{first, second}}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"clear", "answer", "answer"}, order)
	assert.Equal(t, int64(42), written.Version)
	assert.Equal(t, surveyID, written.Value.Survey.ID)
	assert.Equal(t, responses, written.Value.Responses)
}

func TestCreateSurvey_CreatesTheSurveyThenEachQuestionThenReadsItBack(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	surveys := dao.NewMockSurveyDAO(t)
	repo := NewSurveyRepo(nil, surveys, boards)
	write, inTx := boardWriteFixture()
	survey := spec.NewSurvey{OrgID: write.OrgID, BoardID: write.BoardID, ID: uuid.New(), Title: "Pulse"}
	question := spec.NewSurveyQuestion{OrgID: write.OrgID, BoardID: write.BoardID, SurveyID: survey.ID, ID: uuid.New(), Kind: model.QuestionScale, Prompt: "Mood?", ScaleMin: new(1), ScaleMax: new(5)}
	var order []string
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	surveys.EXPECT().Create(mock.Anything, survey, inTx).RunAndReturn(func(context.Context, spec.NewSurvey, ...*sql.Tx) (*model.Survey, error) {
		order = append(order, "survey")
		return &model.Survey{ID: survey.ID}, nil
	}).Once()
	surveys.EXPECT().AddQuestion(mock.Anything, question, inTx).RunAndReturn(func(context.Context, spec.NewSurveyQuestion, ...*sql.Tx) (*model.SurveyQuestion, error) {
		order = append(order, "question")
		return &model.SurveyQuestion{ID: question.ID}, nil
	}).Once()
	surveys.EXPECT().Get(mock.Anything, spec.SurveyKey{BoardID: write.BoardID, SurveyID: survey.ID}, inTx).RunAndReturn(func(context.Context, spec.SurveyKey, ...*sql.Tx) (*model.Survey, error) {
		order = append(order, "read")
		return &model.Survey{ID: survey.ID, Questions: []model.SurveyQuestion{{ID: question.ID}}}, nil
	}).Once()

	// when
	created, err := repo.CreateSurvey(t.Context(), spec.SurveyCreateWrite{BoardWrite: write, Survey: survey, Questions: []spec.NewSurveyQuestion{question}}, inTx...)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"survey", "question", "read"}, order)
	assert.Len(t, created.Value.Questions, 1)
}

func TestDeleteSurvey_AnUnknownSurveyIsNotFound(t *testing.T) {
	// given
	boards := dao.NewMockBoardDAO(t)
	surveys := dao.NewMockSurveyDAO(t)
	repo := NewSurveyRepo(nil, surveys, boards)
	write, inTx := boardWriteFixture()
	boards.EXPECT().BumpVersion(mock.Anything, write, inTx).Return(42, nil).Once()
	boards.EXPECT().RecordOp(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	surveys.EXPECT().Get(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	deleted, err := repo.DeleteSurvey(t.Context(), spec.SurveyWrite{BoardWrite: write, SurveyID: uuid.New()}, inTx...)

	// then the strict mock refuses the delete
	assert.Nil(t, deleted)
	require.ErrorIs(t, err, dao.ErrNotFound)
}
