package controllers

import (
	"errors"
	"net/http"
	"strings"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org/invite"
	"retrotide/internal/orgauthz"
	"retrotide/internal/user"

	"github.com/gin-gonic/gin"
)

var (
	inviteErrorAnswers = []errorAnswer{
		{err: invite.ErrInvalidInvite, status: http.StatusBadRequest, code: "invite_invalid"},
		{err: invite.ErrInvalidRole, status: http.StatusBadRequest},
		{err: invite.ErrInvalidEmail, status: http.StatusBadRequest},
		{err: invite.ErrInvalidMaxUses, status: http.StatusBadRequest},
		{err: user.ErrDisplayNameRequired, status: http.StatusBadRequest, code: "display_name_required"},
		{err: user.ErrPasswordTooShort, status: http.StatusBadRequest, code: "password_too_short"},
		{err: user.ErrPasswordTooLong, status: http.StatusBadRequest},
		{err: invite.ErrInviteNotFound, status: http.StatusNotFound},
		{err: invite.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: invite.ErrTeamNotFound, status: http.StatusNotFound},
		{err: invite.ErrAdminInviteForbidden, status: http.StatusForbidden},
		{err: invite.ErrAdminRevokeForbidden, status: http.StatusForbidden},
		{err: invite.ErrEmailMismatch, status: http.StatusForbidden, code: "invite_email_mismatch"},
		{err: invite.ErrEmailUnverified, status: http.StatusForbidden, code: "email_unverified"},
		{err: invite.ErrPasswordSignInDisabled, status: http.StatusForbidden, code: "password_sign_in_disabled"},
		{err: invite.ErrAlreadyMember, status: http.StatusConflict, code: "already_member"},
		{err: invite.ErrInvitePending, status: http.StatusConflict, code: "invite_pending"},
		{err: invite.ErrNotResendable, status: http.StatusConflict, code: "invite_not_resendable"},
		{err: invite.ErrTeamArchived, status: http.StatusConflict, code: "team_archived"},
		{err: invite.ErrEmailTaken, status: http.StatusConflict, code: "email_taken"},
		{err: invite.ErrEmailUnavailable, status: http.StatusServiceUnavailable, code: emailUnavailableCode},
	}

	inviteFailuresWithTheirOwnMessage = []error{
		invite.ErrAccountLeftBehind,
		invite.ErrInviteNotSent,
		invite.ErrInviteNotReissued,
	}

	inviteAftermaths = []error{
		invite.ErrNotAudited,
		invite.ErrCacheNotCleared,
		invite.ErrVerificationNotSent,
	}
)

func (s *Service) getAllInviteRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/orgs/:orgID/invites", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageMembers)}, Handler: s.listInvites},
		{Method: http.MethodPost, Path: "/orgs/:orgID/invites", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.OrgPermission(orgauthz.PermOrgManageMembers)}, Handler: s.createInvite},
		{Method: http.MethodPost, Path: "/orgs/:orgID/invites/:inviteID/resend", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.OrgPermission(orgauthz.PermOrgManageMembers)}, Handler: s.resendInvite},
		{Method: http.MethodDelete, Path: "/orgs/:orgID/invites/:inviteID", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageMembers)}, Handler: s.revokeInvite},
		{Method: http.MethodPost, Path: "/invites/lookup", Handler: s.lookUpInvite},
		{Method: http.MethodPost, Path: "/invites/accept", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.acceptInvite},
		{Method: http.MethodPost, Path: "/invites/register", Before: gin.HandlersChain{s.Guards.RateLimitCredentials, s.Guards.RateLimitMail}, Handler: s.registerThroughInvite},
	}
}

func (s *Service) listInvites(c *gin.Context) {
	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	listing, err := s.InviteService.List(c.Request.Context(), orgID)
	if inviteFailed(c, err, "failed to list the invites") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) createInvite(c *gin.Context) {
	actorID, ok := utils.UserID(c)
	if !ok {
		return
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateInviteRequest](c)
	if !ok {
		return
	}

	creation := spec.InviteCreation{
		OrgID:   orgID,
		ActorID: actorID,
		Email:   req.Email,
		Role:    model.OrgRole(strings.TrimSpace(req.Role)),
		TeamID:  req.TeamID,
		MaxUses: req.MaxUses,
		IP:      c.ClientIP(),
	}

	created, err := s.InviteService.Create(c.Request.Context(), creation)
	if inviteFailed(c, err, "failed to create the invite") {
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (s *Service) resendInvite(c *gin.Context) {
	action, ok := inviteAction(c)
	if !ok {
		return
	}

	renewed, err := s.InviteService.Resend(c.Request.Context(), action)
	if inviteFailed(c, err, "failed to resend the invite") {
		return
	}

	c.JSON(http.StatusOK, renewed)
}

func (s *Service) revokeInvite(c *gin.Context) {
	action, ok := inviteAction(c)
	if !ok {
		return
	}

	err := s.InviteService.Revoke(c.Request.Context(), action)
	if inviteFailed(c, err, "failed to revoke the invite") {
		return
	}

	noContent(c)
}

func (s *Service) lookUpInvite(c *gin.Context) {
	req, ok := utils.BindJSON[dto.InviteTokenRequest](c)
	if !ok {
		return
	}

	if req.Token == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token is required"})
		return
	}

	details, err := s.InviteService.Lookup(c.Request.Context(), req.Token)
	if inviteFailed(c, err, "failed to read the invite") {
		return
	}

	c.JSON(http.StatusOK, details)
}

func (s *Service) acceptInvite(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.InviteTokenRequest](c)
	if !ok {
		return
	}

	if req.Token == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token is required"})
		return
	}

	joined, err := s.InviteService.Accept(c.Request.Context(), spec.InviteJoin{Token: req.Token, UserID: userID, IP: c.ClientIP()})
	if inviteFailed(c, err, "failed to accept the invite") {
		return
	}

	c.JSON(http.StatusOK, joined)
}

func (s *Service) registerThroughInvite(c *gin.Context) {
	req, ok := utils.BindJSON[dto.InviteRegistrationRequest](c)
	if !ok {
		return
	}

	if req.Token == "" || req.Password == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token and password are required"})
		return
	}

	registration := spec.InviteRegistration{Token: req.Token, Email: req.Email, DisplayName: req.DisplayName, Password: req.Password, IP: c.ClientIP()}

	registered, err := s.InviteService.Register(c.Request.Context(), registration)
	if inviteFailed(c, err, "failed to create the account") {
		return
	}

	c.JSON(http.StatusCreated, registered)
}

func inviteAction(c *gin.Context) (spec.InviteAction, bool) {
	actorID, ok := utils.UserID(c)
	if !ok {
		return spec.InviteAction{}, false
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return spec.InviteAction{}, false
	}

	inviteID, ok := utils.ParseIDParam(c, inviteIDParam)
	if !ok {
		return spec.InviteAction{}, false
	}

	return spec.InviteAction{OrgID: orgID, InviteID: inviteID, ActorID: actorID, IP: c.ClientIP()}, true
}

func inviteFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	for _, failure := range inviteFailuresWithTheirOwnMessage {
		if errors.Is(err, failure) {
			utils.InternalError(c, failure.Error(), err)
			return true
		}
	}

	for _, aftermath := range inviteAftermaths {
		if errors.Is(err, aftermath) {
			c.Error(err)
			return false
		}
	}

	if utils.MapEntitlementError(c, err) {
		return true
	}

	answerError(c, inviteErrorAnswers, err, msg)

	return true
}
