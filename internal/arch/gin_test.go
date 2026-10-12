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

type (
	ginScope struct {
		file    sourceFile
		context string
		body    *ast.BlockStmt
	}
)

const (
	ginImport = "github.com/gin-gonic/gin"
)

var (
	middlewareAnswers = map[string]struct{}{
		"AbortWithError":  {},
		"AbortWithStatus": {},
		"AsciiJSON":       {},
		"BSON":            {},
		"Data":            {},
		"DataFromReader":  {},
		"File":            {},
		"FileAttachment":  {},
		"FileFromFS":      {},
		"HTML":            {},
		"IndentedJSON":    {},
		"JSON":            {},
		"JSONP":           {},
		"Negotiate":       {},
		"ProtoBuf":        {},
		"PureJSON":        {},
		"Redirect":        {},
		"Render":          {},
		"SSEvent":         {},
		"SecureJSON":      {},
		"Status":          {},
		"Stream":          {},
		"String":          {},
		"TOML":            {},
		"XML":             {},
		"YAML":            {},
	}

	writerAnswers = map[string]struct{}{
		"Flush":          {},
		"Write":          {},
		"WriteHeader":    {},
		"WriteHeaderNow": {},
		"WriteString":    {},
	}

	bodyReaders = map[string]struct{}{
		"Bind":           {},
		"BindJSON":       {},
		"ShouldBind":     {},
		"ShouldBindJSON": {},
	}

	serviceSuffixes = []string{"Service", "Svc"}

	middlewareAnswerExceptions = map[string]string{
		"internal/middleware/dronebl.go:c.Data": "SPEC 6.4 answers a blocked browser navigation with an HTML page, written only after c.Abort()",
	}
)

func TestMiddlewareAnswersOnlyByAborting(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := middlewareAnswerOffenders(files, middlewareAnswerExceptions)

	// then
	assert.Empty(t, offenders, "these middleware answer through their *gin.Context without aborting, so Gin would still run the rest of the chain; answer with c.AbortWithStatusJSON, or through the writer the middleware wraps:\n%s", strings.Join(offenders, "\n"))
}

