package controllers

import (
	"errors"
	"net/http"
	"strings"

	"retrotide/internal/admin"
	"retrotide/internal/audit"
	"retrotide/internal/authz"
	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/gin-gonic/gin"
)

type (
	adminErrorAnswer struct {
		err      error
		status   int
		code     string
		detailed bool
	}
)

var (
	adminErrorAnswers = []adminErrorAnswer{
		{err: admin.ErrUserNotFound, status: http.StatusNotFound},
		{err: admin.ErrRoleNotHeld, status: http.StatusNotFound},
		{err: admin.ErrProtectedUser, status: http.StatusForbidden},
		{err: admin.ErrRoleOutranksActor, status: http.StatusForbidden},
		{err: admin.ErrImmutableRole, status: http.StatusForbidden},
		{err: admin.ErrUnknownRole, status: http.StatusBadRequest},
		{err: admin.ErrUnknownPermission, status: http.StatusBadRequest, detailed: true},
		{err: admin.ErrRestrictedPermission, status: http.StatusBadRequest, detailed: true},
		{err: admin.ErrReasonRequired, status: http.StatusBadRequest},
		{err: admin.ErrInvalidEmail, status: http.StatusBadRequest},
		{err: admin.ErrEmailUnchanged, status: http.StatusBadRequest},
		{err: admin.ErrEmailTaken, status: http.StatusConflict},
		{err: admin.ErrEmailUnavailable, status: http.StatusServiceUnavailable, code: emailUnavailableCode},
		{err: admin.ErrDisplayNameRequired, status: http.StatusBadRequest},
		{err: admin.ErrUnknownSetting, status: http.StatusBadRequest, detailed: true},
		{err: admin.ErrInvalidSetting, status: http.StatusBadRequest, detailed: true},
		{err: admin.ErrNoSignInMethod, status: http.StatusConflict, code: "no_sign_in_method"},
		{err: admin.ErrInvitationNotFound, status: http.StatusNotFound},
		{err: admin.ErrOrganisationNotFound, status: http.StatusNotFound},
		{err: admin.ErrSitePublic, status: http.StatusConflict, code: "site_public"},
		{err: admin.ErrAccountExists, status: http.StatusConflict, code: "account_exists"},
		{err: admin.ErrInvitationPending, status: http.StatusConflict, code: "invitation_pending"},
		{err: admin.ErrSoleOwner, status: http.StatusConflict, code: "sole_owner"},
	}
)

