package controllers

import (
	"errors"
	"net/http"
	"strings"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org"
	"retrotide/internal/orgauthz"
	"retrotide/internal/team"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	orgIDParam    = "orgID"
	teamIDParam   = "teamID"
	userIDParam   = "userID"
	inviteIDParam = "inviteID"
)

var (
	orgErrorAnswers = []errorAnswer{
		{err: org.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: org.ErrMemberNotFound, status: http.StatusNotFound},
		{err: org.ErrNameRequired, status: http.StatusBadRequest, code: "org_name_required"},
		{err: org.ErrNameTooLong, status: http.StatusBadRequest, code: "org_name_too_long"},
		{err: team.ErrNameRequired, status: http.StatusBadRequest, code: "team_name_required"},
		{err: team.ErrNameTooLong, status: http.StatusBadRequest, code: "team_name_too_long"},
		{err: org.ErrInvalidSlug, status: http.StatusBadRequest, code: "slug_invalid"},
		{err: org.ErrSlugReserved, status: http.StatusBadRequest, code: "slug_reserved"},
		{err: org.ErrInvalidRole, status: http.StatusBadRequest},
		{err: hierarchy.ErrOutranked, status: http.StatusForbidden},
		{err: org.ErrOwnerProtected, status: http.StatusForbidden},
		{err: org.ErrAdminsNeedOwner, status: http.StatusForbidden},
		{err: org.ErrNotOwner, status: http.StatusForbidden},
		{err: org.ErrSlugTaken, status: http.StatusConflict, code: "slug_taken"},
		{err: org.ErrOwnedLimitReached, status: http.StatusConflict, code: "owned_limit_reached"},
		{err: org.ErrNewOwnerLimitReached, status: http.StatusConflict, code: "new_owner_limit_reached"},
		{err: org.ErrOwnerCannotLeave, status: http.StatusConflict},
		{err: org.ErrOwnershipNotTransferred, status: http.StatusConflict},
		{err: org.ErrOrganisationNotDeleted, status: http.StatusConflict},
		{err: org.ErrNotRestorable, status: http.StatusConflict},
	}
)

func (s *Service) getAllOrganisationRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodPost, Path: "/orgs", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.createWorkspace},
		{Method: http.MethodGet, Path: "/orgs", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.listOrganisations},
		{Method: http.MethodGet, Path: "/orgs/:orgID", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.getOrganisation},
		{Method: http.MethodPut, Path: "/orgs/:orgID", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgUpdate)}, Handler: s.updateOrganisation},
		{Method: http.MethodDelete, Path: "/orgs/:orgID", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgDelete)}, Handler: s.deleteOrganisation},
		{Method: http.MethodPost, Path: "/orgs/:orgID/restore", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgDelete)}, Handler: s.restoreOrganisation},
		{Method: http.MethodPost, Path: "/orgs/:orgID/transfer", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgTransferOwnership)}, Handler: s.transferOwnership},
		{Method: http.MethodPost, Path: "/orgs/:orgID/leave", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.leaveOrganisation},
		{Method: http.MethodGet, Path: "/orgs/:orgID/members", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageMembers)}, Handler: s.listOrganisationMembers},
		{Method: http.MethodPut, Path: "/orgs/:orgID/members/:userID/role", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageMembers)}, Handler: s.changeOrganisationMemberRole},
		{Method: http.MethodDelete, Path: "/orgs/:orgID/members/:userID", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageMembers)}, Handler: s.removeOrganisationMember},
		{Method: http.MethodGet, Path: "/orgs/:orgID/entitlements", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.getEntitlements},
	}
}