func TestMiddlewareAnswerOffenders_FlagsEveryWayToAnswer(t *testing.T) {
	tests := []struct {
		call string
		want int
	}{
		{call: `c.AbortWithError(http.StatusBadRequest, err)`, want: 1},
		{call: `c.AbortWithStatus(http.StatusForbidden)`, want: 1},
		{call: `c.AsciiJSON(http.StatusOK, body)`, want: 1},
		{call: `c.BSON(http.StatusOK, body)`, want: 1},
		{call: `c.Data(http.StatusOK, "text/plain", nil)`, want: 1},
		{call: `c.DataFromReader(http.StatusOK, 0, "text/plain", reader, nil)`, want: 1},
		{call: `c.File("index.html")`, want: 1},
		{call: `c.FileAttachment("export.csv", "export.csv")`, want: 1},
		{call: `c.FileFromFS("index.html", files)`, want: 1},
		{call: `c.HTML(http.StatusOK, "page", nil)`, want: 1},
		{call: `c.IndentedJSON(http.StatusOK, body)`, want: 1},
		{call: `c.JSON(http.StatusOK, body)`, want: 1},
		{call: `c.JSONP(http.StatusOK, body)`, want: 1},
		{call: `c.Negotiate(http.StatusOK, offer)`, want: 1},
		{call: `c.ProtoBuf(http.StatusOK, body)`, want: 1},
		{call: `c.PureJSON(http.StatusOK, body)`, want: 1},
		{call: `c.Redirect(http.StatusFound, "/")`, want: 1},
		{call: `c.Render(http.StatusOK, view)`, want: 1},
		{call: `c.SSEvent("tick", body)`, want: 1},
		{call: `c.SecureJSON(http.StatusOK, body)`, want: 1},
		{call: `c.Status(http.StatusNoContent)`, want: 1},
		{call: `c.Stream(step)`, want: 1},
		{call: `c.String(http.StatusOK, "ok")`, want: 1},
		{call: `c.TOML(http.StatusOK, body)`, want: 1},
		{call: `c.XML(http.StatusOK, body)`, want: 1},
		{call: `c.YAML(http.StatusOK, body)`, want: 1},
		{call: `c.Writer.Flush()`, want: 1},
		{call: `c.Writer.Write(nil)`, want: 1},
		{call: `c.Writer.WriteHeader(http.StatusOK)`, want: 1},
		{call: `c.Writer.WriteHeaderNow()`, want: 1},
		{call: `c.Writer.WriteString("ok")`, want: 1},
		{call: `c.AbortWithStatusJSON(http.StatusForbidden, body)`, want: 0},
		{call: `c.Writer.Header().Set("X-Frame-Options", "DENY")`, want: 0},
		{call: `c.Error(err)`, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.call, func(t *testing.T) {
			// given
			file := parseFixture(t, "internal/middleware/fixture.go", "package middleware\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc guard(c *gin.Context) {\n\t"+tc.call+"\n}\n")

			// when
			offenders := middlewareAnswerOffenders([]sourceFile{file}, nil)

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func TestMiddlewareAnswerOffenders_HonoursOnlyANamedException(t *testing.T) {
	tests := []struct {
		name       string
		exceptions map[string]string
		want       []string
	}{
		{name: "the named file and call", exceptions: map[string]string{"internal/middleware/fixture.go:c.Data": "a page"}, want: nil},
		{name: "another call in the same file", exceptions: map[string]string{"internal/middleware/fixture.go:c.String": "a page"}, want: []string{"internal/middleware/fixture.go:6: c.Data answers without aborting", "internal/middleware/fixture.go:c.String: excepted (a page) but matches no answer; remove the entry"}},
		{name: "the same call in another file", exceptions: map[string]string{"internal/middleware/other.go:c.Data": "a page"}, want: []string{"internal/middleware/fixture.go:6: c.Data answers without aborting", "internal/middleware/other.go:c.Data: excepted (a page) but matches no answer; remove the entry"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			file := parseFixture(t, "internal/middleware/fixture.go", "package middleware\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc guard(c *gin.Context) {\n\tc.Data(403, \"text/html\", nil)\n}\n")

			// when
			offenders := middlewareAnswerOffenders([]sourceFile{file}, tc.exceptions)

			// then
			assert.Equal(t, tc.want, offenders)
		})
	}
}

func TestNoGinBindingThatReadsTheBodyDirectly(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := bodyReaderOffenders(files)

	// then
	assert.Empty(t, offenders, "these calls read the request body directly or answer an empty 400 on failure; bind through utils.BindJSON, or c.ShouldBindBodyWithJSON in a guard that peeks at the body:\n%s", strings.Join(offenders, "\n"))
}

func TestGinContextNeverLeavesTheRequest(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := escapingContextOffenders(files)

	// then
	assert.Empty(t, offenders, "these hand a *gin.Context to a goroutine or a service, which outlives the pooled context; pass c.Request.Context(), or copy the values and use context.WithoutCancel(c.Request.Context()):\n%s", strings.Join(offenders, "\n"))
}

func TestNoGinContextCopy(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := contextCopyOffenders(files)

	// then
	assert.Empty(t, offenders, "these call c.Copy(), whose copy shares c.Request, so on a GET its context still ends with the request; copy the values needed and use context.WithoutCancel(c.Request.Context()):\n%s", strings.Join(offenders, "\n"))
}

func middlewareAnswerOffenders(files []sourceFile, exceptions map[string]string) []string {
	matched := make(map[string]struct{})

	var offenders []string
	for _, scope := range ginScopes(files) {
		if scope.file.test || !within(scope.file.dir, "internal/middleware") {
			continue
		}

		for _, node := range scope.nodes() {
			if method, ok := contextCall(node, scope.context); ok {
				if _, answers := middlewareAnswers[method]; !answers {
					continue
				}

				call := scope.context + "." + method
				key := scope.file.path + ":" + call
				if _, excepted := exceptions[key]; excepted {
					matched[key] = struct{}{}

					continue
				}

				offenders = append(offenders, scope.file.offence(node, call+" answers without aborting"))

				continue
			}

			if method, ok := writerCall(node, scope.context); ok {
				if _, answers := writerAnswers[method]; answers {
					offenders = append(offenders, scope.file.offence(node, scope.context+".Writer."+method+" answers without aborting"))
				}
			}
		}
	}

	for _, key := range slices.Sorted(maps.Keys(exceptions)) {
		if _, ok := matched[key]; !ok {
			offenders = append(offenders, key+": excepted ("+exceptions[key]+") but matches no answer; remove the entry")
		}
	}

	return offenders
}

func bodyReaderOffenders(files []sourceFile) []string {
	var offenders []string
	for _, scope := range ginScopes(files) {
		for _, node := range scope.nodes() {
			method, ok := contextCall(node, scope.context)
			if !ok {
				continue
			}

			if _, reads := bodyReaders[method]; reads {
				offenders = append(offenders, scope.file.offence(node, scope.context+"."+method+" reads the body outside utils.BindJSON"))
			}
		}
	}

	return offenders
}

func escapingContextOffenders(files []sourceFile) []string {
	var offenders []string
	for _, scope := range ginScopes(files) {
		if scope.file.test || !within(scope.file.dir, "internal/controllers", "internal/middleware") {
			continue
		}

		for _, node := range scope.nodes() {
			switch typed := node.(type) {
			case *ast.GoStmt:
				if mentions(typed.Call, scope.context) {
					offenders = append(offenders, scope.file.offence(typed, "go statement captures "+scope.context))
				}

			case *ast.CallExpr:
				receiver, ok := serviceReceiver(typed)
				if ok && passes(typed, scope.context) {
					offenders = append(offenders, scope.file.offence(typed, "call on "+receiver+" receives "+scope.context))
				}
			}
		}
	}

	return offenders
}

func contextCopyOffenders(files []sourceFile) []string {
	var offenders []string
	for _, scope := range ginScopes(files) {
		for _, node := range scope.nodes() {
			method, ok := contextCall(node, scope.context)
			if ok && method == "Copy" {
				offenders = append(offenders, scope.file.offence(node, scope.context+".Copy() shares the request"))
			}
		}
	}

	return offenders
}

func ginScopes(files []sourceFile) []ginScope {
	var scopes []ginScope
	for _, file := range files {
		alias, ok := file.localName(ginImport)
		if !ok {
			continue
		}

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			var fnType *ast.FuncType
			var body *ast.BlockStmt
			switch typed := node.(type) {
			case *ast.FuncDecl:
				fnType, body = typed.Type, typed.Body

			case *ast.FuncLit:
				fnType, body = typed.Type, typed.Body

			default:
				return true
			}

			if body == nil || fnType.Params == nil {
				return true
			}

			for _, field := range fnType.Params.List {
				if !isGinContext(field.Type, alias) {
					continue
				}

				for _, name := range field.Names {
					if name.Name != "_" {
						scopes = append(scopes, ginScope{file: file, context: name.Name, body: body})
					}
				}
			}

			return true
		})
	}

	return scopes
}

func isGinContext(expr ast.Expr, alias string) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}

	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Context" {
		return false
	}

	pkg, ok := selector.X.(*ast.Ident)

	return ok && pkg.Name == alias
}

