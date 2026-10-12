package orgauthz

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	orgTable = map[Permission][3]bool{
		"org.view":               {true, true, true},
		"org.update":             {true, true, false},
		"org.manage_members":     {true, true, false},
		"org.manage_admins":      {true, false, false},
		"org.manage_teams":       {true, true, false},
		"org.create_board":       {true, true, true},
		"org.manage_boards":      {true, true, false},
		"org.manage_billing":     {true, true, false},
		"org.view_audit_log":     {true, true, false},
		"org.export":             {true, true, false},
		"org.transfer_ownership": {true, false, false},
		"org.delete":             {true, false, false},
	}

	teamTable = map[Permission][2]bool{
		"team.view":                {true, true},
		"team.update":              {true, false},
		"team.manage_members":      {true, false},
		"team.create_board":        {true, true},
		"team.manage_boards":       {true, false},
		"team.manage_action_items": {true, true},
	}

	orgRoles  = []model.OrgRole{model.OrgRoleOwner, model.OrgRoleAdmin, model.OrgRoleMember}
	teamRoles = []model.TeamRole{model.TeamRoleLead, model.TeamRoleMember}
)

func declaredPermissionConstants(t *testing.T) map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "permissions.go", nil, 0)
	require.NoError(t, err)

	declared := make(map[string]string)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}

		for _, item := range gen.Specs {
			value, ok := item.(*ast.ValueSpec)
			if !ok {
				continue
			}

			ident, ok := value.Type.(*ast.Ident)
			if !ok || ident.Name != "Permission" {
				continue
			}

			for i, name := range value.Names {
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok {
					continue
				}

				declared[name.Name] = literal.Value
			}
		}
	}

	return declared
}

func TestPermissionCatalogue_ClassifiesEveryDeclaredPermission(t *testing.T) {
	// given
	declared := declaredPermissionConstants(t)
	require.NotEmpty(t, declared)

	classified := make(map[string]struct{}, len(permissionCatalogue))
	for _, def := range permissionCatalogue {
		classified[`"`+string(def.Permission)+`"`] = struct{}{}
	}

	// when
	var unclassified []string
	for name, literal := range declared {
		if _, ok := classified[literal]; !ok {
			unclassified = append(unclassified, name)
		}
	}

	// then
	assert.Empty(t, unclassified, "every Permission constant must be classified in permissionCatalogue")
	assert.Len(t, permissionCatalogue, len(declared), "permissionCatalogue must hold each declared permission exactly once")
}

func TestPermissionCatalogue_EveryEntryHasALabelAndTheScopeItsNameSays(t *testing.T) {
	for _, def := range permissionCatalogue {
		t.Run(string(def.Permission), func(t *testing.T) {
			// given
			_, isOrg := orgTable[def.Permission]
			_, isTeam := teamTable[def.Permission]

			// when
			scope := def.Scope

			// then
			assert.NotEmpty(t, def.Label, "every permission needs a human label")
			assert.True(t, isOrg || isTeam, "%s is in neither table of the spec", def.Permission)
			if isOrg {
				assert.Equal(t, ScopeOrg, scope)
			}
			if isTeam {
				assert.Equal(t, ScopeTeam, scope)
			}
		})
	}
}

func TestOrgRoleGrants_MatchTheSpecifiedTable(t *testing.T) {
	for perm, allowed := range orgTable {
		for i, r := range orgRoles {
			t.Run(string(r)+" "+string(perm), func(t *testing.T) {
				// given the table of section 6.5.2, owner then admin then member
				want := allowed[i]

				// when
				got := OrgRoleGrants(r, perm)

				// then
				assert.Equal(t, want, got)
			})
		}
	}
}

func TestTeamRoleGrants_MatchTheSpecifiedTable(t *testing.T) {
	for perm, allowed := range teamTable {
		for i, r := range teamRoles {
			t.Run(string(r)+" "+string(perm), func(t *testing.T) {
				// given the table of section 6.5.2, lead then member
				want := allowed[i]

				// when
				got := TeamRoleGrants(r, perm)

				// then
				assert.Equal(t, want, got)
			})
		}
	}
}

func TestRoleGrants_NeverCrossTheirScope(t *testing.T) {
	for _, def := range permissionCatalogue {
		t.Run(string(def.Permission), func(t *testing.T) {
			// given
			perm := def.Permission

			// when
			byOrgRole := OrgRoleGrants(model.OrgRoleOwner, perm)
			byTeamRole := TeamRoleGrants(model.TeamRoleLead, perm)

			// then an organisation role grants only organisation permissions, and a team role only team ones
			assert.Equal(t, def.Scope == ScopeOrg, byOrgRole)
			assert.Equal(t, def.Scope == ScopeTeam, byTeamRole)
		})
	}
}

func TestRoleGrants_AnUnknownRoleGrantsNothing(t *testing.T) {
	for _, def := range permissionCatalogue {
		t.Run(string(def.Permission), func(t *testing.T) {
			// given
			perm := def.Permission

			// when
			byOrgRole := OrgRoleGrants("", perm) || OrgRoleGrants("guest", perm)
			byTeamRole := TeamRoleGrants("", perm) || TeamRoleGrants("guest", perm)

			// then
			assert.False(t, byOrgRole)
			assert.False(t, byTeamRole)
		})
	}
}

func TestEffectiveTeamRole_OwnersAndAdminsLeadEveryTeam(t *testing.T) {
	tests := []struct {
		name string
		org  model.OrgRole
		team model.TeamRole
		want model.TeamRole
	}{
		{name: "an owner on no team", org: model.OrgRoleOwner, team: "", want: model.TeamRoleLead},
		{name: "an owner who is a team member", org: model.OrgRoleOwner, team: model.TeamRoleMember, want: model.TeamRoleLead},
		{name: "an admin on no team", org: model.OrgRoleAdmin, team: "", want: model.TeamRoleLead},
		{name: "a member who leads the team", org: model.OrgRoleMember, team: model.TeamRoleLead, want: model.TeamRoleLead},
		{name: "a member of the team", org: model.OrgRoleMember, team: model.TeamRoleMember, want: model.TeamRoleMember},
		{name: "a member who is not on the team", org: model.OrgRoleMember, team: "", want: ""},
		{name: "someone outside the organisation", org: "", team: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := EffectiveTeamRole(tc.org, tc.team)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLookupPermission(t *testing.T) {
	tests := []struct {
		name      string
		perm      Permission
		wantFound bool
		wantScope PermissionScope
	}{
		{name: "an organisation permission", perm: PermOrgManageAdmins, wantFound: true, wantScope: ScopeOrg},
		{name: "a team permission", perm: PermTeamManageBoards, wantFound: true, wantScope: ScopeTeam},
		{name: "a staff permission is not a tenant one", perm: "view_users", wantFound: false},
		{name: "an unknown permission", perm: "org.teleport", wantFound: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			def, found := LookupPermission(tc.perm)

			// then
			assert.Equal(t, tc.wantFound, found)
			assert.Equal(t, tc.wantScope, def.Scope)
		})
	}
}
