package main

import (
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type (
	entity struct {
		Entity string
		Table  string
		FK     string
	}

	family struct {
		Name     string
		Template string
		Suffix   string
		Entities []entity
	}

	binding struct {
		entity
		Recv    string
		FKField string
	}
)

var (
	families []family
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}

	tmplDir := filepath.Join(root, "internal", "dao", "queries", "templates")
	genDir := filepath.Join(root, "internal", "dao", "queries", "gen")

	if err := os.RemoveAll(genDir); err != nil {
		fail(err)
	}

	if err := os.MkdirAll(genDir, 0o755); err != nil {
		fail(err)
	}

	for _, old := range mustGlob(filepath.Join(root, "internal", "dao", "*_bind_gen.go")) {
		if err := os.Remove(old); err != nil {
			fail(err)
		}
	}

	total := 0
	for _, f := range families {
		sqlPath := filepath.Join(tmplDir, f.Template+".sql.tmpl")
		sqlTmpl, err := template.ParseFiles(sqlPath)
		if err != nil {
			fail(err)
		}

		bindTmpl := bindTemplate(filepath.Join(tmplDir, f.Template+"_bind.go.tmpl"))

		for _, e := range f.Entities {
			writeRendered(sqlTmpl, e, filepath.Join(genDir, f.Name+"_"+e.Table+".sql"), false)

			if bindTmpl == nil {
				continue
			}

			b := binding{
				entity:  e,
				Recv:    lowerFirst(e.Entity) + f.Suffix + "Querier",
				FKField: fkField(e.FK),
			}

			writeRendered(bindTmpl, b, filepath.Join(root, "internal", "dao", f.Name+"_"+e.Table+"_bind_gen.go"), true)
			total++
		}
	}

	fmt.Printf("expanded %d families, %d bindings\n", len(families), total)
}

func bindTemplate(path string) *template.Template {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		fail(err)
	}

	tmpl, err := template.ParseFiles(path)
	if err != nil {
		fail(err)
	}

	return tmpl
}

func writeRendered(t *template.Template, data any, path string, gofmtIt bool) {
	var out strings.Builder
	if err := t.Execute(&out, data); err != nil {
		fail(err)
	}

	body := []byte(out.String())
	if gofmtIt {
		formatted, err := format.Source(body)
		if err != nil {
			fail(fmt.Errorf("format %s: %w", path, err))
		}

		body = formatted
	}

	if err := os.WriteFile(path, body, 0o644); err != nil {
		fail(err)
	}
}

func mustGlob(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fail(err)
	}

	return matches
}

func lowerFirst(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}

func fkField(fk string) string {
	if fk == "" {
		return ""
	}

	var out strings.Builder
	for p := range strings.SplitSeq(fk, "_") {
		switch p {
		case "id":
			out.WriteString("ID")
		default:
			out.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}

	return out.String()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "expand:", err)
	os.Exit(1)
}