func (s *Service) getAllAdminRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/admin/users", Before: gin.HandlersChain{s.Guards.Permission(authz.PermViewUsers)}, Handler: s.adminListUsers},
		{Method: http.MethodGet, Path: "/admin/users/:id", Before: gin.HandlersChain{s.Guards.Permission(authz.PermViewUsers)}, Handler: s.adminGetUser},
		{Method: http.MethodGet, Path: "/admin/users/:id/audit-log", Before: gin.HandlersChain{s.Guards.Permission(authz.PermViewAuditLog)}, Handler: s.adminGetUserAuditLog},
		{Method: http.MethodPost, Path: "/admin/users/:id/role", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageRoles)}, Handler: s.adminSetUserRole},
		{Method: http.MethodDelete, Path: "/admin/users/:id/role", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageRoles)}, Handler: s.adminRemoveUserRole},
		{Method: http.MethodPost, Path: "/admin/users/:id/ban", Before: gin.HandlersChain{s.Guards.Permission(authz.PermBanUser)}, Handler: s.adminBanUser},
		{Method: http.MethodPost, Path: "/admin/users/:id/unban", Before: gin.HandlersChain{s.Guards.Permission(authz.PermBanUser)}, Handler: s.adminUnbanUser},
		{Method: http.MethodPost, Path: "/admin/users/:id/lock", Before: gin.HandlersChain{s.Guards.Permission(authz.PermBanUser)}, Handler: s.adminLockUser},
		{Method: http.MethodPost, Path: "/admin/users/:id/unlock", Before: gin.HandlersChain{s.Guards.Permission(authz.PermBanUser)}, Handler: s.adminUnlockUser},
		{Method: http.MethodPost, Path: "/admin/users/:id/force-logout", Before: gin.HandlersChain{s.Guards.Permission(authz.PermBanUser)}, Handler: s.adminForceLogout},
		{Method: http.MethodDelete, Path: "/admin/users/:id", Before: gin.HandlersChain{s.Guards.Permission(authz.PermDeleteAnyUser)}, Handler: s.adminDeleteUser},
		{Method: http.MethodPost, Path: "/admin/users/:id/reset-password", Before: gin.HandlersChain{s.Guards.Permission(authz.PermResetPassword)}, Handler: s.adminResetUserPassword},
		{Method: http.MethodPut, Path: "/admin/users/:id/email", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.Permission(authz.PermManageUserEmail)}, Handler: s.adminSetUserEmail},
		{Method: http.MethodPost, Path: "/admin/users/:id/verify-email", Before: gin.HandlersChain{s.Guards.Permission(authz.PermSetEmailVerified)}, Handler: s.adminVerifyUserEmail},
		{Method: http.MethodPost, Path: "/admin/users/:id/unverify-email", Before: gin.HandlersChain{s.Guards.Permission(authz.PermSetEmailVerified)}, Handler: s.adminUnverifyUserEmail},
		{Method: http.MethodPut, Path: "/admin/users/:id/display-name", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageUserAccount)}, Handler: s.adminSetUserDisplayName},
		{Method: http.MethodGet, Path: "/admin/invitations", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageUserAccount)}, Handler: s.adminListInvitations},
		{Method: http.MethodPost, Path: "/admin/invitations", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.Permission(authz.PermManageUserAccount)}, Handler: s.adminInviteUser},
		{Method: http.MethodPost, Path: "/admin/invitations/:id/resend", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.Permission(authz.PermManageUserAccount)}, Handler: s.adminResendInvitation},
		{Method: http.MethodDelete, Path: "/admin/invitations/:id", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageUserAccount)}, Handler: s.adminCancelInvitation},
		{Method: http.MethodGet, Path: "/admin/settings", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageSettings)}, Handler: s.adminGetSettings},
		{Method: http.MethodPut, Path: "/admin/settings", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageSettings)}, Handler: s.adminUpdateSettings},
		{Method: http.MethodPost, Path: "/admin/settings/test-email", Before: gin.HandlersChain{s.Guards.RateLimitMail, s.Guards.Permission(authz.PermManageSettings)}, Handler: s.adminSendTestEmail},
		{Method: http.MethodGet, Path: "/admin/permissions", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageRoles)}, Handler: s.adminGetPermissions},
		{Method: http.MethodPut, Path: "/admin/permissions/roles/:role", Before: gin.HandlersChain{s.Guards.Permission(authz.PermManageRoles)}, Handler: s.adminUpdateRolePermissions},
		{Method: http.MethodGet, Path: "/admin/audit-log", Before: gin.HandlersChain{s.Guards.Permission(authz.PermViewAuditLog)}, Handler: s.adminGetAuditLog},
		{Method: http.MethodGet, Path: "/admin/stats", Before: gin.HandlersChain{s.Guards.Permission(authz.PermViewStats)}, Handler: s.adminGetStats},
		{Method: http.MethodGet, Path: "/admin/organisations", Before: gin.HandlersChain{s.Guards.Permission(authz.PermViewOrganisations)}, Handler: s.adminListOrganisations},
		{Method: http.MethodGet, Path: "/admin/organisations/:" + orgIDParam, Before: gin.HandlersChain{s.Guards.Permission(authz.PermViewOrganisations)}, Handler: s.adminGetOrganisation},
	}
}

