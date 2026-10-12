package savedtemplate

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
)

var (
	errDatabaseDown  = errors.New("database down")
	errRowMissing    = fmt.Errorf("the row is gone: %w", dao.ErrNotFound)
	errQuotaUsed     = fmt.Errorf("the insert found no room: %w", dao.ErrQuotaExceeded)
	errRowConflicted = fmt.Errorf("the row conflicts: %w", dao.ErrConflict)
)
