package arch

import (
	"go/ast"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	exemptNames = map[string]struct{}{
		"init":     {},
		"main":     {},
		"TestMain": {},
	}

	interfaceMethods = map[string]string{
		"internal/cache/engines/observability/valkey.go:Hook.Receive":       "valkeyhook.Hook requires Receive, and a subscription is neither traced nor timed",
		"internal/cache/engines/observability/valkey.go:Hook.DoStream":      "valkeyhook.Hook requires DoStream, and a stream is neither traced nor timed",
		"internal/cache/engines/observability/valkey.go:Hook.DoMultiStream": "valkeyhook.Hook requires DoMultiStream, and a stream is neither traced nor timed",
		"internal/entitlements/errors.go:DeniedError.Error":                 "error requires Error, and the refusal names the plan, the limit and both counts",
		"internal/board/live/errors.go:RetryError.Error":                    "error requires Error, and the wait rides beside the wrapped refusal",
	}
)

func TestNoForwardingFunctions(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := forwardingOffenders(files, interfaceMethods)

	// then
	assert.Empty(t, offenders, "these functions are empty or their whole body is a single call with no function literal in it, or are methods whose whole body is a single call; delete them and call the target directly, or allowlist a method that satisfies an interface:\n%s", strings.Join(offenders, "\n"))
}

func TestForwardingOffenders_FlagsASingleCallBody(t *testing.T) {
	tests := []struct {
		name string
		decl string
		want int
	}{
		{name: "a call that forwards its own parameters", decl: "func reload(page string) {\n\trefresh(page)\n}", want: 1},
		{name: "a call around another call", decl: "func genQueries(handle *sql.DB, tx []*sql.Tx) *sqlcgen.Queries {\n\treturn sqlcgen.New(db.TxOrDB(handle, tx))\n}", want: 1},
		{name: "an empty body", decl: "func nothing() {\n}", want: 1},
		{name: "a call around a function literal", decl: "func positive() any {\n\treturn mock.MatchedBy(func(n int) bool {\n\t\treturn n > 0\n\t})\n}", want: 0},
		{name: "two steps", decl: "func load(key string) (string, error) {\n\tvalue, err := read(key)\n\n\treturn value, err\n}", want: 0},
		{name: "a composite literal", decl: "func newThing() *thing {\n\treturn &thing{}\n}", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			file := parseFixture(t, "internal/fixture/fixture.go", "package fixture\n\n"+tc.decl+"\n")

			// when
			offenders := forwardingOffenders([]sourceFile{file}, nil)

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func forwardingOffenders(files []sourceFile, allowlist map[string]string) []string {
	matched := make(map[string]struct{})

	var offenders []string
	for _, file := range files {
		for _, decl := range file.syntax.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if forwards(fn) {
				offenders = append(offenders, file.offence(fn, "func "+fn.Name.Name+" is empty or its whole body is a single call"))

				continue
			}

			if !singleCallMethod(fn) {
				continue
			}

			method := receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name
			key := file.path + ":" + method
			if _, allowed := allowlist[key]; allowed {
				matched[key] = struct{}{}

				continue
			}

			offenders = append(offenders, file.offence(fn, "method "+method+" is a single call; call the target directly, or allowlist "+key+" if it satisfies an interface"))
		}
	}

	for _, key := range slices.Sorted(maps.Keys(allowlist)) {
		if _, ok := matched[key]; !ok {
			offenders = append(offenders, key+": allowlisted ("+allowlist[key]+") but matches no single-call method; remove the entry")
		}
	}

	return offenders
}

func forwards(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil {
		return false
	}

	if _, exempt := exemptNames[fn.Name.Name]; exempt {
		return false
	}

	if len(fn.Body.List) == 0 {
		return true
	}

	if len(fn.Body.List) != 1 {
		return false
	}

	call := onlyCall(fn.Body.List[0])

	return call != nil && !holdsFuncLit(call)
}

func holdsFuncLit(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(child ast.Node) bool {
		if _, ok := child.(*ast.FuncLit); ok {
			found = true
		}

		return !found
	})

	return found
}

func singleCallMethod(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Body == nil || len(fn.Body.List) != 1 {
		return false
	}

	return onlyCall(fn.Body.List[0]) != nil
}

func onlyCall(stmt ast.Stmt) *ast.CallExpr {
	switch typed := stmt.(type) {
	case *ast.ExprStmt:
		call, _ := typed.X.(*ast.CallExpr)
		return call

	case *ast.ReturnStmt:
		if len(typed.Results) != 1 {
			return nil
		}

		call, _ := typed.Results[0].(*ast.CallExpr)
		return call
	}

	return nil
}

func receiverName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiverName(typed.X)

	case *ast.ParenExpr:
		return receiverName(typed.X)

	case *ast.IndexExpr:
		return receiverName(typed.X)

	case *ast.IndexListExpr:
		return receiverName(typed.X)

	case *ast.Ident:
		return typed.Name
	}

	return ""
}
