package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewTeam struct {
		OrgID    uuid.UUID
		Name     string
		MaxTeams int
	}

	TeamKey struct {
		OrgID  uuid.UUID
		TeamID uuid.UUID
	}

	TeamRename struct {
		OrgID  uuid.UUID
		TeamID uuid.UUID
		Name   string
	}

	NewTeamMember struct {
		OrgID  uuid.UUID
		TeamID uuid.UUID
		UserID uuid.UUID
		Role   model.TeamRole
	}

	TeamMemberKey struct {
		OrgID  uuid.UUID
		TeamID uuid.UUID
		UserID uuid.UUID
	}

	TeamMemberRoleChange struct {
		OrgID  uuid.UUID
		TeamID uuid.UUID
		UserID uuid.UUID
		Role   model.TeamRole
	}

	TeamPermissionCheck struct {
		TeamID     uuid.UUID
		UserID     uuid.UUID
		Permission model.TenantPermission
	}
)
