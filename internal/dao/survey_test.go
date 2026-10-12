package dao_test

import (
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	surveyFixture struct {
		boardFixture

		surveyID uuid.UUID
		choiceID uuid.UUID
		scaleID  uuid.UUID
	}
)

func newSurveyFixture(t *testing.T) surveyFixture {
	t.Helper()

	f := newBoardFixture(t)
	sf := surveyFixture{boardFixture: f, surveyID: uuid.New(), choiceID: uuid.New(), scaleID: uuid.New()}

	_, err := f.repos.Survey.Create(t.Context(), spec.NewSurvey{OrgID: f.orgID(), BoardID: f.boardID(), ID: sf.surveyID, Title: "Sprint health", TemplateKey: "health_check"})
	require.NoError(t, err)

	_, err = f.repos.Survey.AddQuestion(t.Context(), spec.NewSurveyQuestion{
		OrgID: f.orgID(), BoardID: f.boardID(), SurveyID: sf.surveyID, ID: sf.scaleID, Position: 1,
		Kind: model.QuestionScale, Prompt: "How was the pace?", ScaleMin: new(1), ScaleMax: new(5),
	})
	require.NoError(t, err)

	_, err = f.repos.Survey.AddQuestion(t.Context(), spec.NewSurveyQuestion{
		OrgID: f.orgID(), BoardID: f.boardID(), SurveyID: sf.surveyID, ID: sf.choiceID, Position: 0,
		Kind: model.QuestionMultipleChoice, Prompt: "What slowed us down?", Options: []string{"Reviews", "Meetings", "Flaky tests"},
	})
	require.NoError(t, err)

	return sf
}

func (sf surveyFixture) key() spec.SurveyKey {
	return spec.SurveyKey{BoardID: sf.boardID(), SurveyID: sf.surveyID}
}

func (sf surveyFixture) answer(t *testing.T, participantID, questionID uuid.UUID, answer ...int) {
	t.Helper()

	err := sf.repos.Survey.AddResponse(t.Context(), spec.NewSurveyResponse{OrgID: sf.orgID(), BoardID: sf.boardID(), SurveyID: sf.surveyID, QuestionID: questionID, ParticipantID: participantID, Answer: answer})
	require.NoError(t, err)
}

func TestSurvey_GetReadsTheQuestionsInPositionOrder(t *testing.T) {
	// given
	sf := newSurveyFixture(t)

	// when
	survey, err := sf.repos.Survey.Get(t.Context(), sf.key())

	// then
	require.NoError(t, err)
	assert.Equal(t, "Sprint health", survey.Title)
	assert.Equal(t, "health_check", survey.TemplateKey)
	assert.Equal(t, model.SurveyOpen, survey.Status)
	assert.Nil(t, survey.ClosedAt)
	require.Len(t, survey.Questions, 2)
	assert.Equal(t, model.SurveyQuestion{ID: sf.choiceID, SurveyID: sf.surveyID, Position: 0, Kind: model.QuestionMultipleChoice, Prompt: "What slowed us down?", Options: []string{"Reviews", "Meetings", "Flaky tests"}}, survey.Questions[0])
	assert.Equal(t, model.SurveyQuestion{ID: sf.scaleID, SurveyID: sf.surveyID, Position: 1, Kind: model.QuestionScale, Prompt: "How was the pace?", Options: []string{}, ScaleMin: new(1), ScaleMax: new(5)}, survey.Questions[1])
}

func TestSurvey_ListAndCountReadEverySurveyOfTheBoardWithItsQuestions(t *testing.T) {
	// given a second survey with no questions yet
	sf := newSurveyFixture(t)
	emptyID := uuid.New()
	_, err := sf.repos.Survey.Create(t.Context(), spec.NewSurvey{OrgID: sf.orgID(), BoardID: sf.boardID(), ID: emptyID, Title: "Mood"})
	require.NoError(t, err)

	// when
	listed, listErr := sf.repos.Survey.List(t.Context(), sf.boardID())
	counted, countErr := sf.repos.Survey.Count(t.Context(), sf.boardID())

	// then
	require.NoError(t, listErr)
	require.Len(t, listed, 2)
	assert.Equal(t, sf.surveyID, listed[0].ID)
	assert.Len(t, listed[0].Questions, 2)
	assert.Equal(t, emptyID, listed[1].ID)
	assert.NotNil(t, listed[1].Questions)
	assert.Empty(t, listed[1].Questions)
	require.NoError(t, countErr)
	assert.Equal(t, 2, counted)
}

