package user

import (
	"html"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

const (
	MaxDisplayNameRunes = 40
)

var (
	strictPolicy = sync.OnceValue(bluemonday.StrictPolicy)
)

func ClampDisplayName(raw string) string {
	stripped := raw
	for {
		next := html.UnescapeString(strictPolicy().Sanitize(stripped))
		if next == stripped {
			break
		}

		stripped = next
	}

	collapsed := strings.Join(strings.Fields(stripped), " ")

	runes := []rune(collapsed)
	if len(runes) > MaxDisplayNameRunes {
		runes = runes[:MaxDisplayNameRunes]
	}

	return strings.TrimSpace(string(runes))
}
