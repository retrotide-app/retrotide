package logger

import "errors"

var (
	errEmptyLevel = errors.New("log level must not be empty")
)
