package microsoft

import "errors"

var (
	errSettingsDown      = errors.New("settings unavailable")
	errUnexpectedSetting = errors.New("the adapter read a setting the test did not provide")
)