func (s *Service) createWorkspace(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateWorkspaceRequest](c)
	if !ok {
		return
	}

	created, err := s.OrgService.CreateWorkspace(c.Request.Context(), org.WorkspaceRequest{Actor: actor, Name: req.Name, TeamName: req.TeamName})
	if orgFailed(c, err, "failed to create the workspace") {
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (s *Service) listOrganisations(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	listing, err := s.OrgService.ListForUser(c.Request.Context(), userID)
	if err != nil {
		utils.InternalError(c, "failed to list your organisations", err)
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) getOrganisation(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	found, err := s.OrgService.Get(c.Request.Context(), viewer)
	if orgFailed(c, err, "failed to load the organisation") {
		return
	}

	c.JSON(http.StatusOK, found)
}

func (s *Service) updateOrganisation(c *gin.Context) {
	command, ok := orgCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.UpdateOrganisationRequest](c)
	if !ok {
		return
	}

	updated, err := s.OrgService.Update(c.Request.Context(), org.UpdateRequest{Command: command, Name: req.Name, Slug: req.Slug, AllowGuests: req.AllowGuests})
	if orgFailed(c, err, "failed to update the organisation") {
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (s *Service) deleteOrganisation(c *gin.Context) {
	command, ok := orgCommand(c)
	if !ok {
		return
	}

	err := s.OrgService.Delete(c.Request.Context(), command)
	if orgFailed(c, err, "failed to delete the organisation") {
		return
	}

	noContent(c)
}

func (s *Service) restoreOrganisation(c *gin.Context) {
	command, ok := orgCommand(c)
	if !ok {
		return
	}

	restored, err := s.OrgService.Restore(c.Request.Context(), command)
	if orgFailed(c, err, "failed to restore the organisation") {
		return
	}

	c.JSON(http.StatusOK, restored)
}

func (s *Service) transferOwnership(c *gin.Context) {
	command, ok := orgCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.TransferOwnershipRequest](c)
	if !ok {
		return
	}

	if req.UserID == uuid.Nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "user_id is required"})
		return
	}

	err := s.OrgService.TransferOwnership(c.Request.Context(), org.MemberCommand{Command: command, UserID: req.UserID})
	if orgFailed(c, err, "failed to transfer ownership") {
		return
	}

	noContent(c)
}

func (s *Service) leaveOrganisation(c *gin.Context) {
	command, ok := orgCommand(c)
	if !ok {
		return
	}

	err := s.OrgService.Leave(c.Request.Context(), command)
	if orgFailed(c, err, "failed to leave the organisation") {
		return
	}

	noContent(c)
}

func (s *Service) listOrganisationMembers(c *gin.Context) {
	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	members, err := s.OrgService.ListMembers(c.Request.Context(), orgID)
	if orgFailed(c, err, "failed to list the members") {
		return
	}

	c.JSON(http.StatusOK, members)
}

func (s *Service) changeOrganisationMemberRole(c *gin.Context) {
	command, ok := orgMemberCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ChangeMemberRoleRequest](c)
	if !ok {
		return
	}

	err := s.OrgService.ChangeMemberRole(c.Request.Context(), org.RoleChange{MemberCommand: command, Role: model.OrgRole(strings.TrimSpace(req.Role))})
	if orgFailed(c, err, "failed to change the member's role") {
		return
	}

	noContent(c)
}

func (s *Service) removeOrganisationMember(c *gin.Context) {
	command, ok := orgMemberCommand(c)
	if !ok {
		return
	}

	err := s.OrgService.RemoveMember(c.Request.Context(), command)
	if orgFailed(c, err, "failed to remove the member") {
		return
	}

	noContent(c)
}

func (s *Service) getEntitlements(c *gin.Context) {
	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	plan, err := s.EntitlementsService.For(c.Request.Context(), orgID)
	if err != nil {
		utils.InternalError(c, "failed to load the organisation's plan", err)
		return
	}

	c.JSON(http.StatusOK, dto.EntitlementsResponse{Plan: plan.Code, Limits: plan.Limits})
}

func orgActor(c *gin.Context) (org.Actor, bool) {
	actorID, ok := utils.UserID(c)
	if !ok {
		return org.Actor{}, false
	}

	return org.Actor{ActorID: actorID, IP: c.ClientIP()}, true
}

func orgViewer(c *gin.Context) (spec.OrganisationMemberKey, bool) {
	userID, ok := utils.UserID(c)
	if !ok {
		return spec.OrganisationMemberKey{}, false
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return spec.OrganisationMemberKey{}, false
	}

	return spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}, true
}

func orgCommand(c *gin.Context) (org.Command, bool) {
	actor, ok := orgActor(c)
	if !ok {
		return org.Command{}, false
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return org.Command{}, false
	}

	return org.Command{Actor: actor, OrgID: orgID}, true
}

func orgMemberCommand(c *gin.Context) (org.MemberCommand, bool) {
	command, ok := orgCommand(c)
	if !ok {
		return org.MemberCommand{}, false
	}

	userID, ok := utils.ParseIDParam(c, userIDParam)
	if !ok {
		return org.MemberCommand{}, false
	}

	return org.MemberCommand{Command: command, UserID: userID}, true
}

func orgFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, org.ErrCacheStale) {
		utils.InternalError(c, org.ErrCacheStale.Error(), err)
		return true
	}

	if errors.Is(err, org.ErrNotAudited) {
		c.Error(err)
		return false
	}

	answerError(c, orgErrorAnswers, err, msg)

	return true
}
