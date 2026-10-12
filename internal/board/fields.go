package board

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	placed struct {
		id       uuid.UUID
		position string
	}

	placement struct {
		position string
		rewrite  []spec.PositionChange
	}
)

const (
	ContextMaxRunes           = 500
	ColumnDescriptionMaxRunes = 200
)

func cleanTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", ErrTitleRequired
	}

	if utf8.RuneCountInString(title) > model.BoardTitleMaxRunes {
		return "", ErrTitleTooLong
	}

	return title, nil
}

func cleanContext(raw string) (string, error) {
	line := strings.TrimSpace(raw)
	if utf8.RuneCountInString(line) > ContextMaxRunes {
		return "", ErrContextTooLong
	}

	return line, nil
}

func cleanAnonymity(raw model.Anonymity) (model.Anonymity, error) {
	if raw == "" {
		return model.AnonymityNamed, nil
	}

	if raw != model.AnonymityNamed && raw != model.AnonymityAnonymous {
		return "", ErrInvalidAnonymity
	}

	return raw, nil
}

func CleanColumn(raw ColumnFields) (ColumnFields, error) {
	title := strings.TrimSpace(raw.Title)
	if title == "" {
		return ColumnFields{}, ErrColumnTitleRequired
	}

	if utf8.RuneCountInString(title) > model.BoardColumnTitleMaxRunes {
		return ColumnFields{}, ErrColumnTitleTooLong
	}

	description := strings.TrimSpace(raw.Description)
	if utf8.RuneCountInString(description) > ColumnDescriptionMaxRunes {
		return ColumnFields{}, ErrColumnDescriptionTooLong
	}

	colour := raw.Colour
	if colour == "" {
		colour = model.ColourGrey
	}

	if !slices.Contains(model.Colours, colour) {
		return ColumnFields{}, ErrInvalidColour
	}

	return ColumnFields{Title: title, Description: description, Colour: colour}, nil
}

func CleanCardBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", ErrCardBodyRequired
	}

	if utf8.RuneCountInString(body) > model.CardBodyMaxRunes {
		return "", ErrCardBodyTooLong
	}

	return body, nil
}

func checkColour(colour *model.Colour) error {
	if colour != nil && !slices.Contains(model.Colours, *colour) {
		return ErrInvalidColour
	}

	return nil
}

func lastPosition(siblings []placed) string {
	if len(siblings) == 0 {
		return ""
	}

	return siblings[len(siblings)-1].position
}

func appendKey(siblings []placed) (string, error) {
	key, err := fracindex.Between(lastPosition(siblings), "")
	if err != nil {
		return "", fmt.Errorf("place after the last item: %w", err)
	}

	return key, nil
}

func anchorIndex(siblings []placed, after *uuid.UUID) int {
	if after == nil {
		return 0
	}

	at := slices.IndexFunc(siblings, func(sibling placed) bool {
		return sibling.id == *after
	})
	if at < 0 {
		return len(siblings)
	}

	return at + 1
}

func place(siblings []placed, after *uuid.UUID) placement {
	at := anchorIndex(siblings, after)

	lower := ""
	if at > 0 {
		lower = siblings[at-1].position
	}

	upper := ""
	if at < len(siblings) {
		upper = siblings[at].position
	}

	key, err := fracindex.Between(lower, upper)
	if err == nil && len(key) <= fracindex.MaxKeyLength {
		return placement{position: key}
	}

	keys := fracindex.Spread(len(siblings) + 1)
	rewrite := make([]spec.PositionChange, 0, len(siblings))
	for i, sibling := range siblings {
		key := keys[i]
		if i >= at {
			key = keys[i+1]
		}

		rewrite = append(rewrite, spec.PositionChange{ID: sibling.id, Position: key})
	}

	return placement{position: keys[at], rewrite: rewrite}
}
