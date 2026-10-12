package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/admin"
	"retrotide/internal/audit"
	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	adminHarness struct {
		*testutil.Harness
		admin *admin.MockService
	}

	adminCommandCase struct {
		name       string
		method     string
		suffix     string
		permission authz.Permission
		expect     func(h *adminHarness, action spec.StaffUserAction) *mock.Call
	}
)

const (
	adminPeerIP   = "198.51.100.7"
	adminTargetID = "0b6f3a52-6a55-4d8c-9d0e-6f1f4c1b2a77"
)

func newAdminHarness(t *testing.T) *adminHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	a := &adminHarness{Harness: h, admin: admin.NewMockService(t)}

	s := &Service{AdminService: a.admin, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllAdminRoutes())

	return a
}

func (h *adminHarness) signInStaff(userID uuid.UUID, permission authz.Permission, write bool) *http.Cookie {
	cookie := h.SignIn(userID)
	if write {
		h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	}

	h.Authz.EXPECT().Can(mock.Anything, userID, permission).Return(true, nil)

	return cookie
}

func staffUser(actorID uuid.UUID, userID uuid.UUID) spec.StaffUserAction {
	return spec.StaffUserAction{ActorID: actorID, IP: adminPeerIP, UserID: userID}
}

func TestAdminController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{AdminService: admin.NewMockService(t), Guards: h.Guards}

	// when the list is mounted exactly as production mounts it, so a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllAdminRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/admin/users",
		"HEAD /api/v1/admin/users",
		"GET /api/v1/admin/users/:id",
		"HEAD /api/v1/admin/users/:id",
		"GET /api/v1/admin/users/:id/audit-log",
		"HEAD /api/v1/admin/users/:id/audit-log",
		"POST /api/v1/admin/users/:id/role",
		"DELETE /api/v1/admin/users/:id/role",
		"POST /api/v1/admin/users/:id/ban",
		"POST /api/v1/admin/users/:id/unban",
		"POST /api/v1/admin/users/:id/lock",
		"POST /api/v1/admin/users/:id/unlock",
		"POST /api/v1/admin/users/:id/force-logout",
		"DELETE /api/v1/admin/users/:id",
		"POST /api/v1/admin/users/:id/reset-password",
		"PUT /api/v1/admin/users/:id/email",
		"POST /api/v1/admin/users/:id/verify-email",
		"POST /api/v1/admin/users/:id/unverify-email",
		"PUT /api/v1/admin/users/:id/display-name",
		"GET /api/v1/admin/invitations",
		"HEAD /api/v1/admin/invitations",
		"POST /api/v1/admin/invitations",
		"POST /api/v1/admin/invitations/:id/resend",
		"DELETE /api/v1/admin/invitations/:id",
		"GET /api/v1/admin/settings",
		"HEAD /api/v1/admin/settings",
		"PUT /api/v1/admin/settings",
		"POST /api/v1/admin/settings/test-email",
		"GET /api/v1/admin/permissions",
		"HEAD /api/v1/admin/permissions",
		"PUT /api/v1/admin/permissions/roles/:role",
		"GET /api/v1/admin/audit-log",
		"HEAD /api/v1/admin/audit-log",
		"GET /api/v1/admin/stats",
		"HEAD /api/v1/admin/stats",
		"GET /api/v1/admin/organisations",
		"HEAD /api/v1/admin/organisations",
		"GET /api/v1/admin/organisations/:orgID",
		"HEAD /api/v1/admin/organisations/:orgID",
	}, registeredRoutes(h.Engine))
}

