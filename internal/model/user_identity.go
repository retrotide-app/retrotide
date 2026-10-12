package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	IdentityProvider string

	UserIdentity struct {
		Provider  IdentityProvider
		Subject   string
		UserID    uuid.UUID
		Email     string
		CreatedAt time.Time
	}

	ProviderIdentity struct {
		Provider           IdentityProvider
		Subject            string
		Email              string
		EmailAliases       []string
		EmailVerified      bool
		EmailAuthoritative bool
		Name               string
	}
)

const (
	ProviderGoogle    IdentityProvider = "google"
	ProviderMicrosoft IdentityProvider = "microsoft"
)
