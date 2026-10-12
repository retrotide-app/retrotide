package arch

import (
	"bufio"
	"go/ast"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	parameter struct {
		name string
		typ  ast.Expr
	}

	interfaceMethod struct {
		iface  string
		name   string
		node   ast.Node
		params []parameter
	}
)

const (
	specImport = "retrotide/internal/model/spec"
	uuidImport = "github.com/google/uuid"

	daoPackageDir  = "internal/dao"
	specPackageDir = "internal/model/spec"
	vetDisableMark = "@sqlc-vet-disable"
)

var (
	tenantDAOs = []string{"OrganisationDAO", "TeamDAO", "BoardDAO", "BoardColumnDAO", "CardDAO", "CardVoteDAO", "CardCommentDAO", "ActionItemDAO", "StatsDAO", "SavedTemplateDAO", "CardReactionDAO", "MediaDAO", "SurveyDAO"}

	tenantKeyFields = []string{"OrgID", "BoardID"}

	tenantIDParams = []string{"orgID", "boardID"}

	globalQueries = map[string]string{
		"OrganisationDAO.Create":                    "creates the tenant root itself, which has no tenant until it exists",
		"OrganisationDAO.PurgeDeletedBefore":        "the daily retention sweep of organisations deleted over 30 days ago",
		"BoardDAO.ListIdle":                         "the auto-end job's sweep of boards idle for 14 days, across every organisation",
		"BoardDAO.ListAutoLockTimers":               "the startup restore of running timers that lock their board when they end, across every organisation",
		"BoardDAO.PurgeDeletedBefore":               "the daily retention sweep of boards deleted over 30 days ago",
		"MediaDAO.PurgeUnattachedBefore":            "the daily sweep of uploaded images no card or board uses",
		"OrganisationDAO.ListForUser":               "a read keyed on the signed-in user alone: their own organisations",
		"OrganisationDAO.CountOwnedByUser":          "a read keyed on the signed-in user alone: the organisations they own, for the per-owner quota",
		"OrganisationDAO.ListOwnedWithOtherMembers": "a read keyed on the signed-in user alone: the sole-owner check before account deletion",
		"OrganisationDAO.LockAllOfMember":           "account deletion locks every organisation of the account it deletes",
		"OrganisationDAO.SoftDeleteOwnedAlone":      "account deletion soft-deletes the organisations the account owns alone",
		"OrganisationDAO.GetLiveInviteByTokenHash":  "the invite lookup by token hash, which is how an invite finds its tenant",
		"OrganisationDAO.ConsumeInvite":             "spends the invite that the token hash names, inside the acceptance that locked its organisation",
		"OrganisationDAO.DeleteExpiredInvites":      "the hourly retention sweep of expired invites",
		"OrganisationDAO.ListForStaff":              "the staff organisation list behind view_organisations, a page across every tenant with counts only",
		"OrganisationDAO.CountForStaff":             "the total behind the staff organisation list, across every tenant",
		"StatsDAO.CountTotals":                      "the staff dashboard totals behind view_stats, site-wide counts that read no tenant content",
		"StatsDAO.CountActivity":                    "the staff dashboard activity table behind view_stats, site-wide counts that read no tenant content",
		"StatsDAO.ListDaily":                        "the staff dashboard daily series behind view_stats, site-wide counts that read no tenant content",
		"TeamDAO.GetHeader":                         "the team header behind TeamPermission, which finds the tenant from the path; the caller checks it in Go",
		"BoardDAO.GetHeader":                        "the board header behind live.Service.Resolve, which finds the tenant from the path; the caller checks it in Go",
		"BoardDAO.GetHeaderBySlug":                  "the share-link lookup by slug, which finds the board from the link; live.Service checks access in Go",
		"BoardDAO.ListForGuestDevice":               "the guest-device claim: the boards one browser's own guest device joined, keyed on that device alone",
		"ActionItemDAO.ListMentionsForUser":         "a read keyed on the signed-in user alone: the action items they were mentioned in, which the action item service then checks against their current access in Go",
		"CardDAO.ListMentionsForUser":               "a read keyed on the signed-in user alone: the cards they were mentioned on, which the projection drops while hidden from them and the action item service then checks against their current access in Go",
		"CardCommentDAO.ListMentionsForUser":        "a read keyed on the signed-in user alone: the comments they were mentioned in, which the projection drops while their card is hidden from them and the action item service then checks against their current access in Go",
		"CardDAO.ListWrittenByUser":                 "a read keyed on the signed-in user alone: the cards still linked to them through their own seats, for their own self-export",
		"CardCommentDAO.ListWrittenByUser":          "a read keyed on the signed-in user alone: the comments still linked to them through their own seats, for their own self-export",
		"ActionItemDAO.ListForUser":                 "a read keyed on the signed-in user alone: the action items they raised or are assigned, for their own self-export",
	}

	scopedQuotaQueries = map[string]string{
		"OrganisationDAO.Restore": "writes only its own organisation, but its per-owner quota counts the organisations the owner holds across tenants, as CountOwnedByUser does",
	}

	queryNamePattern = regexp.MustCompile(`^--\s*name:\s*(\w+)`)
	boardUpdates     = regexp.MustCompile(`(?is)\bupdate\s+boards\b\s+(?:(?:as\s+)?\w+\s+)?set\b(.*?)(?:\bwhere\b|\breturning\b|\bfrom\b|;|\z)`)
	orgIDAssignment  = regexp.MustCompile(`(?i)\borg_id\s*=`)
)