func TestAdminController_RoutesRunThePermissionFailureSuite(t *testing.T) {
	userPath := "/api/v1/admin/users/" + adminTargetID
	invitationPath := "/api/v1/admin/invitations/" + adminTargetID

	routes := []testutil.PermissionRoute{
		{Method: http.MethodGet, Path: "/api/v1/admin/users", Permission: authz.PermViewUsers},
		{Method: http.MethodGet, Path: userPath, Permission: authz.PermViewUsers},
		{Method: http.MethodGet, Path: userPath + "/audit-log", Permission: authz.PermViewAuditLog},
		{Method: http.MethodPost, Path: userPath + "/role", Body: dto.SetRoleRequest{Role: "support"}, Permission: authz.PermManageRoles},
		{Method: http.MethodDelete, Path: userPath + "/role", Body: dto.SetRoleRequest{Role: "support"}, Permission: authz.PermManageRoles},
		{Method: http.MethodPost, Path: userPath + "/ban", Body: dto.BanUserRequest{Reason: "spam"}, Permission: authz.PermBanUser},
		{Method: http.MethodPost, Path: userPath + "/unban", Permission: authz.PermBanUser},
		{Method: http.MethodPost, Path: userPath + "/lock", Body: dto.LockUserRequest{Reason: "spam"}, Permission: authz.PermBanUser},
		{Method: http.MethodPost, Path: userPath + "/unlock", Permission: authz.PermBanUser},
		{Method: http.MethodPost, Path: userPath + "/force-logout", Permission: authz.PermBanUser},
		{Method: http.MethodDelete, Path: userPath, Permission: authz.PermDeleteAnyUser},
		{Method: http.MethodPost, Path: userPath + "/reset-password", Permission: authz.PermResetPassword},
		{Method: http.MethodPut, Path: userPath + "/email", Body: dto.AdminSetEmailRequest{Email: "new@example.com"}, Permission: authz.PermManageUserEmail},
		{Method: http.MethodPost, Path: userPath + "/verify-email", Permission: authz.PermSetEmailVerified},
		{Method: http.MethodPost, Path: userPath + "/unverify-email", Permission: authz.PermSetEmailVerified},
		{Method: http.MethodPut, Path: userPath + "/display-name", Body: dto.UpdateDisplayNameRequest{DisplayName: "Renamed"}, Permission: authz.PermManageUserAccount},
		{Method: http.MethodGet, Path: "/api/v1/admin/invitations", Permission: authz.PermManageUserAccount},
		{Method: http.MethodPost, Path: "/api/v1/admin/invitations", Body: dto.AdminInviteRequest{Email: "carol@example.com"}, Permission: authz.PermManageUserAccount},
		{Method: http.MethodPost, Path: invitationPath + "/resend", Permission: authz.PermManageUserAccount},
		{Method: http.MethodDelete, Path: invitationPath, Permission: authz.PermManageUserAccount},
		{Method: http.MethodGet, Path: "/api/v1/admin/settings", Permission: authz.PermManageSettings},
		{Method: http.MethodPut, Path: "/api/v1/admin/settings", Body: dto.UpdateSettingsRequest{}, Permission: authz.PermManageSettings},
		{Method: http.MethodPost, Path: "/api/v1/admin/settings/test-email", Permission: authz.PermManageSettings},
		{Method: http.MethodGet, Path: "/api/v1/admin/permissions", Permission: authz.PermManageRoles},
		{Method: http.MethodPut, Path: "/api/v1/admin/permissions/roles/support", Body: dto.UpdatePermissionsRequest{}, Permission: authz.PermManageRoles},
		{Method: http.MethodGet, Path: "/api/v1/admin/audit-log", Permission: authz.PermViewAuditLog},
		{Method: http.MethodGet, Path: "/api/v1/admin/stats", Permission: authz.PermViewStats},
		{Method: http.MethodGet, Path: "/api/v1/admin/organisations", Permission: authz.PermViewOrganisations},
		{Method: http.MethodGet, Path: "/api/v1/admin/organisations/" + adminTargetID, Permission: authz.PermViewOrganisations},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newAdminHarness(t).Harness
			}, route)
		})
	}
}

func TestAdminController_ListsUsersWithTheSearchAndPage(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID := uuid.New()
	cookie := h.signInStaff(actorID, authz.PermViewUsers, false)
	listing := &dto.AdminUserListResponse{Users: []dto.AdminUserResponse{}, Total: 0, Limit: 10, Offset: 30}
	h.admin.EXPECT().ListUsers(mock.Anything, spec.UserListing{Search: "alice", Limit: 10, Offset: 30}).Return(listing, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/users?search=alice&limit=10&offset=30").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"users":[],"total":0,"limit":10,"offset":30}`, rec.Body.String())
}

func TestAdminController_RefusesAMalformedPage(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermViewAuditLog, false)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/audit-log?offset=later").WithCookie(cookie).Record()

	// then the service is never asked for a page it did not request
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid offset"}`, rec.Body.String())
}

func TestAdminController_ShowsOneUser(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermViewUsers, false)
	detail := &dto.AdminUserDetailResponse{ID: userID, Email: "alice@example.com", Role: role.Support, CreatedAt: "2026-09-01T12:00:00Z", HasPassword: true}
	h.admin.EXPECT().GetUser(mock.Anything, userID).Return(detail, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/users/"+userID.String()).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"id":%q,"email":"alice@example.com","display_name":"","role":"support","email_verified":false,"banned":false,"locked":false,"created_at":"2026-09-01T12:00:00Z","has_password":true}`, userID), rec.Body.String())
}

func TestAdminController_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	h := newAdminHarness(t)
	userID := uuid.New()
	cookie := h.signInStaff(uuid.New(), authz.PermViewUsers, false)
	h.admin.EXPECT().GetUser(mock.Anything, userID).Return(nil, admin.ErrUserNotFound)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/users/"+userID.String()).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"user not found"}`, rec.Body.String())
}

func TestAdminController_RefusesAMalformedUserID(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermBanUser, true)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/not-a-uuid/unban").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid id"}`, rec.Body.String())
}

func TestAdminController_ListsOneUsersAuditLog(t *testing.T) {
	// given
	h := newAdminHarness(t)
	userID := uuid.New()
	cookie := h.signInStaff(uuid.New(), authz.PermViewAuditLog, false)
	h.admin.EXPECT().GetUserAuditLog(mock.Anything, spec.AuditLogSubjectListing{SubjectID: userID, Limit: 5}).Return(&dto.AuditLogListResponse{Entries: []dto.AuditLogEntryResponse{}, Limit: 5}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/users/"+userID.String()+"/audit-log?limit=5").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"entries":[],"total":0,"limit":5,"offset":0}`, rec.Body.String())
}

