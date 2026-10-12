package board

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
)

var (
	errDatabase = errors.New("the database is unreachable")
	errNotFound = fmt.Errorf("lookup: %w", dao.ErrNotFound)
)
