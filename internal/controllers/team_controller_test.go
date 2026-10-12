package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/role"
	"retrotide/internal/team"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	teamHarness struct {
		*testutil.Harness
		teams *team.MockService
	}
)

func newTeamHarness(t *testing.T) *teamHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	th := &teamHarness{Harness: h, teams: team.NewMockService(t)}

	s := &Service{TeamService: th.teams, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllTeamRoutes())

	return th
}

func teamPath(teamID uuid.UUID, rest string) string {
	return "/api/v1/teams/" + teamID.String() + rest
}

func teamRoute(method string, teamID uuid.UUID, rest string, perm orgauthz.Permission, body any) testutil.TeamPermissionRoute {
	return testutil.TeamPermissionRoute{Method: method, Path: teamPath(teamID, rest), Body: body, TeamID: teamID, Permission: perm}
}

func teamActorFor(actorID uuid.UUID) team.Actor {
	return team.Actor{ActorID: actorID, IP: tenantPeerIP}
}

func platformResponse(teamID uuid.UUID, orgID uuid.UUID, role model.TeamRole) *dto.TeamResponse {
	return &dto.TeamResponse{ID: teamID, OrgID: orgID, Name: teamName, Role: role, CreatedAt: orgCreatedAt}
}

func platformJSON(teamID uuid.UUID, orgID uuid.UUID, role model.TeamRole) string {
	return `{"id":"` + teamID.String() + `","org_id":"` + orgID.String() + `","name":"Platform","role":"` + string(role) + `","created_at":"` + orgCreatedAt + `"}`
}

func TestTeamController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{TeamService: team.NewMockService(t), Guards: h.Guards}

	// when the list is mounted exactly as production mounts it, so a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllTeamRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/orgs/:orgID/teams",
		"HEAD /api/v1/orgs/:orgID/teams",
		"POST /api/v1/orgs/:orgID/teams",
		"POST /api/v1/orgs/:orgID/teams/:teamID/archive",
		"DELETE /api/v1/orgs/:orgID/teams/:teamID",
		"GET /api/v1/teams/:teamID",
		"HEAD /api/v1/teams/:teamID",
		"PUT /api/v1/teams/:teamID",
		"GET /api/v1/teams/:teamID/members",
		"HEAD /api/v1/teams/:teamID/members",
		"POST /api/v1/teams/:teamID/members",
		"PUT /api/v1/teams/:teamID/members/:userID/role",
		"DELETE /api/v1/teams/:teamID/members/:userID",
	}, registeredRoutes(h.Engine))
}

func TestTeamController_OrganisationRoutesRunTheOrgPermissionFailureSuite(t *testing.T) {
	orgID, teamID := uuid.New(), uuid.New()

	routes := []testutil.OrgPermissionRoute{
		orgRoute(http.MethodGet, orgID, "/teams", orgauthz.PermOrgView, nil),
		orgRoute(http.MethodPost, orgID, "/teams", orgauthz.PermOrgManageTeams, dto.CreateTeamRequest{Name: teamName}),
		orgRoute(http.MethodPost, orgID, "/teams/"+teamID.String()+"/archive", orgauthz.PermOrgManageTeams, nil),
		orgRoute(http.MethodDelete, orgID, "/teams/"+teamID.String(), orgauthz.PermOrgManageTeams, nil),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newTeamHarness(t).Harness
			}, route)
		})
	}
}

func TestTeamController_TeamRoutesRunTheTeamPermissionFailureSuite(t *testing.T) {
	teamID, userID := uuid.New(), uuid.New()

	routes := []testutil.TeamPermissionRoute{
		teamRoute(http.MethodGet, teamID, "", orgauthz.PermTeamView, nil),
		teamRoute(http.MethodPut, teamID, "", orgauthz.PermTeamUpdate, dto.RenameTeamRequest{Name: teamName}),
		teamRoute(http.MethodGet, teamID, "/members", orgauthz.PermTeamView, nil),
		teamRoute(http.MethodPost, teamID, "/members", orgauthz.PermTeamManageMembers, dto.AddTeamMemberRequest{UserID: userID, Role: "member"}),
		teamRoute(http.MethodPut, teamID, "/members/"+userID.String()+"/role", orgauthz.PermTeamManageMembers, dto.ChangeTeamMemberRoleRequest{Role: "lead"}),
		teamRoute(http.MethodDelete, teamID, "/members/"+userID.String(), orgauthz.PermTeamManageMembers, nil),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunTeamPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newTeamHarness(t).Harness
			}, route)
		})
	}
}

func TestTeamController_RefusesAMalformedTeamIDAfterSigningIn(t *testing.T) {
	// given
	h := newTeamHarness(t)
	cookie := h.SignIn(uuid.New())

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/teams/not-a-uuid").WithCookie(cookie).Record()

	// then the team's organisation is never looked up
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid teamID"}`, rec.Body.String())
}

