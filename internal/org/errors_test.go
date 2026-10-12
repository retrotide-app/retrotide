package org

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/hierarchy"
	"retrotide/internal/repository"
)

var (
	errDatabaseDown        = errors.New("database down")
	errAuditDown           = errors.New("audit log unavailable")
	errCacheDown           = fmt.Errorf("%w: %w", repository.ErrStaleTenantCache, errors.New("valkey unreachable"))
	errOrganisationMissing = fmt.Errorf("get organisation: %w", dao.ErrNotFound)
	errMembershipMissing   = fmt.Errorf("get member: %w", dao.ErrNotFound)
	errNotMember           = fmt.Errorf("%w: user in organisation: %w", hierarchy.ErrNotMember, dao.ErrNotFound)
	errOutranked           = fmt.Errorf("%w: user acting on user", hierarchy.ErrOutranked)
)
