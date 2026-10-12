package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org/invite"
	"retrotide/internal/orgauthz"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	inviteHarness struct {
		*testutil.Harness
		invites *invite.MockService
	}
)

const (
	inviteToken     = "3e1d5c7b9a0f2e4d6c8b0a1f3e5d7c9b1a3f5e7d9c0b2a4f6e8d0c2b4a6f8e0d"
	inviteExpiresAt = "2026-10-04T09:00:00Z"
	inviteLink      = "https://retrotide.test/invite#" + inviteToken
)

func newInviteHarness(t *testing.T) *inviteHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	return mountInviteRoutes(t, h)
}

func mountInviteRoutes(t *testing.T, h *testutil.Harness) *inviteHarness {
	t.Helper()

	i := &inviteHarness{Harness: h, invites: invite.NewMockService(t)}

	s := &Service{InviteService: i.invites, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllInviteRoutes())

	return i
}

func invitePath(orgID uuid.UUID, inviteID uuid.UUID, rest string) string {
	return "/api/v1/orgs/" + orgID.String() + "/invites/" + inviteID.String() + rest
}

func pendingInvite(inviteID uuid.UUID) dto.OrganisationInviteResponse {
	return dto.OrganisationInviteResponse{ID: inviteID, Email: memberEmail, Role: model.OrgRoleMember, MaxUses: new(1), ExpiresAt: inviteExpiresAt, CreatedAt: orgCreatedAt}
}

func pendingInviteJSON(inviteID uuid.UUID) string {
	return `{"id":"` + inviteID.String() + `","email":"bob@example.com","role":"member","max_uses":1,"use_count":0,"expired":false,"expires_at":"` + inviteExpiresAt + `","created_at":"` + orgCreatedAt + `"}`
}

func joinedAcme(orgID uuid.UUID) *dto.JoinedOrganisationResponse {
	return &dto.JoinedOrganisationResponse{OrgID: orgID, OrgSlug: orgSlug, OrgName: orgName, Role: model.OrgRoleMember}
}

func joinedAcmeJSON(orgID uuid.UUID) string {
	return `{"org_id":"` + orgID.String() + `","org_slug":"acme","org_name":"Acme","role":"member"}`
}

func registrationBody() dto.InviteRegistrationRequest {
	return dto.InviteRegistrationRequest{Token: inviteToken, Email: memberEmail, DisplayName: memberName, Password: authPassword}
}

func TestInviteController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{InviteService: invite.NewMockService(t), Guards: h.Guards}

	// when the list is mounted exactly as production mounts it, so a conflicting path panics here
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllInviteRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"GET /api/v1/orgs/:orgID/invites",
		"HEAD /api/v1/orgs/:orgID/invites",
		"POST /api/v1/orgs/:orgID/invites",
		"POST /api/v1/orgs/:orgID/invites/:inviteID/resend",
		"DELETE /api/v1/orgs/:orgID/invites/:inviteID",
		"POST /api/v1/invites/lookup",
		"POST /api/v1/invites/accept",
		"POST /api/v1/invites/register",
	}, registeredRoutes(h.Engine))
}

func TestInviteController_ManagementRoutesRunTheOrgPermissionFailureSuite(t *testing.T) {
	orgID, inviteID := uuid.New(), uuid.New()

	routes := []testutil.OrgPermissionRoute{
		orgRoute(http.MethodGet, orgID, "/invites", orgauthz.PermOrgManageMembers, nil),
		orgRoute(http.MethodPost, orgID, "/invites", orgauthz.PermOrgManageMembers, dto.CreateInviteRequest{Email: memberEmail}),
		orgRoute(http.MethodPost, orgID, "/invites/"+inviteID.String()+"/resend", orgauthz.PermOrgManageMembers, nil),
		orgRoute(http.MethodDelete, orgID, "/invites/"+inviteID.String(), orgauthz.PermOrgManageMembers, nil),
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunOrgPermissionFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newInviteHarness(t).Harness
			}, route)
		})
	}
}

