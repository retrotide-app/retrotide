package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org"
	"retrotide/internal/orgauthz"
	"retrotide/internal/role"
	"retrotide/internal/team"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	orgHarness struct {
		*testutil.Harness
		orgs         *org.MockService
		entitlements *entitlements.MockService
	}
)

const (
	tenantPeerIP  = "198.51.100.7"
	orgCreatedAt  = "2026-09-20T09:00:00Z"
	orgSlug       = "acme"
	orgName       = "Acme"
	teamName      = "Platform"
	memberEmail   = "bob@example.com"
	memberName    = "Bob"
	orgJoinedTime = "2026-09-21T10:30:00Z"
)

func newOrgHarness(t *testing.T) *orgHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	o := &orgHarness{Harness: h, orgs: org.NewMockService(t), entitlements: entitlements.NewMockService(t)}

	s := &Service{OrgService: o.orgs, EntitlementsService: o.entitlements, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllOrganisationRoutes())

	return o
}

func orgPath(orgID uuid.UUID, rest string) string {
	return "/api/v1/orgs/" + orgID.String() + rest
}

func orgRoute(method string, orgID uuid.UUID, rest string, perm orgauthz.Permission, body any) testutil.OrgPermissionRoute {
	return testutil.OrgPermissionRoute{Method: method, Path: orgPath(orgID, rest), Body: body, OrgID: orgID, Permission: perm}
}

func orgActorFor(actorID uuid.UUID) org.Actor {
	return org.Actor{ActorID: actorID, IP: tenantPeerIP}
}

func acmeResponse(orgID uuid.UUID, role model.OrgRole) *dto.OrganisationResponse {
	return &dto.OrganisationResponse{ID: orgID, Slug: orgSlug, Name: orgName, Role: role, CreatedAt: orgCreatedAt}
}

func acmeJSON(orgID uuid.UUID, role model.OrgRole) string {
	return `{"id":"` + orgID.String() + `","slug":"acme","name":"Acme","allow_guests":false,"role":"` + string(role) + `","created_at":"` + orgCreatedAt + `"}`
}

func TestOrgController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{OrgService: org.NewMockService(t), EntitlementsService: entitlements.NewMockService(t), Guards: h.Guards}

	// when the list is mounted exactly as production mounts it, so a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllOrganisationRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"POST /api/v1/orgs",
		"GET /api/v1/orgs",
		"HEAD /api/v1/orgs",
		"GET /api/v1/orgs/:orgID",
		"HEAD /api/v1/orgs/:orgID",
		"PUT /api/v1/orgs/:orgID",
		"DELETE /api/v1/orgs/:orgID",
		"POST /api/v1/orgs/:orgID/restore",
		"POST /api/v1/orgs/:orgID/transfer",
		"POST /api/v1/orgs/:orgID/leave",
		"GET /api/v1/orgs/:orgID/members",
		"HEAD /api/v1/orgs/:orgID/members",
		"PUT /api/v1/orgs/:orgID/members/:userID/role",
		"DELETE /api/v1/orgs/:orgID/members/:userID",
		"GET /api/v1/orgs/:orgID/entitlements",
		"HEAD /api/v1/orgs/:orgID/entitlements",
	}, registeredRoutes(h.Engine))
}

func TestOrgController_AccountRoutesRunTheAuthFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: "/api/v1/orgs", Body: dto.CreateWorkspaceRequest{Name: orgName, TeamName: teamName}},
		{Method: http.MethodGet, Path: "/api/v1/orgs"},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newOrgHarness(t).Harness
			}, route)
		})
	}
}

