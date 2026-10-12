package session

import "errors"

var (
	errDatabaseDown = errors.New("database down")
	errSettingsDown = errors.New("settings unreadable")
)
