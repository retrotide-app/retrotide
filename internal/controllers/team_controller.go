package controllers

import (
	"errors"
	"net/http"
	"strings"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/orgauthz"
	"retrotide/internal/team"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	teamErrorAnswers = []errorAnswer{
		{err: team.ErrTeamNotFound, status: http.StatusNotFound},
		{err: team.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: team.ErrMemberNotFound, status: http.StatusNotFound},
		{err: team.ErrTeamMemberNotFound, status: http.StatusNotFound},
		{err: team.ErrInvalidRole, status: http.StatusBadRequest},
		{err: team.ErrNameRequired, status: http.StatusBadRequest, code: "team_name_required"},
		{err: team.ErrNameTooLong, status: http.StatusBadRequest, code: "team_name_too_long"},
		{err: hierarchy.ErrOutranked, status: http.StatusForbidden},
		{err: team.ErrAlreadyTeamMember, status: http.StatusConflict},
		{err: team.ErrNameTaken, status: http.StatusConflict, code: "team_name_taken"},
		{err: team.ErrTeamArchived, status: http.StatusConflict, code: "team_archived"},
	}
)

func (s *Service) getAllTeamRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/orgs/:orgID/teams", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.listTeams},
		{Method: http.MethodPost, Path: "/orgs/:orgID/teams", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageTeams)}, Handler: s.createTeam},
		{Method: http.MethodPost, Path: "/orgs/:orgID/teams/:teamID/archive", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageTeams)}, Handler: s.archiveTeam},
		{Method: http.MethodDelete, Path: "/orgs/:orgID/teams/:teamID", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgManageTeams)}, Handler: s.deleteTeam},
		{Method: http.MethodGet, Path: "/teams/:teamID", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamView)}, Handler: s.getTeam},
		{Method: http.MethodPut, Path: "/teams/:teamID", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamUpdate)}, Handler: s.renameTeam},
		{Method: http.MethodGet, Path: "/teams/:teamID/members", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamView)}, Handler: s.listTeamMembers},
		{Method: http.MethodPost, Path: "/teams/:teamID/members", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamManageMembers)}, Handler: s.addTeamMember},
		{Method: http.MethodPut, Path: "/teams/:teamID/members/:userID/role", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamManageMembers)}, Handler: s.changeTeamMemberRole},
		{Method: http.MethodDelete, Path: "/teams/:teamID/members/:userID", Before: gin.HandlersChain{s.Guards.TeamPermission(orgauthz.PermTeamManageMembers)}, Handler: s.removeTeamMember},
	}
}

func (s *Service) listTeams(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	listing, err := s.TeamService.List(c.Request.Context(), viewer)
	if teamFailed(c, err, "failed to list the teams") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) createTeam(c *gin.Context) {
	actor, ok := teamActor(c)
	if !ok {
		return
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateTeamRequest](c)
	if !ok {
		return
	}

	created, err := s.TeamService.Create(c.Request.Context(), team.CreateRequest{Actor: actor, OrgID: orgID, Name: req.Name})
	if teamFailed(c, err, "failed to create the team") {
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (s *Service) archiveTeam(c *gin.Context) {
	command, ok := orgTeamCommand(c)
	if !ok {
		return
	}

	err := s.TeamService.Archive(c.Request.Context(), command)
	if teamFailed(c, err, "failed to archive the team") {
		return
	}

	noContent(c)
}

func (s *Service) deleteTeam(c *gin.Context) {
	command, ok := orgTeamCommand(c)
	if !ok {
		return
	}

	err := s.TeamService.Delete(c.Request.Context(), command)
	if teamFailed(c, err, "failed to delete the team") {
		return
	}

	noContent(c)
}

func (s *Service) getTeam(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	teamID, ok := utils.ParseIDParam(c, teamIDParam)
	if !ok {
		return
	}

	found, err := s.TeamService.Get(c.Request.Context(), team.Viewer{TeamID: teamID, UserID: userID})
	if teamFailed(c, err, "failed to load the team") {
		return
	}

	c.JSON(http.StatusOK, found)
}

func (s *Service) renameTeam(c *gin.Context) {
	command, ok := teamCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.RenameTeamRequest](c)
	if !ok {
		return
	}

	renamed, err := s.TeamService.Rename(c.Request.Context(), team.RenameRequest{Command: command, Name: req.Name})
	if teamFailed(c, err, "failed to rename the team") {
		return
	}

	c.JSON(http.StatusOK, renamed)
}

func (s *Service) listTeamMembers(c *gin.Context) {
	teamID, ok := utils.ParseIDParam(c, teamIDParam)
	if !ok {
		return
	}

	members, err := s.TeamService.ListMembers(c.Request.Context(), teamID)
	if teamFailed(c, err, "failed to list the team's members") {
		return
	}

	c.JSON(http.StatusOK, members)
}

func (s *Service) addTeamMember(c *gin.Context) {
	command, ok := teamCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.AddTeamMemberRequest](c)
	if !ok {
		return
	}

	if req.UserID == uuid.Nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "user_id is required"})
		return
	}

	change := team.MemberRole{Command: command, UserID: req.UserID, Role: model.TeamRole(strings.TrimSpace(req.Role))}

	err := s.TeamService.AddMember(c.Request.Context(), change)
	if teamFailed(c, err, "failed to add the team member") {
		return
	}

	noContent(c)
}