func TestOrgController_TenantRoutesRunTheOrgPermissionFailureSuite(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()

	routes := []testutil.OrgPermissionRoute{
		orgRoute(http.MethodGet, orgID, "", orgauthz.PermOrgView, nil),
		orgRoute(http.MethodPut, orgID, "", orgauthz.PermOrgUpdate, dto.UpdateOrganisationRequest{Name: orgName, Slug: orgSlug}),
		orgRoute(http.MethodDelete, orgID, "", orgauthz.PermOrgDelete, nil),
		orgRoute(http.MethodPost, orgID, "/restore", orgauthz.PermOrgDelete, nil),
		orgRoute(http.MethodPost, orgID, "/transfer", orgauthz.PermOrgTransferOwnership, dto.TransferOwnershipRequest{UserID: userID}),
		orgRoute(http.MethodPost, orgID, "/leave", orgauthz.PermOrgView, nil),
		orgRoute(http.MethodGet, orgID, "/members", orgauthz.PermOrgManageMembers, nil),
		orgRoute(http.MethodPut, orgID, "/members/"+userID.String()+"/role", orgauthz.PermOrgManageMembers, dto.ChangeMemberRoleRequest{Role: "admin"}),
		orgRoute(http.MethodDelete, orgID, "/members/"+userID.String(), orgauthz.PermOrgManageMembers, nil),
		orgRoute(http.MethodGet, orgID, "/entitlements", orgauthz.PermOrgView, nil),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newOrgHarness(t).Harness
			}, route)
		})
	}
}

func TestOrgController_RefusesAMalformedOrganisationIDAfterSigningIn(t *testing.T) {
	// given
	h := newOrgHarness(t)
	cookie := h.SignIn(uuid.New())

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/orgs/not-a-uuid").WithCookie(cookie).Record()

	// then the permission is never checked for an organisation that cannot exist
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid orgID"}`, rec.Body.String())
}

