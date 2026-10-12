package controllers

import (
	"net/http"

	"retrotide/internal/board/live"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/savedtemplate"

	"github.com/gin-gonic/gin"
)

const (
	savedTemplateIDParam = "templateID"
)

var (
	savedTemplateErrorAnswers = []errorAnswer{
		{err: savedtemplate.ErrNameRequired, status: http.StatusBadRequest, code: "template_name_required"},
		{err: savedtemplate.ErrNameTooLong, status: http.StatusBadRequest, code: "template_name_too_long"},
		{err: savedtemplate.ErrDescriptionTooLong, status: http.StatusBadRequest, code: "template_description_too_long"},
		{err: savedtemplate.ErrInvalidScope, status: http.StatusBadRequest, code: "invalid_template_scope"},
		{err: savedtemplate.ErrNotTeamBoard, status: http.StatusConflict, code: "not_team_board"},
		{err: savedtemplate.ErrNoColumns, status: http.StatusConflict, code: "board_has_no_columns"},
		{err: savedtemplate.ErrNotAllowed, status: http.StatusForbidden, code: "save_not_allowed"},
		{err: savedtemplate.ErrDeleteNotAllowed, status: http.StatusForbidden, code: "delete_not_allowed"},
		{err: savedtemplate.ErrTemplateNotFound, status: http.StatusNotFound},
		{err: savedtemplate.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: live.ErrNoAccess, status: http.StatusNotFound},
		{err: live.ErrRemoved, status: http.StatusForbidden, code: "removed"},
		{err: live.ErrLinkDisabled, status: http.StatusForbidden, code: "link_disabled"},
		{err: live.ErrPasswordRequired, status: http.StatusForbidden, code: "password_required"},
		{err: live.ErrAccountRequired, status: http.StatusForbidden, code: "account_required"},
		{err: live.ErrGuestsNotAllowed, status: http.StatusForbidden, code: "guests_not_allowed"},
	}
)

func (s *Service) getAllSavedTemplateRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/orgs/:orgID/saved-templates", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.listSavedTemplates},
		{Method: http.MethodPost, Path: "/boards/:boardID/saved-templates", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.saveBoardAsTemplate},
		{Method: http.MethodDelete, Path: "/orgs/:orgID/saved-templates/:templateID", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.deleteSavedTemplate},
	}
}

func (s *Service) listSavedTemplates(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	listing, err := s.SavedTemplateService.List(c.Request.Context(), viewer)
	if savedTemplateFailed(c, err, "failed to list the saved templates") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) saveBoardAsTemplate(c *gin.Context) {
	principal, ok := utils.Principal(c)
	if !ok {
		return
	}

	boardID, ok := utils.ParseIDParam(c, boardIDParam)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.SaveTemplateRequest](c)
	if !ok {
		return
	}

	command := savedtemplate.SaveCommand{
		Principal:   principal,
		BoardID:     boardID,
		Name:        req.Name,
		Description: req.Description,
		Scope:       model.SavedTemplateScope(req.Scope),
	}

	saved, err := s.SavedTemplateService.Save(c.Request.Context(), command)
	if savedTemplateFailed(c, err, "failed to save the template") {
		return
	}

	c.JSON(http.StatusCreated, saved)
}

func (s *Service) deleteSavedTemplate(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	templateID, ok := utils.ParseIDParam(c, savedTemplateIDParam)
	if !ok {
		return
	}

	removal := spec.SavedTemplateRemoval{OrgID: viewer.OrgID, ID: templateID, UserID: viewer.UserID}

	err := s.SavedTemplateService.Delete(c.Request.Context(), removal)
	if savedTemplateFailed(c, err, "failed to delete the template") {
		return
	}

	noContent(c)
}

func savedTemplateFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	if utils.MapEntitlementError(c, err) {
		return true
	}

	answerError(c, savedTemplateErrorAnswers, err, msg)

	return true
}