func TestSurvey_TheSchemaRefusesAMalformedQuestion(t *testing.T) {
	tests := []struct {
		name     string
		question func(sf surveyFixture) spec.NewSurveyQuestion
	}{
		{
			name: "a scale with options",
			question: func(sf surveyFixture) spec.NewSurveyQuestion {
				return spec.NewSurveyQuestion{OrgID: sf.orgID(), BoardID: sf.boardID(), SurveyID: sf.surveyID, ID: uuid.New(), Position: 2, Kind: model.QuestionScale, Prompt: "Scale?", Options: []string{"a", "b"}, ScaleMin: new(0), ScaleMax: new(3)}
			},
		},
		{
			name: "a scale running backwards",
			question: func(sf surveyFixture) spec.NewSurveyQuestion {
				return spec.NewSurveyQuestion{OrgID: sf.orgID(), BoardID: sf.boardID(), SurveyID: sf.surveyID, ID: uuid.New(), Position: 2, Kind: model.QuestionScale, Prompt: "Scale?", ScaleMin: new(5), ScaleMax: new(1)}
			},
		},
		{
			name: "a choice with one option",
			question: func(sf surveyFixture) spec.NewSurveyQuestion {
				return spec.NewSurveyQuestion{OrgID: sf.orgID(), BoardID: sf.boardID(), SurveyID: sf.surveyID, ID: uuid.New(), Position: 2, Kind: model.QuestionSingleChoice, Prompt: "Choice?", Options: []string{"only"}}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sf := newSurveyFixture(t)

			// when
			_, err := sf.repos.Survey.AddQuestion(t.Context(), tc.question(sf))

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), "survey_questions_shape_check")
		})
	}
}

func TestSurvey_ATakenPositionIsAConflict(t *testing.T) {
	// given
	sf := newSurveyFixture(t)

	// when
	_, err := sf.repos.Survey.AddQuestion(t.Context(), spec.NewSurveyQuestion{OrgID: sf.orgID(), BoardID: sf.boardID(), SurveyID: sf.surveyID, ID: uuid.New(), Position: 0, Kind: model.QuestionScale, Prompt: "Again?", ScaleMin: new(0), ScaleMax: new(1)})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
}

func TestSurvey_CloseIsIdempotentAndCloseOpenClosesOnlyTheOpenOnes(t *testing.T) {
	// given a closed survey and another still open
	sf := newSurveyFixture(t)
	openID := uuid.New()
	_, err := sf.repos.Survey.Create(t.Context(), spec.NewSurvey{OrgID: sf.orgID(), BoardID: sf.boardID(), ID: openID, Title: "Mood"})
	require.NoError(t, err)

	// when
	first, firstErr := sf.repos.Survey.Close(t.Context(), sf.key())
	second, secondErr := sf.repos.Survey.Close(t.Context(), sf.key())
	closed, closeOpenErr := sf.repos.Survey.CloseOpen(t.Context(), sf.boardID())

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.Equal(t, model.SurveyClosed, first.Status)
	require.NotNil(t, first.ClosedAt)
	assert.Equal(t, first.ClosedAt, second.ClosedAt)
	assert.Len(t, first.Questions, 2)
	require.NoError(t, closeOpenErr)
	assert.Equal(t, int64(1), closed)
	other, err := sf.repos.Survey.Get(t.Context(), spec.SurveyKey{BoardID: sf.boardID(), SurveyID: openID})
	require.NoError(t, err)
	assert.Equal(t, model.SurveyClosed, other.Status)
}

func TestSurvey_DeleteRemovesTheSurveyItsQuestionsAndItsAnswers(t *testing.T) {
	// given
	sf := newSurveyFixture(t)
	sf.answer(t, sf.facilitator(), sf.scaleID, 3)

	// when
	err := sf.repos.Survey.Delete(t.Context(), sf.key())
	againErr := sf.repos.Survey.Delete(t.Context(), sf.key())

	// then
	require.NoError(t, err)
	require.ErrorIs(t, againErr, dao.ErrNotFound)
	assert.Zero(t, countWhere(t, sf.repos, `SELECT COUNT(*) FROM survey_questions WHERE board_id = $1`, sf.boardID()))
	assert.Zero(t, countWhere(t, sf.repos, `SELECT COUNT(*) FROM survey_responses WHERE board_id = $1`, sf.boardID()))
}

func TestSurvey_AnswersAreOnePerQuestionAndDeleteResponsesClearsOnlyTheParticipantsOwn(t *testing.T) {
	// given
	sf := newSurveyFixture(t)
	other := sf.secondParticipant(t)
	sf.answer(t, sf.facilitator(), sf.choiceID, 0, 2)
	sf.answer(t, sf.facilitator(), sf.scaleID, 4)
	sf.answer(t, other, sf.scaleID, 2)

	// when
	duplicateErr := sf.repos.Survey.AddResponse(t.Context(), spec.NewSurveyResponse{OrgID: sf.orgID(), BoardID: sf.boardID(), SurveyID: sf.surveyID, QuestionID: sf.scaleID, ParticipantID: other, Answer: []int{5}})
	cleared, clearErr := sf.repos.Survey.DeleteResponses(t.Context(), spec.SurveyParticipantKey{BoardID: sf.boardID(), SurveyID: sf.surveyID, ParticipantID: sf.facilitator()})
	left, listErr := sf.repos.Survey.ListResponsesForSurvey(t.Context(), sf.key())

	// then
	require.ErrorIs(t, duplicateErr, dao.ErrConflict)
	require.NoError(t, clearErr)
	assert.Equal(t, int64(2), cleared)
	require.NoError(t, listErr)
	require.Len(t, left, 1)
	assert.Equal(t, new(other), left[0].ParticipantID)
	assert.Equal(t, []int{2}, left[0].Answer)
}