func TestOrgController_CreatesTheWorkspaceForTheCaller(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	workspace := &dto.WorkspaceResponse{
		Organisation: *acmeResponse(orgID, model.OrgRoleOwner),
		Team:         dto.TeamResponse{ID: teamID, OrgID: orgID, Name: teamName, Role: model.TeamRoleLead, CreatedAt: orgCreatedAt},
	}
	h.orgs.EXPECT().CreateWorkspace(mock.Anything, org.WorkspaceRequest{Actor: orgActorFor(userID), Name: orgName, TeamName: teamName}).Return(workspace, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/orgs").WithJSONBody(dto.CreateWorkspaceRequest{Name: orgName, TeamName: teamName}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"organisation":%s,"team":{"id":%q,"org_id":%q,"name":"Platform","role":"lead","created_at":%q}}`, acmeJSON(orgID, model.OrgRoleOwner), teamID, orgID, orgCreatedAt), rec.Body.String())
}

func TestOrgController_CreatingAWorkspaceStillAnswersCreatedWhenTheAuditEntryFails(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	workspace := &dto.WorkspaceResponse{Organisation: *acmeResponse(orgID, model.OrgRoleOwner)}
	h.orgs.EXPECT().CreateWorkspace(mock.Anything, mock.Anything).Return(workspace, fmt.Errorf("%w: audit log down", org.ErrNotAudited))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/orgs").WithJSONBody(dto.CreateWorkspaceRequest{Name: orgName, TeamName: teamName}).WithCookie(cookie).Record()

	// then the workspace exists, so the caller is told so
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestOrgController_ListsTheCallersOrganisations(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignIn(userID)
	h.orgs.EXPECT().ListForUser(mock.Anything, userID).Return(&dto.OrganisationListResponse{Organisations: []dto.OrganisationResponse{*acmeResponse(orgID, model.OrgRoleMember)}}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/orgs").WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"organisations":[`+acmeJSON(orgID, model.OrgRoleMember)+`]}`, rec.Body.String())
}

func TestOrgController_ListsADeletedOrganisationWithTheEndOfItsRestoreWindow(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignIn(userID)
	deleted := acmeResponse(orgID, model.OrgRoleOwner)
	deleted.DeletedAt = "2026-09-26T12:00:00Z"
	deleted.RestorableUntil = "2026-10-26T12:00:00Z"
	h.orgs.EXPECT().ListForUser(mock.Anything, userID).Return(&dto.OrganisationListResponse{Organisations: []dto.OrganisationResponse{*deleted}}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/orgs").WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"organisations":[{"id":%q,"slug":"acme","name":"Acme","allow_guests":false,"role":"owner","created_at":%q,"deleted_at":"2026-09-26T12:00:00Z","restorable_until":"2026-10-26T12:00:00Z"}]}`, orgID, orgCreatedAt), rec.Body.String())
}

func TestOrgController_ReportsAFailedListOfOrganisations(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.orgs.EXPECT().ListForUser(mock.Anything, userID).Return(nil, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/orgs").WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to list your organisations"}`, rec.Body.String())
}

func TestOrgController_ShowsTheOrganisationToAMember(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodGet, orgID, "", orgauthz.PermOrgView, nil), userID)
	h.orgs.EXPECT().Get(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}).Return(acmeResponse(orgID, model.OrgRoleAdmin), nil)

	// when
	rec := h.NewRequest(http.MethodGet, orgPath(orgID, "")).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, acmeJSON(orgID, model.OrgRoleAdmin), rec.Body.String())
}

func TestOrgController_UpdatesTheOrganisation(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	body := dto.UpdateOrganisationRequest{Name: orgName, Slug: orgSlug, AllowGuests: true}
	cookie := h.SignInToOrg(orgRoute(http.MethodPut, orgID, "", orgauthz.PermOrgUpdate, body), userID)
	updated := acmeResponse(orgID, model.OrgRoleOwner)
	updated.AllowGuests = true
	request := org.UpdateRequest{Actor: orgActorFor(userID), OrgID: orgID, Name: orgName, Slug: orgSlug, AllowGuests: true}
	h.orgs.EXPECT().Update(mock.Anything, request).Return(updated, nil)

	// when
	rec := h.NewRequest(http.MethodPut, orgPath(orgID, "")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"id":%q,"slug":"acme","name":"Acme","allow_guests":true,"role":"owner","created_at":%q}`, orgID, orgCreatedAt), rec.Body.String())
}

func TestOrgController_DeletesTheOrganisation(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, "", orgauthz.PermOrgDelete, nil), userID)
	h.orgs.EXPECT().Delete(mock.Anything, org.Command{Actor: orgActorFor(userID), OrgID: orgID}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, orgPath(orgID, "")).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestOrgController_RestoresTheOrganisation(t *testing.T) {
	// given the owner reaches the restore route of a deleted organisation through its membership
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/restore", orgauthz.PermOrgDelete, nil), userID)
	h.orgs.EXPECT().Restore(mock.Anything, org.Command{Actor: orgActorFor(userID), OrgID: orgID}).Return(acmeResponse(orgID, model.OrgRoleOwner), nil)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/restore")).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, acmeJSON(orgID, model.OrgRoleOwner), rec.Body.String())
}

func TestOrgController_TransfersOwnership(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID, newOwnerID := uuid.New(), uuid.New(), uuid.New()
	body := dto.TransferOwnershipRequest{UserID: newOwnerID}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/transfer", orgauthz.PermOrgTransferOwnership, body), userID)
	command := org.MemberCommand{Actor: orgActorFor(userID), OrgID: orgID, UserID: newOwnerID}
	h.orgs.EXPECT().TransferOwnership(mock.Anything, command).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/transfer")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestOrgController_ATransferPastTheNewOwnersAllowanceIsRefusedWithItsCode(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID, newOwnerID := uuid.New(), uuid.New(), uuid.New()
	body := dto.TransferOwnershipRequest{UserID: newOwnerID}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/transfer", orgauthz.PermOrgTransferOwnership, body), userID)
	h.orgs.EXPECT().TransferOwnership(mock.Anything, mock.Anything).Return(org.ErrNewOwnerLimitReached)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/transfer")).WithJSONBody(body).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"the new owner already owns as many organisations as they can","code":"new_owner_limit_reached"}`, rec.Body.String())
}

func TestOrgController_TransferNeedsTheNewOwner(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/transfer", orgauthz.PermOrgTransferOwnership, dto.TransferOwnershipRequest{}), userID)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/transfer")).WithJSONBody(dto.TransferOwnershipRequest{}).WithCookie(cookie).Record()

	// then the service is never asked
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"user_id is required"}`, rec.Body.String())
}

func TestOrgController_LeavesTheOrganisation(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/leave", orgauthz.PermOrgView, nil), userID)
	h.orgs.EXPECT().Leave(mock.Anything, org.Command{Actor: orgActorFor(userID), OrgID: orgID}).Return(org.ErrOwnerCannotLeave)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/leave")).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"the owner cannot leave the organisation, so transfer ownership or delete it first"}`, rec.Body.String())
}

