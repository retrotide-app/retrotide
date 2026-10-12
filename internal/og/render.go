package og

import (
	"bytes"
	"html"
	"regexp"
	"strings"
)

type (
	rewrite struct {
		pattern *regexp.Regexp
		pick    func(Meta) string
	}
)

const (
	baseURLPlaceholder = "__BASE_URL__"
)

var (
	titleTag = regexp.MustCompile(`(?s)<title>.*?</title>`)

	rewrites = []rewrite{
		{pattern: regexp.MustCompile(`(<meta\s+name="description"\s+content=")[^"]*(")`), pick: func(m Meta) string { return m.Description }},
		{pattern: regexp.MustCompile(`(<meta\s+name="robots"\s+content=")[^"]*(")`), pick: func(m Meta) string { return m.Robots }},
		{pattern: regexp.MustCompile(`(<link\s+rel="canonical"\s+href=")[^"]*(")`), pick: func(m Meta) string { return m.Canonical }},
		{pattern: regexp.MustCompile(`(<meta\s+property="og:url"\s+content=")[^"]*(")`), pick: func(m Meta) string { return m.Canonical }},
		{pattern: regexp.MustCompile(`(<meta\s+property="og:title"\s+content=")[^"]*(")`), pick: func(m Meta) string { return m.Title }},
		{pattern: regexp.MustCompile(`(<meta\s+property="og:description"\s+content=")[^"]*(")`), pick: func(m Meta) string { return m.Description }},
		{pattern: regexp.MustCompile(`(<meta\s+name="twitter:title"\s+content=")[^"]*(")`), pick: func(m Meta) string { return m.Title }},
		{pattern: regexp.MustCompile(`(<meta\s+name="twitter:description"\s+content=")[^"]*(")`), pick: func(m Meta) string { return m.Description }},
	}
)

func Render(shell []byte, meta Meta) []byte {
	rendered := bytes.ReplaceAll(shell, []byte(baseURLPlaceholder), []byte(html.EscapeString(meta.BaseURL)))
	if meta.Title != "" {
		rendered = titleTag.ReplaceAllLiteral(rendered, []byte("<title>"+html.EscapeString(meta.Title)+"</title>"))
	}

	for _, tag := range rewrites {
		value := tag.pick(meta)
		if value == "" {
			continue
		}

		literal := strings.ReplaceAll(html.EscapeString(value), "$", "$$")
		rendered = tag.pattern.ReplaceAll(rendered, []byte("${1}"+literal+"${2}"))
	}

	return rendered
}