func TestInviteController_AcceptingRunsTheAuthFailureSuite(t *testing.T) {
	// given
	route := testutil.AuthRoute{Method: http.MethodPost, Path: "/api/v1/invites/accept", Body: dto.InviteTokenRequest{Token: inviteToken}}

	// when and then an invite is only ever accepted by a signed-in account
	testutil.RunAuthFailureSuite(t, func(t *testing.T) *testutil.Harness {
		return newInviteHarness(t).Harness
	}, route)
}

func TestInviteController_ListsTheInvites(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID, inviteID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodGet, orgID, "/invites", orgauthz.PermOrgManageMembers, nil), userID)
	h.invites.EXPECT().List(mock.Anything, orgID).Return(&dto.OrganisationInviteListResponse{Invites: []dto.OrganisationInviteResponse{pendingInvite(inviteID)}}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, orgPath(orgID, "/invites")).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"invites":[`+pendingInviteJSON(inviteID)+`]}`, rec.Body.String())
}

func TestInviteController_CreatesAnEmailInvite(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID, inviteID, teamID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	body := dto.CreateInviteRequest{Email: memberEmail, Role: " member ", TeamID: new(teamID), MaxUses: new(1)}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/invites", orgauthz.PermOrgManageMembers, body), userID)
	creation := spec.InviteCreation{OrgID: orgID, ActorID: userID, Email: memberEmail, Role: model.OrgRoleMember, TeamID: new(teamID), MaxUses: new(1), IP: tenantPeerIP}
	h.invites.EXPECT().Create(mock.Anything, creation).Return(&dto.CreatedInviteResponse{OrganisationInviteResponse: pendingInvite(inviteID)}, nil)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/invites")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then an email invite never shows its link
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, pendingInviteJSON(inviteID), rec.Body.String())
}

func TestInviteController_ShowsALinkInvitesURLOnceWhenItIsCreated(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID, inviteID := uuid.New(), uuid.New(), uuid.New()
	body := dto.CreateInviteRequest{}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/invites", orgauthz.PermOrgManageMembers, body), userID)
	created := &dto.CreatedInviteResponse{
		ID: inviteID, Role: model.OrgRoleMember, ExpiresAt: inviteExpiresAt, CreatedAt: orgCreatedAt,
		URL: inviteLink,
	}
	h.invites.EXPECT().Create(mock.Anything, spec.InviteCreation{OrgID: orgID, ActorID: userID, IP: tenantPeerIP}).Return(created, nil)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/invites")).WithJSONBody(body).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"id":%q,"role":"member","use_count":0,"expired":false,"expires_at":%q,"created_at":%q,"url":%q}`, inviteID, inviteExpiresAt, orgCreatedAt, inviteLink), rec.Body.String())
}