func TestOrgController_ListsTheMembers(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID, memberID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodGet, orgID, "/members", orgauthz.PermOrgManageMembers, nil), userID)
	staffID := uuid.New()
	members := &dto.OrganisationMemberListResponse{Members: []dto.OrganisationMemberResponse{
		{UserID: memberID, Email: memberEmail, DisplayName: memberName, Role: model.OrgRoleMember, JoinedAt: orgJoinedTime},
		{UserID: staffID, Email: "sam@example.com", DisplayName: "Sam", Role: model.OrgRoleMember, SiteRole: role.Support, JoinedAt: orgJoinedTime},
	}}
	h.orgs.EXPECT().ListMembers(mock.Anything, orgID).Return(members, nil)

	// when
	rec := h.NewRequest(http.MethodGet, orgPath(orgID, "/members")).WithCookie(cookie).Record()

	// then only site staff carry a site role
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"members":[{"user_id":%q,"email":"bob@example.com","display_name":"Bob","role":"member","joined_at":%q},{"user_id":%q,"email":"sam@example.com","display_name":"Sam","role":"member","site_role":"support","joined_at":%q}]}`, memberID, orgJoinedTime, staffID, orgJoinedTime), rec.Body.String())
}

func TestOrgController_ChangesAMembersRole(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID, memberID := uuid.New(), uuid.New(), uuid.New()
	body := dto.ChangeMemberRoleRequest{Role: " admin "}
	cookie := h.SignInToOrg(orgRoute(http.MethodPut, orgID, "/members/"+memberID.String()+"/role", orgauthz.PermOrgManageMembers, body), userID)
	change := org.RoleChange{Actor: orgActorFor(userID), OrgID: orgID, UserID: memberID, Role: model.OrgRoleAdmin}
	h.orgs.EXPECT().ChangeMemberRole(mock.Anything, change).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPut, orgPath(orgID, "/members/"+memberID.String()+"/role")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestOrgController_RemovesAMember(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID, memberID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, "/members/"+memberID.String(), orgauthz.PermOrgManageMembers, nil), userID)
	command := org.MemberCommand{Actor: orgActorFor(userID), OrgID: orgID, UserID: memberID}
	h.orgs.EXPECT().RemoveMember(mock.Anything, command).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, orgPath(orgID, "/members/"+memberID.String())).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestOrgController_RefusesAMalformedMemberID(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, "/members/not-a-uuid", orgauthz.PermOrgManageMembers, nil), userID)

	// when
	rec := h.NewRequest(http.MethodDelete, orgPath(orgID, "/members/not-a-uuid")).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid userID"}`, rec.Body.String())
}

func TestOrgController_ShowsThePlanAndItsLimits(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodGet, orgID, "/entitlements", orgauthz.PermOrgView, nil), userID)
	plan, known := entitlements.PlanFor(model.PlanFree)
	require.True(t, known)
	h.entitlements.EXPECT().For(mock.Anything, orgID).Return(plan, nil)

	// when
	rec := h.NewRequest(http.MethodGet, orgPath(orgID, "/entitlements")).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"plan":"free","limits":{"teams":1,"members":10,"saved_templates":3,"card_image_bytes":5242880}}`, rec.Body.String())
}

func TestOrgController_ReportsAPlanThatCannotBeRead(t *testing.T) {
	// given
	h := newOrgHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodGet, orgID, "/entitlements", orgauthz.PermOrgView, nil), userID)
	h.entitlements.EXPECT().For(mock.Anything, orgID).Return(nil, entitlements.ErrUnknownPlan)

	// when
	rec := h.NewRequest(http.MethodGet, orgPath(orgID, "/entitlements")).WithCookie(cookie).Record()

	// then an unknown plan is never shown as unlimited
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to load the organisation's plan"}`, rec.Body.String())
}

