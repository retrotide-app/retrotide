package entitlements

import (
	"errors"
	"fmt"

	"retrotide/internal/model"
)

type (
	DeniedError struct {
		Plan    model.PlanCode
		Limit   model.Limit
		Allowed int
		Current int
	}
)

const (
	UpgradeRequiredCode = "upgrade_required"
)

var (
	ErrUpgradeRequired = errors.New("upgrade required")
	ErrUnknownPlan     = errors.New("unknown plan")
	ErrUnknownLimit    = errors.New("unknown limit")
)

func (e *DeniedError) Error() string {
	return fmt.Sprintf("%s: the %s plan allows %d %s and the organisation has %d", ErrUpgradeRequired, e.Plan, e.Allowed, e.Limit, e.Current)
}

func (e *DeniedError) Unwrap() error {
	return ErrUpgradeRequired
}
