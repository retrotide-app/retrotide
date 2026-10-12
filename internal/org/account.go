package org

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

func (s *service) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	err := s.users.DeleteAccount(ctx, userID)
	if errors.Is(err, repository.ErrOwnsSharedOrganisation) {
		return ErrSoleOwner
	}
	if errors.Is(err, dao.ErrNotFound) {
		return ErrAccountNotFound
	}
	if err != nil {
		return fmt.Errorf("delete the account of user %s: %w", userID, err)
	}

	return nil
}
