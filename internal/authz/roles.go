package authz

import (
	"slices"

	"retrotide/internal/role"
)

var (
	defaultSupportPermissions = []Permission{
		PermViewAdminPanel,
		PermViewStats,
		PermViewUsers,
		PermBanUser,
		PermManageUserAccount,
		PermViewOrganisations,
	}
)

func IsImmutableRole(r role.Role) bool {
	return r == role.Admin || r == role.SuperAdmin
}

func IsEditableRole(r role.Role) bool {
	return r == role.Support
}

func EditableRoles() []role.Role {
	return []role.Role{role.Support}
}

func DefaultRolePermissions(r role.Role) []Permission {
	if !IsEditableRole(r) {
		return nil
	}

	return slices.Clone(defaultSupportPermissions)
}