func (s *Service) changeTeamMemberRole(c *gin.Context) {
	command, ok := teamMemberCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ChangeTeamMemberRoleRequest](c)
	if !ok {
		return
	}

	err := s.TeamService.SetMemberRole(c.Request.Context(), team.MemberRole{MemberCommand: command, Role: model.TeamRole(strings.TrimSpace(req.Role))})
	if teamFailed(c, err, "failed to change the team member's role") {
		return
	}

	noContent(c)
}

func (s *Service) removeTeamMember(c *gin.Context) {
	command, ok := teamMemberCommand(c)
	if !ok {
		return
	}

	err := s.TeamService.RemoveMember(c.Request.Context(), command)
	if teamFailed(c, err, "failed to remove the team member") {
		return
	}

	noContent(c)
}

func teamActor(c *gin.Context) (team.Actor, bool) {
	actorID, ok := utils.UserID(c)
	if !ok {
		return team.Actor{}, false
	}

	return team.Actor{ActorID: actorID, IP: c.ClientIP()}, true
}

func orgTeamCommand(c *gin.Context) (team.OrgTeamCommand, bool) {
	actor, ok := teamActor(c)
	if !ok {
		return team.OrgTeamCommand{}, false
	}

	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return team.OrgTeamCommand{}, false
	}

	teamID, ok := utils.ParseIDParam(c, teamIDParam)
	if !ok {
		return team.OrgTeamCommand{}, false
	}

	return team.OrgTeamCommand{Actor: actor, OrgID: orgID, TeamID: teamID}, true
}

func teamCommand(c *gin.Context) (team.Command, bool) {
	actor, ok := teamActor(c)
	if !ok {
		return team.Command{}, false
	}

	teamID, ok := utils.ParseIDParam(c, teamIDParam)
	if !ok {
		return team.Command{}, false
	}

	return team.Command{Actor: actor, TeamID: teamID}, true
}

func teamMemberCommand(c *gin.Context) (team.MemberCommand, bool) {
	command, ok := teamCommand(c)
	if !ok {
		return team.MemberCommand{}, false
	}

	userID, ok := utils.ParseIDParam(c, userIDParam)
	if !ok {
		return team.MemberCommand{}, false
	}

	return team.MemberCommand{Command: command, UserID: userID}, true
}

func teamFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, team.ErrCacheStale) {
		utils.InternalError(c, team.ErrCacheStale.Error(), err)
		return true
	}

	if errors.Is(err, team.ErrNotAudited) {
		c.Error(err)
		return false
	}

	if utils.MapEntitlementError(c, err) {
		return true
	}

	answerError(c, teamErrorAnswers, err, msg)

	return true
}
