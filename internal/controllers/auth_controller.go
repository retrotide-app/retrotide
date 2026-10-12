package controllers

import (
	"errors"
	"net/http"

	"retrotide/internal/auth"
	"retrotide/internal/authctx"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/identity"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org"
	"retrotide/internal/user"

	"github.com/gin-gonic/gin"
)

type (
	errorAnswer struct {
		err         error
		status      int
		code        string
		recordCause bool
	}
)

const (
	accountField         = "email"
	emailUnavailableCode = "email_unavailable"
)

var (
	authErrorAnswers = []errorAnswer{
		{err: auth.ErrRegistrationPrivate, status: http.StatusForbidden, code: "site_private"},
		{err: auth.ErrRegistrationClosed, status: http.StatusForbidden, code: "registration_closed"},
		{err: auth.ErrInvalidInvitation, status: http.StatusBadRequest, code: "invitation_invalid"},
		{err: auth.ErrInvalidEmail, status: http.StatusBadRequest},
		{err: auth.ErrEmailTaken, status: http.StatusConflict},
		{err: auth.ErrEmailUnchanged, status: http.StatusBadRequest},
		{err: auth.ErrUserBanned, status: http.StatusForbidden},
		{err: auth.ErrEmailUnverified, status: http.StatusForbidden, code: "email_unverified"},
		{err: auth.ErrInvalidVerificationToken, status: http.StatusBadRequest},
		{err: auth.ErrInvalidResetToken, status: http.StatusBadRequest},
		{err: auth.ErrEmailUnavailable, status: http.StatusServiceUnavailable, code: emailUnavailableCode},
		{err: auth.ErrPasswordSignInDisabled, status: http.StatusForbidden, code: "password_sign_in_disabled"},
		{err: user.ErrInvalidCredentials, status: http.StatusUnauthorized},
		{err: user.ErrIncorrectPassword, status: http.StatusForbidden, code: "incorrect_password"},
		{err: user.ErrPasswordTooShort, status: http.StatusBadRequest, code: "password_too_short"},
		{err: user.ErrPasswordTooLong, status: http.StatusBadRequest},
		{err: user.ErrDisplayNameRequired, status: http.StatusBadRequest},
		{err: user.ErrUserNotFound, status: http.StatusNotFound},
		{err: org.ErrSoleOwner, status: http.StatusConflict, code: "sole_owner"},
		{err: org.ErrAccountNotFound, status: http.StatusNotFound},
	}
)

func (s *Service) getAllAuthRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/auth/register", Before: gin.HandlersChain{s.Guards.RateLimitCredentials, s.Guards.RateLimitMail, s.Guards.Turnstile}, Handler: s.register},
		{Method: http.MethodPost, Path: "/auth/invitation", Handler: s.getInvitation},
		{Method: http.MethodPost, Path: "/auth/invitation/accept", Before: gin.HandlersChain{s.Guards.RateLimitCredentials}, Handler: s.acceptInvitation},
		{Method: http.MethodPost, Path: "/auth/login", Before: gin.HandlersChain{s.Guards.RateLimitCredentials, s.Guards.RateLimitCredentialsByAccount(accountField), s.Guards.Turnstile}, Handler: s.login},
		{Method: http.MethodPost, Path: "/auth/logout", Handler: s.logout},
		{Method: http.MethodGet, Path: "/auth/session", Before: gin.HandlersChain{s.Guards.OptionalPrincipal}, Handler: s.currentSession},
		{Method: http.MethodPost, Path: "/auth/verify-email", Handler: s.verifyEmail},
		{Method: http.MethodPost, Path: "/auth/resend-verification", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.RateLimitMailByAccount(accountField), s.Guards.Turnstile}, Handler: s.resendVerification},
		{Method: http.MethodPost, Path: "/auth/forgot-password", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.RateLimitMailByAccount(accountField), s.Guards.Turnstile}, Handler: s.forgotPassword},
		{Method: http.MethodPost, Path: "/auth/reset-password", Before: gin.HandlersChain{s.Guards.RateLimitCredentials}, Handler: s.resetPassword},
		{Method: http.MethodPost, Path: "/auth/set-email", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.Auth}, Handler: s.setEmail},
		{Method: http.MethodPut, Path: "/auth/password", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.changePassword},
		{Method: http.MethodPut, Path: "/auth/profile", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.updateProfile},
		{Method: http.MethodPost, Path: "/auth/sign-out-other-devices", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.signOutOtherDevices},
		{Method: http.MethodDelete, Path: "/auth/account", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.deleteAccount},
		{Method: http.MethodGet, Path: "/auth/identities", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.listIdentities},
		{Method: http.MethodDelete, Path: "/auth/identities/:provider", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.unlinkIdentity},
	}
}

