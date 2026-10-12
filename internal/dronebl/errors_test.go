package dronebl

import "errors"

var (
	errDNSBroken    = errors.New("server misbehaving")
	errSettingsDown = errors.New("settings unavailable")
)
