package surveytemplate

import (
	"slices"

	"retrotide/internal/model"
)

type (
	Question struct {
		Kind     model.QuestionKind
		Prompt   string
		Options  []string
		ScaleMin *int
		ScaleMax *int
	}

	Template struct {
		Key         string
		Title       string
		Description string
		Category    string
		Questions   []Question
	}
)

const (
	CategoryHealthCheck = "health_check"
	CategorySurvey      = "survey"
)

var (
	catalogue = []Template{
		{
			Key:         "health_check",
			Title:       "Team health check",
			Description: "Eight quick ratings of how the team is doing, from 1 (struggling) to 5 (thriving).",
			Category:    CategoryHealthCheck,
			Questions: []Question{
				{Kind: model.QuestionScale, Prompt: "Delivering value: how well does our work give users what they need?", ScaleMin: new(1), ScaleMax: new(5)},
				{Kind: model.QuestionScale, Prompt: "Easy to release: how easy is it to get our work into users' hands?", ScaleMin: new(1), ScaleMax: new(5)},
				{Kind: model.QuestionScale, Prompt: "Fun: how much do you enjoy working on this team?", ScaleMin: new(1), ScaleMax: new(5)},
				{Kind: model.QuestionScale, Prompt: "Health of the code: how healthy is the code we work in?", ScaleMin: new(1), ScaleMax: new(5)},
				{Kind: model.QuestionScale, Prompt: "Learning: how much are we learning as a team?", ScaleMin: new(1), ScaleMax: new(5)},
				{Kind: model.QuestionScale, Prompt: "Mission: how clear and motivating is our purpose?", ScaleMin: new(1), ScaleMax: new(5)},
				{Kind: model.QuestionScale, Prompt: "Speed: how quickly do we get things done?", ScaleMin: new(1), ScaleMax: new(5)},
				{Kind: model.QuestionScale, Prompt: "Support: how well do we get help when we ask for it?", ScaleMin: new(1), ScaleMax: new(5)},
			},
		},
		{
			Key:         "sprint_confidence",
			Title:       "Sprint confidence",
			Description: "One rating of how sure the team feels about meeting the sprint goal.",
			Category:    CategoryHealthCheck,
			Questions: []Question{
				{Kind: model.QuestionScale, Prompt: "How confident are you that we will meet the sprint goal?", ScaleMin: new(1), ScaleMax: new(5)},
			},
		},
		{
			Key:         "roti",
			Title:       "Return on time invested",
			Description: "Ask whether this meeting was worth the time, from 1 (wasted) to 5 (well worth it).",
			Category:    CategorySurvey,
			Questions: []Question{
				{Kind: model.QuestionScale, Prompt: "Was this retro worth the time you put into it?", ScaleMin: new(1), ScaleMax: new(5)},
			},
		},
		{
			Key:         "team_mood",
			Title:       "Team mood",
			Description: "A single pick of how everyone is feeling.",
			Category:    CategorySurvey,
			Questions: []Question{
				{Kind: model.QuestionSingleChoice, Prompt: "How are you feeling about the last sprint?", Options: []string{"Great", "Good", "Okay", "Not great", "Bad"}},
			},
		},
		{
			Key:         "retro_focus",
			Title:       "Retro focus",
			Description: "Let everyone pick the areas this retro should spend its time on.",
			Category:    CategorySurvey,
			Questions: []Question{
				{Kind: model.QuestionMultipleChoice, Prompt: "Which areas should this retro focus on?", Options: []string{"Quality", "Speed", "Collaboration", "Learning", "Process", "Tooling"}},
			},
		},
	}
)

func Lookup(key string) (Template, bool) {
	for _, template := range catalogue {
		if template.Key == key {
			return copied(template), true
		}
	}

	return Template{}, false
}

func All() []Template {
	templates := make([]Template, 0, len(catalogue))
	for _, template := range catalogue {
		templates = append(templates, copied(template))
	}

	return templates
}

func copied(template Template) Template {
	questions := make([]Question, 0, len(template.Questions))
	for _, question := range template.Questions {
		question.Options = slices.Clone(question.Options)
		question.ScaleMin = copyInt(question.ScaleMin)
		question.ScaleMax = copyInt(question.ScaleMax)
		questions = append(questions, question)
	}

	template.Questions = questions

	return template
}

func copyInt(value *int) *int {
	if value == nil {
		return nil
	}

	return new(*value)
}
