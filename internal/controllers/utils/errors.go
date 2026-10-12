package utils

import "errors"

var (
	errNoPrincipal     = errors.New("the route carries no principal guard, so the request has no caller")
	errNoUserPrincipal = errors.New("the route carries no user guard, so the caller is not a signed-in user")
)
