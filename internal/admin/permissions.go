package admin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"retrotide/internal/audit"
	"retrotide/internal/authz"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/role"
)

var (
	editableRoleLabels = map[role.Role]string{
		role.Support: "Support",
	}
)

func (s *service) GetPermissions(ctx context.Context) (*dto.AdminPermissionsResponse, error) {
	catalogue := authz.RoleAssignablePermissions()
	permissions := make([]dto.PermissionCatalogueItem, 0, len(catalogue))
	for _, def := range catalogue {
		permissions = append(permissions, dto.PermissionCatalogueItem{Permission: string(def.Permission), Label: def.Label})
	}

	stored, err := s.permissions.GetRolePermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the role permissions: %w", err)
	}

	editable := authz.EditableRoles()
	roles := make([]dto.RolePermissionsItem, 0, len(editable))
	for _, r := range editable {
		roles = append(roles, dto.RolePermissionsItem{Role: r, Label: editableRoleLabels[r], Permissions: assignablePermissions(stored[r])})
	}

	return &dto.AdminPermissionsResponse{Permissions: permissions, Roles: roles}, nil
}

func (s *service) UpdateRolePermissions(ctx context.Context, update spec.StaffRolePermissionsUpdate) error {
	if authz.IsImmutableRole(update.Role) {
		return ErrImmutableRole
	}

	if !authz.IsEditableRole(update.Role) {
		return ErrUnknownRole
	}

	clean, err := normalisePermissions(update.Permissions)
	if err != nil {
		return err
	}

	err = s.permissions.SetRolePermissions(ctx, spec.RolePermissionsUpdate{Role: update.Role, Permissions: clean})
	if err != nil && !errors.Is(err, repository.ErrStaleStaffCache) {
		return fmt.Errorf("set the permissions of role %s: %w", update.Role, err)
	}

	details := map[string]string{"permissions": strings.Join(clean, ",")}

	return s.recordStaffChange(ctx, err, staffEntry(update.StaffAction, audit.ActionUpdateRolePermissions, audit.TargetRole, string(update.Role), details))
}

func normalisePermissions(requested []string) ([]string, error) {
	clean := make([]string, 0, len(requested))

	for _, raw := range requested {
		perm := authz.Permission(strings.TrimSpace(raw))

		if !authz.IsKnownPermission(perm) {
			return nil, fmt.Errorf("%w: %q", ErrUnknownPermission, raw)
		}

		if !authz.IsRoleAssignable(perm) {
			return nil, fmt.Errorf("%w: %q", ErrRestrictedPermission, raw)
		}

		clean = append(clean, string(perm))
	}

	slices.Sort(clean)

	return slices.Compact(clean), nil
}

func assignablePermissions(stored []string) []string {
	granted := make([]string, 0, len(stored))
	for _, perm := range stored {
		if !authz.IsRoleAssignable(authz.Permission(perm)) {
			continue
		}

		granted = append(granted, perm)
	}

	slices.Sort(granted)

	return slices.Compact(granted)
}
