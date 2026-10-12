package engines

import "errors"

var (
	errNotConnected = errors.New("cache: valkey is configured but not connected")
)
