package auth

import "errors"

var (
	ErrRegistrationPrivate      = errors.New("this site is private, ask an admin to invite you")
	ErrRegistrationClosed       = errors.New("sign-up is closed until this site can send email")
	ErrInvalidEmail             = errors.New("a valid email address is required")
	ErrEmailTaken               = errors.New("that email address is already in use")
	ErrEmailUnchanged           = errors.New("that is already your email address")
	ErrUserBanned               = errors.New("your account has been banned")
	ErrEmailUnverified          = errors.New("verify your email address before signing in")
	ErrInvalidVerificationToken = errors.New("the verification link is invalid or has expired")
	ErrInvalidResetToken        = errors.New("the reset link is invalid or has expired")
	ErrInvalidInvitation        = errors.New("the invitation link is invalid, used or has expired")
	ErrEmailUnavailable         = errors.New("email is not available on this site")
	ErrEmailNotSent             = errors.New("the account was created, but its verification email was not sent")
	ErrVerificationNotSent      = errors.New("the verification email was not sent")
	ErrNotAudited               = errors.New("the account was created, but its audit entry was not recorded")
	ErrPasswordSignInDisabled   = errors.New("password sign-in is turned off on this site")
)
