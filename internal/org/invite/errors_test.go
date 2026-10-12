package invite

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/repository"
)

var (
	errDatabaseDown = errors.New("database down")
	errMissing      = fmt.Errorf("the row is missing: %w", dao.ErrNotFound)
	errStaleCache   = fmt.Errorf("%w: valkey down", repository.ErrStaleTenantCache)
)
