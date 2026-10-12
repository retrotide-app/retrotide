package auth

import "errors"

var (
	errDatabaseDown = errors.New("database down")
	errSMTPRefused  = errors.New("smtp refused the message")
)
