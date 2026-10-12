package projection_test

import (
	"fmt"
	"testing"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	healthSurveyID = uuid.MustParse("3e4f5a6b-7c8d-4e9f-8a0b-1c2d3e4f5a6b")
	goalQuestionID = uuid.MustParse("4f5a6b7c-8d9e-4fa0-9b1c-2d3e4f5a6b7c")
	paceQuestionID = uuid.MustParse("5a6b7c8d-9eaf-4b1c-8d2e-3f4a5b6c7d8e")

	openQuestionsGolden = `[
		{"id":"4f5a6b7c-8d9e-4fa0-9b1c-2d3e4f5a6b7c","kind":"single_choice","prompt":"Did we hit the sprint goal?","options":["Yes","No","Partly"],"scale_min":null,"scale_max":null,"results":null},
		{"id":"5a6b7c8d-9eaf-4b1c-8d2e-3f4a5b6c7d8e","kind":"scale","prompt":"How sustainable was the pace?","options":[],"scale_min":1,"scale_max":5,"results":null}
	]`
)

func healthSurvey(status model.SurveyStatus) model.SurveyState {
	survey := model.Survey{
		ID:        healthSurveyID,
		OrgID:     orgID,
		BoardID:   boardID,
		Title:     "Sprint health",
		Status:    status,
		CreatedAt: at("2026-09-22T10:10:00Z"),
		Questions: []model.SurveyQuestion{
			{ID: goalQuestionID, SurveyID: healthSurveyID, Position: 0, Kind: model.QuestionSingleChoice, Prompt: "Did we hit the sprint goal?", Options: []string{"Yes", "No", "Partly"}},
			{ID: paceQuestionID, SurveyID: healthSurveyID, Position: 1, Kind: model.QuestionScale, Prompt: "How sustainable was the pace?", ScaleMin: new(1), ScaleMax: new(5)},
		},
	}

	if status == model.SurveyClosed {
		survey.ClosedAt = new(at("2026-09-22T10:20:00Z"))
	}

	return model.SurveyState{
		Survey: survey,
		Responses: []model.SurveyResponse{
			{ID: uuid.New(), SurveyID: healthSurveyID, QuestionID: goalQuestionID, ParticipantID: &samID, Answer: []int{0}},
			{ID: uuid.New(), SurveyID: healthSurveyID, QuestionID: paceQuestionID, ParticipantID: &samID, Answer: []int{4}},
			{ID: uuid.New(), SurveyID: healthSurveyID, QuestionID: goalQuestionID, ParticipantID: &aliID, Answer: []int{2}},
			{ID: uuid.New(), SurveyID: healthSurveyID, QuestionID: paceQuestionID, ParticipantID: &aliID, Answer: []int{5}},
			{ID: uuid.New(), SurveyID: healthSurveyID, QuestionID: goalQuestionID, ParticipantID: &guestID, Answer: []int{0}},
			{ID: uuid.New(), SurveyID: healthSurveyID, QuestionID: paceQuestionID, Answer: []int{5}},
		},
	}
}

func ownAnswers(tc viewerCase) map[string]any {
	if tc.viewer.ParticipantID == nil {
		return map[string]any{}
	}

	switch *tc.viewer.ParticipantID {
	case samID:
		return map[string]any{goalQuestionID.String(): []any{float64(0)}, paceQuestionID.String(): []any{float64(4)}}
	case aliID:
		return map[string]any{goalQuestionID.String(): []any{float64(2)}, paceQuestionID.String(): []any{float64(5)}}
	case guestID:
		return map[string]any{goalQuestionID.String(): []any{float64(0)}}
	default:
		return map[string]any{}
	}
}

func assertNoRespondentIDs(t *testing.T, payload string) {
	t.Helper()

	for _, participant := range []uuid.UUID{samID, aliID, guestID, hostID} {
		assert.NotContains(t, payload, participant.String(), "a survey payload never names who answered")
	}
}

