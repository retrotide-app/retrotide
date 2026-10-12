package repository

import "errors"

var (
	ErrAccountBanned          = errors.New("the account is banned")
	ErrAccountChanged         = errors.New("the account's email address or its verification changed before the link")
	ErrAlreadyLinked          = errors.New("the account already holds an identity from this provider")
	ErrIdentityInUse          = errors.New("the identity is linked to another account")
	ErrNotFirstAccount        = errors.New("another account already exists")
	ErrOwnsSharedOrganisation = errors.New("the account owns an organisation that has other members")
	ErrStaleSettingsCache     = errors.New("settings saved, but their cached copies could not be refreshed")
	ErrStaleStaffCache        = errors.New("saved, but the cached copy could not be cleared")
	ErrStaleTenantCache       = errors.New("saved, but a cached membership or team could not be cleared, so the old one can apply for up to a minute")

	errSettingKeptChanging = errors.New("setting changed again on every attempt to cache it")
)
