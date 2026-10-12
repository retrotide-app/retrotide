package projection

import (
	"math"
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

const (
	EventSurveyUpserted = "survey_upserted"
	EventSurveyRemoved  = "survey_removed"
)

func SurveyViewFor(state model.SurveyState, viewer Viewer) dto.SurveyView {
	survey := state.Survey

	answered := map[uuid.UUID][]model.SurveyResponse{}
	mine := map[uuid.UUID][]int{}
	for _, response := range state.Responses {
		answered[response.QuestionID] = append(answered[response.QuestionID], response)

		if viewer.ParticipantID != nil && response.ParticipantID != nil && *response.ParticipantID == *viewer.ParticipantID {
			mine[response.QuestionID] = slices.Clone(response.Answer)
		}
	}

	respondents := 0
	questions := make([]dto.SurveyQuestionView, 0, len(survey.Questions))
	for _, question := range survey.Questions {
		responses := answered[question.ID]
		respondents = max(respondents, len(responses))

		view := dto.SurveyQuestionView{
			ID:       question.ID,
			Kind:     question.Kind,
			Prompt:   question.Prompt,
			Options:  append([]string{}, question.Options...),
			ScaleMin: copyInt(question.ScaleMin),
			ScaleMax: copyInt(question.ScaleMax),
		}

		if survey.Status == model.SurveyClosed {
			view.Results = surveyResults(question, responses)
		}

		questions = append(questions, view)
	}

	return dto.SurveyView{
		ID:          survey.ID,
		Title:       survey.Title,
		TemplateKey: survey.TemplateKey,
		Status:      survey.Status,
		Questions:   questions,
		Respondents: respondents,
		MyAnswers:   mine,
	}
}

func SurveyUpserted(board model.Board, state model.SurveyState, version int64) Event {
	payload := dto.SurveyUpsertedEvent{
		BoardEventHead: head(EventSurveyUpserted, version),
		Survey:         SurveyViewFor(state, Viewer{}),
	}

	event := everyone(payload, version)

	event.forViewer = func(viewer Viewer) any {
		view := SurveyViewFor(state, viewer)
		if len(view.MyAnswers) == 0 {
			return nil
		}

		return dto.SurveyUpsertedEvent{BoardEventHead: head(EventSurveyUpserted, version), Survey: view}
	}

	return event
}

func SurveyRemoved(surveyID uuid.UUID, version int64) Event {
	payload := dto.SurveyRemovedEvent{
		BoardEventHead: head(EventSurveyRemoved, version),
		SurveyID:       surveyID,
	}

	return everyone(payload, version)
}

func SurveyResponse(board model.Board, state model.SurveyState, viewer Viewer, version int64) dto.BoardCommandResponse {
	view := SurveyViewFor(state, viewer)

	return dto.BoardCommandResponse{Version: version, Survey: &view}
}

func surveyViews(surveys []model.Survey, responses []model.SurveyResponse, viewer Viewer) []dto.SurveyView {
	bySurvey := map[uuid.UUID][]model.SurveyResponse{}
	for _, response := range responses {
		bySurvey[response.SurveyID] = append(bySurvey[response.SurveyID], response)
	}

	views := make([]dto.SurveyView, 0, len(surveys))
	for _, survey := range surveys {
		views = append(views, SurveyViewFor(model.SurveyState{Survey: survey, Responses: bySurvey[survey.ID]}, viewer))
	}

	return views
}

func surveyResults(question model.SurveyQuestion, responses []model.SurveyResponse) *dto.SurveyResultsView {
	if question.Kind == model.QuestionScale {
		return scaleResults(question, responses)
	}

	counts := make([]int, len(question.Options))
	for _, response := range responses {
		for _, picked := range response.Answer {
			if picked < 0 || picked >= len(counts) {
				continue
			}

			counts[picked]++
		}
	}

	return &dto.SurveyResultsView{Counts: counts, Responses: len(responses)}
}

func scaleResults(question model.SurveyQuestion, responses []model.SurveyResponse) *dto.SurveyResultsView {
	if question.ScaleMin == nil || question.ScaleMax == nil || *question.ScaleMax < *question.ScaleMin {
		return &dto.SurveyResultsView{Counts: []int{}, Responses: len(responses)}
	}

	floor := *question.ScaleMin
	counts := make([]int, *question.ScaleMax-floor+1)

	sum := 0
	scored := 0
	for _, response := range responses {
		if len(response.Answer) == 0 {
			continue
		}

		value := response.Answer[0]
		if value < floor || value-floor >= len(counts) {
			continue
		}

		counts[value-floor]++
		sum += value
		scored++
	}

	results := &dto.SurveyResultsView{Counts: counts, Responses: len(responses)}
	if scored > 0 {
		results.Average = new(math.Round(float64(sum)/float64(scored)*100) / 100)
	}

	return results
}

func copyInt(value *int) *int {
	if value == nil {
		return nil
	}

	return new(*value)
}
