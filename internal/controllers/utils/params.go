package utils

import (
	"net/http"
	"strconv"

	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ParseIDParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + name})
		return uuid.Nil, false
	}

	return id, true
}

func QueryInt(c *gin.Context, name string, fallback int) (int, bool) {
	raw, present := c.GetQuery(name)
	if !present {
		return fallback, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + name})
		return 0, false
	}

	return value, true
}
