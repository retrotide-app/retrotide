package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"

	"github.com/gin-gonic/gin"
)

func (s *Service) getAllNotificationRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/notifications", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.listNotifications},
		{Method: http.MethodGet, Path: "/notifications/unread-count", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.unreadNotificationCount},
		{Method: http.MethodPost, Path: "/notifications/read", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.markAllNotificationsRead},
		{Method: http.MethodPost, Path: "/notifications/:id/read", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.markNotificationRead},
	}
}

func (s *Service) listNotifications(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	limit, ok := utils.QueryInt(c, "limit", 0)
	if !ok {
		return
	}

	offset, ok := utils.QueryInt(c, "offset", 0)
	if !ok {
		return
	}

	listing, err := s.NotificationService.List(c.Request.Context(), spec.NotificationListing{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		utils.InternalError(c, "failed to list the notifications", err)
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) unreadNotificationCount(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	count, err := s.NotificationService.UnreadCount(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "failed to count the unread notifications", err)
		return
	}

	c.JSON(http.StatusOK, dto.UnreadCountResponse{Count: count})
}

func (s *Service) markAllNotificationsRead(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	if err := s.NotificationService.MarkAllRead(c.Request.Context(), userID); err != nil {
		utils.InternalError(c, "failed to mark the notifications read", err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) markNotificationRead(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id"})
		return
	}

	err = s.NotificationService.MarkRead(c.Request.Context(), spec.NotificationLookup{UserID: userID, ID: id})
	if errors.Is(err, dao.ErrNotFound) {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: "notification not found"})
		return
	}
	if err != nil {
		utils.InternalError(c, "failed to mark the notification read", err)
		return
	}

	c.Status(http.StatusNoContent)
}
