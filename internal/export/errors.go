package export

import "errors"

var (
	ErrUnknownFormat          = errors.New("unknown export format")
	ErrOrganisationNotFound   = errors.New("organisation not found")
	ErrOrganisationNotAllowed = errors.New("only an owner or an admin can export the organisation")
	ErrAccountNotFound        = errors.New("account not found")
)