func TestSurveyViewFor_GoldenAnOpenSurveyHidesResultsFromAFacilitatorToo(t *testing.T) {
	// given
	state := healthSurvey(model.SurveyOpen)
	facilitator := viewerCases()[3].viewer

	// when
	payload := marshal(t, projection.SurveyViewFor(state, facilitator))

	// then only the respondent count leaves, never a tally
	assert.JSONEq(t, `{"id":"3e4f5a6b-7c8d-4e9f-8a0b-1c2d3e4f5a6b","title":"Sprint health","template_key":"","status":"open",
		"questions":`+openQuestionsGolden+`,"respondents":3,"my_answers":{}}`, payload)
}

func TestSurveyViewFor_OpenResultsAreNilForEveryViewer(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// when
			view := projection.SurveyViewFor(healthSurvey(model.SurveyOpen), tc.viewer)

			// then
			for _, question := range view.Questions {
				assert.Nil(t, question.Results)
			}
		})
	}
}

func TestSurveyViewFor_GoldenAClosedSurveyShowsCountsAndTheRoundedAverage(t *testing.T) {
	// given
	state := healthSurvey(model.SurveyClosed)

	// when
	payload := marshal(t, projection.SurveyViewFor(state, viewerCases()[1].viewer))

	// then the scale counts start at its minimum and the average keeps two places
	assert.JSONEq(t, `{"id":"3e4f5a6b-7c8d-4e9f-8a0b-1c2d3e4f5a6b","title":"Sprint health","template_key":"","status":"closed",
		"questions":[
			{"id":"4f5a6b7c-8d9e-4fa0-9b1c-2d3e4f5a6b7c","kind":"single_choice","prompt":"Did we hit the sprint goal?","options":["Yes","No","Partly"],"scale_min":null,"scale_max":null,
				"results":{"counts":[2,0,1],"responses":3,"average":null}},
			{"id":"5a6b7c8d-9eaf-4b1c-8d2e-3f4a5b6c7d8e","kind":"scale","prompt":"How sustainable was the pace?","options":[],"scale_min":1,"scale_max":5,
				"results":{"counts":[0,0,0,1,2],"responses":3,"average":4.67}}
		],
		"respondents":3,
		"my_answers":{"4f5a6b7c-8d9e-4fa0-9b1c-2d3e4f5a6b7c":[2],"5a6b7c8d-9eaf-4b1c-8d2e-3f4a5b6c7d8e":[5]}}`, payload)
}

func TestSurveyViewFor_MyAnswersOnlyEverGoToTheirOwner(t *testing.T) {
	for _, status := range []model.SurveyStatus{model.SurveyOpen, model.SurveyClosed} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", status, tc.name), func(t *testing.T) {
				// when
				payload := marshal(t, projection.SurveyViewFor(healthSurvey(status), tc.viewer))

				// then
				assert.Equal(t, ownAnswers(tc), decode(t, payload)["my_answers"])
				assertNoRespondentIDs(t, payload)
			})
		}
	}
}

func TestSurveyViewFor_AfterSeveringNobodyHasAnswersButTheCountsSurvive(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given an anonymous board ended, so every answer lost its seat
			state := healthSurvey(model.SurveyClosed)
			for i := range state.Responses {
				state.Responses[i].ParticipantID = nil
			}

			// when
			view := projection.SurveyViewFor(state, tc.viewer)

			// then
			assert.Empty(t, view.MyAnswers)
			assert.NotNil(t, view.MyAnswers)
			assert.Equal(t, 3, view.Respondents)
			require.NotNil(t, view.Questions[1].Results)
			assert.Equal(t, []int{0, 0, 0, 1, 2}, view.Questions[1].Results.Counts)
		})
	}
}

