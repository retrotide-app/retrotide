package controllers

import (
	"context"
	"fmt"
	"net/http"

	"retrotide/internal/authctx"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/identity"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/user"

	"github.com/gin-gonic/gin"
)

const (
	providerParam = "provider"

	reauthenticationRequiredCode = "reauthentication_required"
)

var (
	identityErrorAnswers = []errorAnswer{
		{err: identity.ErrProviderNotConfigured, status: http.StatusNotFound},
		{err: identity.ErrProviderUnavailable, status: http.StatusServiceUnavailable, recordCause: true},
		{err: identity.ErrUnknownPurpose, status: http.StatusBadRequest},
		{err: identity.ErrSignInRequired, status: http.StatusUnauthorized},
		{err: identity.ErrAccountLocked, status: http.StatusForbidden},
		{err: identity.ErrAccountUnverified, status: http.StatusForbidden, code: "email_unverified"},
		{err: identity.ErrIdentityNotFound, status: http.StatusNotFound},
		{err: identity.ErrLastSignInMethod, status: http.StatusConflict, code: "last_sign_in_method"},
		{err: identity.ErrReauthenticationRequired, status: http.StatusBadRequest, code: reauthenticationRequiredCode},
		{err: user.ErrIncorrectPassword, status: http.StatusForbidden, code: "incorrect_password"},
		{err: user.ErrUserNotFound, status: http.StatusNotFound},
	}
)

func (s *Service) getAllIdentityRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/auth/:provider/start", Before: gin.HandlersChain{s.Guards.RateLimitCredentials, s.Guards.OptionalAuth}, Handler: s.startIdentityFlow},
		{Method: http.MethodGet, Path: "/auth/:provider/callback", Handler: s.finishIdentityFlow},
	}
}

func (s *Service) startIdentityFlow(c *gin.Context) {
	ctx := c.Request.Context()
	provider := model.IdentityProvider(c.Param(providerParam))

	enabled, err := s.IdentityService.Enabled(ctx, provider)
	if err != nil {
		utils.InternalError(c, "failed to start the sign-in", err)
		return
	}

	if !enabled {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return
	}

	req, ok := utils.BindJSON[dto.IdentityStartRequest](c)
	if !ok {
		return
	}

	start := spec.IdentityFlowStart{Provider: provider, Purpose: model.OIDCFlowPurpose(req.Purpose), ReturnTo: req.ReturnTo}
	if start.Purpose == model.OIDCFlowLink {
		start, ok = s.linkStart(c, start, req)
		if !ok {
			return
		}
	}

	started, err := s.IdentityService.Start(ctx, start)
	if err != nil {
		answerError(c, identityErrorAnswers, err, "failed to start the sign-in")
		return
	}

	cookie, err := s.FlowCookies.Cookie(ctx, started.FlowToken)
	if err != nil {
		utils.InternalError(c, "failed to build the sign-in flow cookie", err)
		return
	}

	http.SetCookie(c.Writer, cookie)

	c.JSON(http.StatusOK, dto.IdentityStartResponse{URL: started.AuthURL})
}

func (s *Service) linkStart(c *gin.Context, start spec.IdentityFlowStart, req dto.IdentityStartRequest) (spec.IdentityFlowStart, bool) {
	ctx := c.Request.Context()

	userID, signedIn := authctx.UserID(ctx)
	if !signedIn {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: identity.ErrSignInRequired.Error()})
		return start, false
	}

	account, err := s.UserService.GetByID(ctx, userID)
	if err != nil {
		answerError(c, identityErrorAnswers, err, "failed to start the link")
		return start, false
	}

	if account.HasPassword && req.CurrentPassword == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "current password is required"})
		return start, false
	}

	if !account.HasPassword && req.ReauthenticateWith == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: identity.ErrReauthenticationRequired.Error(), Code: reauthenticationRequiredCode})
		return start, false
	}

	if !s.Guards.ChargeCredentialsByAccount(c, accountField, account.Email) {
		return start, false
	}

	sessionToken, err := s.Sessions.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the session cookie", err)
		return start, false
	}

	start.UserID = userID
	start.SessionToken = sessionToken
	start.CurrentPassword = req.CurrentPassword
	start.ReauthenticateWith = model.IdentityProvider(req.ReauthenticateWith)

	return start, true
}

func (s *Service) finishIdentityFlow(c *gin.Context) {
	ctx := context.WithoutCancel(c.Request.Context())
	provider := model.IdentityProvider(c.Param(providerParam))

	enabled, err := s.IdentityService.Enabled(ctx, provider)
	if err != nil {
		redirectAfterFailure(c, provider, fmt.Errorf("check whether %s sign-in is on: %w", provider, err))
		return
	}

	if !enabled {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return
	}

	flowToken, err := s.FlowCookies.Token(c.Request)
	if err != nil {
		redirectAfterFailure(c, provider, err)
		return
	}

	sessionToken, err := s.Sessions.Token(c.Request)
	if err != nil {
		redirectAfterFailure(c, provider, err)
		return
	}

	trustedDevice, err := s.TrustedDevices.Token(c.Request)
	if err != nil {
		redirectAfterFailure(c, provider, err)
		return
	}

	callback := spec.IdentityCallback{
		Provider:           provider,
		FlowToken:          flowToken,
		State:              c.Query("state"),
		Code:               c.Query("code"),
		ProviderError:      c.Query("error"),
		SessionToken:       sessionToken,
		TrustedDeviceToken: trustedDevice,
		IP:                 c.ClientIP(),
	}

	outcome, err := s.IdentityService.Finish(ctx, callback)
	if err != nil {
		c.Error(err)
	}

	flowCookie, err := s.nextFlowCookie(ctx, outcome.FlowToken)
	if err != nil {
		redirectAfterFailure(c, provider, err)
		return
	}

	http.SetCookie(c.Writer, flowCookie)

	if outcome.SessionToken != "" {
		cookie, err := s.Sessions.Cookie(ctx, outcome.SessionToken)
		if err != nil {
			redirectAfterFailure(c, provider, fmt.Errorf("build the session cookie: %w", err))
			return
		}

		http.SetCookie(c.Writer, cookie)
	}

	if outcome.ChallengeToken != "" {
		cookie, err := s.TwoFactorCookies.Cookie(ctx, outcome.ChallengeToken)
		if err != nil {
			redirectAfterFailure(c, provider, fmt.Errorf("build the two-factor cookie: %w", err))
			return
		}

		http.SetCookie(c.Writer, cookie)
	}

	c.Redirect(http.StatusSeeOther, outcome.Location)
}

func (s *Service) nextFlowCookie(ctx context.Context, flowToken string) (*http.Cookie, error) {
	if flowToken == "" {
		cleared, err := s.FlowCookies.ExpiredCookie(ctx)
		if err != nil {
			return nil, fmt.Errorf("build the expired sign-in flow cookie: %w", err)
		}

		return cleared, nil
	}

	cookie, err := s.FlowCookies.Cookie(ctx, flowToken)
	if err != nil {
		return nil, fmt.Errorf("build the sign-in flow cookie: %w", err)
	}

	return cookie, nil
}

func redirectAfterFailure(c *gin.Context, provider model.IdentityProvider, err error) {
	c.Error(err)

	c.Redirect(http.StatusSeeOther, identity.RefusalPath(identity.LoginPage, identity.CodeFailed, provider))
}