func TestAdminController_SetsARoleWithTheCallersIP(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermManageRoles, true)
	action := staffUser(actorID, userID)
	h.admin.EXPECT().SetUserRole(mock.Anything, spec.StaffRoleChange{StaffAction: action.StaffAction, UserID: userID, Role: role.Support}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/role").WithJSONBody(dto.SetRoleRequest{Role: " support "}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestAdminController_RemovesARole(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermManageRoles, true)
	action := staffUser(actorID, userID)
	h.admin.EXPECT().RemoveUserRole(mock.Anything, spec.StaffRoleChange{StaffAction: action.StaffAction, UserID: userID, Role: role.Support}).Return(admin.ErrRoleNotHeld)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+userID.String()+"/role").WithJSONBody(dto.SetRoleRequest{Role: "support"}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"the user does not hold that role"}`, rec.Body.String())
}

func TestAdminController_MapsTheRankRefusals(t *testing.T) {
	tests := []struct {
		err  error
		body string
	}{
		{err: admin.ErrProtectedUser, body: `{"error":"this user cannot be modified"}`},
		{err: admin.ErrRoleOutranksActor, body: `{"error":"cannot grant a role equal to or above your own"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			actorID, userID := uuid.New(), uuid.New()
			cookie := h.signInStaff(actorID, authz.PermManageRoles, true)
			h.admin.EXPECT().SetUserRole(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/role").WithJSONBody(dto.SetRoleRequest{Role: "admin"}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAdminController_BansWithTheReason(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermBanUser, true)
	action := staffUser(actorID, userID)
	h.admin.EXPECT().BanUser(mock.Anything, spec.StaffUserRestriction{StaffAction: action.StaffAction, UserID: userID, Reason: "spam"}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/ban").WithJSONBody(dto.BanUserRequest{Reason: "spam"}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminController_ABanWithoutAReasonIsRefused(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermBanUser, true)
	h.admin.EXPECT().BanUser(mock.Anything, mock.Anything).Return(admin.ErrReasonRequired)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+uuid.NewString()+"/ban").WithJSONBody(dto.BanUserRequest{}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"a reason is required"}`, rec.Body.String())
}

func TestAdminController_LocksWithTheReason(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermBanUser, true)
	action := staffUser(actorID, userID)
	h.admin.EXPECT().LockUser(mock.Anything, spec.StaffUserRestriction{StaffAction: action.StaffAction, UserID: userID, Reason: "cooling off"}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/lock").WithJSONBody(dto.LockUserRequest{Reason: "cooling off"}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminController_RunsEachBodilessUserCommand(t *testing.T) {
	tests := []adminCommandCase{
		{name: "unban", method: http.MethodPost, suffix: "/unban", permission: authz.PermBanUser, expect: func(h *adminHarness, action spec.StaffUserAction) *mock.Call {
			return h.admin.EXPECT().UnbanUser(mock.Anything, action).Call
		}},
		{name: "unlock", method: http.MethodPost, suffix: "/unlock", permission: authz.PermBanUser, expect: func(h *adminHarness, action spec.StaffUserAction) *mock.Call {
			return h.admin.EXPECT().UnlockUser(mock.Anything, action).Call
		}},
		{name: "force logout", method: http.MethodPost, suffix: "/force-logout", permission: authz.PermBanUser, expect: func(h *adminHarness, action spec.StaffUserAction) *mock.Call {
			return h.admin.EXPECT().ForceLogout(mock.Anything, action).Call
		}},
		{name: "delete", method: http.MethodDelete, suffix: "", permission: authz.PermDeleteAnyUser, expect: func(h *adminHarness, action spec.StaffUserAction) *mock.Call {
			return h.admin.EXPECT().DeleteUser(mock.Anything, action).Call
		}},
		{name: "verify email", method: http.MethodPost, suffix: "/verify-email", permission: authz.PermSetEmailVerified, expect: func(h *adminHarness, action spec.StaffUserAction) *mock.Call {
			return h.admin.EXPECT().VerifyUserEmail(mock.Anything, action).Call
		}},
		{name: "unverify email", method: http.MethodPost, suffix: "/unverify-email", permission: authz.PermSetEmailVerified, expect: func(h *adminHarness, action spec.StaffUserAction) *mock.Call {
			return h.admin.EXPECT().UnverifyUserEmail(mock.Anything, action).Call
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			actorID, userID := uuid.New(), uuid.New()
			cookie := h.signInStaff(actorID, tc.permission, true)
			tc.expect(h, staffUser(actorID, userID)).Return(nil)

			// when
			rec := h.NewRequest(tc.method, "/api/v1/admin/users/"+userID.String()+tc.suffix).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

			// then
			assert.Equal(t, http.StatusNoContent, rec.Code)
			assert.Empty(t, rec.Body.String())
		})
	}
}

func TestAdminController_DeletingTheSoleOwnerOfASharedOrganisationIsAConflict(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermDeleteAnyUser, true)
	h.admin.EXPECT().DeleteUser(mock.Anything, staffUser(actorID, userID)).Return(fmt.Errorf("%w: delete account: the account owns an organisation that has other members", admin.ErrSoleOwner))

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+userID.String()).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"the user owns an organisation that has other members, so its ownership must be transferred or the organisation deleted first","code":"sole_owner"}`, rec.Body.String())
}

