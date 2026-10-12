package engines

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"retrotide/internal/dto"
	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines/utils"
	"retrotide/internal/model"
)

type (
	Markdown struct{}

	markdownCard struct {
		Card   dto.ExportCard
		Indent int
		Merged bool
	}
)

const (
	markdownSpecial = "\\`*_{}[]()#+-.!|~=$"

	markdownCardTemplate = `{{define "card" -}}
{{repeat " " .Indent}}- {{if .Merged}}` + utils.MergedPrefix + `{{end}}{{para .Card.Body (add .Indent 2)}}{{votes .Card.Votes}}{{by .Card.Author}}{{if .Card.Hidden}}` + utils.HiddenMark + `{{end}}
{{range .Card.Comments -}}
{{repeat " " (add $.Indent 2)}}- Comment{{by .Author}}: {{para .Body (add $.Indent 4)}}
{{end -}}
{{range .Card.ActionItems -}}
{{repeat " " (add $.Indent 2)}}- {{action .}}
{{end -}}
{{range .Card.MergedCards -}}
{{template "card" (stacked . (add $.Indent 2))}}
{{- end -}}
{{end}}`

	markdownBoardTemplate = `# {{line .Title}}
{{with .Context -}}
{{"\n"}}{{para . 0}}
{{end -}}
{{range .Columns -}}
{{"\n"}}## {{line .Title}}
{{with .Description -}}
{{"\n"}}{{para . 0}}
{{end -}}
{{if .Cards -}}
{{"\n"}}
{{- range .Cards -}}
{{template "card" (top .)}}
{{- end -}}
{{end -}}
{{with .HiddenCount -}}
{{"\n"}}_{{hidden .}}_
{{end -}}
{{if not (or .Cards .HiddenCount) -}}
{{"\n"}}_No cards_
{{end -}}
{{end -}}
{{with .OtherActions -}}
{{"\n"}}## ` + utils.OtherActionsTitle + `
{{"\n"}}
{{- range . -}}
- {{action .}}
{{end -}}
{{end -}}`
)

var (
	markdownFuncs = template.FuncMap{
		"line":    markdownLine,
		"para":    markdownParagraph,
		"votes":   utils.VotesText,
		"by":      markdownBy,
		"hidden":  utils.HiddenText,
		"action":  markdownAction,
		"repeat":  strings.Repeat,
		"add":     addInts,
		"top":     topCard,
		"stacked": stackedCard,
	}

	markdownTemplate = template.Must(template.New("board").Funcs(markdownFuncs).Parse(markdownCardTemplate + markdownBoardTemplate))
)

func (Markdown) Format() engine.Format {
	return engine.FormatMarkdown
}

func (Markdown) ContentType() string {
	return "text/markdown; charset=utf-8"
}

func (Markdown) Extension() string {
	return "md"
}

func (Markdown) WriteBoard(w io.Writer, board dto.BoardExport) error {
	if err := markdownTemplate.Execute(w, board); err != nil {
		return fmt.Errorf("write the Markdown export: %w", err)
	}

	return nil
}

func markdownLine(text string) string {
	collapsed := strings.Join(strings.Fields(text), " ")

	return markdownEscape(collapsed)
}

func markdownParagraph(text string, indent int) string {
	normalised := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")

	var lines []string
	for line := range strings.SplitSeq(normalised, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		lines = append(lines, markdownEscape(trimmed))
	}

	return strings.Join(lines, "\\\n"+strings.Repeat(" ", indent))
}

func markdownBy(author string) string {
	if author == "" {
		return ""
	}

	return " by " + markdownLine(author)
}

func markdownAction(item dto.ExportActionItem) string {
	box := "[ ]"
	if item.Status == model.ActionItemDone {
		box = "[x]"
	}

	return box + " " + markdownLine(item.Title) + " " + utils.ActionDetails(item, markdownLine(item.Assignee))
}

func markdownEscape(text string) string {
	var escaped strings.Builder
	for _, r := range text {
		switch {
		case r == '<':
			escaped.WriteString("&lt;")
		case r == '>':
			escaped.WriteString("&gt;")
		case r == '&':
			escaped.WriteString("&amp;")
		case strings.ContainsRune(markdownSpecial, r):
			escaped.WriteByte('\\')
			escaped.WriteRune(r)
		default:
			escaped.WriteRune(r)
		}
	}

	return escaped.String()
}

func addInts(a, b int) int {
	return a + b
}

func topCard(card dto.ExportCard) markdownCard {
	return markdownCard{Card: card}
}

func stackedCard(card dto.ExportCard, indent int) markdownCard {
	return markdownCard{Card: card, Indent: indent, Merged: true}
}
