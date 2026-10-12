package team

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/hierarchy"
	"retrotide/internal/repository"
)

var (
	errDatabaseDown  = errors.New("database down")
	errAuditDown     = errors.New("audit log unavailable")
	errCacheDown     = fmt.Errorf("%w: %w", repository.ErrStaleTenantCache, errors.New("valkey unreachable"))
	errRowMissing    = fmt.Errorf("the row is gone: %w", dao.ErrNotFound)
	errRowConflicted = fmt.Errorf("the row conflicts: %w", dao.ErrConflict)
	errNotMember     = fmt.Errorf("%w: user in organisation", hierarchy.ErrNotMember)
	errOutranked     = fmt.Errorf("%w: user acting on user", hierarchy.ErrOutranked)
)