func TestInviteController_AnInviteSavedWithoutItsEmailAnswersItsOwnError(t *testing.T) {
	// given the invite was written and audited, then the send failed
	h := newInviteHarness(t)
	userID, orgID, inviteID := uuid.New(), uuid.New(), uuid.New()
	body := dto.CreateInviteRequest{Email: memberEmail}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/invites", orgauthz.PermOrgManageMembers, body), userID)
	sendErr := fmt.Errorf("%w: %w", invite.ErrInviteNotSent, errors.Join(errors.New("smtp refused"), fmt.Errorf("%w: audit down", invite.ErrNotAudited)))
	h.invites.EXPECT().Create(mock.Anything, mock.Anything).Return(&dto.CreatedInviteResponse{OrganisationInviteResponse: pendingInvite(inviteID)}, sendErr)

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/invites")).WithJSONBody(body).WithCookie(cookie).Record()

	// then a lost audit entry never turns a failed send into a success
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"the invite was saved, but its email was not sent, so resend it"}`, rec.Body.String())
}

func TestInviteController_AnInviteWhoseAuditEntryFailedStillAnswersCreated(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID, inviteID := uuid.New(), uuid.New(), uuid.New()
	body := dto.CreateInviteRequest{Email: memberEmail}
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/invites", orgauthz.PermOrgManageMembers, body), userID)
	h.invites.EXPECT().Create(mock.Anything, mock.Anything).Return(&dto.CreatedInviteResponse{OrganisationInviteResponse: pendingInvite(inviteID)}, fmt.Errorf("%w: audit down", invite.ErrNotAudited))

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/invites")).WithJSONBody(body).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, pendingInviteJSON(inviteID), rec.Body.String())
}

func TestInviteController_ResendsAnInviteAsItsReplacement(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID, inviteID, replacementID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/invites/"+inviteID.String()+"/resend", orgauthz.PermOrgManageMembers, nil), userID)
	h.invites.EXPECT().Resend(mock.Anything, spec.InviteAction{OrgID: orgID, InviteID: inviteID, ActorID: userID, IP: tenantPeerIP}).Return(new(pendingInvite(replacementID)), nil)

	// when
	rec := h.NewRequest(http.MethodPost, invitePath(orgID, inviteID, "/resend")).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, pendingInviteJSON(replacementID), rec.Body.String())
}

func TestInviteController_AReplacementThatCouldNotBeCreatedAnswersItsOwnError(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID, inviteID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodPost, orgID, "/invites/"+inviteID.String()+"/resend", orgauthz.PermOrgManageMembers, nil), userID)
	h.invites.EXPECT().Resend(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("%w: %w", invite.ErrInviteNotReissued, errServiceDown))

	// when
	rec := h.NewRequest(http.MethodPost, invitePath(orgID, inviteID, "/resend")).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"the earlier invite was revoked, but its replacement could not be created, so invite the address again"}`, rec.Body.String())
}

func TestInviteController_RevokesAnInvite(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID, inviteID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, "/invites/"+inviteID.String(), orgauthz.PermOrgManageMembers, nil), userID)
	h.invites.EXPECT().Revoke(mock.Anything, spec.InviteAction{OrgID: orgID, InviteID: inviteID, ActorID: userID, IP: tenantPeerIP}).Return(nil)

	// when
	rec := h.NewRequest(http.MethodDelete, invitePath(orgID, inviteID, "")).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestInviteController_AnAdminInviteRevokedWithoutTheGrantToMakeAdminsIsForbidden(t *testing.T) {
	// given an organisation admin, who manages members but not admins
	h := newInviteHarness(t)
	userID, orgID, inviteID := uuid.New(), uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, "/invites/"+inviteID.String(), orgauthz.PermOrgManageMembers, nil), userID)
	h.invites.EXPECT().Revoke(mock.Anything, spec.InviteAction{OrgID: orgID, InviteID: inviteID, ActorID: userID, IP: tenantPeerIP}).Return(invite.ErrAdminRevokeForbidden)

	// when
	rec := h.NewRequest(http.MethodDelete, invitePath(orgID, inviteID, "")).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"revoking an admin invite needs the permission to make admins"}`, rec.Body.String())
}

func TestInviteController_RefusesAMalformedInviteID(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignInToOrg(orgRoute(http.MethodDelete, orgID, "/invites/not-a-uuid", orgauthz.PermOrgManageMembers, nil), userID)

	// when
	rec := h.NewRequest(http.MethodDelete, orgPath(orgID, "/invites/not-a-uuid")).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid inviteID"}`, rec.Body.String())
}

