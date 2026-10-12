package admin

import (
	"errors"

	"retrotide/internal/settings"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrProtectedUser        = errors.New("this user cannot be modified")
	ErrUnknownRole          = errors.New("unknown role")
	ErrRoleOutranksActor    = errors.New("cannot grant a role equal to or above your own")
	ErrRoleNotHeld          = errors.New("the user does not hold that role")
	ErrImmutableRole        = errors.New("this role's permissions cannot be edited")
	ErrUnknownPermission    = errors.New("unknown permission")
	ErrRestrictedPermission = errors.New("this permission is reserved for admins and cannot be granted to another role")
	ErrReasonRequired       = errors.New("a reason is required")
	ErrInvalidEmail         = errors.New("a valid email address is required")
	ErrEmailUnchanged       = errors.New("that is already the user's email address")
	ErrEmailTaken           = errors.New("that email address is already in use")
	ErrEmailUnavailable     = errors.New("email is not available on this site")
	ErrEmailNotSent         = errors.New("the new address is pending, but its emails could not be sent")
	ErrDisplayNameRequired  = errors.New("a display name is required")
	ErrUnknownSetting       = errors.New("unknown setting")
	ErrInvalidSetting       = settings.ErrInvalidValue
	ErrSettingsCacheStale   = errors.New("the settings were saved, but their cached copies could not be refreshed")
	ErrNoSignInMethod       = errors.New("password sign-in can be turned off only on a public site with a sign-in provider configured")
	ErrStaffCacheStale      = errors.New("the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply")
	ErrNotAudited           = errors.New("the change was made, but its audit entry was not recorded")
	ErrSitePublic           = errors.New("invitations are only for a private site")
	ErrAccountExists        = errors.New("that email address already has an account")
	ErrInvitationPending    = errors.New("that email address already has a pending invitation")
	ErrInvitationNotFound   = errors.New("invitation not found")
	ErrInvitationNotSent    = errors.New("the invitation was saved, but its email was not sent, so resend it")
	ErrOrganisationNotFound = errors.New("organisation not found")
	ErrSoleOwner            = errors.New("the user owns an organisation that has other members, so its ownership must be transferred or the organisation deleted first")
)
