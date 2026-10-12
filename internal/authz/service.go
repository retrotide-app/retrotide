package authz

import (
	"context"
	"fmt"
	"slices"

	"retrotide/internal/repository"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	Service interface {
		Can(ctx context.Context, userID uuid.UUID, perm Permission) (bool, error)
		EffectivePermissions(ctx context.Context, userID uuid.UUID) ([]Permission, error)
		GetRole(ctx context.Context, userID uuid.UUID) (role.Role, error)
		BypassesRestrictions(ctx context.Context, userID uuid.UUID) (bool, error)
		IsBanned(ctx context.Context, userID uuid.UUID) (bool, error)
		IsLocked(ctx context.Context, userID uuid.UUID) (bool, error)
	}

	service struct {
		roles       repository.RoleRepository
		permissions repository.PermissionRepository
		users       repository.UserRepository
	}
)

func NewService(roles repository.RoleRepository, permissions repository.PermissionRepository, users repository.UserRepository) Service {
	return &service{roles: roles, permissions: permissions, users: users}
}

func (s *service) Can(ctx context.Context, userID uuid.UUID, perm Permission) (bool, error) {
	if !IsKnownPermission(perm) {
		return false, fmt.Errorf("%w: %q", ErrUnknownPermission, perm)
	}

	if userID == uuid.Nil {
		return false, nil
	}

	current, err := s.GetRole(ctx, userID)
	if err != nil {
		return false, err
	}

	if IsImmutableRole(current) {
		return true, nil
	}

	if !IsEditableRole(current) || !IsRoleAssignable(perm) {
		return false, nil
	}

	granted, err := s.storedPermissions(ctx, current)
	if err != nil {
		return false, err
	}

	return slices.Contains(granted, string(perm)), nil
}

func (s *service) EffectivePermissions(ctx context.Context, userID uuid.UUID) ([]Permission, error) {
	effective := make([]Permission, 0, len(permissionCatalogue))

	if userID == uuid.Nil {
		return effective, nil
	}

	current, err := s.GetRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	if IsImmutableRole(current) {
		for _, def := range permissionCatalogue {
			effective = append(effective, def.Permission)
		}

		return effective, nil
	}

	if !IsEditableRole(current) {
		return effective, nil
	}

	granted, err := s.storedPermissions(ctx, current)
	if err != nil {
		return nil, err
	}

	for _, def := range permissionCatalogue {
		if def.Scope == ScopeRestricted || !slices.Contains(granted, string(def.Permission)) {
			continue
		}

		effective = append(effective, def.Permission)
	}

	return effective, nil
}

func (s *service) GetRole(ctx context.Context, userID uuid.UUID) (role.Role, error) {
	current, err := s.roles.GetRole(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("read the role of user %s: %w", userID, err)
	}

	return current, nil
}

func (s *service) BypassesRestrictions(ctx context.Context, userID uuid.UUID) (bool, error) {
	current, err := s.GetRole(ctx, userID)
	if err != nil {
		return false, err
	}

	return current.BypassesRestrictions(), nil
}

func (s *service) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	banned, err := s.users.IsBanned(ctx, userID)
	if err != nil {
		return true, err
	}

	return s.enforced(ctx, userID, banned)
}

func (s *service) IsLocked(ctx context.Context, userID uuid.UUID) (bool, error) {
	locked, err := s.users.IsLocked(ctx, userID)
	if err != nil {
		return true, err
	}

	return s.enforced(ctx, userID, locked)
}

func (s *service) enforced(ctx context.Context, userID uuid.UUID, restricted bool) (bool, error) {
	if !restricted {
		return false, nil
	}

	exempt, err := s.BypassesRestrictions(ctx, userID)
	if err != nil {
		return true, err
	}

	return !exempt, nil
}

func (s *service) storedPermissions(ctx context.Context, r role.Role) ([]string, error) {
	table, err := s.permissions.GetRolePermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the permissions of role %s: %w", r, err)
	}

	return table[r], nil
}
