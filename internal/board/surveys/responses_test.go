package surveys

import (
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func respondCommand(answers ...spec.SurveyAnswer) RespondCommand {
	return RespondCommand{Command: baseCommand(), SurveyID: surveyID, Answers: answers}
}

func TestRespond_AParticipantAnswersAnOpenSurveyAndOnlyTheCountGoesOut(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(openSurvey(), nil).Once()
	responses := append(answersFrom(myID, 4, 1), answersFrom(theirID, 2, 0)...)
	m.surveys.EXPECT().Respond(mock.Anything, spec.SurveyResponseWrite{
		BoardWrite:    target(),
		SurveyID:      surveyID,
		ParticipantID: myID,
		Answers: []spec.SurveyAnswer{
			{QuestionID: paceID, Answer: []int{4}},
			{QuestionID: focusID, Answer: []int{0, 2}},
		},
	}).Return(&model.Versioned[model.SurveyState]{Value: model.SurveyState{Survey: *openSurvey(), Responses: responses}, Version: 8}, nil).Once()

	// when
	got, err := svc.Respond(t.Context(), respondCommand(
		spec.SurveyAnswer{QuestionID: paceID, Answer: []int{4}},
		spec.SurveyAnswer{QuestionID: focusID, Answer: []int{2, 0}},
	))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Survey)
	assert.Equal(t, map[uuid.UUID][]int{paceID: {4}, moodID: {1}}, got.Survey.MyAnswers)
	assert.Equal(t, 2, got.Survey.Respondents)
	assert.Nil(t, got.Survey.Questions[0].Results)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	upserted := upsertedFor(t, event)
	assert.Equal(t, 2, upserted.Survey.Respondents)
	assert.Empty(t, upserted.Survey.MyAnswers)
	for _, question := range upserted.Survey.Questions {
		assert.Nil(t, question.Results, "results stay hidden while the survey is open")
	}
	payload, ok := event.PayloadFor(projection.Viewer{ParticipantID: new(myID)})
	require.True(t, ok)
	mine, ok := payload.(dto.SurveyUpsertedEvent)
	require.True(t, ok)
	assert.Equal(t, map[uuid.UUID][]int{paceID: {4}, moodID: {1}}, mine.Survey.MyAnswers)
	assert.Nil(t, mine.Survey.Questions[0].Results)
}

func TestRespond_AGuestWithASeatAnswersAnOpenSurvey(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, asGuest))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(openSurvey(), nil).Once()
	m.surveys.EXPECT().Respond(mock.Anything, spec.SurveyResponseWrite{
		BoardWrite:    target(),
		SurveyID:      surveyID,
		ParticipantID: myID,
		Answers:       []spec.SurveyAnswer{{QuestionID: moodID, Answer: []int{2}}},
	}).Return(&model.Versioned[model.SurveyState]{Value: model.SurveyState{Survey: *openSurvey(), Responses: answersFrom(myID, 3, 2)}, Version: 8}, nil).Once()

	// when
	got, err := svc.Respond(t.Context(), RespondCommand{
		Command:  board.Command{Principal: model.Principal{Kind: model.PrincipalGuest, ID: guestDevice}, BoardID: boardID, OpID: opID},
		SurveyID: surveyID,
		Answers:  []spec.SurveyAnswer{{QuestionID: moodID, Answer: []int{2}}},
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.Len(t, m.events.published, 1)
}

func TestRespond_AClosedSurveyTakesNoMoreAnswers(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(closedSurvey(), nil).Once()

	// when
	_, err := svc.Respond(t.Context(), respondCommand(spec.SurveyAnswer{QuestionID: paceID, Answer: []int{3}}))

	// then the strict survey mock proves nothing was written
	require.ErrorIs(t, err, policy.ErrSurveyClosed)
	assert.Empty(t, m.events.published)
}

func TestRespond_SomeoneWithoutASeatCannotAnswer(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, notJoined))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(openSurvey(), nil).Once()

	// when
	_, err := svc.Respond(t.Context(), respondCommand(spec.SurveyAnswer{QuestionID: paceID, Answer: []int{3}}))

	// then
	require.ErrorIs(t, err, policy.ErrNotParticipant)
	assert.Empty(t, m.events.published)
}

func TestRespond_AMissingSurveyIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(nil, errNotFound).Once()

	// when
	_, err := svc.Respond(t.Context(), respondCommand(spec.SurveyAnswer{QuestionID: paceID, Answer: []int{3}}))

	// then
	require.ErrorIs(t, err, policy.ErrSurveyNotFound)
}

