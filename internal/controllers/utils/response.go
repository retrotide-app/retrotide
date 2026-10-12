package utils

import (
	"errors"
	"fmt"
	"net/http"

	"retrotide/internal/dto"
	"retrotide/internal/entitlements"

	"github.com/gin-gonic/gin"
)

func InternalError(c *gin.Context, msg string, err error) {
	c.Error(fmt.Errorf("%s: %w", msg, err))

	c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{Error: msg})
}

func MapEntitlementError(c *gin.Context, err error) bool {
	denied, ok := errors.AsType[*entitlements.DeniedError](err)
	if !ok {
		return false
	}

	c.AbortWithStatusJSON(http.StatusPaymentRequired, dto.UpgradeRequiredResponse{
		Error:   entitlements.ErrUpgradeRequired.Error(),
		Code:    entitlements.UpgradeRequiredCode,
		Limit:   denied.Limit,
		Allowed: denied.Allowed,
		Current: denied.Current,
		Plan:    denied.Plan,
	})

	return true
}
