package twofactor

import "errors"

var (
	ErrInvalidCode      = errors.New("that code is not right")
	ErrChallengeExpired = errors.New("signing in took too long, sign in again")
	ErrAlreadyEnabled   = errors.New("two-factor authentication is already on")
	ErrNotEnrolling     = errors.New("start setting up two-factor authentication first")
	ErrNotEnabled       = errors.New("two-factor authentication is not on")
	ErrPasswordRequired = errors.New("set a password before you turn on two-factor authentication")
	ErrNotAudited       = errors.New("the change was made, but its audit entry was not written")
)