func TestAdminController_ACommandWhoseAuditEntryFailedStillSucceeds(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermBanUser, true)
	h.admin.EXPECT().UnbanUser(mock.Anything, mock.Anything).Return(errAuditLost)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/unban").WithCookie(cookie).Record()

	// then the unban happened, so it is not reported as a failure, and the lost entry is logged as the request's cause
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestAdminController_ReportsAnUnexpectedFailureWithoutItsCause(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermBanUser, true)
	h.admin.EXPECT().BanUser(mock.Anything, mock.Anything).Return(errServiceDown)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+uuid.NewString()+"/ban").WithJSONBody(dto.BanUserRequest{Reason: "spam"}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to ban the user"}`, rec.Body.String())
}

func TestAdminController_ResetsAPasswordAndHandsItBackOnce(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "recorded", err: nil},
		{name: "with its audit entry lost", err: errAuditLost},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			actorID, userID := uuid.New(), uuid.New()
			cookie := h.signInStaff(actorID, authz.PermResetPassword, true)
			h.admin.EXPECT().ResetUserPassword(mock.Anything, staffUser(actorID, userID)).Return(&dto.AdminPasswordResetResponse{Password: "FRESHPASSWORD"}, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/reset-password").WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

			// then the password is already set, so it is always handed back
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.JSONEq(t, `{"password":"FRESHPASSWORD"}`, rec.Body.String())
		})
	}
}

func TestAdminController_ChangesAUsersEmailThroughThePendingPath(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermManageUserEmail, true)
	action := staffUser(actorID, userID)
	h.admin.EXPECT().SetUserEmail(mock.Anything, spec.StaffEmailChange{StaffAction: action.StaffAction, UserID: userID, Email: "new@example.com"}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/users/"+userID.String()+"/email").WithJSONBody(dto.AdminSetEmailRequest{Email: "new@example.com"}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminController_MapsTheEmailChangeRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: admin.ErrInvalidEmail, status: http.StatusBadRequest, body: `{"error":"a valid email address is required"}`},
		{err: admin.ErrEmailUnchanged, status: http.StatusBadRequest, body: `{"error":"that is already the user's email address"}`},
		{err: admin.ErrEmailTaken, status: http.StatusConflict, body: `{"error":"that email address is already in use"}`},
		{err: admin.ErrEmailUnavailable, status: http.StatusServiceUnavailable, body: `{"error":"email is not available on this site","code":"email_unavailable"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			cookie := h.signInStaff(uuid.New(), authz.PermManageUserEmail, true)
			h.admin.EXPECT().SetUserEmail(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPut, "/api/v1/admin/users/"+uuid.NewString()+"/email").WithJSONBody(dto.AdminSetEmailRequest{Email: "new@example.com"}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAdminController_AFailedEmailChangeSendIsSeenByTheAdminEvenWhenUnaudited(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageUserEmail, true)
	h.admin.EXPECT().SetUserEmail(mock.Anything, mock.Anything).Return(fmt.Errorf("%w: %w", admin.ErrEmailNotSent, errors.Join(errServiceDown, errAuditLost)))

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/users/"+uuid.NewString()+"/email").WithJSONBody(dto.AdminSetEmailRequest{Email: "new@example.com"}).WithCookie(cookie).Record()

	// then a lost audit entry never turns the failed send into a success
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"the new address is pending, but its emails could not be sent"}`, rec.Body.String())
}

func TestAdminController_AnEmailChangeNeedsAnAddress(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageUserEmail, true)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/users/"+uuid.NewString()+"/email").WithJSONBody(dto.AdminSetEmailRequest{Email: "  "}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"email is required"}`, rec.Body.String())
}

func TestAdminController_EmailingRoutesDrawOnTheMailBucket(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "email change", method: http.MethodPut, path: "/api/v1/admin/users/" + adminTargetID + "/email", body: dto.AdminSetEmailRequest{Email: "new@example.com"}},
		{name: "test email", method: http.MethodPost, path: "/api/v1/admin/settings/test-email"},
		{name: "invitation", method: http.MethodPost, path: "/api/v1/admin/invitations", body: dto.AdminInviteRequest{Email: "carol@example.com"}},
		{name: "invitation resend", method: http.MethodPost, path: "/api/v1/admin/invitations/" + adminTargetID + "/resend"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the mail bucket of five for this IP is spent by requests that carry no session
			h := newAdminHarness(t)
			for range 5 {
				request := h.NewRequest(tc.method, tc.path).WithRemoteAddr(publicPeer)
				if tc.body != nil {
					request = request.WithJSONBody(tc.body)
				}

				require.Equal(t, http.StatusUnauthorized, request.Record().Code)
			}

			// when
			request := h.NewRequest(tc.method, tc.path).WithRemoteAddr(publicPeer).WithCookie(&http.Cookie{Name: testutil.SessionCookieName, Value: testutil.TestSessionToken})
			if tc.body != nil {
				request = request.WithJSONBody(tc.body)
			}

			rec := request.Record()

			// then the limiter refuses before the session is read, which the strict session mock proves
			assert.Equal(t, http.StatusTooManyRequests, rec.Code)
			assert.NotEmpty(t, rec.Header().Get("Retry-After"))
		})
	}
}

