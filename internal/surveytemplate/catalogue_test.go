package surveytemplate_test

import (
	"testing"

	"retrotide/internal/board/surveys"
	"retrotide/internal/model"
	"retrotide/internal/surveytemplate"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAll_HoldsTheFiveSurveyTemplatesInOrder(t *testing.T) {
	// when
	templates := surveytemplate.All()

	// then
	keys := make([]string, 0, len(templates))
	for _, template := range templates {
		keys = append(keys, template.Key)
	}

	assert.Equal(t, []string{"health_check", "sprint_confidence", "roti", "team_mood", "retro_focus"}, keys)
}

func TestAll_EveryTemplatePassesTheSurveyRules(t *testing.T) {
	for _, template := range surveytemplate.All() {
		t.Run(template.Key, func(t *testing.T) {
			// given
			drafts := make([]surveys.QuestionDraft, 0, len(template.Questions))
			for _, question := range template.Questions {
				drafts = append(drafts, surveys.QuestionDraft{
					ID:       uuid.New(),
					Kind:     question.Kind,
					Prompt:   question.Prompt,
					Options:  question.Options,
					ScaleMin: question.ScaleMin,
					ScaleMax: question.ScaleMax,
				})
			}

			// when
			title, titleErr := surveys.CleanTitle(template.Title)
			cleaned, questionsErr := surveys.CleanQuestions(drafts)

			// then
			require.NoError(t, titleErr)
			require.NoError(t, questionsErr)
			assert.Equal(t, template.Title, title)
			assert.Len(t, cleaned, len(template.Questions))
			assert.NotEmpty(t, template.Description)
			assert.Contains(t, []string{surveytemplate.CategoryHealthCheck, surveytemplate.CategorySurvey}, template.Category)
		})
	}
}

func TestLookup_TheHealthCheckAsksEightQuestionsOnAOneToFiveScale(t *testing.T) {
	// when
	template, ok := surveytemplate.Lookup("health_check")

	// then
	require.True(t, ok)
	assert.Equal(t, "Team health check", template.Title)
	assert.Equal(t, surveytemplate.CategoryHealthCheck, template.Category)
	require.Len(t, template.Questions, 8)
	for _, question := range template.Questions {
		assert.Equal(t, model.QuestionScale, question.Kind)
		assert.Equal(t, new(1), question.ScaleMin)
		assert.Equal(t, new(5), question.ScaleMax)
		assert.Empty(t, question.Options)
	}
}

func TestLookup_TheChoiceTemplatesOfferTheirOptions(t *testing.T) {
	tests := []struct {
		key     string
		kind    model.QuestionKind
		options []string
	}{
		{key: "team_mood", kind: model.QuestionSingleChoice, options: []string{"Great", "Good", "Okay", "Not great", "Bad"}},
		{key: "retro_focus", kind: model.QuestionMultipleChoice, options: []string{"Quality", "Speed", "Collaboration", "Learning", "Process", "Tooling"}},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			// when
			template, ok := surveytemplate.Lookup(tc.key)

			// then
			require.True(t, ok)
			require.Len(t, template.Questions, 1)
			assert.Equal(t, tc.kind, template.Questions[0].Kind)
			assert.Equal(t, tc.options, template.Questions[0].Options)
			assert.Nil(t, template.Questions[0].ScaleMin)
		})
	}
}

func TestLookup_AnUnknownKeyFindsNothing(t *testing.T) {
	for _, key := range []string{"", "Health_Check", "no-such-template"} {
		// when
		template, ok := surveytemplate.Lookup(key)

		// then
		assert.False(t, ok, key)
		assert.Equal(t, surveytemplate.Template{}, template)
	}
}

func TestLookup_ACallerCannotChangeTheCatalogue(t *testing.T) {
	// given
	template, ok := surveytemplate.Lookup("team_mood")
	require.True(t, ok)

	// when
	template.Questions[0].Options[0] = "Changed"
	template.Questions[0].Prompt = "Changed"

	// then
	again, ok := surveytemplate.Lookup("team_mood")
	require.True(t, ok)
	assert.Equal(t, "Great", again.Questions[0].Options[0])
	assert.Equal(t, "How are you feeling about the last sprint?", again.Questions[0].Prompt)
}

func TestAll_ACallerCannotChangeTheCatalogue(t *testing.T) {
	// given
	templates := surveytemplate.All()

	// when
	templates[0].Title = "Changed"
	*templates[0].Questions[0].ScaleMax = 9

	// then
	again := surveytemplate.All()
	assert.Equal(t, "Team health check", again[0].Title)
	assert.Equal(t, new(5), again[0].Questions[0].ScaleMax)
}
