package arch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	sourceFile struct {
		path    string
		dir     string
		test    bool
		fset    *token.FileSet
		syntax  *ast.File
		imports []importedPackage
	}

	importedPackage struct {
		path string
		name string
		spec *ast.ImportSpec
	}
)

const (
	repoRoot = "../.."

	rootPackage = "."
)

var (
	skippedDirs = map[string]struct{}{
		".claude":      {},
		".git":         {},
		".idea":        {},
		"build":        {},
		"data":         {},
		"design":       {},
		"dist":         {},
		"docs":         {},
		"frontend":     {},
		"node_modules": {},
		"scripts":      {},
		"static":       {},
		"vendor":       {},
	}

	loadRepo = sync.OnceValues(parseRepo)
)

func parseRepo() ([]sourceFile, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}

	fset := token.NewFileSet()

	var files []sourceFile
	walkErr := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if _, skip := skippedDirs[entry.Name()]; skip {
				return fs.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}

		syntax, parseErr := parser.ParseFile(fset, filePath, nil, parser.ParseComments|parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}

		if ast.IsGenerated(syntax) {
			return nil
		}

		rel, relErr := filepath.Rel(root, filePath)
		if relErr != nil {
			return relErr
		}

		imports, importErr := readImports(syntax)
		if importErr != nil {
			return fmt.Errorf("%s: %w", rel, importErr)
		}

		slashed := filepath.ToSlash(rel)
		files = append(files, sourceFile{
			path:    slashed,
			dir:     path.Dir(slashed),
			test:    strings.HasSuffix(slashed, "_test.go"),
			fset:    fset,
			syntax:  syntax,
			imports: imports,
		})

		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", root, walkErr)
	}

	return files, nil
}

func readImports(syntax *ast.File) ([]importedPackage, error) {
	imports := make([]importedPackage, 0, len(syntax.Imports))
	for _, spec := range syntax.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("unquote import %s: %w", spec.Path.Value, err)
		}

		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}

		imports = append(imports, importedPackage{path: importPath, name: name, spec: spec})
	}

	return imports, nil
}

func parseFixture(t *testing.T, filePath string, src string) sourceFile {
	t.Helper()

	fset := token.NewFileSet()

	syntax, err := parser.ParseFile(fset, filePath, src, parser.ParseComments|parser.SkipObjectResolution)
	require.NoError(t, err)

	imports, err := readImports(syntax)
	require.NoError(t, err)

	return sourceFile{
		path:    filePath,
		dir:     path.Dir(filePath),
		test:    strings.HasSuffix(filePath, "_test.go"),
		fset:    fset,
		syntax:  syntax,
		imports: imports,
	}
}

func (f sourceFile) localName(importPath string) (string, bool) {
	index := slices.IndexFunc(f.imports, func(imported importedPackage) bool {
		return imported.path == importPath
	})
	if index < 0 {
		return "", false
	}

	return f.imports[index].name, true
}

func (f sourceFile) offence(node ast.Node, rule string) string {
	position := f.fset.Position(node.Pos())

	return fmt.Sprintf("%s:%d: %s", f.path, position.Line, rule)
}

func within(dir string, roots ...string) bool {
	for _, root := range roots {
		if dir == root || strings.HasPrefix(dir, root+"/") {
			return true
		}
	}

	return false
}