func TestAdminController_ListsTheInvitationsByPage(t *testing.T) {
	// given
	h := newAdminHarness(t)
	invitationID := uuid.New()
	cookie := h.signInStaff(uuid.New(), authz.PermManageUserAccount, false)
	listing := &dto.AdminInvitationListResponse{
		Invitations: []dto.AdminInvitationResponse{{ID: invitationID, Email: "carol@example.com", Expired: true, ExpiresAt: "2026-09-08T12:00:00Z", SentAt: "2026-09-01T12:00:00Z", CreatedAt: "2026-09-01T12:00:00Z"}},
		Total:       1,
		Limit:       10,
		Offset:      0,
	}
	h.admin.EXPECT().ListInvitations(mock.Anything, spec.InvitationListing{Limit: 10}).Return(listing, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/invitations?limit=10").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{
		"invitations": [{"id": %q, "email": "carol@example.com", "expired": true, "expires_at": "2026-09-08T12:00:00Z", "sent_at": "2026-09-01T12:00:00Z", "created_at": "2026-09-01T12:00:00Z"}],
		"total": 1,
		"limit": 10,
		"offset": 0
	}`, invitationID), rec.Body.String())
}

func TestAdminController_InvitesAnAddressWithTheCallersIP(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "recorded", err: nil},
		{name: "with its audit entry lost", err: errAuditLost},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			actorID, invitationID := uuid.New(), uuid.New()
			cookie := h.signInStaff(actorID, authz.PermManageUserAccount, true)
			invited := &dto.AdminInvitationResponse{ID: invitationID, Email: "carol@example.com", ExpiresAt: "2026-10-03T12:00:00Z", SentAt: "2026-09-26T12:00:00Z", CreatedAt: "2026-09-26T12:00:00Z"}
			h.admin.EXPECT().InviteUser(mock.Anything, spec.StaffInvitation{ActorID: actorID, IP: adminPeerIP, Email: "carol@example.com"}).Return(invited, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/admin/invitations").WithJSONBody(dto.AdminInviteRequest{Email: "carol@example.com"}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

			// then the invitation was saved and sent, so a lost audit entry is not a failure
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.JSONEq(t, fmt.Sprintf(`{"id": %q, "email": "carol@example.com", "expired": false, "expires_at": "2026-10-03T12:00:00Z", "sent_at": "2026-09-26T12:00:00Z", "created_at": "2026-09-26T12:00:00Z"}`, invitationID), rec.Body.String())
		})
	}
}

func TestAdminController_AnInvitationNeedsAnAddress(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageUserAccount, true)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/invitations").WithJSONBody(dto.AdminInviteRequest{Email: " "}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"email is required"}`, rec.Body.String())
}

