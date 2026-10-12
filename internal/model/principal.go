package model

import (
	"github.com/google/uuid"
)

type (
	PrincipalKind string

	Principal struct {
		Kind PrincipalKind
		ID   uuid.UUID
	}
)

const (
	PrincipalUser  PrincipalKind = "user"
	PrincipalGuest PrincipalKind = "guest"
)

func (p Principal) Guest() bool {
	return p.Kind == PrincipalGuest
}
