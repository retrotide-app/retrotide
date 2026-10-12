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

const (
	ginContribPrefix = "github.com/gin-contrib/"
	websocketImport  = "github.com/coder/websocket"
	oidcImport       = "github.com/coreos/go-oidc/v3"
	oauth2Import     = "golang.org/x/oauth2"
	dbImport         = "retrotide/internal/db"
	daoImport        = "retrotide/internal/dao"

	identityCoreDir    = "internal/identity"
	identityCoreImport = "retrotide/internal/identity"

	daoConstructorPrefix = "New"
	daoMockPrefix        = "NewMock"
)

var (
	identityAdapterLibraries = map[string][]string{
		"internal/identity/google":    {oidcImport, oauth2Import},
		"internal/identity/microsoft": {oidcImport, oauth2Import},
	}
)

func TestGinStaysInTheHTTPLayer(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := ginImportOffenders(files)

	// then
	assert.Empty(t, offenders, "these import Gin outside internal/controllers, internal/middleware, internal/routes, internal/utils and the root package; take a context.Context, an http.Handler or an io.Writer instead:\n%s", strings.Join(offenders, "\n"))
}

func TestWebsocketStaysInWS(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := websocketImportOffenders(files)

	// then
	assert.Empty(t, offenders, "these non-test files import coder/websocket outside internal/ws; mount the hub's http.Handler instead:\n%s", strings.Join(offenders, "\n"))
}

func TestProviderLibrariesStayInTheirAdapter(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := adapterLibraryOffenders(files, identityAdapterLibraries)

	// then
	assert.Empty(t, offenders, "these non-test files import a sign-in provider's library outside that provider's adapter under internal/identity; call identity.Provider instead:\n%s", strings.Join(offenders, "\n"))
}