func TestSurvey_TheSchemaRefusesAnEmptyAnswer(t *testing.T) {
	// given
	sf := newSurveyFixture(t)

	// when
	err := sf.repos.Survey.AddResponse(t.Context(), spec.NewSurveyResponse{OrgID: sf.orgID(), BoardID: sf.boardID(), SurveyID: sf.surveyID, QuestionID: sf.scaleID, ParticipantID: sf.facilitator(), Answer: []int{}})

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "survey_responses_answer_array")
}

func TestSurvey_HandOverResponsesMovesTheGuestsAnswersAndTheAccountsOwnWin(t *testing.T) {
	// given a guest who answered both questions, and an account seat that answered only the scale
	sf := newSurveyFixture(t)
	device, _ := daotest.CreateGuest(t, sf.repos)
	header, err := sf.repos.Board.GetHeader(t.Context(), sf.boardID())
	require.NoError(t, err)
	guest := daotest.CreateGuestParticipant(t, sf.repos, *header, device.ID)
	seat := sf.secondParticipant(t)
	sf.answer(t, guest.ID, sf.choiceID, 1)
	sf.answer(t, guest.ID, sf.scaleID, 1)
	sf.answer(t, seat, sf.scaleID, 5)

	// when
	moved, err := sf.repos.Survey.HandOverResponses(t.Context(), spec.ParticipantHandover{BoardID: sf.boardID(), FromParticipantID: guest.ID, ToParticipantID: seat})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), moved)
	responses, err := sf.repos.Survey.ListResponses(t.Context(), sf.boardID())
	require.NoError(t, err)
	require.Len(t, responses, 2)
	answers := map[uuid.UUID][]int{}
	for _, response := range responses {
		assert.Equal(t, new(seat), response.ParticipantID)
		answers[response.QuestionID] = response.Answer
	}
	assert.Equal(t, map[uuid.UUID][]int{sf.choiceID: {1}, sf.scaleID: {5}}, answers)
}

func TestSurvey_SeverResponsesKeepsTheAnswersAndDropsWhoGaveThem(t *testing.T) {
	// given
	sf := newSurveyFixture(t)
	sf.answer(t, sf.facilitator(), sf.scaleID, 4)
	sf.answer(t, sf.secondParticipant(t), sf.scaleID, 2)

	// when
	severed, err := sf.repos.Survey.SeverResponses(t.Context(), sf.boardID())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), severed)
	responses, err := sf.repos.Survey.ListResponses(t.Context(), sf.boardID())
	require.NoError(t, err)
	require.Len(t, responses, 2)
	for _, response := range responses {
		assert.Nil(t, response.ParticipantID)
	}
}

func TestBoard_CloseBoardClosesOpenSurveysAndSeversAnAnonymousBoardsReactionsAndAnswers(t *testing.T) {
	// given an open survey with an answer and a reaction on a card
	sf := newSurveyFixture(t)
	card := sf.addCard(t, sf.column(0), "a0")
	sf.react(t, card.ID, sf.facilitator(), model.ReactionParty)
	sf.answer(t, sf.facilitator(), sf.scaleID, 4)

	// when the board ends and severs its authorship
	_, err := sf.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: sf.write(t), Sever: true})

	// then
	require.NoError(t, err)
	survey, err := sf.repos.Survey.Get(t.Context(), sf.key())
	require.NoError(t, err)
	assert.Equal(t, model.SurveyClosed, survey.Status)
	reactions, err := sf.repos.CardReaction.List(t.Context(), sf.boardID())
	require.NoError(t, err)
	require.Len(t, reactions, 1)
	assert.Nil(t, reactions[0].ParticipantID)
	responses, err := sf.repos.Survey.ListResponses(t.Context(), sf.boardID())
	require.NoError(t, err)
	require.Len(t, responses, 1)
	assert.Nil(t, responses[0].ParticipantID)
}

func TestBoard_GetSnapshotLoadsReactionsSurveysAndAnswers(t *testing.T) {
	// given
	sf := newSurveyFixture(t)
	card := sf.addCard(t, sf.column(0), "a0")
	sf.react(t, card.ID, sf.facilitator(), model.ReactionThinking)
	sf.answer(t, sf.facilitator(), sf.scaleID, 3)

	// when
	snapshot, err := sf.repos.Board.GetSnapshot(t.Context(), sf.boardFixture.key())

	// then
	require.NoError(t, err)
	require.Len(t, snapshot.Reactions, 1)
	assert.Equal(t, model.ReactionThinking, snapshot.Reactions[0].Reaction)
	require.Len(t, snapshot.Surveys, 1)
	assert.Len(t, snapshot.Surveys[0].Questions, 2)
	require.Len(t, snapshot.SurveyResponses, 1)
	assert.Equal(t, []int{3}, snapshot.SurveyResponses[0].Answer)
}
