package orgauthz

import "errors"

var (
	ErrUnknownPermission = errors.New("unknown organisation or team permission")
	ErrOutOfScope        = errors.New("the permission belongs to the other scope")
)
