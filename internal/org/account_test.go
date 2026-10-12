package org

import (
	"fmt"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleteAccount_DeletesAnAccountThatOwnsNoSharedOrganisation(t *testing.T) {
	// given the repository soft-deletes what the user owns alone in the same transaction
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().DeleteAccount(mock.Anything, userID).Return(nil)

	// when
	err := svc.DeleteAccount(t.Context(), userID)

	// then
	assert.NoError(t, err)
}

func TestDeleteAccount_MapsWhatTheRepositoryRefused(t *testing.T) {
	tests := []struct {
		name     string
		refusal  error
		want     error
		wantBare bool
	}{
		{name: "the sole owner of an organisation with other members", refusal: fmt.Errorf("delete account: %w: acme-retros", repository.ErrOwnsSharedOrganisation), want: ErrSoleOwner, wantBare: true},
		{name: "an account that is gone", refusal: fmt.Errorf("delete account: %w", dao.ErrNotFound), want: ErrAccountNotFound, wantBare: true},
		{name: "a failed write", refusal: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			userID := uuid.New()
			m.users.EXPECT().DeleteAccount(mock.Anything, userID).Return(tc.refusal)

			// when
			err := svc.DeleteAccount(t.Context(), userID)

			// then
			assert.ErrorIs(t, err, tc.want)
			if tc.wantBare {
				assert.Equal(t, tc.want, err)
			}
		})
	}
}