func (s *Service) adminGetStats(c *gin.Context) {
	viewerID, ok := utils.UserID(c)
	if !ok {
		return
	}

	stats, err := s.AdminService.GetStats(c.Request.Context(), viewerID)
	if adminFailed(c, err, "failed to load the site stats") {
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (s *Service) adminListOrganisations(c *gin.Context) {
	limit, offset, ok := pageQuery(c)
	if !ok {
		return
	}

	listing, err := s.AdminService.ListOrganisations(c.Request.Context(), spec.OrganisationListing{Search: c.Query("search"), Limit: limit, Offset: offset})
	if adminFailed(c, err, "failed to list the organisations") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) adminGetOrganisation(c *gin.Context) {
	orgID, ok := utils.ParseIDParam(c, orgIDParam)
	if !ok {
		return
	}

	detail, err := s.AdminService.GetOrganisation(c.Request.Context(), orgID)
	if adminFailed(c, err, "failed to load the organisation") {
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (s *Service) adminListUsers(c *gin.Context) {
	limit, offset, ok := pageQuery(c)
	if !ok {
		return
	}

	listing, err := s.AdminService.ListUsers(c.Request.Context(), spec.UserListing{Search: c.Query("search"), Limit: limit, Offset: offset})
	if adminFailed(c, err, "failed to list the users") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) adminGetUser(c *gin.Context) {
	userID, ok := utils.ParseIDParam(c, "id")
	if !ok {
		return
	}

	detail, err := s.AdminService.GetUser(c.Request.Context(), userID)
	if adminFailed(c, err, "failed to load the user") {
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (s *Service) adminGetUserAuditLog(c *gin.Context) {
	userID, ok := utils.ParseIDParam(c, "id")
	if !ok {
		return
	}

	limit, offset, ok := pageQuery(c)
	if !ok {
		return
	}

	listing, err := s.AdminService.GetUserAuditLog(c.Request.Context(), spec.AuditLogSubjectListing{SubjectID: userID, Limit: limit, Offset: offset})
	if adminFailed(c, err, "failed to list the user's audit log") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) adminSetUserRole(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.SetRoleRequest](c)
	if !ok {
		return
	}

	err := s.AdminService.SetUserRole(c.Request.Context(), spec.StaffRoleChange{StaffAction: action.StaffAction, UserID: action.UserID, Role: role.Role(strings.TrimSpace(req.Role))})
	if adminFailed(c, err, "failed to set the user's role") {
		return
	}

	noContent(c)
}

func (s *Service) adminRemoveUserRole(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.SetRoleRequest](c)
	if !ok {
		return
	}

	err := s.AdminService.RemoveUserRole(c.Request.Context(), spec.StaffRoleChange{StaffAction: action.StaffAction, UserID: action.UserID, Role: role.Role(strings.TrimSpace(req.Role))})
	if adminFailed(c, err, "failed to remove the user's role") {
		return
	}

	noContent(c)
}

func (s *Service) adminBanUser(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.BanUserRequest](c)
	if !ok {
		return
	}

	err := s.AdminService.BanUser(c.Request.Context(), spec.StaffUserRestriction{StaffAction: action.StaffAction, UserID: action.UserID, Reason: req.Reason})
	if adminFailed(c, err, "failed to ban the user") {
		return
	}

	noContent(c)
}

func (s *Service) adminUnbanUser(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	err := s.AdminService.UnbanUser(c.Request.Context(), action)
	if adminFailed(c, err, "failed to unban the user") {
		return
	}

	noContent(c)
}

func (s *Service) adminLockUser(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.LockUserRequest](c)
	if !ok {
		return
	}

	err := s.AdminService.LockUser(c.Request.Context(), spec.StaffUserRestriction{StaffAction: action.StaffAction, UserID: action.UserID, Reason: req.Reason})
	if adminFailed(c, err, "failed to lock the user") {
		return
	}

	noContent(c)
}

func (s *Service) adminUnlockUser(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	err := s.AdminService.UnlockUser(c.Request.Context(), action)
	if adminFailed(c, err, "failed to unlock the user") {
		return
	}

	noContent(c)
}

func (s *Service) adminForceLogout(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	err := s.AdminService.ForceLogout(c.Request.Context(), action)
	if adminFailed(c, err, "failed to sign the user out") {
		return
	}

	noContent(c)
}

func (s *Service) adminDeleteUser(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	err := s.AdminService.DeleteUser(c.Request.Context(), action)
	if adminFailed(c, err, "failed to delete the user") {
		return
	}

	noContent(c)
}

func (s *Service) adminResetUserPassword(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	reset, err := s.AdminService.ResetUserPassword(c.Request.Context(), action)
	if adminFailed(c, err, "failed to reset the user's password") {
		return
	}

	c.JSON(http.StatusOK, reset)
}

func (s *Service) adminSetUserEmail(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.AdminSetEmailRequest](c)
	if !ok {
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email is required"})
		return
	}

	err := s.AdminService.SetUserEmail(c.Request.Context(), spec.StaffEmailChange{StaffAction: action.StaffAction, UserID: action.UserID, Email: req.Email})
	if adminFailed(c, err, "failed to change the user's email address") {
		return
	}

	noContent(c)
}

func (s *Service) adminVerifyUserEmail(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	err := s.AdminService.VerifyUserEmail(c.Request.Context(), action)
	if adminFailed(c, err, "failed to mark the user's email address verified") {
		return
	}

	noContent(c)
}

func (s *Service) adminUnverifyUserEmail(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	err := s.AdminService.UnverifyUserEmail(c.Request.Context(), action)
	if adminFailed(c, err, "failed to mark the user's email address unverified") {
		return
	}

	noContent(c)
}

func (s *Service) adminSetUserDisplayName(c *gin.Context) {
	action, ok := staffUserAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.UpdateDisplayNameRequest](c)
	if !ok {
		return
	}

	err := s.AdminService.SetUserDisplayName(c.Request.Context(), spec.StaffDisplayNameChange{StaffAction: action.StaffAction, UserID: action.UserID, DisplayName: req.DisplayName})
	if adminFailed(c, err, "failed to set the user's display name") {
		return
	}

	noContent(c)
}

