package live

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
)

var (
	errDatabase     = errors.New("the database is unreachable")
	errDatabaseDown = errors.New("database down")
	errNotFound     = fmt.Errorf("lookup: %w", dao.ErrNotFound)
	errNoRow        = fmt.Errorf("the row is gone: %w", dao.ErrNotFound)
)