func TestAdminController_MapsTheInvitationRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: admin.ErrInvalidEmail, status: http.StatusBadRequest, body: `{"error":"a valid email address is required"}`},
		{err: admin.ErrSitePublic, status: http.StatusConflict, body: `{"error":"invitations are only for a private site","code":"site_public"}`},
		{err: admin.ErrAccountExists, status: http.StatusConflict, body: `{"error":"that email address already has an account","code":"account_exists"}`},
		{err: admin.ErrInvitationPending, status: http.StatusConflict, body: `{"error":"that email address already has a pending invitation","code":"invitation_pending"}`},
		{err: admin.ErrEmailUnavailable, status: http.StatusServiceUnavailable, body: `{"error":"email is not available on this site","code":"email_unavailable"}`},
		{err: fmt.Errorf("%w: %w", admin.ErrInvitationNotSent, errors.Join(errServiceDown, errAuditLost)), status: http.StatusInternalServerError, body: `{"error":"the invitation was saved, but its email was not sent, so resend it"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to invite the user"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			cookie := h.signInStaff(uuid.New(), authz.PermManageUserAccount, true)
			h.admin.EXPECT().InviteUser(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/admin/invitations").WithJSONBody(dto.AdminInviteRequest{Email: "carol@example.com"}).WithCookie(cookie).Record()

			// then a lost audit entry never turns a failed send into a success
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAdminController_ResendsAnInvitation(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, invitationID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermManageUserAccount, true)
	h.admin.EXPECT().ResendInvitation(mock.Anything, spec.StaffInvitationAction{ActorID: actorID, IP: adminPeerIP, InvitationID: invitationID}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/admin/invitations/"+invitationID.String()+"/resend").WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestAdminController_MapsTheResendRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: admin.ErrInvitationNotFound, status: http.StatusNotFound, body: `{"error":"invitation not found"}`},
		{err: admin.ErrSitePublic, status: http.StatusConflict, body: `{"error":"invitations are only for a private site","code":"site_public"}`},
		{err: admin.ErrAccountExists, status: http.StatusConflict, body: `{"error":"that email address already has an account","code":"account_exists"}`},
		{err: fmt.Errorf("%w: %w", admin.ErrInvitationNotSent, errServiceDown), status: http.StatusInternalServerError, body: `{"error":"the invitation was saved, but its email was not sent, so resend it"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			cookie := h.signInStaff(uuid.New(), authz.PermManageUserAccount, true)
			h.admin.EXPECT().ResendInvitation(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/admin/invitations/"+uuid.NewString()+"/resend").WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAdminController_CancelsAnInvitation(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID, invitationID := uuid.New(), uuid.New()
	cookie := h.signInStaff(actorID, authz.PermManageUserAccount, true)
	h.admin.EXPECT().CancelInvitation(mock.Anything, spec.StaffInvitationAction{ActorID: actorID, IP: adminPeerIP, InvitationID: invitationID}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/admin/invitations/"+invitationID.String()).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestAdminController_CancellingAnUnknownInvitationIsNotFound(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageUserAccount, true)
	h.admin.EXPECT().CancelInvitation(mock.Anything, mock.Anything).Return(admin.ErrInvitationNotFound)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/admin/invitations/"+uuid.NewString()).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"invitation not found"}`, rec.Body.String())
}

func TestAdminController_RefusesAMalformedInvitationID(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageUserAccount, true)

	// when
	rec := h.NewRequest(http.MethodDelete, "/api/v1/admin/invitations/not-a-uuid").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid id"}`, rec.Body.String())
}

func TestAdminController_ReadsTheMaskedSettings(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageSettings, false)
	h.admin.EXPECT().GetSettings(mock.Anything).Return(&dto.AdminSettingsResponse{Settings: map[string]string{"smtp_password": "********", "site_name": "RetroTide"}}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/settings").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"settings":{"smtp_password":"********","site_name":"RetroTide"}}`, rec.Body.String())
}

func TestAdminController_UpdatesTheSettings(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID := uuid.New()
	cookie := h.signInStaff(actorID, authz.PermManageSettings, true)
	values := map[string]string{"site_name": "Team Retros"}
	h.admin.EXPECT().UpdateSettings(mock.Anything, spec.StaffSettingsUpdate{ActorID: actorID, IP: adminPeerIP, Values: values}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/settings").WithJSONBody(dto.UpdateSettingsRequest{Settings: values}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminController_ExplainsARefusedSetting(t *testing.T) {
	tests := []struct {
		name string
		err  error
		body string
	}{
		{name: "unknown", err: fmt.Errorf("%w: private_mode", admin.ErrUnknownSetting), body: `{"error":"unknown setting: private_mode"}`},
		{name: "invalid", err: fmt.Errorf("%w: max_body_size must be a whole number", admin.ErrInvalidSetting), body: `{"error":"invalid setting: max_body_size must be a whole number"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			cookie := h.signInStaff(uuid.New(), authz.PermManageSettings, true)
			h.admin.EXPECT().UpdateSettings(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPut, "/api/v1/admin/settings").WithJSONBody(dto.UpdateSettingsRequest{Settings: map[string]string{"x": "y"}}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAdminController_RefusesToLeaveNoWayToSignIn(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageSettings, true)
	h.admin.EXPECT().UpdateSettings(mock.Anything, mock.Anything).Return(admin.ErrNoSignInMethod)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/settings").WithJSONBody(dto.UpdateSettingsRequest{Settings: map[string]string{"allow_password_sign_in": "false"}}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"password sign-in can be turned off only on a public site with a sign-in provider configured","code":"no_sign_in_method"}`, rec.Body.String())
}

func TestAdminController_AStaleSettingsCacheIsSeenByTheAdmin(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageSettings, true)
	h.admin.EXPECT().UpdateSettings(mock.Anything, mock.Anything).Return(fmt.Errorf("%w: %w", admin.ErrSettingsCacheStale, errServiceDown))

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/settings").WithJSONBody(dto.UpdateSettingsRequest{Settings: map[string]string{"site_name": "x"}}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"the settings were saved, but their cached copies could not be refreshed"}`, rec.Body.String())
}

func TestAdminController_AStaleStaffCacheIsSeenByTheAdmin(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageRoles, true)
	h.admin.EXPECT().UpdateRolePermissions(mock.Anything, mock.Anything).Return(fmt.Errorf("%w: %w", admin.ErrStaffCacheStale, errServiceDown))

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/permissions/roles/support").WithJSONBody(dto.UpdatePermissionsRequest{Permissions: []string{"view_users"}}).WithCookie(cookie).Record()

	// then the change stands, and the admin learns it can take a minute to apply
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply"}`, rec.Body.String())
}

func TestAdminController_RefusesASettingsBodyThatDoesNotDecode(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageSettings, true)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/settings").WithRawBody(`{"settings":{"site_name":7}}`, "application/json").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid request body"}`, rec.Body.String())
}

func TestAdminController_SendsATestEmail(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "sent", err: nil, status: http.StatusNoContent, body: ""},
		{name: "no provider", err: admin.ErrEmailUnavailable, status: http.StatusServiceUnavailable, body: `{"error":"email is not available on this site","code":"email_unavailable"}`},
		{name: "refused by the provider", err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to send the test email"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			actorID := uuid.New()
			cookie := h.signInStaff(actorID, authz.PermManageSettings, true)
			h.admin.EXPECT().SendTestEmail(mock.Anything, spec.StaffAction{ActorID: actorID, IP: adminPeerIP}).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/admin/settings/test-email").WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			if tc.body == "" {
				assert.Empty(t, rec.Body.String())
				return
			}

			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAdminController_ReadsThePermissions(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermManageRoles, false)
	h.admin.EXPECT().GetPermissions(mock.Anything).Return(&dto.AdminPermissionsResponse{
		Permissions: []dto.PermissionCatalogueItem{{Permission: "view_users", Label: "View user records"}},
		Roles:       []dto.RolePermissionsItem{{Role: role.Support, Label: "Support", Permissions: []string{"view_users"}}},
	}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/permissions").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"permissions":[{"permission":"view_users","label":"View user records"}],"roles":[{"role":"support","label":"Support","permissions":["view_users"]}]}`, rec.Body.String())
}