func (s *Service) adminListInvitations(c *gin.Context) {
	limit, offset, ok := pageQuery(c)
	if !ok {
		return
	}

	listing, err := s.AdminService.ListInvitations(c.Request.Context(), spec.InvitationListing{Limit: limit, Offset: offset})
	if adminFailed(c, err, "failed to list the invitations") {
		return
	}

	c.JSON(http.StatusOK, listing)
}

func (s *Service) adminInviteUser(c *gin.Context) {
	action, ok := staffAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.AdminInviteRequest](c)
	if !ok {
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email is required"})
		return
	}

	invited, err := s.AdminService.InviteUser(c.Request.Context(), spec.StaffInvitation{StaffAction: action, Email: req.Email})
	if adminFailed(c, err, "failed to invite the user") {
		return
	}

	c.JSON(http.StatusCreated, invited)
}

func (s *Service) adminResendInvitation(c *gin.Context) {
	action, ok := staffInvitationAction(c)
	if !ok {
		return
	}

	err := s.AdminService.ResendInvitation(c.Request.Context(), action)
	if adminFailed(c, err, "failed to resend the invitation") {
		return
	}

	noContent(c)
}

func (s *Service) adminCancelInvitation(c *gin.Context) {
	action, ok := staffInvitationAction(c)
	if !ok {
		return
	}

	err := s.AdminService.CancelInvitation(c.Request.Context(), action)
	if adminFailed(c, err, "failed to cancel the invitation") {
		return
	}

	noContent(c)
}

func (s *Service) adminGetSettings(c *gin.Context) {
	current, err := s.AdminService.GetSettings(c.Request.Context())
	if adminFailed(c, err, "failed to load the settings") {
		return
	}

	c.JSON(http.StatusOK, current)
}

func (s *Service) adminUpdateSettings(c *gin.Context) {
	action, ok := staffAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.UpdateSettingsRequest](c)
	if !ok {
		return
	}

	err := s.AdminService.UpdateSettings(c.Request.Context(), spec.StaffSettingsUpdate{StaffAction: action, Values: req.Settings})
	if adminFailed(c, err, "failed to update the settings") {
		return
	}

	noContent(c)
}

func (s *Service) adminSendTestEmail(c *gin.Context) {
	action, ok := staffAction(c)
	if !ok {
		return
	}

	err := s.AdminService.SendTestEmail(c.Request.Context(), action)
	if adminFailed(c, err, "failed to send the test email") {
		return
	}

	noContent(c)
}

