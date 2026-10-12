package spec

import (
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	StaffAction struct {
		ActorID uuid.UUID
		IP      string
	}

	StaffUserAction struct {
		StaffAction

		UserID uuid.UUID
	}

	StaffUserRestriction struct {
		StaffAction

		UserID uuid.UUID
		Reason string
	}

	StaffRoleChange struct {
		StaffAction

		UserID uuid.UUID
		Role   role.Role
	}

	StaffEmailChange struct {
		StaffAction

		UserID uuid.UUID
		Email  string
	}

	StaffDisplayNameChange struct {
		StaffAction

		UserID      uuid.UUID
		DisplayName string
	}

	StaffSettingsUpdate struct {
		StaffAction

		Values map[string]string
	}

	StaffRolePermissionsUpdate struct {
		StaffAction

		Role        role.Role
		Permissions []string
	}
)