func TestTenantDAOMethodsTakeTheirTenant(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := tenancyOffenders(files, tenantDAOs, globalQueries)

	// then
	assert.Empty(t, offenders, "every method on a tenant DAO takes a spec with an OrgID or BoardID field, or the tenant's own ID as orgID or boardID; a global query goes in globalQueries with its reason:\n%s", strings.Join(offenders, "\n"))
}

func TestTenancyOffenders_FlagsAMethodThatTakesNoTenant(t *testing.T) {
	tests := []struct {
		name   string
		method string
		want   int
	}{
		{name: "a spec with the tenant", method: "Get(ctx context.Context, s spec.Scoped, tx ...*sql.Tx) error", want: 0},
		{name: "the tenant's own ID", method: "Get(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) error", want: 0},
		{name: "a spec without the tenant", method: "Get(ctx context.Context, s spec.Unscoped, tx ...*sql.Tx) error", want: 1},
		{name: "another ID", method: "Get(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) error", want: 1},
		{name: "no argument at all", method: "Get(ctx context.Context, tx ...*sql.Tx) error", want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			specs := parseFixture(t, "internal/model/spec/fixture.go", "package spec\n\ntype (\n\tScoped struct {\n\t\tOrgID string\n\t}\n\n\tUnscoped struct {\n\t\tUserID string\n\t}\n)\n")
			dao := parseFixture(t, "internal/dao/fixture.go", "package dao\n\nimport (\n\t\"retrotide/internal/model/spec\"\n\n\t\"github.com/google/uuid\"\n)\n\ntype (\n\tFixtureDAO interface {\n\t\t"+tc.method+"\n\t}\n)\n")

			// when
			offenders := tenancyOffenders([]sourceFile{specs, dao}, []string{"FixtureDAO"}, nil)

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func TestTenancyOffenders_FlagsAStaleOrNeedlessAllowlistEntry(t *testing.T) {
	// given
	specs := parseFixture(t, "internal/model/spec/fixture.go", "package spec\n\ntype (\n\tScoped struct {\n\t\tOrgID string\n\t}\n)\n")
	dao := parseFixture(t, "internal/dao/fixture.go", "package dao\n\nimport \"retrotide/internal/model/spec\"\n\ntype (\n\tFixtureDAO interface {\n\t\tGet(ctx context.Context, s spec.Scoped) error\n\t}\n)\n")
	allowlist := map[string]string{"FixtureDAO.Get": "not global at all", "FixtureDAO.Gone": "deleted since"}

	// when
	offenders := tenancyOffenders([]sourceFile{specs, dao}, []string{"FixtureDAO", "MissingDAO"}, allowlist)

	// then the needless entry, the stale entry and the missing DAO are each reported
	assert.Len(t, offenders, 3, "%v", offenders)
}

func TestVetDisabledQueriesRunOnlyInGlobalMethods(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	disabled, err := vetDisabledQueries()
	require.NoError(t, err)

	allowed := maps.Clone(globalQueries)
	maps.Copy(allowed, scopedQuotaQueries)

	// when
	offenders := vetDisabledOffenders(files, disabled, allowed)

	// then
	assert.Empty(t, offenders, "a query that disables a tenancy vet rule may run only from a DAO method in globalQueries, or in scopedQuotaQueries for a scoped write whose quota counts across tenants:\n%s", strings.Join(offenders, "\n"))
}

func TestDisabledQueryNames_ReadsTheAnnotationOfEachQuery(t *testing.T) {
	// given
	source := "-- name: Scoped :one\nSELECT * FROM teams WHERE org_id = $1;\n\n-- name: Global :one\n-- @sqlc-vet-disable org-scoped\nSELECT * FROM teams WHERE id = $1;\n"

	// when
	names, err := disabledQueryNames(strings.NewReader(source))

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"Global"}, names)
}

func TestNoStatementMovesABoardBetweenOrganisations(t *testing.T) {
	// given
	statements, err := querySources()
	require.NoError(t, err)

	// when
	var offenders []string
	for path, source := range statements {
		if movesABoard(source) {
			offenders = append(offenders, path)
		}
	}

	// then a board may change teams, never organisations
	assert.Empty(t, offenders, "these query files update boards.org_id:\n%s", strings.Join(offenders, "\n"))
}

func TestMovesABoard_LooksOnlyAtTheSetList(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      bool
	}{
		{name: "setting the organisation", statement: "UPDATE boards SET org_id = $1 WHERE id = $2;", want: true},
		{name: "setting it beside another column", statement: "UPDATE boards\nSET team_id = $3, org_id = $1\nWHERE id = $2;", want: true},
		{name: "through an alias", statement: "UPDATE boards AS b SET org_id = $1 WHERE b.id = $2;", want: true},
		{name: "filtering by the organisation", statement: "UPDATE boards SET version = version + 1 WHERE org_id = $1 AND id = $2;", want: false},
		{name: "another table", statement: "UPDATE teams SET org_id = $1 WHERE id = $2;", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := movesABoard(tc.statement)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func tenancyOffenders(files []sourceFile, daos []string, allowlist map[string]string) []string {
	specFields := specStructFields(files)
	found := make(map[string]bool)
	matched := make(map[string]bool)

	var offenders []string
	for _, file := range files {
		if file.test || file.dir != daoPackageDir {
			continue
		}

		for _, method := range tenantDAOMethods(file, daos) {
			key := method.iface + "." + method.name
			found[method.iface] = true

			_, allowed := allowlist[key]
			if allowed {
				matched[key] = true
			}

			scoped := takesTenant(file, method.params, specFields)
			if scoped && allowed {
				offenders = append(offenders, file.offence(method.node, key+" takes its tenant, so remove it from the allowlist"))
			}

			if !scoped && !allowed {
				offenders = append(offenders, file.offence(method.node, key+" takes no spec with "+strings.Join(tenantKeyFields, ", ")+" and is not an allowlisted global query"))
			}
		}
	}

	for _, name := range daos {
		if !found[name] {
			offenders = append(offenders, "tenant DAO "+name+" declares no method")
		}
	}

	for _, key := range slices.Sorted(maps.Keys(allowlist)) {
		if !matched[key] {
			offenders = append(offenders, key+": allowlisted ("+allowlist[key]+") but matches no method; remove the entry")
		}
	}

	return offenders
}

func tenantDAOMethods(file sourceFile, daos []string) []interfaceMethod {
	var methods []interfaceMethod
	ast.Inspect(file.syntax, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok || !slices.Contains(daos, typeSpec.Name.Name) {
			return true
		}

		iface, ok := typeSpec.Type.(*ast.InterfaceType)
		if !ok {
			return false
		}

		for _, field := range iface.Methods.List {
			fn, isFunc := field.Type.(*ast.FuncType)
			if !isFunc || len(field.Names) == 0 {
				continue
			}

			methods = append(methods, interfaceMethod{iface: typeSpec.Name.Name, name: field.Names[0].Name, node: field, params: flattenParams(fn)})
		}

		return false
	})

	return methods
}

func flattenParams(fn *ast.FuncType) []parameter {
	var params []parameter
	for _, field := range fn.Params.List {
		if len(field.Names) == 0 {
			params = append(params, parameter{typ: field.Type})
			continue
		}

		for _, name := range field.Names {
			params = append(params, parameter{name: name.Name, typ: field.Type})
		}
	}

	return params
}

func takesTenant(file sourceFile, params []parameter, specFields map[string][]string) bool {
	if len(params) < 2 {
		return false
	}

	selector, ok := params[1].typ.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	specAlias, importsSpec := file.localName(specImport)
	if importsSpec && pkg.Name == specAlias {
		return slices.ContainsFunc(specFields[selector.Sel.Name], func(field string) bool {
			return slices.Contains(tenantKeyFields, field)
		})
	}

	uuidAlias, importsUUID := file.localName(uuidImport)

	return importsUUID && pkg.Name == uuidAlias && selector.Sel.Name == "UUID" && slices.Contains(tenantIDParams, params[1].name)
}

func specStructFields(files []sourceFile) map[string][]string {
	fields := make(map[string][]string)
	for _, file := range files {
		if file.test || file.dir != specPackageDir {
			continue
		}

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			typeSpec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return false
			}

			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					fields[typeSpec.Name.Name] = append(fields[typeSpec.Name.Name], name.Name)
				}
			}

			return false
		})
	}

	return fields
}

