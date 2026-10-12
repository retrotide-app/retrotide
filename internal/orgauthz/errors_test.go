package orgauthz

import (
	"errors"
	"fmt"

	"retrotide/internal/dao"
)

var (
	errDatabaseDown = errors.New("database down")
	errNoRow        = fmt.Errorf("the row is gone: %w", dao.ErrNotFound)
)