func TestAdminController_UpdatesARolesPermissions(t *testing.T) {
	// given
	h := newAdminHarness(t)
	actorID := uuid.New()
	cookie := h.signInStaff(actorID, authz.PermManageRoles, true)
	h.admin.EXPECT().UpdateRolePermissions(mock.Anything, spec.StaffRolePermissionsUpdate{ActorID: actorID, IP: adminPeerIP, Role: role.Support, Permissions: []string{"view_users"}}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPut, "/api/v1/admin/permissions/roles/support").WithJSONBody(dto.UpdatePermissionsRequest{Permissions: []string{"view_users"}}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAdminController_MapsThePermissionRefusals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "immutable", err: admin.ErrImmutableRole, status: http.StatusForbidden, body: `{"error":"this role's permissions cannot be edited"}`},
		{name: "unknown role", err: admin.ErrUnknownRole, status: http.StatusBadRequest, body: `{"error":"unknown role"}`},
		{name: "unknown permission", err: fmt.Errorf("%w: %q", admin.ErrUnknownPermission, "fly"), status: http.StatusBadRequest, body: `{"error":"unknown permission: \"fly\""}`},
		{name: "restricted permission", err: fmt.Errorf("%w: %q", admin.ErrRestrictedPermission, "manage_settings"), status: http.StatusBadRequest, body: `{"error":"this permission is reserved for admins and cannot be granted to another role: \"manage_settings\""}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newAdminHarness(t)
			cookie := h.signInStaff(uuid.New(), authz.PermManageRoles, true)
			h.admin.EXPECT().UpdateRolePermissions(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPut, "/api/v1/admin/permissions/roles/admin").WithJSONBody(dto.UpdatePermissionsRequest{}).WithCookie(cookie).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestAdminController_ListsTheAuditLogByAction(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermViewAuditLog, false)
	h.admin.EXPECT().GetAuditLog(mock.Anything, spec.AuditLogListing{Action: audit.ActionBanUser, Limit: 20}).Return(&dto.AuditLogListResponse{Entries: []dto.AuditLogEntryResponse{}, Limit: 20}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/audit-log?action=ban_user&limit=20").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"entries":[],"total":0,"limit":20,"offset":0}`, rec.Body.String())
}

func TestAdminController_ServesTheStatsForTheSignedInViewer(t *testing.T) {
	// given
	h := newAdminHarness(t)
	viewerID := uuid.New()
	cookie := h.signInStaff(viewerID, authz.PermViewStats, false)
	stats := &dto.AdminStatsResponse{
		GeneratedAt: "2026-10-05T10:30:00Z",
		Totals:      dto.AdminStatTotals{Users: dto.AdminStatTotal{Total: 40, Last7Days: 4}},
		Daily:       []dto.AdminStatDay{{Date: "2026-10-05", Users: 2, Boards: 1, Cards: 9}},
		Activity:    dto.AdminActivity{Last24Hours: dto.AdminActivityCounts{Users: 1}},
		Health:      dto.AdminSiteHealth{SiteMode: config.SiteModePublic, EmailStatus: "failing"},
	}
	h.admin.EXPECT().GetStats(mock.Anything, viewerID).Return(stats, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/stats").WithCookie(cookie).Record()

	// then the signups key is absent for a viewer the service gave none, and failing email is told apart from no email
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"generated_at": "2026-10-05T10:30:00Z",
		"totals": {
			"users": {"total": 40, "last_7_days": 4},
			"verified_users": {"total": 0, "last_7_days": 0},
			"organisations": {"total": 0, "last_7_days": 0},
			"teams": {"total": 0, "last_7_days": 0},
			"boards": {"total": 0, "last_7_days": 0},
			"cards": {"total": 0, "last_7_days": 0}
		},
		"daily": [{"date": "2026-10-05", "users": 2, "boards": 1, "cards": 9}],
		"activity": {
			"last_24_hours": {"users": 1, "organisations": 0, "boards": 0, "cards": 0},
			"last_7_days": {"users": 0, "organisations": 0, "boards": 0, "cards": 0},
			"last_30_days": {"users": 0, "organisations": 0, "boards": 0, "cards": 0}
		},
		"health": {"site_mode": "public", "maintenance_mode": false, "email_configured": false, "email_status": "failing", "google_sign_in_configured": false}
	}`, rec.Body.String())
}

func TestAdminController_ServesTheNewestSignupsWhenTheServiceGivesThem(t *testing.T) {
	// given
	h := newAdminHarness(t)
	viewerID, userID := uuid.New(), uuid.New()
	cookie := h.signInStaff(viewerID, authz.PermViewStats, false)
	signups := []dto.AdminRecentSignup{{ID: userID, DisplayName: "Alice", Email: "alice@example.com", EmailVerified: true, CreatedAt: "2026-10-04T08:00:00Z"}}
	h.admin.EXPECT().GetStats(mock.Anything, viewerID).Return(&dto.AdminStatsResponse{Daily: []dto.AdminStatDay{}, RecentSignups: &signups}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/stats").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"recent_signups":[{"id":"`+userID.String()+`","display_name":"Alice","email":"alice@example.com","email_verified":true,"created_at":"2026-10-04T08:00:00Z"}]`)
}

