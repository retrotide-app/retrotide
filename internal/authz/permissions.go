package authz

type (
	Permission string

	PermissionScope string

	PermissionDef struct {
		Permission Permission
		Label      string
		Scope      PermissionScope
	}
)

const (
	ScopeStaff      PermissionScope = "staff"
	ScopeRestricted PermissionScope = "restricted"
)

const (
	PermViewAdminPanel    Permission = "view_admin_panel"
	PermViewStats         Permission = "view_stats"
	PermViewAuditLog      Permission = "view_audit_log"
	PermManageSettings    Permission = "manage_settings"
	PermManageRoles       Permission = "manage_roles"
	PermViewUsers         Permission = "view_users"
	PermBanUser           Permission = "ban_user"
	PermManageUserAccount Permission = "manage_user_account"
	PermManageUserEmail   Permission = "manage_user_email"
	PermSetEmailVerified  Permission = "set_email_verified"
	PermResetPassword     Permission = "reset_password"
	PermDeleteAnyUser     Permission = "delete_any_user"
	PermViewOrganisations Permission = "view_organisations"
	PermGrantEntitlements Permission = "grant_entitlements"
)

var (
	permissionCatalogue = []PermissionDef{
		{PermViewAdminPanel, "View admin panel", ScopeStaff},
		{PermViewStats, "View site stats", ScopeStaff},
		{PermViewAuditLog, "View audit log", ScopeStaff},
		{PermManageSettings, "Manage site settings", ScopeRestricted},
		{PermManageRoles, "Manage roles and permissions", ScopeRestricted},
		{PermViewUsers, "View user records", ScopeStaff},
		{PermBanUser, "Ban and lock users", ScopeStaff},
		{PermManageUserAccount, "Manage user accounts", ScopeStaff},
		{PermManageUserEmail, "Manage user email addresses", ScopeStaff},
		{PermSetEmailVerified, "Set email verified", ScopeStaff},
		{PermResetPassword, "Reset user passwords", ScopeStaff},
		{PermDeleteAnyUser, "Delete any user", ScopeStaff},
		{PermViewOrganisations, "View organisations", ScopeStaff},
		{PermGrantEntitlements, "Grant entitlements", ScopeStaff},
	}

	permissionIndex = buildPermissionIndex()
)

func buildPermissionIndex() map[Permission]PermissionDef {
	index := make(map[Permission]PermissionDef, len(permissionCatalogue))
	for _, def := range permissionCatalogue {
		index[def.Permission] = def
	}

	return index
}

func LookupPermission(perm Permission) (PermissionDef, bool) {
	def, ok := permissionIndex[perm]

	return def, ok
}

func IsKnownPermission(perm Permission) bool {
	_, ok := permissionIndex[perm]

	return ok
}

func IsRoleAssignable(perm Permission) bool {
	def, ok := permissionIndex[perm]

	return ok && def.Scope != ScopeRestricted
}

func RoleAssignablePermissions() []PermissionDef {
	assignable := make([]PermissionDef, 0, len(permissionCatalogue))
	for _, def := range permissionCatalogue {
		if def.Scope == ScopeRestricted {
			continue
		}

		assignable = append(assignable, def)
	}

	return assignable
}
