package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	InviteCreation struct {
		OrgID   uuid.UUID
		ActorID uuid.UUID
		Email   string
		Role    model.OrgRole
		TeamID  *uuid.UUID
		MaxUses *int
		IP      string
	}

	InviteAction struct {
		OrgID    uuid.UUID
		InviteID uuid.UUID
		ActorID  uuid.UUID
		IP       string
	}

	InviteJoin struct {
		Token  string
		UserID uuid.UUID
		IP     string
	}

	InviteRegistration struct {
		Token       string
		Email       string
		DisplayName string
		Password    string
		IP          string
	}
)
