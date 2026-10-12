package microsoft

import (
	"context"

	"github.com/google/uuid"
)

const (
	canonicalGUIDLength = 36
)

func ValidateTenantID(_ context.Context, value string) error {
	if value == "" {
		return nil
	}

	if !isGUID(value) {
		return errInvalidTenantID
	}

	return nil
}

func isGUID(value string) bool {
	_, err := uuid.Parse(value)

	return err == nil && len(value) == canonicalGUIDLength
}
