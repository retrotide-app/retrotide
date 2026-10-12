package spec

import (
	"github.com/google/uuid"
)

type (
	UserActionCheck struct {
		ActorID  uuid.UUID
		TargetID uuid.UUID
	}

	MemberActionCheck struct {
		OrgID    uuid.UUID
		ActorID  uuid.UUID
		TargetID uuid.UUID
	}

	TeamMemberActionCheck struct {
		OrgID    uuid.UUID
		TeamID   uuid.UUID
		ActorID  uuid.UUID
		TargetID uuid.UUID
	}
)