func vetDisabledOffenders(files []sourceFile, disabled []string, allowlist map[string]string) []string {
	allowed := make(map[string]bool, len(allowlist))
	for key := range allowlist {
		allowed[strings.ToLower(key)] = true
	}

	callers := queryCallers(files, disabled)

	var offenders []string
	for _, query := range disabled {
		if len(callers[query]) == 0 {
			offenders = append(offenders, query+" disables a vet rule but no DAO method runs it")
		}

		for _, caller := range callers[query] {
			if !allowed[strings.ToLower(caller)] {
				offenders = append(offenders, query+" disables a vet rule and runs in "+caller+", which is not in globalQueries")
			}
		}
	}

	return offenders
}

func queryCallers(files []sourceFile, queries []string) map[string][]string {
	callers := make(map[string][]string)
	for _, file := range files {
		if file.test || file.dir != daoPackageDir {
			continue
		}

		for _, decl := range file.syntax.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Body == nil {
				continue
			}

			caller := receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name

			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}

				selector, isSelector := call.Fun.(*ast.SelectorExpr)
				if isSelector && slices.Contains(queries, selector.Sel.Name) {
					callers[selector.Sel.Name] = append(callers[selector.Sel.Name], caller)
				}

				return true
			})
		}
	}

	return callers
}