func TestAdapterLibraryOffenders_FlagsEveryPackageButTheAdaptersThatOwnTheLibrary(t *testing.T) {
	libraries := map[string][]string{
		"internal/identity/google":  {oidcImport, oauth2Import},
		"internal/identity/example": {oauth2Import},
	}

	tests := []struct {
		name       string
		path       string
		importPath string
		want       int
	}{
		{name: "go-oidc in the identity core", path: "internal/identity/service.go", importPath: "github.com/coreos/go-oidc/v3/oidc", want: 1},
		{name: "x/oauth2 in a controller", path: "internal/controllers/identity_controller.go", importPath: "golang.org/x/oauth2", want: 1},
		{name: "an x/oauth2 subpackage", path: "internal/auth/service.go", importPath: "golang.org/x/oauth2/google", want: 1},
		{name: "go-oidc in the adapter that owns it", path: "internal/identity/google/adapter.go", importPath: "github.com/coreos/go-oidc/v3/oidc", want: 0},
		{name: "x/oauth2 in the adapter that owns it", path: "internal/identity/google/adapter.go", importPath: "golang.org/x/oauth2", want: 0},
		{name: "x/oauth2 in a second adapter that also owns it", path: "internal/identity/example/adapter.go", importPath: "golang.org/x/oauth2", want: 0},
		{name: "go-oidc in an adapter that does not own it", path: "internal/identity/example/adapter.go", importPath: "github.com/coreos/go-oidc/v3/oidc", want: 1},
		{name: "go-oidc in a test elsewhere", path: "internal/identity/service_test.go", importPath: "github.com/coreos/go-oidc/v3/oidc/oidctest", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			file := parseFixture(t, tc.path, "package fixture\n\nimport _ \""+tc.importPath+"\"\n")

			// when
			offenders := adapterLibraryOffenders([]sourceFile{file}, libraries)

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func TestIdentityCoreImportsNoAdapter(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := identityCoreOffenders(files)

	// then
	assert.Empty(t, offenders, "these files in the provider-neutral identity core import a provider adapter; take the adapter as an identity.Provider, composed in init_services.go, instead:\n%s", strings.Join(offenders, "\n"))
}

func TestIdentityCoreOffenders_FlagsOnlyAnAdapterImportedByTheCore(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		importPath string
		want       int
	}{
		{name: "an adapter in the core", path: "internal/identity/service.go", importPath: "retrotide/internal/identity/google", want: 1},
		{name: "an adapter in a core test", path: "internal/identity/service_test.go", importPath: "retrotide/internal/identity/google", want: 1},
		{name: "the core in an adapter", path: "internal/identity/google/adapter.go", importPath: "retrotide/internal/identity", want: 0},
		{name: "an adapter in the composition root", path: "init_services.go", importPath: "retrotide/internal/identity/google", want: 0},
		{name: "a shared package in the core", path: "internal/identity/flow.go", importPath: "retrotide/internal/token", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			file := parseFixture(t, tc.path, "package fixture\n\nimport _ \""+tc.importPath+"\"\n")

			// when
			offenders := identityCoreOffenders([]sourceFile{file})

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func TestDatabaseStaysBelowServices(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := dbImportOffenders(files)

	// then
	assert.Empty(t, offenders, "these non-test files import internal/db outside internal/db, internal/dao, internal/repository, internal/store and the root package; ask a repository instead:\n%s", strings.Join(offenders, "\n"))
}

func TestDAOConstructorsOnlyInStore(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := daoConstructorOffenders(files)

	// then
	assert.Empty(t, offenders, "these build a DAO outside internal/store and internal/dao; take the repository that store.New assembles instead:\n%s", strings.Join(offenders, "\n"))
}

func ginImportOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if file.dir == rootPackage || within(file.dir, "internal/controllers") || file.dir == "internal/middleware" || file.dir == "internal/routes" || file.dir == "internal/utils" {
			continue
		}

		for _, imported := range file.imports {
			if within(imported.path, ginImport) || strings.HasPrefix(imported.path, ginContribPrefix) {
				offenders = append(offenders, file.offence(imported.spec, "imports "+imported.path))
			}
		}
	}

	return offenders
}

func websocketImportOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if file.test || within(file.dir, "internal/ws") {
			continue
		}

		for _, imported := range file.imports {
			if within(imported.path, websocketImport) {
				offenders = append(offenders, file.offence(imported.spec, "imports "+imported.path))
			}
		}
	}

	return offenders
}

func adapterLibraryOffenders(files []sourceFile, libraries map[string][]string) []string {
	var offenders []string
	for _, file := range files {
		if file.test {
			continue
		}

		for _, imported := range file.imports {
			owners := libraryOwners(imported.path, libraries)
			if len(owners) > 0 && !within(file.dir, owners...) {
				offenders = append(offenders, file.offence(imported.spec, "imports "+imported.path+", which belongs to "+strings.Join(owners, " and ")))
			}
		}
	}

	return offenders
}

func libraryOwners(importPath string, libraries map[string][]string) []string {
	var owners []string
	for _, adapterDir := range slices.Sorted(maps.Keys(libraries)) {
		if within(importPath, libraries[adapterDir]...) {
			owners = append(owners, adapterDir)
		}
	}

	return owners
}

func identityCoreOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if file.dir != identityCoreDir {
			continue
		}

		for _, imported := range file.imports {
			if strings.HasPrefix(imported.path, identityCoreImport+"/") {
				offenders = append(offenders, file.offence(imported.spec, "imports the adapter "+imported.path))
			}
		}
	}

	return offenders
}

func dbImportOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if file.test || file.dir == rootPackage || within(file.dir, "internal/db", "internal/dao", "internal/repository", "internal/store") {
			continue
		}

		for _, imported := range file.imports {
			if within(imported.path, dbImport) {
				offenders = append(offenders, file.offence(imported.spec, "imports "+imported.path))
			}
		}
	}

	return offenders
}

func daoConstructorOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if within(file.dir, "internal/store", "internal/dao") {
			continue
		}

		alias, ok := file.localName(daoImport)
		if !ok {
			continue
		}

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}

			selector, isSelector := call.Fun.(*ast.SelectorExpr)
			if !isSelector {
				return true
			}

			pkg, isIdent := selector.X.(*ast.Ident)
			if !isIdent || pkg.Name != alias {
				return true
			}

			name := selector.Sel.Name
			if strings.HasPrefix(name, daoConstructorPrefix) && !strings.HasPrefix(name, daoMockPrefix) {
				offenders = append(offenders, file.offence(call, "calls "+alias+"."+name))
			}

			return true
		})
	}

	return offenders
}