func (s *Service) adminGetPermissions(c *gin.Context) {
	permissions, err := s.AdminService.GetPermissions(c.Request.Context())
	if adminFailed(c, err, "failed to load the permissions") {
		return
	}

	c.JSON(http.StatusOK, permissions)
}

func (s *Service) adminUpdateRolePermissions(c *gin.Context) {
	action, ok := staffAction(c)
	if !ok {
		return
	}

	req, ok := utils.BindJSON[dto.UpdatePermissionsRequest](c)
	if !ok {
		return
	}

	update := spec.StaffRolePermissionsUpdate{StaffAction: action, Role: role.Role(strings.TrimSpace(c.Param("role"))), Permissions: req.Permissions}

	err := s.AdminService.UpdateRolePermissions(c.Request.Context(), update)
	if adminFailed(c, err, "failed to update the role's permissions") {
		return
	}

	noContent(c)
}

func (s *Service) adminGetAuditLog(c *gin.Context) {
	limit, offset, ok := pageQuery(c)
	if !ok {
		return
	}

	listing := spec.AuditLogListing{Action: audit.Action(strings.TrimSpace(c.Query("action"))), Limit: limit, Offset: offset}

	entries, err := s.AdminService.GetAuditLog(c.Request.Context(), listing)
	if adminFailed(c, err, "failed to list the audit log") {
		return
	}

	c.JSON(http.StatusOK, entries)
}

func staffAction(c *gin.Context) (spec.StaffAction, bool) {
	actorID, ok := utils.UserID(c)
	if !ok {
		return spec.StaffAction{}, false
	}

	return spec.StaffAction{ActorID: actorID, IP: c.ClientIP()}, true
}

func staffUserAction(c *gin.Context) (spec.StaffUserAction, bool) {
	action, ok := staffAction(c)
	if !ok {
		return spec.StaffUserAction{}, false
	}

	userID, ok := utils.ParseIDParam(c, "id")
	if !ok {
		return spec.StaffUserAction{}, false
	}

	return spec.StaffUserAction{StaffAction: action, UserID: userID}, true
}

func staffInvitationAction(c *gin.Context) (spec.StaffInvitationAction, bool) {
	action, ok := staffAction(c)
	if !ok {
		return spec.StaffInvitationAction{}, false
	}

	invitationID, ok := utils.ParseIDParam(c, "id")
	if !ok {
		return spec.StaffInvitationAction{}, false
	}

	return spec.StaffInvitationAction{StaffAction: action, InvitationID: invitationID}, true
}

func noContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
	c.Writer.WriteHeaderNow()
}

func pageQuery(c *gin.Context) (int, int, bool) {
	limit, ok := utils.QueryInt(c, "limit", 0)
	if !ok {
		return 0, 0, false
	}

	offset, ok := utils.QueryInt(c, "offset", 0)
	if !ok {
		return 0, 0, false
	}

	return limit, offset, true
}

func adminFailed(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, admin.ErrSettingsCacheStale) {
		utils.InternalError(c, admin.ErrSettingsCacheStale.Error(), err)
		return true
	}

	if errors.Is(err, admin.ErrStaffCacheStale) {
		utils.InternalError(c, admin.ErrStaffCacheStale.Error(), err)
		return true
	}

	if errors.Is(err, admin.ErrEmailNotSent) {
		utils.InternalError(c, admin.ErrEmailNotSent.Error(), err)
		return true
	}

	if errors.Is(err, admin.ErrInvitationNotSent) {
		utils.InternalError(c, admin.ErrInvitationNotSent.Error(), err)
		return true
	}

	if errors.Is(err, admin.ErrNotAudited) {
		c.Error(err)
		return false
	}

	for _, answer := range adminErrorAnswers {
		if !errors.Is(err, answer.err) {
			continue
		}

		message := answer.err.Error()
		if answer.detailed {
			message = err.Error()
		}

		c.AbortWithStatusJSON(answer.status, dto.ErrorResponse{Error: message, Code: answer.code})
		return true
	}

	utils.InternalError(c, msg, err)

	return true
}
