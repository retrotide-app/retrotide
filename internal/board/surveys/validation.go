package surveys

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"retrotide/internal/model"
)

func CleanTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", ErrTitleRequired
	}

	if utf8.RuneCountInString(title) > model.SurveyTitleMaxRunes {
		return "", ErrTitleTooLong
	}

	return title, nil
}

func CleanQuestions(drafts []QuestionDraft) ([]QuestionDraft, error) {
	if len(drafts) == 0 {
		return nil, ErrQuestionsRequired
	}

	if len(drafts) > model.SurveyMaxQuestions {
		return nil, ErrTooManyQuestions
	}

	cleaned := make([]QuestionDraft, 0, len(drafts))
	for position, draft := range drafts {
		question, err := cleanQuestion(draft)
		if err != nil {
			return nil, fmt.Errorf("question %d: %w", position+1, err)
		}

		cleaned = append(cleaned, question)
	}

	return cleaned, nil
}

func cleanQuestion(draft QuestionDraft) (QuestionDraft, error) {
	prompt := strings.TrimSpace(draft.Prompt)
	if prompt == "" {
		return QuestionDraft{}, ErrPromptRequired
	}

	if utf8.RuneCountInString(prompt) > model.SurveyPromptMaxRunes {
		return QuestionDraft{}, ErrPromptTooLong
	}

	switch draft.Kind {
	case model.QuestionScale:
		if len(draft.Options) > 0 {
			return QuestionDraft{}, ErrInvalidOptions
		}

		if err := checkScale(draft.ScaleMin, draft.ScaleMax); err != nil {
			return QuestionDraft{}, err
		}

		return QuestionDraft{ID: draft.ID, Kind: draft.Kind, Prompt: prompt, Options: []string{}, ScaleMin: new(*draft.ScaleMin), ScaleMax: new(*draft.ScaleMax)}, nil
	case model.QuestionSingleChoice, model.QuestionMultipleChoice:
		if draft.ScaleMin != nil || draft.ScaleMax != nil {
			return QuestionDraft{}, ErrInvalidScale
		}

		options, err := cleanOptions(draft.Options)
		if err != nil {
			return QuestionDraft{}, err
		}

		return QuestionDraft{ID: draft.ID, Kind: draft.Kind, Prompt: prompt, Options: options}, nil
	default:
		return QuestionDraft{}, fmt.Errorf("%w: %q", ErrInvalidQuestionKind, draft.Kind)
	}
}

func cleanOptions(raw []string) ([]string, error) {
	if len(raw) < model.SurveyMinOptions || len(raw) > model.SurveyMaxOptions {
		return nil, fmt.Errorf("%w: %d options", ErrInvalidOptions, len(raw))
	}

	options := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, option := range raw {
		text := strings.TrimSpace(option)
		if text == "" || utf8.RuneCountInString(text) > model.SurveyOptionMaxRunes {
			return nil, ErrInvalidOptions
		}

		folded := strings.ToLower(text)
		if _, repeated := seen[folded]; repeated {
			return nil, fmt.Errorf("%w: %q appears twice", ErrInvalidOptions, text)
		}

		seen[folded] = struct{}{}
		options = append(options, text)
	}

	return options, nil
}

func checkScale(low, high *int) error {
	if low == nil || high == nil {
		return ErrInvalidScale
	}

	if *low < model.SurveyScaleFloor || *high > model.SurveyScaleCeiling || *low >= *high {
		return fmt.Errorf("%w: %d to %d", ErrInvalidScale, *low, *high)
	}

	return nil
}
