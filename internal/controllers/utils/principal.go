package utils

import (
	"retrotide/internal/authctx"
	"retrotide/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Principal(c *gin.Context) (model.Principal, bool) {
	principal, ok := authctx.Principal(c.Request.Context())
	if !ok {
		InternalError(c, "failed to identify the caller", errNoPrincipal)
		return model.Principal{}, false
	}

	return principal, true
}

func OptionalPrincipal(c *gin.Context) *model.Principal {
	principal, ok := authctx.Principal(c.Request.Context())
	if !ok {
		return nil
	}

	return &principal
}

func UserID(c *gin.Context) (uuid.UUID, bool) {
	principal, ok := Principal(c)
	if !ok {
		return uuid.Nil, false
	}

	if principal.Kind != model.PrincipalUser {
		InternalError(c, "failed to identify the signed-in user", errNoUserPrincipal)
		return uuid.Nil, false
	}

	return principal.ID, true
}
