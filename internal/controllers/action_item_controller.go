package controllers

import (
	"net/http"

	"retrotide/internal/actionitem"
	"retrotide/internal/board/actionitems"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	actionItemIDParam = "itemID"
	statusQuery       = "status"
	assigneeQuery     = "assignee"
	teamQuery         = "team"
	assigneeMe        = "me"
	assigneeNobody    = "none"
	teamOrganisation  = "organisation"
)

var (
	actionItemErrorAnswers = []errorAnswer{
		{err: actionitem.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: actionitem.ErrInvalidStatus, status: http.StatusBadRequest, code: "invalid_action_item_status"},
	}
)

func (s *Service) getAllActionItemRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/orgs/:orgID/action-items", Before: gin.HandlersChain{s.Guards.OrgPermission(orgauthz.PermOrgView)}, Handler: s.listActionItems},
		{Method: http.MethodGet, Path: "/auth/mentions", Before: gin.HandlersChain{s.Guards.Auth}, Handler: s.listMentions},
		{Method: http.MethodGet, Path: "/boards/:boardID/people", Before: gin.HandlersChain{s.Guards.Principal}, Handler: s.listBoardPeople},
		{Method: http.MethodPost, Path: "/boards/:boardID/action-items", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.addActionItem},
		{Method: http.MethodPut, Path: "/boards/:boardID/action-items/:itemID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.editActionItem},
		{Method: http.MethodPost, Path: "/boards/:boardID/action-items/:itemID/status", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.markActionItem},
		{Method: http.MethodDelete, Path: "/boards/:boardID/action-items/:itemID", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.deleteActionItem},
		{Method: http.MethodPost, Path: "/boards/:boardID/previous-actions/:itemID/review", Before: gin.HandlersChain{s.Guards.Principal, s.Guards.BoardWriteLimit}, Handler: s.reviewActionItem},
	}
}

func (s *Service) listActionItems(c *gin.Context) {
	viewer, ok := orgViewer(c)
	if !ok {
		return
	}

	limit, offset, ok := pageQuery(c)
	if !ok {
		return
	}

	query := actionitem.ListQuery{Viewer: viewer, Limit: limit, Offset: offset}

	if raw, present := c.GetQuery(statusQuery); present {
		query.Status = new(model.ActionItemStatus(raw))
	}

	if !assigneeFilter(c, viewer.UserID, &query) {
		return
	}

	if !teamFilter(c, &query) {
		return
	}

	listing, err := s.ActionItemService.List(c.Request.Context(), query)
	if err != nil {
		answerError(c, actionItemErrorAnswers, err, "failed to list the action items")
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) listMentions(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		return
	}

	listing, err := s.ActionItemService.Mentions(c.Request.Context(), userID)
	if err != nil {
		answerError(c, actionItemErrorAnswers, err, "failed to list your mentions")
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) listBoardPeople(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	people, err := s.BoardService.People(c.Request.Context(), spec.BoardAccessQuery{Principal: command.Principal, BoardID: command.BoardID})
	if boardFailed(c, err, "failed to list the board's people") {
		return
	}

	c.JSON(http.StatusOK, people)
}

func (s *Service) addActionItem(c *gin.Context) {
	command, ok := boardCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.CreateActionItemRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	add := actionitems.AddCommand{
		Command:      command,
		Fields:       actionItemFields(req.ActionItemFieldsRequest),
		ItemID:       req.ID,
		SourceCardID: req.SourceCardID,
	}

	result, err := s.BoardActionItems.Add(c.Request.Context(), add)
	if boardFailed(c, err, "failed to add the action item") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) editActionItem(c *gin.Context) {
	command, ok := actionItemCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ActionItemFieldsRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	edit := actionitems.EditCommand{Command: command.Command, Fields: actionItemFields(req), ItemID: command.ItemID}

	result, err := s.BoardActionItems.Edit(c.Request.Context(), edit)
	if boardFailed(c, err, "failed to edit the action item") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) markActionItem(c *gin.Context) {
	command, ok := actionItemCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ActionItemStatusRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	mark := actionitems.StatusCommand{Command: command.Command, ItemID: command.ItemID, Status: req.Status}

	result, err := s.BoardActionItems.SetStatus(c.Request.Context(), mark)
	if boardFailed(c, err, "failed to mark the action item") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) deleteActionItem(c *gin.Context) {
	command, ok := actionItemCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BoardCommandRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	result, err := s.BoardActionItems.Delete(c.Request.Context(), command)
	if boardFailed(c, err, "failed to delete the action item") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Service) reviewActionItem(c *gin.Context) {
	command, ok := actionItemCommand(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.ReviewActionItemRequest](c)
	if !ok {
		return
	}

	command.OpID = req.OpID

	review := actionitems.ReviewCommand{Command: command.Command, ItemID: command.ItemID, Outcome: req.Outcome}

	result, err := s.BoardActionItems.Review(c.Request.Context(), review)
	if boardFailed(c, err, "failed to review the action item") {
		return
	}

	c.JSON(http.StatusOK, result)
}

func actionItemCommand(c *gin.Context) (actionitems.ItemCommand, bool) {
	command, ok := boardCommand(c)
	if !ok {
		return actionitems.ItemCommand{}, false
	}

	itemID, ok := utils.ParseIDParam(c, actionItemIDParam)
	if !ok {
		return actionitems.ItemCommand{}, false
	}

	return actionitems.ItemCommand{Command: command, ItemID: itemID}, true
}

func actionItemFields(req dto.ActionItemFieldsRequest) actionitems.Fields {
	return actionitems.Fields{
		Title:          req.Title,
		AssigneeUserID: req.AssigneeUserID,
		DueOn:          req.DueOn,
		Mentions:       req.Mentions,
	}
}

func assigneeFilter(c *gin.Context, viewerID uuid.UUID, query *actionitem.ListQuery) bool {
	raw, present := c.GetQuery(assigneeQuery)
	if !present || raw == "" {
		return true
	}

	switch raw {
	case assigneeMe:
		query.AssigneeUserID = new(viewerID)
	case assigneeNobody:
		query.Unassigned = true
	default:
		assigneeID, err := uuid.Parse(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + assigneeQuery})
			return false
		}

		query.AssigneeUserID = new(assigneeID)
	}

	return true
}

func teamFilter(c *gin.Context, query *actionitem.ListQuery) bool {
	raw, present := c.GetQuery(teamQuery)
	if !present || raw == "" {
		return true
	}

	if raw == teamOrganisation {
		query.OrganisationBoards = true
		return true
	}

	teamID, err := uuid.Parse(raw)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid " + teamQuery})
		return false
	}

	query.TeamID = new(teamID)

	return true
}
