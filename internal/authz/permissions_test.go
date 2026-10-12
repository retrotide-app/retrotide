package authz

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"retrotide/internal/role"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestPermissionCatalogue_MatchesTheSpecifiedStaffPermissions(t *testing.T) {
	// given the catalogue section 6.5.1 of the spec fixes, with the two restricted ones
	want := map[Permission]PermissionScope{
		"view_admin_panel":    ScopeStaff,
		"view_stats":          ScopeStaff,
		"view_audit_log":      ScopeStaff,
		"manage_settings":     ScopeRestricted,
		"manage_roles":        ScopeRestricted,
		"view_users":          ScopeStaff,
		"ban_user":            ScopeStaff,
		"manage_user_account": ScopeStaff,
		"manage_user_email":   ScopeStaff,
		"set_email_verified":  ScopeStaff,
		"reset_password":      ScopeStaff,
		"delete_any_user":     ScopeStaff,
		"view_organisations":  ScopeStaff,
		"grant_entitlements":  ScopeStaff,
	}

	// when
	got := make(map[Permission]PermissionScope, len(permissionCatalogue))
	for _, def := range permissionCatalogue {
		got[def.Permission] = def.Scope
	}

	// then
	assert.Equal(t, want, got)
}

func TestPermissionCatalogue_EveryEntryHasALabel(t *testing.T) {
	for _, def := range permissionCatalogue {
		t.Run(string(def.Permission), func(t *testing.T) {
			// given
			label := def.Label

			// when
			empty := label == ""

			// then
			assert.False(t, empty, "every permission needs a human label for the admin panel")
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
		{name: "a staff permission", perm: PermBanUser, wantFound: true, wantScope: ScopeStaff},
		{name: "a restricted permission", perm: PermManageRoles, wantFound: true, wantScope: ScopeRestricted},
		{name: "an unknown permission", perm: "teleport", wantFound: false},
		{name: "a wildcard is not a permission", perm: "*", wantFound: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			perm := tc.perm

			// when
			def, found := LookupPermission(perm)

			// then
			assert.Equal(t, tc.wantFound, found)
			assert.Equal(t, tc.wantFound, IsKnownPermission(perm))
			assert.Equal(t, tc.wantScope, def.Scope)
		})
	}
}

func TestIsRoleAssignable(t *testing.T) {
	tests := []struct {
		name string
		perm Permission
		want bool
	}{
		{name: "a staff permission can be granted to support", perm: PermViewUsers, want: true},
		{name: "manage settings is restricted", perm: PermManageSettings, want: false},
		{name: "manage roles is restricted", perm: PermManageRoles, want: false},
		{name: "an unknown permission is never assignable", perm: "teleport", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			perm := tc.perm

			// when
			got := IsRoleAssignable(perm)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRoleAssignablePermissions_LeavesOutTheRestrictedOnes(t *testing.T) {
	// given
	assignable := RoleAssignablePermissions()

	// when
	var restricted []Permission
	for _, def := range assignable {
		if def.Scope == ScopeRestricted {
			restricted = append(restricted, def.Permission)
		}
	}

	// then
	assert.Empty(t, restricted)
	assert.Len(t, assignable, len(permissionCatalogue)-2)
}

func TestImmutableAndEditableRoles(t *testing.T) {
	tests := []struct {
		name          string
		role          role.Role
		wantImmutable bool
		wantEditable  bool
	}{
		{name: "super admin is immutable", role: role.SuperAdmin, wantImmutable: true, wantEditable: false},
		{name: "admin is immutable", role: role.Admin, wantImmutable: true, wantEditable: false},
		{name: "support is editable", role: role.Support, wantImmutable: false, wantEditable: true},
		{name: "no role is neither", role: "", wantImmutable: false, wantEditable: false},
		{name: "an unknown role is neither", role: "moderator", wantImmutable: false, wantEditable: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			r := tc.role

			// when
			immutable, editable := IsImmutableRole(r), IsEditableRole(r)

			// then
			assert.Equal(t, tc.wantImmutable, immutable)
			assert.Equal(t, tc.wantEditable, editable)
		})
	}
}

func TestEditableRoles_OnlySupport(t *testing.T) {
	// given
	editable := EditableRoles()

	// when
	editable[0] = role.Admin

	// then the caller's copy is its own, and support stays the only editable role
	assert.Equal(t, []role.Role{role.Support}, EditableRoles())
}

func TestDefaultRolePermissions(t *testing.T) {
	// given the support defaults the migrations seed
	want := []Permission{PermViewAdminPanel, PermViewStats, PermViewUsers, PermBanUser, PermManageUserAccount, PermViewOrganisations}

	// when
	got := DefaultRolePermissions(role.Support)

	// then
	assert.Equal(t, want, got)
	assert.Nil(t, DefaultRolePermissions(role.Admin))
	assert.Nil(t, DefaultRolePermissions(role.SuperAdmin))
	assert.Nil(t, DefaultRolePermissions(""))
	for _, perm := range got {
		assert.True(t, IsRoleAssignable(perm), "%s is seeded for support but cannot be granted to it", perm)
	}
}
