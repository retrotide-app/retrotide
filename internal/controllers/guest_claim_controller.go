package controllers

import (
	"errors"
	"net/http"

	"retrotide/internal/board/live"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/guest"
	"retrotide/internal/model/spec"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	guestClaimErrorAnswers = []errorAnswer{
		{err: guest.ErrNothingClaimed, status: http.StatusBadRequest, code: "nothing_claimed"},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, code: "board_busy"},
	}
)

func (s *Service) getAllGuestClaimRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/auth/guest-claim", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.guestClaimOffer},
		{Method: http.MethodPost, Path: "/auth/guest-claim", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.claimGuest},
		{Method: http.MethodDelete, Path: "/auth/guest-claim", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.declineGuestClaim},
	}
}

func (s *Service) guestClaimOffer(c *gin.Context) {
	deviceID, _, ok := s.heldGuestDevice(c)
	if !ok {
		return
	}

	offer, err := s.GuestClaims.Offer(c.Request.Context(), deviceID)
	if err != nil {
		utils.InternalError(c, "failed to list the boards this browser joined as a guest", err)
		return
	}

	c.JSON(http.StatusOK, offer)
}

func (s *Service) claimGuest(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.GuestClaimRequest](c)
	if !ok {
		return
	}

	deviceID, held, ok := s.heldGuestDevice(c)
	if !ok {
		return
	}

	if !held {
		c.AbortWithStatusJSON(http.StatusConflict, dto.ErrorResponse{Error: "this browser holds no guest identity to claim", Code: "no_guest"})
		return
	}

	result, err := s.GuestClaims.Claim(c.Request.Context(), spec.GuestClaim{UserID: userID, DeviceID: deviceID, BoardIDs: req.BoardIDs})
	if err != nil && !errors.Is(err, guest.ErrNotAudited) {
		answerError(c, guestClaimErrorAnswers, err, "failed to claim the guest boards")
		return
	}

	if err != nil {
		c.Error(err)
	}

	if !s.clearGuestCookie(c) {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) declineGuestClaim(c *gin.Context) {
	if !s.clearGuestCookie(c) {
		return
	}

	noContent(c)
}

func (s *Service) heldGuestDevice(c *gin.Context) (uuid.UUID, bool, bool) {
	token, err := s.Guests.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the guest cookie", err)
		return uuid.Nil, false, false
	}

	if token == "" {
		return uuid.Nil, false, true
	}

	deviceID, err := s.Guests.Validate(c.Request.Context(), token)
	if errors.Is(err, guest.ErrInvalidGuest) {
		return uuid.Nil, false, true
	}
	if err != nil {
		utils.InternalError(c, "failed to check the guest cookie", err)
		return uuid.Nil, false, false
	}

	return deviceID, true, true
}

func (s *Service) clearGuestCookie(c *gin.Context) bool {
	cookie, err := s.Guests.ExpiredCookie(c.Request.Context())
	if err != nil {
		utils.InternalError(c, "failed to build the guest cookie", err)
		return false
	}

	http.SetCookie(c.Writer, cookie)

	return true
}