func TestTeamController_ListsTheOrganisationsTeams(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodGet, orgID, "/teams", orgauthz.PermOrgView, nil), userID)
	h.teams.EXPECT().List(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}).Return(&dto.TeamListResponse{Teams: []dto.TeamResponse{*platformResponse(teamID, orgID, model.TeamRoleMember)}}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, orgPath(orgID, "/teams")).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"teams":[`+platformJSON(teamID, orgID, model.TeamRoleMember)+`]}`, rec.Body.String())
}

func TestTeamController_CreatesATeam(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	body := dto.CreateTeamRequest{Name: teamName}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/teams", orgauthz.PermOrgManageTeams, body), userID)
	h.teams.EXPECT().Create(mock.Anything, team.CreateRequest{Actor: teamActorFor(userID), OrgID: orgID, Name: teamName}).Return(platformResponse(teamID, orgID, model.TeamRoleLead), nil)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/teams")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, platformJSON(teamID, orgID, model.TeamRoleLead), rec.Body.String())
}

func TestTeamController_ASecondTeamOnFreeAnswersPaymentRequired(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	body := dto.CreateTeamRequest{Name: "Second"}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/teams", orgauthz.PermOrgManageTeams, body), userID)
	denied := &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitTeams, Allowed: 1, Current: 1}
	h.teams.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("check the team allowance: %w", denied))

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/teams")).WithJSONBody(body).WithCookie(cookie).Record()

	// then the SPA opens its upgrade prompt on the code
	assert.Equal(t, http.StatusPaymentRequired, rec.Code)
	assert.JSONEq(t, `{"error":"upgrade required","code":"upgrade_required","limit":"teams","allowed":1,"current":1,"plan":"free"}`, rec.Body.String())
}

func TestTeamController_ArchivesATeamOfThePathsOrganisation(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	rest := "/teams/" + teamID.String() + "/archive"
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, rest, orgauthz.PermOrgManageTeams, nil), userID)
	h.teams.EXPECT().Archive(mock.Anything, team.OrgTeamCommand{Actor: teamActorFor(userID), OrgID: orgID, TeamID: teamID}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, rest)).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTeamController_DeletesATeamOfThePathsOrganisation(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	rest := "/teams/" + teamID.String()
	cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, rest, orgauthz.PermOrgManageTeams, nil), userID)
	h.teams.EXPECT().Delete(mock.Anything, team.OrgTeamCommand{Actor: teamActorFor(userID), OrgID: orgID, TeamID: teamID}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, orgPath(orgID, rest)).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTeamController_ShowsTheTeamWithTheViewersRole(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToTeam(teamRoute(http.MethodGet, teamID, "", orgauthz.PermTeamView, nil), userID)
	h.teams.EXPECT().Get(mock.Anything, team.Viewer{TeamID: teamID, UserID: userID}).Return(platformResponse(teamID, orgID, model.TeamRoleLead), nil)

	// when
	rec := h.NewRequest(http.MethodGet, teamPath(teamID, "")).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, platformJSON(teamID, orgID, model.TeamRoleLead), rec.Body.String())
}

func TestTeamController_RenamesTheTeam(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, orgID, teamID := uuid.New(), uuid.New(), uuid.New()
	body := dto.RenameTeamRequest{Name: teamName}
	cookie := h.SignInToTeam(teamRoute(http.MethodPut, teamID, "", orgauthz.PermTeamUpdate, body), userID)
	request := team.RenameRequest{Actor: teamActorFor(userID), TeamID: teamID, Name: teamName}
	h.teams.EXPECT().Rename(mock.Anything, request).Return(platformResponse(teamID, orgID, model.TeamRoleLead), nil)

	// when
	rec := h.NewRequest(http.MethodPut, teamPath(teamID, "")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, platformJSON(teamID, orgID, model.TeamRoleLead), rec.Body.String())
}

func TestTeamController_ListsTheTeamsMembers(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, teamID, memberID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToTeam(teamRoute(http.MethodGet, teamID, "/members", orgauthz.PermTeamView, nil), userID)
	staffID := uuid.New()
	members := &dto.TeamMemberListResponse{Members: []dto.TeamMemberResponse{
		{UserID: memberID, Email: memberEmail, DisplayName: memberName, Role: model.TeamRoleMember, OrgRole: model.OrgRoleMember, JoinedAt: orgJoinedTime},
		{UserID: staffID, Email: "sam@example.com", DisplayName: "Sam", Role: model.TeamRoleLead, OrgRole: model.OrgRoleAdmin, SiteRole: role.Admin, JoinedAt: orgJoinedTime},
	}}
	h.teams.EXPECT().ListMembers(mock.Anything, teamID).Return(members, nil)

	// when
	rec := h.NewRequest(http.MethodGet, teamPath(teamID, "/members")).WithCookie(cookie).Record()

	// then every member carries their organisation role, and site staff their site role
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"members":[{"user_id":%q,"email":"bob@example.com","display_name":"Bob","role":"member","org_role":"member","joined_at":%q},{"user_id":%q,"email":"sam@example.com","display_name":"Sam","role":"lead","org_role":"admin","site_role":"admin","joined_at":%q}]}`, memberID, orgJoinedTime, staffID, orgJoinedTime), rec.Body.String())
}

