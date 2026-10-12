package arch

import (
	"go/ast"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rankMethod = "Rank"

	roleImport  = "retrotide/internal/role"
	modelImport = "retrotide/internal/model"
)

var (
	rankOwners = []string{"internal/role", "internal/hierarchy"}

	roleTypes = map[string][]string{
		roleImport:  {"Role"},
		modelImport: {"OrgRole", "TeamRole"},
	}

	roleTypeDirs = map[string]string{
		"internal/role":  roleImport,
		"internal/model": modelImport,
	}

	integerTypes = []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64"}
)

func TestOnlyTheHierarchyComparesRanks(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := rankOffenders(files)

	// then
	assert.Empty(t, offenders, "these read a role's rank or keep a table of role ranks outside internal/hierarchy, the one place that decides who may act on whom; ask hierarchy.Service, or a hierarchy.Standing it returned, instead:\n%s", strings.Join(offenders, "\n"))
}

func TestRankOffenders_FlagsARankReadOrARankTableOutsideTheHierarchy(t *testing.T) {
	tests := []struct {
		name string
		path string
		src  string
		want int
	}{
		{name: "a rank read in a service", path: "internal/admin/users.go", src: "import \"retrotide/internal/role\"\n\nfunc outranks(a role.Role, b role.Role) bool {\n\treturn a.Rank() > b.Rank()\n}\n", want: 2},
		{name: "a rank read in a test elsewhere", path: "internal/admin/users_test.go", src: "import \"retrotide/internal/role\"\n\nfunc staff(r role.Role) bool {\n\treturn r.Rank() > 0\n}\n", want: 1},
		{name: "a rank read in the hierarchy", path: "internal/hierarchy/ladder.go", src: "import \"retrotide/internal/role\"\n\nfunc level(r role.Role) int {\n\treturn r.Rank() * 100\n}\n", want: 0},
		{name: "a rank read in the role package", path: "internal/role/role.go", src: "func (r Role) IsSiteStaff() bool {\n\treturn r.Rank() > 0\n}\n", want: 0},
		{name: "a call named Rank that takes an argument", path: "internal/search/service.go", src: "import \"retrotide/internal/score\"\n\nfunc order(rows []int) int {\n\treturn score.Rank(rows)\n}\n", want: 0},
		{name: "a site rank table", path: "internal/admin/ranks.go", src: "import \"retrotide/internal/role\"\n\nvar ranks = map[role.Role]int{role.Support: 1}\n", want: 1},
		{name: "an organisation rank table under an alias", path: "internal/org/ranks.go", src: "import m \"retrotide/internal/model\"\n\nvar ranks = map[m.OrgRole]int8{}\n", want: 1},
		{name: "a team rank table built with make", path: "internal/team/ranks.go", src: "import \"retrotide/internal/model\"\n\nfunc ranks() map[model.TeamRole]int {\n\treturn make(map[model.TeamRole]int)\n}\n", want: 2},
		{name: "a rank table inside the model package", path: "internal/model/ranks.go", src: "var ranks = map[OrgRole]int{}\n", want: 1},
		{name: "a rank table in the hierarchy", path: "internal/hierarchy/ladder.go", src: "import \"retrotide/internal/model\"\n\nvar ranks = map[model.OrgRole]int{}\n", want: 0},
		{name: "a table of role labels", path: "internal/admin/labels.go", src: "import \"retrotide/internal/role\"\n\nvar labels = map[role.Role]string{}\n", want: 0},
		{name: "a set of roles", path: "internal/orgauthz/roles.go", src: "import \"retrotide/internal/model\"\n\nvar leads = map[model.OrgRole]struct{}{}\n", want: 0},
		{name: "a type named Role in another package", path: "internal/board/ranks.go", src: "import \"retrotide/internal/board/policy\"\n\nvar ranks = map[policy.Role]int{}\n", want: 0},
		{name: "a local type named OrgRole outside the model package", path: "internal/org/ranks.go", src: "type OrgRole string\n\nvar ranks = map[OrgRole]int{}\n", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			file := parseFixture(t, tc.path, "package fixture\n\n"+tc.src)

			// when
			offenders := rankOffenders([]sourceFile{file})

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func rankOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if within(file.dir, rankOwners...) {
			continue
		}

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.CallExpr:
				if readsRank(typed) {
					offenders = append(offenders, file.offence(typed, "reads a role's rank"))
				}

			case *ast.MapType:
				if ranksRoles(file, typed) {
					offenders = append(offenders, file.offence(typed, "keeps a table of role ranks"))
				}
			}

			return true
		})
	}

	return offenders
}

func readsRank(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)

	return ok && selector.Sel.Name == rankMethod && len(call.Args) == 0
}

func ranksRoles(file sourceFile, table *ast.MapType) bool {
	value, ok := table.Value.(*ast.Ident)
	if !ok || !slices.Contains(integerTypes, value.Name) {
		return false
	}

	return isRoleType(file, table.Key)
}

func isRoleType(file sourceFile, expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		pkg, ok := typed.X.(*ast.Ident)
		if !ok {
			return false
		}

		for importPath, names := range roleTypes {
			alias, imported := file.localName(importPath)
			if imported && alias == pkg.Name && slices.Contains(names, typed.Sel.Name) {
				return true
			}
		}

	case *ast.Ident:
		importPath, declaring := roleTypeDirs[file.dir]

		return declaring && slices.Contains(roleTypes[importPath], typed.Name)
	}

	return false
}
