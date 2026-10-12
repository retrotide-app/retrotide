package admin

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
)

var (
	errDatabaseDown = errors.New("database down")
	errAuditDown    = errors.New("audit log unavailable")
	errRowMissing   = fmt.Errorf("the row is gone: %w", dao.ErrNotFound)
)