func TestRespond_NoAnswersIsRefusedBeforeTouchingTheBoard(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	_, err := svc.Respond(t.Context(), respondCommand())

	// then
	require.ErrorIs(t, err, ErrAnswersRequired)
}

func TestRespond_AnAnswerThatDoesNotFitIsRefused(t *testing.T) {
	tests := []struct {
		name   string
		answer spec.SurveyAnswer
		want   error
	}{
		{name: "a question from elsewhere", answer: spec.SurveyAnswer{QuestionID: uuid.New(), Answer: []int{1}}, want: ErrUnknownQuestion},
		{name: "a score above the scale", answer: spec.SurveyAnswer{QuestionID: paceID, Answer: []int{6}}, want: ErrInvalidAnswer},
		{name: "two picks for a single choice", answer: spec.SurveyAnswer{QuestionID: moodID, Answer: []int{0, 1}}, want: ErrInvalidAnswer},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer))
			m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(openSurvey(), nil).Once()

			// when
			_, err := svc.Respond(t.Context(), respondCommand(tc.answer))

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestCheckAnswers_AcceptsAnswersThatFitTheirQuestions(t *testing.T) {
	tests := []struct {
		name   string
		answer spec.SurveyAnswer
		want   []int
	}{
		{name: "the lowest score", answer: spec.SurveyAnswer{QuestionID: paceID, Answer: []int{1}}, want: []int{1}},
		{name: "the highest score", answer: spec.SurveyAnswer{QuestionID: paceID, Answer: []int{5}}, want: []int{5}},
		{name: "one pick", answer: spec.SurveyAnswer{QuestionID: moodID, Answer: []int{2}}, want: []int{2}},
		{name: "every option, in any order", answer: spec.SurveyAnswer{QuestionID: focusID, Answer: []int{2, 0, 1}}, want: []int{0, 1, 2}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := checkAnswers(*openSurvey(), []spec.SurveyAnswer{tc.answer})

			// then
			require.NoError(t, err)
			assert.Equal(t, []spec.SurveyAnswer{{QuestionID: tc.answer.QuestionID, Answer: tc.want}}, got)
		})
	}
}

func TestCheckAnswers_RefusesAnswersThatDoNotFit(t *testing.T) {
	tests := []struct {
		name    string
		answers []spec.SurveyAnswer
		want    error
	}{
		{name: "nothing", answers: nil, want: ErrAnswersRequired},
		{name: "a question twice", answers: []spec.SurveyAnswer{{QuestionID: paceID, Answer: []int{1}}, {QuestionID: paceID, Answer: []int{2}}}, want: ErrUnknownQuestion},
		{name: "another survey's question", answers: []spec.SurveyAnswer{{QuestionID: uuid.New(), Answer: []int{1}}}, want: ErrUnknownQuestion},
		{name: "a score below the scale", answers: []spec.SurveyAnswer{{QuestionID: paceID, Answer: []int{0}}}, want: ErrInvalidAnswer},
		{name: "two scores", answers: []spec.SurveyAnswer{{QuestionID: paceID, Answer: []int{2, 3}}}, want: ErrInvalidAnswer},
		{name: "no score", answers: []spec.SurveyAnswer{{QuestionID: paceID, Answer: []int{}}}, want: ErrInvalidAnswer},
		{name: "a pick past the options", answers: []spec.SurveyAnswer{{QuestionID: moodID, Answer: []int{3}}}, want: ErrInvalidAnswer},
		{name: "a negative pick", answers: []spec.SurveyAnswer{{QuestionID: moodID, Answer: []int{-1}}}, want: ErrInvalidAnswer},
		{name: "no picks of several", answers: []spec.SurveyAnswer{{QuestionID: focusID, Answer: []int{}}}, want: ErrInvalidAnswer},
		{name: "the same pick twice", answers: []spec.SurveyAnswer{{QuestionID: focusID, Answer: []int{1, 1}}}, want: ErrInvalidAnswer},
		{name: "more picks than options", answers: []spec.SurveyAnswer{{QuestionID: focusID, Answer: []int{0, 1, 2, 2}}}, want: ErrInvalidAnswer},
		{name: "one of several picks out of range", answers: []spec.SurveyAnswer{{QuestionID: focusID, Answer: []int{0, 3}}}, want: ErrInvalidAnswer},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := checkAnswers(*openSurvey(), tc.answers)

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}
