package spec

import (
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	RoleAssignment struct {
		UserID uuid.UUID
		Role   role.Role
	}

	RolePermissionsUpdate struct {
		Role        role.Role
		Permissions []string
	}
)