func (s *Service) register(c *gin.Context) {
	req, ok := utils.BindJSON[dto.RegisterRequest](c)
	if !ok {
		return
	}

	if req.Email == "" || req.Password == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email and password are required"})
		return
	}

	registration := spec.AccountRegistration{Email: req.Email, Password: req.Password, DisplayName: req.DisplayName}

	registered, err := s.AuthService.Register(c.Request.Context(), registration)
	if err != nil && !errors.Is(err, auth.ErrEmailNotSent) {
		answerError(c, authErrorAnswers, err, "failed to register")
		return
	}

	if err != nil {
		c.Error(err)
	}

	c.JSON(http.StatusCreated, registered)
}

func (s *Service) getInvitation(c *gin.Context) {
	req, ok := utils.BindJSON[dto.InvitationTokenRequest](c)
	if !ok {
		return
	}

	if req.Token == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token is required"})
		return
	}

	invitation, err := s.AuthService.GetInvitation(c.Request.Context(), req.Token)
	if err != nil {
		answerError(c, authErrorAnswers, err, "failed to read the invitation")
		return
	}

	c.JSON(http.StatusOK, invitation)
}

func (s *Service) acceptInvitation(c *gin.Context) {
	req, ok := utils.BindJSON[dto.AcceptInvitationRequest](c)
	if !ok {
		return
	}

	if req.Token == "" || req.Password == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token and password are required"})
		return
	}

	acceptance := spec.InvitationAcceptance{Token: req.Token, DisplayName: req.DisplayName, Password: req.Password, IP: c.ClientIP()}

	accepted, err := s.AuthService.AcceptInvitation(c.Request.Context(), acceptance)
	if err != nil && !errors.Is(err, auth.ErrNotAudited) {
		answerError(c, authErrorAnswers, err, "failed to accept the invitation")
		return
	}

	if err != nil {
		c.Error(err)
	}

	c.JSON(http.StatusCreated, accepted)
}

func (s *Service) login(c *gin.Context) {
	req, ok := utils.BindJSON[dto.LoginRequest](c)
	if !ok {
		return
	}

	if req.Email == "" || req.Password == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email and password are required"})
		return
	}

	trustedDevice, err := s.TrustedDevices.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the trusted device cookie", err)
		return
	}

	signIn := spec.PasswordSignIn{
		UserCredentials:    spec.UserCredentials{Email: req.Email, Password: req.Password},
		Remember:           req.Remember,
		TrustedDeviceToken: trustedDevice,
	}

	result, err := s.AuthService.Login(c.Request.Context(), signIn)
	if err != nil {
		answerError(c, authErrorAnswers, err, "failed to sign in")
		return
	}

	if result.ChallengeToken != "" {
		cookie, err := s.TwoFactorCookies.Cookie(c.Request.Context(), result.ChallengeToken)
		if err != nil {
			utils.InternalError(c, "failed to build the two-factor cookie", err)
			return
		}

		http.SetCookie(c.Writer, cookie)

		c.JSON(http.StatusOK, dto.TwoFactorChallengeResponse{TwoFactorRequired: true})

		return
	}

	if !s.writeSessionCookie(c, result.SessionToken, req.Remember) {
		return
	}

	c.JSON(http.StatusOK, result.User)
}

func (s *Service) logout(c *gin.Context) {
	token, err := s.Sessions.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the session cookie", err)
		return
	}

	if err := s.AuthService.Logout(c.Request.Context(), token); err != nil {
		utils.InternalError(c, "failed to sign out", err)
		return
	}

	if !s.clearSessionCookie(c) {
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) currentSession(c *gin.Context) {
	principal, known := authctx.Principal(c.Request.Context())
	if known && principal.Guest() {
		c.JSON(http.StatusOK, dto.SessionResponse{Guest: &dto.GuestView{ID: principal.ID}})
		return
	}

	userID, signedIn := authctx.UserID(c.Request.Context())
	if !signedIn {
		c.JSON(http.StatusOK, dto.SessionResponse{})
		return
	}

	account, err := s.UserService.GetByID(c.Request.Context(), userID)
	if errors.Is(err, user.ErrUserNotFound) {
		c.JSON(http.StatusOK, dto.SessionResponse{})
		return
	}
	if err != nil {
		utils.InternalError(c, "failed to load the session", err)
		return
	}

	effective, err := s.AuthzService.EffectivePermissions(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "failed to load the session", err)
		return
	}

	granted := make([]string, 0, len(effective))
	for _, perm := range effective {
		granted = append(granted, string(perm))
	}

	siteRole, err := s.AuthzService.GetRole(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "failed to load the session", err)
		return
	}

	c.JSON(http.StatusOK, dto.SessionResponse{Authenticated: true, User: account, Permissions: granted, SiteRole: siteRole})
}

func (s *Service) verifyEmail(c *gin.Context) {
	req, ok := utils.BindJSON[dto.VerifyEmailRequest](c)
	if !ok {
		return
	}

	if req.Token == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token is required"})
		return
	}

	if err := s.AuthService.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		answerError(c, authErrorAnswers, err, "failed to verify the email address")
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) resendVerification(c *gin.Context) {
	req, ok := utils.BindJSON[dto.ResendVerificationRequest](c)
	if !ok {
		return
	}

	if req.Email == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email is required"})
		return
	}

	err := s.AuthService.ResendVerification(c.Request.Context(), req.Email)
	if err != nil && !errors.Is(err, auth.ErrVerificationNotSent) {
		answerError(c, authErrorAnswers, err, "failed to resend the verification email")
		return
	}

	if err != nil {
		c.Error(err)
	}

	noContent(c)
}

