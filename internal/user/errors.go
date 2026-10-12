package user

import "errors"

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrIncorrectPassword   = errors.New("the current password is incorrect")
	ErrPasswordTooShort    = errors.New("the password is too short")
	ErrPasswordTooLong     = errors.New("the password is longer than 72 bytes")
	ErrDisplayNameRequired = errors.New("a display name is required")
)