func (s ginScope) nodes() []ast.Node {
	var nodes []ast.Node
	ast.Inspect(s.body, func(node ast.Node) bool {
		if node == nil {
			return false
		}

		if lit, ok := node.(*ast.FuncLit); ok && declares(lit.Type.Params, s.context) {
			return false
		}

		nodes = append(nodes, node)

		return true
	})

	return nodes
}

func declares(params *ast.FieldList, name string) bool {
	if params == nil {
		return false
	}

	for _, field := range params.List {
		for _, ident := range field.Names {
			if ident.Name == name {
				return true
			}
		}
	}

	return false
}

func contextCall(node ast.Node, context string) (string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return "", false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	receiver, ok := selector.X.(*ast.Ident)
	if !ok || receiver.Name != context {
		return "", false
	}

	return selector.Sel.Name, true
}

func writerCall(node ast.Node, context string) (string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return "", false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	writer, ok := selector.X.(*ast.SelectorExpr)
	if !ok || writer.Sel.Name != "Writer" {
		return "", false
	}

	receiver, ok := writer.X.(*ast.Ident)
	if !ok || receiver.Name != context {
		return "", false
	}

	return selector.Sel.Name, true
}

func serviceReceiver(call *ast.CallExpr) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	var name string
	switch receiver := selector.X.(type) {
	case *ast.Ident:
		name = receiver.Name

	case *ast.SelectorExpr:
		name = receiver.Sel.Name

	default:
		return "", false
	}

	for _, suffix := range serviceSuffixes {
		if strings.HasSuffix(name, suffix) {
			return name, true
		}
	}

	return "", false
}

func passes(call *ast.CallExpr, context string) bool {
	for _, arg := range call.Args {
		ident, ok := arg.(*ast.Ident)
		if ok && ident.Name == context {
			return true
		}
	}

	return false
}

func mentions(node ast.Node, name string) bool {
	var found bool
	ast.Inspect(node, func(child ast.Node) bool {
		if found {
			return false
		}

		switch typed := child.(type) {
		case *ast.SelectorExpr:
			found = mentions(typed.X, name)

			return false

		case *ast.Ident:
			found = typed.Name == name
		}

		return true
	})

	return found
}
