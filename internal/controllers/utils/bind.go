package utils

import (
	"errors"
	"net/http"

	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	InvalidClientIDCode    = "invalid_id"
	invalidClientIDMessage = "a board content ID must be a version 4 UUID"
)

func BindJSON[T any](c *gin.Context) (T, bool) {
	var req T

	err := c.ShouldBindBodyWithJSON(&req)
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, dto.ErrorResponse{Error: "request body too large"})
		return req, false
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return req, false
	}

	carrier, ok := any(req).(dto.ClientIDCarrier)
	if !ok {
		return req, true
	}

	for _, id := range carrier.ClientIDs() {
		if id.Version() != 4 || id.Variant() != uuid.RFC4122 {
			c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: invalidClientIDMessage, Code: InvalidClientIDCode})
			return req, false
		}
	}

	return req, true
}
