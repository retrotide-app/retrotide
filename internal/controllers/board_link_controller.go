package controllers

import (
	"net/http"

	"retrotide/internal/board/live"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"

	"github.com/gin-gonic/gin"
)

const (
	slugParam = "slug"
)

func (s *Service) getAllBoardLinkRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/b/:slug", Before: gin.HandlersChain{s.Guards.OptionalPrincipal}, Handler: s.previewBoardLink},
		{Method: http.MethodPost, Path: "/b/:slug/join", Before: gin.HandlersChain{s.Guards.OptionalPrincipal, s.Guards.BoardJoinLimit}, Handler: s.joinBoardLink},
	}
}

func (s *Service) previewBoardLink(c *gin.Context) {
	query := live.LinkQuery{Principal: utils.OptionalPrincipal(c), Slug: c.Param(slugParam)}

	preview, err := s.BoardLive.PreviewLink(c.Request.Context(), query)
	if boardFailed(c, err, "failed to read the share link") {
		return
	}

	c.JSON(http.StatusOK, preview)
}

func (s *Service) joinBoardLink(c *gin.Context) {
	req, ok := utils.BindJSON[dto.JoinBoardRequest](c)
	if !ok {
		return
	}

	command := live.LinkJoinCommand{
		Principal:   utils.OptionalPrincipal(c),
		Slug:        c.Param(slugParam),
		DisplayName: req.DisplayName,
		Password:    req.Password,
	}

	joined, err := s.BoardLive.JoinByLink(c.Request.Context(), command)
	if boardFailed(c, err, "failed to join the board") {
		return
	}

	if joined.GuestToken != "" {
		cookie, err := s.Guests.Cookie(c.Request.Context(), joined.GuestToken)
		if err != nil {
			utils.InternalError(c, "failed to build the guest cookie", err)
			return
		}

		http.SetCookie(c.Writer, cookie)
	}

	c.JSON(http.StatusOK, joined.Response)
}
