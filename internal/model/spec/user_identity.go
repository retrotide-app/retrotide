package spec

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewUserIdentity struct {
		UserID   uuid.UUID
		Provider model.IdentityProvider
		Subject  string
		Email    string
	}

	IdentitySubject struct {
		Provider model.IdentityProvider
		Subject  string
	}

	UserIdentityKey struct {
		UserID   uuid.UUID
		Provider model.IdentityProvider
	}

	IdentityRegistration struct {
		User         NewUser
		FirstAccount bool
		Provider     model.IdentityProvider
		Subject      string
	}

	IdentityLink struct {
		Identity     NewUserIdentity
		Automatic    bool
		MatchedEmail string
	}

	IdentityUnlinking struct {
		UserID   uuid.UUID
		Provider model.IdentityProvider
		IP       string
	}
)