func (s *Service) forgotPassword(c *gin.Context) {
	req, ok := utils.BindJSON[dto.ForgotPasswordRequest](c)
	if !ok {
		return
	}

	if req.Email == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email is required"})
		return
	}

	if err := s.AuthService.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		answerError(c, authErrorAnswers, err, "failed to send the password reset email")
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) resetPassword(c *gin.Context) {
	req, ok := utils.BindJSON[dto.ResetPasswordRequest](c)
	if !ok {
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token and new password are required"})
		return
	}

	if err := s.AuthService.ResetPassword(c.Request.Context(), spec.PasswordResetCompletion{Token: req.Token, NewPassword: req.NewPassword}); err != nil {
		answerError(c, authErrorAnswers, err, "failed to reset the password")
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) setEmail(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.SetEmailRequest](c)
	if !ok {
		return
	}

	if req.Email == "" || req.Password == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email and password are required"})
		return
	}

	if err := s.AuthService.SetEmail(c.Request.Context(), spec.EmailChange{UserID: userID, Email: req.Email, Password: req.Password}); err != nil {
		answerError(c, authErrorAnswers, err, "failed to change the email address")
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) changePassword(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ChangePasswordRequest](c)
	if !ok {
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "current password and new password are required"})
		return
	}

	token, err := s.Sessions.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the session cookie", err)
		return
	}

	update := spec.PasswordUpdate{UserID: userID, CurrentPassword: req.CurrentPassword, NewPassword: req.NewPassword, KeepToken: token}
	if err := s.AuthService.ChangePassword(c.Request.Context(), update); err != nil {
		answerError(c, authErrorAnswers, err, "failed to change the password")
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) updateProfile(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.UpdateDisplayNameRequest](c)
	if !ok {
		return
	}

	account, err := s.UserService.UpdateDisplayName(c.Request.Context(), spec.UserDisplayNameUpdate{UserID: userID, DisplayName: req.DisplayName})
	if err != nil {
		answerError(c, authErrorAnswers, err, "failed to update the profile")
		return
	}

	c.JSON(http.StatusOK, account)
}

func (s *Service) signOutOtherDevices(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	token, err := s.Sessions.Token(c.Request)
	if err != nil {
		utils.InternalError(c, "failed to read the session cookie", err)
		return
	}

	if err := s.AuthService.SignOutOtherDevices(c.Request.Context(), spec.SessionDeletionExcept{UserID: userID, KeepToken: token}); err != nil {
		utils.InternalError(c, "failed to sign out the other devices", err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) deleteAccount(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.DeleteAccountRequest](c)
	if !ok {
		return
	}

	if req.Password == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "password is required"})
		return
	}

	if err := s.AuthService.DeleteAccount(c.Request.Context(), spec.UserPasswordConfirmation{UserID: userID, Password: req.Password}); err != nil {
		answerError(c, authErrorAnswers, err, "failed to delete the account")
		return
	}

	if !s.clearSessionCookie(c) {
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Service) listIdentities(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	identities, err := s.IdentityService.ListIdentities(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "failed to list the sign-in methods", err)
		return
	}

	c.JSON(http.StatusOK, identities)
}

func (s *Service) unlinkIdentity(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	unlinking := spec.IdentityUnlinking{UserID: userID, Provider: model.IdentityProvider(c.Param("provider")), IP: c.ClientIP()}

	err := s.IdentityService.Unlink(c.Request.Context(), unlinking)
	if err != nil && !errors.Is(err, identity.ErrNotAudited) && !errors.Is(err, identity.ErrNoticeNotSent) {
		answerError(c, identityErrorAnswers, err, "failed to remove the sign-in method")
		return
	}

	if err != nil {
		c.Error(err)
	}

	noContent(c)
}

func (s *Service) writeSessionCookie(c *gin.Context, token string, remember bool) bool {
	cookie, err := s.Sessions.GrantCookie(c.Request.Context(), token, remember)
	if err != nil {
		utils.InternalError(c, "failed to build the session cookie", err)
		return false
	}

	http.SetCookie(c.Writer, cookie)

	return true
}

func (s *Service) clearSessionCookie(c *gin.Context) bool {
	cookie, err := s.Sessions.ExpiredCookie(c.Request.Context())
	if err != nil {
		utils.InternalError(c, "failed to build the session cookie", err)
		return false
	}

	http.SetCookie(c.Writer, cookie)

	return true
}

func answerError(c *gin.Context, answers []errorAnswer, err error, msg string) {
	for _, answer := range answers {
		if !errors.Is(err, answer.err) {
			continue
		}

		if answer.recordCause {
			c.Error(err)
		}

		c.AbortWithStatusJSON(answer.status, dto.ErrorResponse{Error: answer.err.Error(), Code: answer.code})

		return
	}

	utils.InternalError(c, msg, err)
}