func TestSurveyUpserted_GoldenEveryoneGetsTheCountAndOnlyTheirOwnAnswers(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given
			state := healthSurvey(model.SurveyOpen)
			event := projection.SurveyUpserted(boardState(model.AnonymityAnonymous, true).Header.Board, state, 1044)

			// when
			payload, seen := seenPayload(t, event, tc.viewer)

			// then
			require.True(t, seen)
			decoded := decode(t, payload)
			assert.Equal(t, "survey_upserted", decoded["type"])
			survey, ok := decoded["survey"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, float64(3), survey["respondents"])
			assert.Equal(t, ownAnswers(tc), survey["my_answers"])
			assertNoRespondentIDs(t, payload)
		})
	}
}

func TestSurveyUpserted_GoldenTheSharedPayloadHasNoAnswers(t *testing.T) {
	// given
	event := projection.SurveyUpserted(boardState(model.AnonymityNamed, false).Header.Board, healthSurvey(model.SurveyOpen), 1044)

	// when
	payload := marshal(t, event.Payload(projection.ClassOthers))

	// then
	assert.JSONEq(t, `{"type":"survey_upserted","version":1044,"survey":{"id":"3e4f5a6b-7c8d-4e9f-8a0b-1c2d3e4f5a6b","title":"Sprint health",
		"template_key":"","status":"open","questions":`+openQuestionsGolden+`,"respondents":3,"my_answers":{}}}`, payload)
}

func TestSurveyRemoved_GoldenReachesEveryone(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// when
			payload, seen := seenPayload(t, projection.SurveyRemoved(healthSurveyID, 1044), tc.viewer)

			// then
			require.True(t, seen)
			assert.JSONEq(t, `{"type":"survey_removed","version":1044,"survey_id":"3e4f5a6b-7c8d-4e9f-8a0b-1c2d3e4f5a6b"}`, payload)
		})
	}
}

func TestSurveyResponse_GoldenCarriesTheRespondentsOwnAnswers(t *testing.T) {
	// given
	state := healthSurvey(model.SurveyOpen)

	// when
	payload := marshal(t, projection.SurveyResponse(boardState(model.AnonymityNamed, false).Header.Board, state, viewerCases()[0].viewer, 1044))

	// then
	assert.JSONEq(t, `{"version":1044,"survey":{"id":"3e4f5a6b-7c8d-4e9f-8a0b-1c2d3e4f5a6b","title":"Sprint health","template_key":"","status":"open",
		"questions":`+openQuestionsGolden+`,"respondents":3,
		"my_answers":{"4f5a6b7c-8d9e-4fa0-9b1c-2d3e4f5a6b7c":[0],"5a6b7c8d-9eaf-4b1c-8d2e-3f4a5b6c7d8e":[4]}}}`, payload)
}

func TestSnapshot_SurveysCarryOnlyTheViewersOwnAnswersAndNoRespondentIDs(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given
				state := boardState(anonymity, false)
				survey := healthSurvey(model.SurveyOpen)
				state.Surveys = []model.Survey{survey.Survey}
				state.SurveyResponses = survey.Responses

				// when
				payload := marshal(t, projection.Snapshot(state, tc.viewer))

				// then
				surveys, ok := decode(t, payload)["surveys"].([]any)
				require.True(t, ok)
				require.Len(t, surveys, 1)

				view, ok := surveys[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, ownAnswers(tc), view["my_answers"])
				assert.Equal(t, float64(3), view["respondents"])
				assertNoRespondentIDs(t, marshal(t, surveys))

				for _, raw := range view["questions"].([]any) {
					question, ok := raw.(map[string]any)
					require.True(t, ok)
					assert.Nil(t, question["results"], "an open survey shows no results in the snapshot either")
				}
			})
		}
	}
}

func TestSnapshot_SurveysAreNeverNull(t *testing.T) {
	// given
	state := boardState(model.AnonymityNamed, false)

	// when
	payload := marshal(t, projection.Snapshot(state, viewerCases()[0].viewer))

	// then
	assert.Equal(t, []any{}, decode(t, payload)["surveys"])
}
