package controllers

import (
	"errors"
	"net/http"

	"retrotide/internal/auth"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	twoFactorExpiredCode = "two_factor_expired"
)

var (
	twoFactorErrorAnswers = []errorAnswer{
		{err: twofactor.ErrInvalidCode, status: http.StatusBadRequest, code: "invalid_code"},
		{err: twofactor.ErrChallengeExpired, status: http.StatusUnauthorized, code: twoFactorExpiredCode},
		{err: twofactor.ErrAlreadyEnabled, status: http.StatusConflict, code: "two_factor_enabled"},
		{err: twofactor.ErrNotEnrolling, status: http.StatusConflict, code: "two_factor_not_started"},
		{err: twofactor.ErrNotEnabled, status: http.StatusNotFound, code: "two_factor_not_enabled"},
		{err: twofactor.ErrPasswordRequired, status: http.StatusConflict, code: "password_required"},
		{err: auth.ErrUserBanned, status: http.StatusForbidden},
		{err: user.ErrIncorrectPassword, status: http.StatusForbidden, code: "incorrect_password"},
		{err: user.ErrUserNotFound, status: http.StatusNotFound},
	}
)

func (s *Service) getAllTwoFactorRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/auth/two-factor", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.twoFactorStatus},
		{Method: http.MethodPost, Path: "/auth/two-factor/enrolment", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.beginTwoFactorEnrolment},
		{Method: http.MethodPost, Path: "/auth/two-factor/confirmation", Before: gin.HandlersChain{s.Guards.Auth, s.Guards.RateLimitCredentials}, Handler: s.confirmTwoFactorEnrolment},
		{Method: http.MethodDelete, Path: "/auth/two-factor", Before: gin.HandlersChain{s.Guards.Auth, s.Guards.RateLimitCredentials}, Handler: s.disableTwoFactor},
		{Method: http.MethodPost, Path: "/auth/two-factor/verify", Before: gin.HandlersChain{s.Guards.RateLimitCredentials}, Handler: s.verifyTwoFactor},
	}
}

func (s *Service) twoFactorStatus(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	status, err := s.TwoFactorService.Status(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "failed to read the two-factor status", err)
		return
	}

	c.JSON(http.StatusOK, status)
}

func (s *Service) beginTwoFactorEnrolment(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.TwoFactorEnrolmentRequest](c)
	if !ok {
		return
	}

	if req.CurrentPassword == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "current password is required"})
		return
	}

	if !s.chargeTwoFactorAccount(c, userID, "failed to start setting up two-factor authentication") {
		return
	}

	start := spec.TwoFactorEnrolmentStart{UserID: userID, CurrentPassword: req.CurrentPassword, IP: c.ClientIP()}

	enrolment, err := s.TwoFactorService.BeginEnrolment(ctx, start)
	if err != nil {
		answerError(c, twoFactorErrorAnswers, err, "failed to start setting up two-factor authentication")
		return
	}

	c.JSON(http.StatusCreated, enrolment)
}

func (s *Service) confirmTwoFactorEnrolment(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.TwoFactorCodeRequest](c)
	if !ok {
		return
	}

	if !s.chargeTwoFactorAccount(c, userID, "failed to turn on two-factor authentication") {
		return
	}

	token, err := s.Sessions.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the session cookie", err)
		return
	}

	confirmation := spec.TwoFactorConfirmation{TwoFactorCodeEntry: spec.TwoFactorCodeEntry{UserID: userID, Code: req.Code, IP: c.ClientIP()}, KeepToken: token}

	recovery, err := s.TwoFactorService.ConfirmEnrolment(c.Request.Context(), confirmation)
	if errors.Is(err, twofactor.ErrNotAudited) && recovery != nil {
		c.Error(err)
		c.JSON(http.StatusOK, recovery)
		return
	}

	if err != nil {
		answerError(c, twoFactorErrorAnswers, err, "failed to turn on two-factor authentication")
		return
	}

	c.JSON(http.StatusOK, recovery)
}

func (s *Service) disableTwoFactor(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.TwoFactorCodeRequest](c)
	if !ok {
		return
	}

	if !s.chargeTwoFactorAccount(c, userID, "failed to turn off two-factor authentication") {
		return
	}

	entry := spec.TwoFactorCodeEntry{UserID: userID, Code: req.Code, IP: c.ClientIP()}

	err := s.TwoFactorService.Disable(c.Request.Context(), entry)
	if err != nil && !errors.Is(err, twofactor.ErrNotAudited) {
		answerError(c, twoFactorErrorAnswers, err, "failed to turn off two-factor authentication")
		return
	}

	if err != nil {
		c.Error(err)
	}

	noContent(c)
}

func (s *Service) verifyTwoFactor(c *gin.Context) {
	ctx := c.Request.Context()

	req, ok := utils.BindJSON[dto.TwoFactorVerifyRequest](c)
	if !ok {
		return
	}

	challengeToken, err := s.TwoFactorCookies.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the two-factor cookie", err)
		return
	}

	if challengeToken == "" {
		if !s.clearTwoFactorCookie(c) {
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: twofactor.ErrChallengeExpired.Error(), Code: twoFactorExpiredCode})
		return
	}

	owner, err := s.TwoFactorService.ChallengeOwner(ctx, challengeToken)
	if err != nil {
		s.refuseTwoFactorVerification(c, err)
		return
	}

	if !s.chargeTwoFactorAccount(c, owner, "failed to check the two-factor code") {
		return
	}

	verification := spec.TwoFactorVerification{ChallengeToken: challengeToken, Code: req.Code, TrustDevice: req.TrustDevice}

	verified, err := s.AuthService.CompleteTwoFactor(ctx, verification)
	if err != nil {
		s.refuseTwoFactorVerification(c, err)
		return
	}

	if !s.clearTwoFactorCookie(c) {
		return
	}

	if !s.writeSessionCookie(c, verified.SessionToken, verified.Remember) {
		return
	}

	if verified.TrustedDeviceToken != "" && !s.writeTrustedDeviceCookie(c, verified.TrustedDeviceToken) {
		return
	}

	c.JSON(http.StatusOK, dto.TwoFactorVerifiedResponse{User: verified.User, Location: verified.Location})
}

func (s *Service) chargeTwoFactorAccount(c *gin.Context, userID uuid.UUID, msg string) bool {
	account, err := s.UserService.GetByID(c.Request.Context(), userID)
	if err != nil {
		answerError(c, twoFactorErrorAnswers, err, msg)
		return false
	}

	return s.Guards.ChargeCredentialsByAccount(c, accountField, account.Email)
}

func (s *Service) refuseTwoFactorVerification(c *gin.Context, err error) {
	if errors.Is(err, twofactor.ErrChallengeExpired) && !s.clearTwoFactorCookie(c) {
		return
	}

	answerError(c, twoFactorErrorAnswers, err, "failed to check the two-factor code")
}

func (s *Service) writeTrustedDeviceCookie(c *gin.Context, token string) bool {
	cookie, err := s.TrustedDevices.Cookie(c.Request.Context(), token)
	if err != nil {
		utils.InternalError(c, "failed to build the trusted device cookie", err)
		return false
	}

	http.SetCookie(c.Writer, cookie)

	return true
}

func (s *Service) clearTwoFactorCookie(c *gin.Context) bool {
	cookie, err := s.TwoFactorCookies.ExpiredCookie(c.Request.Context())
	if err != nil {
		utils.InternalError(c, "failed to build the two-factor cookie", err)
		return false
	}

	http.SetCookie(c.Writer, cookie)

	return true
}