func TestOrgController_MapsEveryRefusal(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: org.ErrOrganisationNotFound, status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{err: org.ErrMemberNotFound, status: http.StatusNotFound, body: `{"error":"that person is not a member of the organisation"}`},
		{err: org.ErrNameRequired, status: http.StatusBadRequest, body: `{"error":"an organisation name is required","code":"org_name_required"}`},
		{err: org.ErrNameTooLong, status: http.StatusBadRequest, body: `{"error":"an organisation name can be at most 80 characters","code":"org_name_too_long"}`},
		{err: team.ErrNameRequired, status: http.StatusBadRequest, body: `{"error":"a team name is required","code":"team_name_required"}`},
		{err: team.ErrNameTooLong, status: http.StatusBadRequest, body: `{"error":"a team name can be at most 80 characters","code":"team_name_too_long"}`},
		{err: org.ErrInvalidSlug, status: http.StatusBadRequest, body: `{"error":"the address must be 3 to 40 lower-case letters, digits or hyphens, starting and ending with a letter or digit","code":"slug_invalid"}`},
		{err: org.ErrSlugReserved, status: http.StatusBadRequest, body: `{"error":"that address is reserved","code":"slug_reserved"}`},
		{err: org.ErrInvalidRole, status: http.StatusBadRequest, body: `{"error":"the role must be admin or member"}`},
		{err: fmt.Errorf("act on user: %w", hierarchy.ErrOutranked), status: http.StatusForbidden, body: `{"error":"nobody may act on someone at their own level or above"}`},
		{err: org.ErrOwnerProtected, status: http.StatusForbidden, body: `{"error":"the owner's role cannot be changed and the owner cannot be removed, so transfer ownership first"}`},
		{err: org.ErrAdminsNeedOwner, status: http.StatusForbidden, body: `{"error":"only the owner can make, unmake or remove an admin"}`},
		{err: org.ErrNotOwner, status: http.StatusForbidden, body: `{"error":"only the owner can restore the organisation"}`},
		{err: org.ErrSlugTaken, status: http.StatusConflict, body: `{"error":"that address is already taken","code":"slug_taken"}`},
		{err: org.ErrOwnedLimitReached, status: http.StatusConflict, body: `{"error":"you already own as many organisations as you can","code":"owned_limit_reached"}`},
		{err: org.ErrNewOwnerLimitReached, status: http.StatusConflict, body: `{"error":"the new owner already owns as many organisations as they can","code":"new_owner_limit_reached"}`},
		{err: fmt.Errorf("%w: organisation changed", org.ErrOwnershipNotTransferred), status: http.StatusConflict, body: `{"error":"ownership was not transferred, because the organisation, its owner or the new owner changed meanwhile"}`},
		{err: org.ErrOrganisationNotDeleted, status: http.StatusConflict, body: `{"error":"the organisation is not deleted"}`},
		{err: org.ErrNotRestorable, status: http.StatusConflict, body: `{"error":"the organisation can no longer be restored"}`},
		{err: fmt.Errorf("%w: %w", org.ErrCacheStale, errors.Join(errServiceDown, fmt.Errorf("%w: audit down", org.ErrNotAudited))), status: http.StatusInternalServerError, body: `{"error":"the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to remove the member"}`},
		{err: fmt.Errorf("%w: audit down", org.ErrNotAudited), status: http.StatusNoContent, body: ``},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			h := newOrgHarness(t)
			userID, orgID, memberID := uuid.New(), uuid.New(), uuid.New()
			cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, "/members/"+memberID.String(), orgauthz.PermOrgManageMembers, nil), userID)
			h.orgs.EXPECT().RemoveMember(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodDelete, orgPath(orgID, "/members/"+memberID.String())).WithCookie(cookie).Record()

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
