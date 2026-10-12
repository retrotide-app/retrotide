package testutil

import "errors"

var (
	errSessionStoreDown    = errors.New("session store down")
	errRoleStoreDown       = errors.New("role store down")
	errMembershipStoreDown = errors.New("membership store down")
)
