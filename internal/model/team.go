package model

import (
	"time"

	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	TeamRole string

	Team struct {
		ID         uuid.UUID
		OrgID      uuid.UUID
		Name       string
		CreatedAt  time.Time
		ArchivedAt *time.Time
	}

	TeamWithCounts struct {
		Team

		MemberCount int
		BoardCount  int
	}

	MemberTeam struct {
		Team

		Role     TeamRole
		JoinedAt time.Time
	}

	TeamMember struct {
		OrgID    uuid.UUID
		TeamID   uuid.UUID
		UserID   uuid.UUID
		Role     TeamRole
		JoinedAt time.Time
	}

	TeamMemberProfile struct {
		TeamMember

		Email       string
		DisplayName string
		OrgRole     OrgRole
		SiteRole    role.Role
	}
)

const (
	TeamRoleLead   TeamRole = "lead"
	TeamRoleMember TeamRole = "member"
)

const (
	TeamNameMaxRunes = 80
)

func (t Team) Archived() bool {
	return t.ArchivedAt != nil
}