func TestInviteController_LooksUpAnInviteWithoutASession(t *testing.T) {
	// given
	h := newInviteHarness(t)
	details := &dto.InviteDetailsResponse{OrgName: orgName, OrgSlug: orgSlug, InviterName: "Alice", Role: model.OrgRoleMember, Email: memberEmail, ExpiresAt: inviteExpiresAt}
	h.invites.EXPECT().Lookup(mock.Anything, inviteToken).Return(details, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/lookup").WithJSONBody(dto.InviteTokenRequest{Token: inviteToken}).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"org_name":"Acme","org_slug":"acme","inviter_name":"Alice","role":"member","email":"bob@example.com","expires_at":%q}`, inviteExpiresAt), rec.Body.String())
}

func TestInviteController_TheTokenRoutesNeedTheToken(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   any
		signIn bool
		want   string
	}{
		{name: "lookup", path: "/api/v1/invites/lookup", body: dto.InviteTokenRequest{}, want: `{"error":"token is required"}`},
		{name: "accept", path: "/api/v1/invites/accept", body: dto.InviteTokenRequest{}, signIn: true, want: `{"error":"token is required"}`},
		{name: "register without a token", path: "/api/v1/invites/register", body: dto.InviteRegistrationRequest{Password: authPassword}, want: `{"error":"token and password are required"}`},
		{name: "register without a password", path: "/api/v1/invites/register", body: dto.InviteRegistrationRequest{Token: inviteToken}, want: `{"error":"token and password are required"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newInviteHarness(t)
			request := h.NewRequest(http.MethodPost, tc.path).WithJSONBody(tc.body)
			if tc.signIn {
				userID := uuid.New()
				request = request.WithCookie(h.SignIn(userID))
				h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			}

			// when
			rec := request.Record()

			// then the service is never asked
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestInviteController_AcceptsAnInviteForTheSignedInAccount(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID, orgID := uuid.New(), uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	h.invites.EXPECT().Accept(mock.Anything, spec.InviteJoin{Token: inviteToken, UserID: userID, IP: tenantPeerIP}).Return(joinedAcme(orgID), nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/accept").WithJSONBody(dto.InviteTokenRequest{Token: inviteToken}).WithRemoteAddr(publicPeer).WithCookie(cookie).Record()

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, joinedAcmeJSON(orgID), rec.Body.String())
}

func TestInviteController_AJoinCommittedWithAnAftermathStillAnswersSuccess(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "a lost audit entry", err: fmt.Errorf("%w: audit down", invite.ErrNotAudited)},
		{name: "a cached membership that could not be cleared", err: errors.Join(fmt.Errorf("%w: valkey down", invite.ErrCacheNotCleared), fmt.Errorf("%w: audit down", invite.ErrNotAudited))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			h := newInviteHarness(t)
			userID, orgID := uuid.New(), uuid.New()
			cookie := h.SignIn(userID)
			h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
			h.invites.EXPECT().Accept(mock.Anything, mock.Anything).Return(joinedAcme(orgID), tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/invites/accept").WithJSONBody(dto.InviteTokenRequest{Token: inviteToken}).WithCookie(cookie).Record()

			// then the account is a member, so the page moves on
			require.Equal(t, http.StatusOK, rec.Code)
			assert.JSONEq(t, joinedAcmeJSON(orgID), rec.Body.String())
		})
	}
}

func TestInviteController_AFullOrganisationAnswersPaymentRequired(t *testing.T) {
	// given
	h := newInviteHarness(t)
	userID := uuid.New()
	cookie := h.SignIn(userID)
	h.Authz.EXPECT().IsLocked(mock.Anything, userID).Return(false, nil)
	denied := &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitMembers, Allowed: 10, Current: 10}
	h.invites.EXPECT().Accept(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("check the member allowance: %w", denied))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/accept").WithJSONBody(dto.InviteTokenRequest{Token: inviteToken}).WithCookie(cookie).Record()

	// then
	assert.Equal(t, http.StatusPaymentRequired, rec.Code)
	assert.JSONEq(t, `{"error":"upgrade required","code":"upgrade_required","limit":"members","allowed":10,"current":10,"plan":"free"}`, rec.Body.String())
}

func TestInviteController_RegistersThroughAnInviteWithoutASession(t *testing.T) {
	// given
	h := newInviteHarness(t)
	orgID := uuid.New()
	registration := spec.InviteRegistration{Token: inviteToken, Email: memberEmail, DisplayName: memberName, Password: authPassword, IP: tenantPeerIP}
	registered := &dto.InviteRegistrationResponse{Email: memberEmail, Joined: *joinedAcme(orgID)}
	h.invites.EXPECT().Register(mock.Anything, registration).Return(registered, nil)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).WithRemoteAddr(publicPeer).Record()

	// then the page offers Sign in, because registering creates no session
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"email":"bob@example.com","verification_required":false,"joined":`+joinedAcmeJSON(orgID)+`}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestInviteController_RefusesARegistrationWhilePasswordSignInIsOff(t *testing.T) {
	// given
	h := newInviteHarness(t)
	h.invites.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, invite.ErrPasswordSignInDisabled)

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).WithRemoteAddr(publicPeer).Record()

	// then the page points the visitor to a provider, and no session is made
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"password sign-in is turned off on this site","code":"password_sign_in_disabled"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestInviteController_ARegistrationWhoseVerificationEmailFailedStillAnswersCreated(t *testing.T) {
	// given
	h := newInviteHarness(t)
	orgID := uuid.New()
	registered := &dto.InviteRegistrationResponse{Email: memberEmail, VerificationRequired: true, Joined: *joinedAcme(orgID)}
	h.invites.EXPECT().Register(mock.Anything, mock.Anything).Return(registered, fmt.Errorf("%w to user: smtp refused", invite.ErrVerificationNotSent))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).Record()

	// then the account exists and has joined, and the login page can send the link again
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"email":"bob@example.com","verification_required":true,"joined":`+joinedAcmeJSON(orgID)+`}`, rec.Body.String())
}

func TestInviteController_AnAccountLeftOutsideTheOrganisationAnswersItsOwnError(t *testing.T) {
	// given the join was refused after the account committed, and removing it failed too
	h := newInviteHarness(t)
	denied := &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitMembers, Allowed: 10, Current: 10}
	h.invites.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("%w: %w", invite.ErrAccountLeftBehind, errors.Join(denied, errServiceDown)))

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).Record()

	// then it is never answered as the refusal it wraps
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"the account was created, but it could not join the organisation or be removed again"}`, rec.Body.String())
}

func TestInviteController_MapsEveryRefusal(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: invite.ErrInvalidInvite, status: http.StatusBadRequest, body: `{"error":"the invite link is invalid, used, revoked or has expired","code":"invite_invalid"}`},
		{err: invite.ErrInvalidRole, status: http.StatusBadRequest, body: `{"error":"an invite's role is admin or member"}`},
		{err: fmt.Errorf("%w: missing @", invite.ErrInvalidEmail), status: http.StatusBadRequest, body: `{"error":"a valid email address is required"}`},
		{err: invite.ErrInvalidMaxUses, status: http.StatusBadRequest, body: `{"error":"an invite link works at least once, and an email invite exactly once"}`},
		{err: user.ErrDisplayNameRequired, status: http.StatusBadRequest, body: `{"error":"` + user.ErrDisplayNameRequired.Error() + `","code":"display_name_required"}`},
		{err: user.ErrPasswordTooShort, status: http.StatusBadRequest, body: `{"error":"` + user.ErrPasswordTooShort.Error() + `","code":"password_too_short"}`},
		{err: user.ErrPasswordTooLong, status: http.StatusBadRequest, body: `{"error":"` + user.ErrPasswordTooLong.Error() + `"}`},
		{err: invite.ErrInviteNotFound, status: http.StatusNotFound, body: `{"error":"invite not found"}`},
		{err: invite.ErrOrganisationNotFound, status: http.StatusNotFound, body: `{"error":"organisation not found"}`},
		{err: invite.ErrTeamNotFound, status: http.StatusNotFound, body: `{"error":"team not found"}`},
		{err: invite.ErrAdminInviteForbidden, status: http.StatusForbidden, body: `{"error":"inviting an admin needs the permission to make admins"}`},
		{err: invite.ErrAdminRevokeForbidden, status: http.StatusForbidden, body: `{"error":"revoking an admin invite needs the permission to make admins"}`},
		{err: invite.ErrEmailMismatch, status: http.StatusForbidden, body: `{"error":"this invite is for a different email address","code":"invite_email_mismatch"}`},
		{err: invite.ErrEmailUnverified, status: http.StatusForbidden, body: `{"error":"verify your email address before accepting this invite","code":"email_unverified"}`},
		{err: invite.ErrPasswordSignInDisabled, status: http.StatusForbidden, body: `{"error":"password sign-in is turned off on this site","code":"password_sign_in_disabled"}`},
		{err: invite.ErrAlreadyMember, status: http.StatusConflict, body: `{"error":"already a member of this organisation","code":"already_member"}`},
		{err: invite.ErrInvitePending, status: http.StatusConflict, body: `{"error":"that email address already has a pending invite","code":"invite_pending"}`},
		{err: invite.ErrNotResendable, status: http.StatusConflict, body: `{"error":"only an invite sent to an email address can be resent","code":"invite_not_resendable"}`},
		{err: invite.ErrTeamArchived, status: http.StatusConflict, body: `{"error":"that team is archived","code":"team_archived"}`},
		{err: invite.ErrEmailTaken, status: http.StatusConflict, body: `{"error":"that email address already has an account, so sign in to accept the invite","code":"email_taken"}`},
		{err: invite.ErrEmailUnavailable, status: http.StatusServiceUnavailable, body: `{"error":"email is not available on this site","code":"email_unavailable"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to create the account"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			h := newInviteHarness(t)
			h.invites.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := h.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).Record()

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestInviteController_CreatingAndResendingDrawOnTheMailBucket(t *testing.T) {
	// given every attempt spends a token before the guard, whatever the guard then answers
	h := newInviteHarness(t)
	orgID, inviteID := uuid.New(), uuid.New()
	attempts := []string{orgPath(orgID, "/invites"), invitePath(orgID, inviteID, "/resend")}

	for i := range middleware.MailAttemptsPerHour {
		rec := h.NewRequest(http.MethodPost, attempts[i%len(attempts)]).WithJSONBody(dto.CreateInviteRequest{Email: memberEmail}).WithRemoteAddr(publicPeer).Record()
		require.Equal(t, http.StatusUnauthorized, rec.Code, "attempt %d", i+1)
	}

	// when
	rec := h.NewRequest(http.MethodPost, orgPath(orgID, "/invites")).WithJSONBody(dto.CreateInviteRequest{Email: memberEmail}).WithRemoteAddr(publicPeer).Record()

	// then
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestInviteController_RegisteringDrawsOnTheMailBucket(t *testing.T) {
	// given
	h := newInviteHarness(t)
	h.invites.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, invite.ErrInvalidInvite)

	for i := range middleware.MailAttemptsPerHour {
		rec := h.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).WithRemoteAddr(publicPeer).Record()
		require.Equal(t, http.StatusBadRequest, rec.Code, "attempt %d", i+1)
	}

	// when
	rec := h.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).WithRemoteAddr(publicPeer).Record()

	// then
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestInviteController_RegisteringSharesTheCredentialBucketWithLogin(t *testing.T) {
	// given the login routes and the invite routes on one engine, as production mounts them
	a := newAuthHarness(t, testutil.TestBaseURL, false)
	i := mountInviteRoutes(t, a.Harness)
	a.auth.EXPECT().Login(mock.Anything, mock.Anything).Return(nil, user.ErrInvalidCredentials)

	for attempt := range middleware.CredentialAttemptsPerMinute {
		rec := a.NewRequest(http.MethodPost, "/api/v1/auth/login").WithJSONBody(dto.LoginRequest{Email: fmt.Sprintf("user%d@example.com", attempt), Password: "x"}).WithRemoteAddr(publicPeer).Record()
		require.Equal(t, http.StatusUnauthorized, rec.Code, "attempt %d", attempt+1)
	}

	// when
	rec := i.NewRequest(http.MethodPost, "/api/v1/invites/register").WithJSONBody(registrationBody()).WithRemoteAddr(publicPeer).Record()

	// then the register handler never runs, which the strict invite mock would report
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
