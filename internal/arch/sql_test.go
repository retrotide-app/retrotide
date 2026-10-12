package arch

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	sqlStatement struct {
		name    string
		pattern *regexp.Regexp
	}
)

var (
	sqlStatements = []sqlStatement{
		{name: "SELECT", pattern: regexp.MustCompile(`\bSELECT\b`)},
		{name: "INSERT INTO", pattern: regexp.MustCompile(`\bINSERT\s+INTO\b`)},
		{name: "UPDATE ... SET", pattern: regexp.MustCompile(`(?s)\bUPDATE\b.*\bSET\b`)},
		{name: "DELETE FROM", pattern: regexp.MustCompile(`\bDELETE\s+FROM\b`)},
	}

	transactionOpeners = map[string]struct{}{
		"WithTx":     {},
		"WithReadTx": {},
	}
)

func TestNoSQLOutsideDAO(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders, err := sqlOffenders(files)
	require.NoError(t, err)

	// then
	assert.Empty(t, offenders, "these non-test files outside internal/dao and internal/db hold SQL; move the statement into a DAO under internal/dao:\n%s", strings.Join(offenders, "\n"))
}

func TestSQLOffenders_FlagsAStatementSplitAcrossLines(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		rule      string
	}{
		{name: "an update with its set on the next line", statement: "UPDATE site_settings\nSET value = $1", rule: "UPDATE ... SET"},
		{name: "an insert with its into on the next line", statement: "INSERT\nINTO site_settings (key) VALUES ($1)", rule: "INSERT INTO"},
		{name: "a delete with its from on the next line", statement: "DELETE\nFROM site_settings", rule: "DELETE FROM"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the usual multi-line raw string, outside internal/dao
			file := parseFixture(t, "internal/board/fixture.go", "package board\n\nconst query = `"+tc.statement+"`\n")

			// when
			offenders, err := sqlOffenders([]sourceFile{file})

			// then
			require.NoError(t, err)
			assert.Equal(t, []string{"internal/board/fixture.go:3: string literal holds " + tc.rule}, offenders)
		})
	}
}

func TestNoTransactionsAboveRepository(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := transactionOffenders(files)

	// then
	assert.Empty(t, offenders, "these non-test files outside internal/repository, internal/dao and internal/db open a transaction; ask the repository for a composite method instead:\n%s", strings.Join(offenders, "\n"))
}

func sqlOffenders(files []sourceFile) ([]string, error) {
	var offenders []string
	var errs []error
	for _, file := range files {
		if file.test || within(file.dir, "internal/dao", "internal/db") {
			continue
		}

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}

			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", file.offence(lit, "unquote string literal"), err))

				return true
			}

			for _, statement := range sqlStatements {
				if statement.pattern.MatchString(value) {
					offenders = append(offenders, file.offence(lit, "string literal holds "+statement.name))
				}
			}

			return true
		})
	}

	return offenders, errors.Join(errs...)
}

func transactionOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if file.test || within(file.dir, "internal/repository", "internal/dao", "internal/db") {
			continue
		}

		alias, importsDB := file.localName(dbImport)

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if selector.Sel.Name == "BeginTx" {
				offenders = append(offenders, file.offence(call, "calls BeginTx"))

				return true
			}

			pkg, ok := selector.X.(*ast.Ident)
			if !ok || !importsDB || pkg.Name != alias {
				return true
			}

			if _, opens := transactionOpeners[selector.Sel.Name]; opens {
				offenders = append(offenders, file.offence(call, "calls "+alias+"."+selector.Sel.Name))
			}

			return true
		})
	}

	return offenders
}