func TestAdminController_AFailedStatsLoadIsAnError(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermViewStats, false)
	h.admin.EXPECT().GetStats(mock.Anything, mock.Anything).Return(nil, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/stats").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to load the site stats"}`, rec.Body.String())
}

func TestAdminController_ListsOrganisationsWithTheSearchAndPage(t *testing.T) {
	// given
	h := newAdminHarness(t)
	orgID, ownerID := uuid.New(), uuid.New()
	cookie := h.signInStaff(uuid.New(), authz.PermViewOrganisations, false)
	listing := &dto.AdminOrganisationListResponse{
		Organisations: []dto.AdminOrganisationResponse{{
			ID: orgID, Name: "Acme", Slug: "acme", Plan: model.PlanFree,
			OwnerID: &ownerID, OwnerDisplayName: "Alice", OwnerEmail: "alice@example.com",
			MemberCount: 3, TeamCount: 2, BoardCount: 5, CreatedAt: "2026-09-10T09:00:00Z", DeletedAt: "2026-10-01T08:00:00Z",
		}},
		Total: 31, Limit: 10, Offset: 20,
	}
	h.admin.EXPECT().ListOrganisations(mock.Anything, spec.OrganisationListing{Search: "acme", Limit: 10, Offset: 20}).Return(listing, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/organisations?search=acme&limit=10&offset=20").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"organisations": [{
			"id": "`+orgID.String()+`", "name": "Acme", "slug": "acme", "plan": "free",
			"owner_id": "`+ownerID.String()+`", "owner_display_name": "Alice", "owner_email": "alice@example.com",
			"member_count": 3, "team_count": 2, "board_count": 5,
			"created_at": "2026-09-10T09:00:00Z", "deleted_at": "2026-10-01T08:00:00Z"
		}],
		"total": 31, "limit": 10, "offset": 20
	}`, rec.Body.String())
}

func TestAdminController_RefusesAMalformedOrganisationPage(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermViewOrganisations, false)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/organisations?limit=many").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid limit"}`, rec.Body.String())
}

func TestAdminController_ShowsOneOrganisation(t *testing.T) {
	// given
	h := newAdminHarness(t)
	orgID, teamID, memberID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.signInStaff(uuid.New(), authz.PermViewOrganisations, false)
	detail := &dto.AdminOrganisationDetailResponse{
		ID: orgID, Name: "Acme", Slug: "acme", Plan: model.PlanFree, MemberCount: 1, TeamCount: 1, CreatedAt: "2026-09-10T09:00:00Z",
		Teams:   []dto.AdminOrganisationTeamResponse{{ID: teamID, Name: "Platform", MemberCount: 1, BoardCount: 0, CreatedAt: "2026-09-10T09:00:00Z"}},
		Members: []dto.OrganisationMemberResponse{{UserID: memberID, Email: "alice@example.com", DisplayName: "Alice", Role: model.OrgRoleOwner, JoinedAt: "2026-09-10T09:00:00Z"}},
	}
	h.admin.EXPECT().GetOrganisation(mock.Anything, orgID).Return(detail, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/organisations/"+orgID.String()).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"id": "`+orgID.String()+`", "name": "Acme", "slug": "acme", "plan": "free",
		"member_count": 1, "team_count": 1, "board_count": 0, "created_at": "2026-09-10T09:00:00Z",
		"teams": [{"id": "`+teamID.String()+`", "name": "Platform", "member_count": 1, "board_count": 0, "created_at": "2026-09-10T09:00:00Z"}],
		"members": [{"user_id": "`+memberID.String()+`", "email": "alice@example.com", "display_name": "Alice", "role": "owner", "joined_at": "2026-09-10T09:00:00Z"}]
	}`, rec.Body.String())
}

func TestAdminController_AnUnknownOrganisationIsNotFound(t *testing.T) {
	// given
	h := newAdminHarness(t)
	orgID := uuid.New()
	cookie := h.signInStaff(uuid.New(), authz.PermViewOrganisations, false)
	h.admin.EXPECT().GetOrganisation(mock.Anything, orgID).Return(nil, admin.ErrOrganisationNotFound)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/organisations/"+orgID.String()).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"organisation not found"}`, rec.Body.String())
}

func TestAdminController_RefusesAMalformedOrganisationID(t *testing.T) {
	// given
	h := newAdminHarness(t)
	cookie := h.signInStaff(uuid.New(), authz.PermViewOrganisations, false)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/admin/organisations/acme").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid orgID"}`, rec.Body.String())
}
