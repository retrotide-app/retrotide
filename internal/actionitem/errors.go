package actionitem

import "errors"

var (
	ErrOrganisationNotFound = errors.New("organisation not found")
	ErrInvalidStatus        = errors.New("an action item's status must be open, done or dropped")
)
