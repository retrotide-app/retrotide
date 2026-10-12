package org

import "errors"

var (
	ErrOrganisationNotFound    = errors.New("organisation not found")
	ErrMemberNotFound          = errors.New("that person is not a member of the organisation")
	ErrNameRequired            = errors.New("an organisation name is required")
	ErrNameTooLong             = errors.New("an organisation name can be at most 80 characters")
	ErrInvalidSlug             = errors.New("the address must be 3 to 40 lower-case letters, digits or hyphens, starting and ending with a letter or digit")
	ErrSlugReserved            = errors.New("that address is reserved")
	ErrSlugTaken               = errors.New("that address is already taken")
	ErrOwnedLimitReached       = errors.New("you already own as many organisations as you can")
	ErrInvalidRole             = errors.New("the role must be admin or member")
	ErrOwnerProtected          = errors.New("the owner's role cannot be changed and the owner cannot be removed, so transfer ownership first")
	ErrAdminsNeedOwner         = errors.New("only the owner can make, unmake or remove an admin")
	ErrOwnerCannotLeave        = errors.New("the owner cannot leave the organisation, so transfer ownership or delete it first")
	ErrOwnershipNotTransferred = errors.New("ownership was not transferred, because the organisation, its owner or the new owner changed meanwhile")
	ErrNewOwnerLimitReached    = errors.New("the new owner already owns as many organisations as they can")
	ErrNotOwner                = errors.New("only the owner can restore the organisation")
	ErrOrganisationNotDeleted  = errors.New("the organisation is not deleted")
	ErrNotRestorable           = errors.New("the organisation can no longer be restored")
	ErrSoleOwner               = errors.New("you own an organisation that has other members, so transfer ownership or delete it before deleting your account")
	ErrAccountNotFound         = errors.New("account not found")
	ErrNotAudited              = errors.New("the change was made, but its audit entry was not recorded")
	ErrCacheStale              = errors.New("the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply")
)
