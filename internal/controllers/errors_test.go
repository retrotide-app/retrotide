package controllers

import (
	"errors"
	"fmt"

	"retrotide/internal/admin"
	"retrotide/internal/dao"
	"retrotide/internal/identity"
)

var (
	errServiceDown    = errors.New("database down")
	errAuditLost      = fmt.Errorf("%w: audit log unavailable", admin.ErrNotAudited)
	errDiscoveryDown  = fmt.Errorf("%w: %w", identity.ErrProviderUnavailable, errServiceDown)
	errTenantNoMember = fmt.Errorf("get member: %w", dao.ErrNotFound)
)
