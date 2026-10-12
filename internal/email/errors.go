package email

import "errors"

var (
	ErrNotConfigured = errors.New("no email provider is configured")
	ErrFailing       = errors.New("email is set up but its settings do not work")
)
