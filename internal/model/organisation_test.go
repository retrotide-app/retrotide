package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganisation_RestorableUntilEndsThirtyDaysAfterTheDeletion(t *testing.T) {
	// given
	deletedAt := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	deleted := Organisation{DeletedAt: &deletedAt}

	// when
	until := deleted.RestorableUntil()

	// then
	require.NotNil(t, until)
	assert.Equal(t, time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC), *until)
}

func TestOrganisation_ALiveOrganisationHasNoRestoreWindow(t *testing.T) {
	// given
	live := Organisation{}

	// when
	until := live.RestorableUntil()

	// then
	assert.Nil(t, until)
}