func querySources() (map[string]string, error) {
	var paths []string
	for _, pattern := range []string{"internal/dao/queries/*.sql", "internal/dao/queries/gen/*.sql"} {
		matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, err
		}

		paths = append(paths, matches...)
	}

	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		sources[filepath.ToSlash(path)] = string(data)
	}

	return sources, nil
}

func vetDisabledQueries() ([]string, error) {
	sources, err := querySources()
	if err != nil {
		return nil, err
	}

	var disabled []string
	for _, path := range slices.Sorted(maps.Keys(sources)) {
		names, err := disabledQueryNames(strings.NewReader(sources[path]))
		if err != nil {
			return nil, err
		}

		disabled = append(disabled, names...)
	}

	return disabled, nil
}

func disabledQueryNames(source *strings.Reader) ([]string, error) {
	var (
		current  string
		disabled []string
	)

	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		line := scanner.Text()

		if match := queryNamePattern.FindStringSubmatch(line); match != nil {
			current = match[1]
			continue
		}

		if current != "" && strings.Contains(line, vetDisableMark) && !slices.Contains(disabled, current) {
			disabled = append(disabled, current)
		}
	}

	return disabled, scanner.Err()
}

func movesABoard(source string) bool {
	for _, match := range boardUpdates.FindAllStringSubmatch(source, -1) {
		if orgIDAssignment.MatchString(match[1]) {
			return true
		}
	}

	return false
}
