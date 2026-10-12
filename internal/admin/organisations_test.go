package admin

import (
	"testing"
	"time"

	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	orgCreatedAt = time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
)

func staffOrganisation(orgID uuid.UUID, ownerID uuid.UUID) model.StaffOrganisation {
	return model.StaffOrganisation{
		ID: orgID, Slug: "acme", Name: "Acme", CreatedAt: orgCreatedAt,
		OwnerID:          new(ownerID),
		OwnerDisplayName: new(testDisplayName),
		OwnerEmail:       new(testEmail),
		MemberCount:      3,
		TeamCount:        2,
		BoardCount:       5,
	}
}

func freePlan(t *testing.T) *entitlements.Plan {
	t.Helper()

	plan, known := entitlements.PlanFor(model.PlanFree)
	require.True(t, known)

	return plan
}

func TestListOrganisations_ClampsThePageAndTrimsTheSearch(t *testing.T) {
	// given
	svc, m := newTestService(t)
	page := spec.OrganisationListing{Search: "acme", Limit: maxPageSize, Offset: 0}
	m.organisations.EXPECT().ListForStaff(mock.Anything, page).Return([]model.StaffOrganisation{}, nil)
	m.organisations.EXPECT().CountForStaff(mock.Anything, "acme").Return(0, nil)

	// when
	listing, err := svc.ListOrganisations(t.Context(), spec.OrganisationListing{Search: "  acme ", Limit: 5000, Offset: -3})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.AdminOrganisationListResponse{Organisations: []dto.AdminOrganisationResponse{}, Total: 0, Limit: maxPageSize, Offset: 0}, listing)
}

func TestListOrganisations_MapsEachOrganisationWithItsOwnerCountsAndPlan(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID, orphanID := uuid.New(), uuid.New(), uuid.New()
	deletedAt := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	orphan := model.StaffOrganisation{ID: orphanID, Slug: "gone", Name: "Gone", CreatedAt: orgCreatedAt, DeletedAt: &deletedAt}
	m.organisations.EXPECT().ListForStaff(mock.Anything, spec.OrganisationListing{Limit: defaultOrganisationPageSize}).Return([]model.StaffOrganisation{staffOrganisation(orgID, ownerID), orphan}, nil)
	m.organisations.EXPECT().CountForStaff(mock.Anything, "").Return(27, nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(t), nil)
	m.entitlements.EXPECT().For(mock.Anything, orphanID).Return(freePlan(t), nil)

	// when
	listing, err := svc.ListOrganisations(t.Context(), spec.OrganisationListing{})

	// then
	require.NoError(t, err)
	assert.Equal(t, 27, listing.Total)
	assert.Equal(t, []dto.AdminOrganisationResponse{
		{
			ID: orgID, Name: "Acme", Slug: "acme", Plan: model.PlanFree,
			OwnerID: &ownerID, OwnerDisplayName: testDisplayName, OwnerEmail: testEmail,
			MemberCount: 3, TeamCount: 2, BoardCount: 5, CreatedAt: "2026-09-10T09:00:00Z",
		},
		{ID: orphanID, Name: "Gone", Slug: "gone", Plan: model.PlanFree, CreatedAt: "2026-09-10T09:00:00Z", DeletedAt: "2026-10-01T08:00:00Z"},
	}, listing.Organisations)
}

func TestListOrganisations_ReturnsAReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.organisations.EXPECT().ListForStaff(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	listing, err := svc.ListOrganisations(t.Context(), spec.OrganisationListing{})

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listing)
}

func TestListOrganisations_ReturnsAFailedPlanRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.organisations.EXPECT().ListForStaff(mock.Anything, mock.Anything).Return([]model.StaffOrganisation{staffOrganisation(orgID, uuid.New())}, nil)
	m.organisations.EXPECT().CountForStaff(mock.Anything, "").Return(1, nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(nil, entitlements.ErrUnknownPlan)

	// when
	listing, err := svc.ListOrganisations(t.Context(), spec.OrganisationListing{})

	// then
	require.ErrorIs(t, err, entitlements.ErrUnknownPlan)
	assert.Nil(t, listing)
}

func TestGetOrganisation_ShowsTheTeamsAndMembersWithTheirRoles(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID, teamID := uuid.New(), uuid.New(), uuid.New()
	archivedAt := time.Date(2026, 9, 30, 7, 0, 0, 0, time.UTC)
	joinedAt := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	m.organisations.EXPECT().GetForStaff(mock.Anything, orgID).Return(new(staffOrganisation(orgID, ownerID)), nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(t), nil)
	m.teams.EXPECT().ListWithCounts(mock.Anything, orgID).Return([]model.TeamWithCounts{
		{ID: teamID, OrgID: orgID, Name: "Platform", CreatedAt: orgCreatedAt, ArchivedAt: &archivedAt, MemberCount: 4, BoardCount: 2},
	}, nil)
	m.organisations.EXPECT().ListMembers(mock.Anything, orgID).Return([]model.OrganisationMemberProfile{
		{OrgID: orgID, UserID: ownerID, Role: model.OrgRoleOwner, JoinedAt: joinedAt, Email: testEmail, DisplayName: testDisplayName, SiteRole: role.Support},
	}, nil)

	// when
	detail, err := svc.GetOrganisation(t.Context(), orgID)

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.AdminOrganisationDetailResponse{
		ID: orgID, Name: "Acme", Slug: "acme", Plan: model.PlanFree,
		OwnerID: &ownerID, OwnerDisplayName: testDisplayName, OwnerEmail: testEmail,
		MemberCount: 3, TeamCount: 2, BoardCount: 5, CreatedAt: "2026-09-10T09:00:00Z",
		Teams: []dto.AdminOrganisationTeamResponse{
			{ID: teamID, Name: "Platform", MemberCount: 4, BoardCount: 2, CreatedAt: "2026-09-10T09:00:00Z", ArchivedAt: "2026-09-30T07:00:00Z"},
		},
		Members: []dto.OrganisationMemberResponse{
			{UserID: ownerID, Email: testEmail, DisplayName: testDisplayName, Role: model.OrgRoleOwner, SiteRole: role.Support, JoinedAt: "2026-09-11T09:00:00Z"},
		},
	}, detail)
}

func TestGetOrganisation_AnswersNotFoundForAnUnknownOrganisation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.organisations.EXPECT().GetForStaff(mock.Anything, orgID).Return(nil, errRowMissing)

	// when
	detail, err := svc.GetOrganisation(t.Context(), orgID)

	// then
	require.ErrorIs(t, err, ErrOrganisationNotFound)
	assert.Nil(t, detail)
}

func TestGetOrganisation_ReturnsAFailedMemberRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.organisations.EXPECT().GetForStaff(mock.Anything, orgID).Return(new(staffOrganisation(orgID, uuid.New())), nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(t), nil)
	m.teams.EXPECT().ListWithCounts(mock.Anything, orgID).Return([]model.TeamWithCounts{}, nil)
	m.organisations.EXPECT().ListMembers(mock.Anything, orgID).Return(nil, errDatabaseDown)

	// when
	detail, err := svc.GetOrganisation(t.Context(), orgID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, detail)
}
