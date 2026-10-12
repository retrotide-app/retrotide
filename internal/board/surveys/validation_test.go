package surveys

import (
	"strings"
	"testing"

	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanTitle_TrimsATitleWithinTheLimit(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "  Sprint health  ", want: "Sprint health"},
		{raw: strings.Repeat("é", model.SurveyTitleMaxRunes), want: strings.Repeat("é", model.SurveyTitleMaxRunes)},
	}

	for _, tc := range tests {
		// when
		got, err := CleanTitle(tc.raw)

		// then
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}

func TestCleanTitle_RefusesAMissingOrLongTitle(t *testing.T) {
	tests := []struct {
		raw  string
		want error
	}{
		{raw: "", want: ErrTitleRequired},
		{raw: " \t ", want: ErrTitleRequired},
		{raw: strings.Repeat("a", model.SurveyTitleMaxRunes+1), want: ErrTitleTooLong},
	}

	for _, tc := range tests {
		// when
		_, err := CleanTitle(tc.raw)

		// then
		require.ErrorIs(t, err, tc.want)
	}
}

func TestCleanQuestions_TrimsEveryQuestionAndKeepsItsOrder(t *testing.T) {
	// given
	drafts := []QuestionDraft{
		{ID: paceID, Kind: model.QuestionScale, Prompt: "  How sustainable was the pace?  ", ScaleMin: new(0), ScaleMax: new(10)},
		{ID: moodID, Kind: model.QuestionSingleChoice, Prompt: "How do you feel?", Options: []string{"  Good ", "Okay"}},
		{ID: focusID, Kind: model.QuestionMultipleChoice, Prompt: "Focus?", Options: []string{"Quality", "Speed", "Process"}},
	}

	// when
	got, err := CleanQuestions(drafts)

	// then
	require.NoError(t, err)
	assert.Equal(t, []QuestionDraft{
		{ID: paceID, Kind: model.QuestionScale, Prompt: "How sustainable was the pace?", Options: []string{}, ScaleMin: new(0), ScaleMax: new(10)},
		{ID: moodID, Kind: model.QuestionSingleChoice, Prompt: "How do you feel?", Options: []string{"Good", "Okay"}},
		{ID: focusID, Kind: model.QuestionMultipleChoice, Prompt: "Focus?", Options: []string{"Quality", "Speed", "Process"}},
	}, got)
}

func TestCleanQuestions_RefusesASurveyOutsideTheModelLimits(t *testing.T) {
	scale := QuestionDraft{ID: paceID, Kind: model.QuestionScale, Prompt: "Pace?", ScaleMin: new(1), ScaleMax: new(5)}
	choice := QuestionDraft{ID: moodID, Kind: model.QuestionSingleChoice, Prompt: "Mood?", Options: []string{"Good", "Bad"}}

	tooMany := make([]QuestionDraft, 0, model.SurveyMaxQuestions+1)
	for range model.SurveyMaxQuestions + 1 {
		tooMany = append(tooMany, scale)
	}

	elevenOptions := make([]string, 0, model.SurveyMaxOptions+1)
	for index := range model.SurveyMaxOptions + 1 {
		elevenOptions = append(elevenOptions, strings.Repeat("o", index+1))
	}

	with := func(base QuestionDraft, change func(*QuestionDraft)) []QuestionDraft {
		change(&base)

		return []QuestionDraft{base}
	}

	tests := []struct {
		name   string
		drafts []QuestionDraft
		want   error
	}{
		{name: "no questions", drafts: nil, want: ErrQuestionsRequired},
		{name: "twenty-one questions", drafts: tooMany, want: ErrTooManyQuestions},
		{name: "an unknown kind", drafts: with(scale, func(q *QuestionDraft) { q.Kind = "ranking" }), want: ErrInvalidQuestionKind},
		{name: "a blank prompt", drafts: with(scale, func(q *QuestionDraft) { q.Prompt = "   " }), want: ErrPromptRequired},
		{name: "a long prompt", drafts: with(scale, func(q *QuestionDraft) { q.Prompt = strings.Repeat("p", model.SurveyPromptMaxRunes+1) }), want: ErrPromptTooLong},
		{name: "one option", drafts: with(choice, func(q *QuestionDraft) { q.Options = []string{"Only"} }), want: ErrInvalidOptions},
		{name: "eleven options", drafts: with(choice, func(q *QuestionDraft) { q.Options = elevenOptions }), want: ErrInvalidOptions},
		{name: "a blank option", drafts: with(choice, func(q *QuestionDraft) { q.Options = []string{"Good", "  "} }), want: ErrInvalidOptions},
		{name: "a long option", drafts: with(choice, func(q *QuestionDraft) {
			q.Options = []string{"Good", strings.Repeat("o", model.SurveyOptionMaxRunes+1)}
		}), want: ErrInvalidOptions},
		{name: "the same option twice", drafts: with(choice, func(q *QuestionDraft) { q.Options = []string{"Good", " good "} }), want: ErrInvalidOptions},
		{name: "a choice with a scale", drafts: with(choice, func(q *QuestionDraft) { q.ScaleMin = new(1) }), want: ErrInvalidScale},
		{name: "a scale with options", drafts: with(scale, func(q *QuestionDraft) { q.Options = []string{"Low", "High"} }), want: ErrInvalidOptions},
		{name: "a scale with no ends", drafts: with(scale, func(q *QuestionDraft) { q.ScaleMin = nil }), want: ErrInvalidScale},
		{name: "a scale below zero", drafts: with(scale, func(q *QuestionDraft) { q.ScaleMin = new(-1) }), want: ErrInvalidScale},
		{name: "a scale past ten", drafts: with(scale, func(q *QuestionDraft) { q.ScaleMax = new(11) }), want: ErrInvalidScale},
		{name: "a scale of one value", drafts: with(scale, func(q *QuestionDraft) { q.ScaleMin = new(5) }), want: ErrInvalidScale},
		{name: "a scale upside down", drafts: with(scale, func(q *QuestionDraft) { q.ScaleMin, q.ScaleMax = new(5), new(1) }), want: ErrInvalidScale},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := CleanQuestions(tc.drafts)

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestCleanQuestions_AcceptsTheLimitsThemselves(t *testing.T) {
	// given
	tenOptions := make([]string, 0, model.SurveyMaxOptions)
	for index := range model.SurveyMaxOptions {
		tenOptions = append(tenOptions, strings.Repeat("o", index+1))
	}

	drafts := make([]QuestionDraft, 0, model.SurveyMaxQuestions)
	for range model.SurveyMaxQuestions {
		drafts = append(drafts, QuestionDraft{ID: focusID, Kind: model.QuestionMultipleChoice, Prompt: strings.Repeat("p", model.SurveyPromptMaxRunes), Options: tenOptions})
	}

	// when
	got, err := CleanQuestions(drafts)

	// then
	require.NoError(t, err)
	assert.Len(t, got, model.SurveyMaxQuestions)
}