func TestTeamController_AddsAMember(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, teamID, memberID := uuid.New(), uuid.New(), uuid.New()
	body := dto.AddTeamMemberRequest{UserID: memberID, Role: " member "}
	cookie := h.SignInToTeam(teamRoute(http.MethodPost, teamID, "/members", orgauthz.PermTeamManageMembers, body), userID)
	change := team.MemberRole{Actor: teamActorFor(userID), TeamID: teamID, UserID: memberID, Role: model.TeamRoleMember}
	h.teams.EXPECT().AddMember(mock.Anything, change).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPost, teamPath(teamID, "/members")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTeamController_AddingAMemberNeedsThePerson(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, teamID := uuid.New(), uuid.New()
	body := dto.AddTeamMemberRequest{Role: "member"}
	cookie := h.SignInToTeam(teamRoute(http.MethodPost, teamID, "/members", orgauthz.PermTeamManageMembers, body), userID)

	// when
	rec := h.NewRequest(http.MethodPost, teamPath(teamID, "/members")).WithJSONBody(body).WithCookie(cookie).Record()

	// then the service is never asked
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"user_id is required"}`, rec.Body.String())
}

func TestTeamController_ChangesAMembersRole(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, teamID, memberID := uuid.New(), uuid.New(), uuid.New()
	rest := "/members/" + memberID.String() + "/role"
	body := dto.ChangeTeamMemberRoleRequest{Role: "lead"}
	cookie := h.SignInToTeam(teamRoute(http.MethodPut, teamID, rest, orgauthz.PermTeamManageMembers, body), userID)
	change := team.MemberRole{Actor: teamActorFor(userID), TeamID: teamID, UserID: memberID, Role: model.TeamRoleLead}
	h.teams.EXPECT().SetMemberRole(mock.Anything, change).Return(nil)

	// when
	rec := h.NewRequest(http.MethodPut, teamPath(teamID, rest)).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTeamController_RemovesAMember(t *testing.T) {
	// given
	h := newTeamHarness(t)
	userID, teamID, memberID := uuid.New(), uuid.New(), uuid.New()
	rest := "/members/" + memberID.String()
	cookie := h.SignInToTeam(teamRoute(http.MethodDelete, teamID, rest, orgauthz.PermTeamManageMembers, nil), userID)
	command := team.MemberCommand{Actor: teamActorFor(userID), TeamID: teamID, UserID: memberID}
	h.teams.EXPECT().RemoveMember(mock.Anything, command).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, teamPath(teamID, rest)).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTeamController_MapsEveryRefusal(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: team.ErrTeamNotFound, status: http.StatusNotFound, body: `{"error":"team not found"}`},
		{err: team.ErrOrganisationNotFound, status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{err: team.ErrMemberNotFound, status: http.StatusNotFound, body: `{"error":"that person is not a member of the organisation"}`},
		{err: team.ErrTeamMemberNotFound, status: http.StatusNotFound, body: `{"error":"that person is not a member of the team"}`},
		{err: team.ErrInvalidRole, status: http.StatusBadRequest, body: `{"error":"the role must be lead or member"}`},
		{err: team.ErrNameRequired, status: http.StatusBadRequest, body: `{"error":"a team name is required","code":"team_name_required"}`},
		{err: team.ErrNameTooLong, status: http.StatusBadRequest, body: `{"error":"a team name can be at most 80 characters","code":"team_name_too_long"}`},
		{err: fmt.Errorf("user cannot make them a lead: %w", hierarchy.ErrOutranked), status: http.StatusForbidden, body: `{"error":"nobody may act on someone at their own level or above"}`},
		{err: team.ErrAlreadyTeamMember, status: http.StatusConflict, body: `{"error":"that person is already a member of the team"}`},
		{err: team.ErrNameTaken, status: http.StatusConflict, body: `{"error":"the organisation already has a team with that name","code":"team_name_taken"}`},
		{err: team.ErrTeamArchived, status: http.StatusConflict, body: `{"error":"the team is archived, so it can no longer be changed","code":"team_archived"}`},
		{err: fmt.Errorf("%w: valkey down", team.ErrCacheStale), status: http.StatusInternalServerError, body: `{"error":"the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to change the team member's role"}`},
		{err: fmt.Errorf("%w: audit down", team.ErrNotAudited), status: http.StatusNoContent},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			h := newTeamHarness(t)
			userID, teamID, memberID := uuid.New(), uuid.New(), uuid.New()
			rest := "/members/" + memberID.String() + "/role"
			body := dto.ChangeTeamMemberRoleRequest{Role: "lead"}
			cookie := h.SignInToTeam(teamRoute(http.MethodPut, teamID, rest, orgauthz.PermTeamManageMembers, body), userID)
			h.teams.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(tc.err)

			// when
			rec := h.NewRequest(http.MethodPut, teamPath(teamID, rest)).WithJSONBody(body).WithCookie(cookie).Record()

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
