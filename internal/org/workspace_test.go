package org

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/team"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func workspace(ownerID uuid.UUID, slug string) *model.Workspace {
	orgID, teamID := uuid.New(), uuid.New()
	created := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	return &model.Workspace{
		Organisation: model.Organisation{ID: orgID, Slug: slug, Name: "Acme Retros", AllowGuests: true, CreatedBy: &ownerID, CreatedAt: created, UpdatedAt: created},
		Owner:        model.OrganisationMember{OrgID: orgID, UserID: ownerID, Role: model.OrgRoleOwner, JoinedAt: created},
		Team:         model.Team{ID: teamID, OrgID: orgID, Name: "Platform", CreatedAt: created},
		Lead:         model.TeamMember{OrgID: orgID, TeamID: teamID, UserID: ownerID, Role: model.TeamRoleLead, JoinedAt: created},
	}
}

func workspaceRequest(ownerID uuid.UUID, name string) WorkspaceRequest {
	return WorkspaceRequest{ActorID: ownerID, IP: testIP, Name: name, TeamName: "  Platform  "}
}

func newWorkspace(ownerID uuid.UUID, slug string) spec.NewWorkspace {
	return spec.NewWorkspace{OwnerID: ownerID, Slug: slug, Name: "Acme Retros", TeamName: "Platform", MaxOwned: 1}
}

func (m *testMocks) ownsNone(ownerID uuid.UUID) {
	m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(1, nil)
	m.organisations.EXPECT().CountOwnedByUser(mock.Anything, ownerID).Return(0, nil)
}

func (m *testMocks) workspaceAudited(ownerID uuid.UUID, created *model.Workspace) {
	orgID := created.Organisation.ID
	teamCreated := expectedOrgEntry(orgID, ownerID, audit.ActionTeamCreated, map[string]string{"name": "Platform"})
	teamCreated.TargetType = audit.TargetTeam
	teamCreated.TargetID = created.Team.ID.String()

	m.audited(expectedOrgEntry(orgID, ownerID, audit.ActionOrgCreated, map[string]string{"name": "Acme Retros", "slug": created.Organisation.Slug}), teamCreated)
}

func TestCreateWorkspace_MakesTheCreatorOwnerOfTheOrganisationAndLeadOfItsFirstTeam(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ownerID := uuid.New()
	created := workspace(ownerID, "acme-retros")
	m.ownsNone(ownerID)
	m.organisations.EXPECT().CreateWorkspace(mock.Anything, newWorkspace(ownerID, "acme-retros")).Return(created, nil)
	m.workspaceAudited(ownerID, created)

	// when
	got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, "  Acme Retros  "))

	// then
	require.NoError(t, err)
	assert.Equal(t, created.Organisation.ID, got.Organisation.ID)
	assert.Equal(t, "acme-retros", got.Organisation.Slug)
	assert.Equal(t, model.OrgRoleOwner, got.Organisation.Role)
	assert.Equal(t, created.Team.ID, got.Team.ID)
	assert.Equal(t, "Platform", got.Team.Name)
	assert.Equal(t, model.TeamRoleLead, got.Team.Role)
}

func TestCreateWorkspace_RefusesANameThatIsMissingOrTooLongBeforeReadingAnything(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		teamName string
		want     error
	}{
		{name: "no organisation name", orgName: "   ", teamName: "Platform", want: ErrNameRequired},
		{name: "an organisation name over 80 characters", orgName: strings.Repeat("é", 81), teamName: "Platform", want: ErrNameTooLong},
		{name: "no team name", orgName: "Acme Retros", teamName: "", want: team.ErrNameRequired},
		{name: "a team name over 80 characters", orgName: "Acme Retros", teamName: strings.Repeat("t", 81), want: team.ErrNameTooLong},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.CreateWorkspace(t.Context(), WorkspaceRequest{ActorID: uuid.New(), IP: testIP, Name: tc.orgName, TeamName: tc.teamName})

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestCreateWorkspace_AcceptsANameOfExactlyEightyCharacters(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ownerID := uuid.New()
	name := strings.Repeat("é", 80)
	created := workspace(ownerID, testSlugSuffix)
	created.Organisation.Name = name
	m.ownsNone(ownerID)
	m.organisations.EXPECT().CreateWorkspace(mock.Anything, spec.NewWorkspace{OwnerID: ownerID, Slug: testSlugSuffix, Name: name, TeamName: "Platform", MaxOwned: 1}).Return(created, nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Twice()

	// when
	got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, name))

	// then
	require.NoError(t, err)
	assert.Equal(t, name, got.Organisation.Name)
}

func TestCreateWorkspace_RefusesAnOwnerWhoAlreadyOwnsTheirAllowance(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ownerID := uuid.New()
	m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(1, nil)
	m.organisations.EXPECT().CountOwnedByUser(mock.Anything, ownerID).Return(1, nil)

	// when
	got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, "Acme Retros"))

	// then the strict mocks prove nothing was written
	assert.ErrorIs(t, err, ErrOwnedLimitReached)
	assert.Nil(t, got)
}

func TestCreateWorkspace_TheConditionalInsertRefusesAnOwnerWhoRacedPastTheirAllowance(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ownerID := uuid.New()
	m.ownsNone(ownerID)
	m.organisations.EXPECT().CreateWorkspace(mock.Anything, newWorkspace(ownerID, "acme-retros")).Return(nil, fmt.Errorf("create workspace: %w", dao.ErrQuotaExceeded))

	// when
	got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, "Acme Retros"))

	// then
	assert.ErrorIs(t, err, ErrOwnedLimitReached)
	assert.Nil(t, got)
}

func TestCreateWorkspace_TriesASuffixedAddressWhenTheNameIsTaken(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ownerID := uuid.New()
	created := workspace(ownerID, "acme-retros-"+testSlugSuffix)
	m.ownsNone(ownerID)
	m.organisations.EXPECT().CreateWorkspace(mock.Anything, newWorkspace(ownerID, "acme-retros")).Return(nil, fmt.Errorf("create workspace: %w", dao.ErrConflict))
	m.organisations.EXPECT().CreateWorkspace(mock.Anything, newWorkspace(ownerID, "acme-retros-"+testSlugSuffix)).Return(created, nil)
	m.workspaceAudited(ownerID, created)

	// when
	got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, "Acme Retros"))

	// then
	require.NoError(t, err)
	assert.Equal(t, "acme-retros-"+testSlugSuffix, got.Organisation.Slug)
}

func TestCreateWorkspace_SuffixesANameThatMakesNoUsableAddress(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		wantSlug string
	}{
		{name: "no letters or digits", orgName: "日本のチーム", wantSlug: testSlugSuffix},
		{name: "a reserved path segment", orgName: "Admin", wantSlug: "admin-" + testSlugSuffix},
		{name: "fewer than three characters", orgName: "QA", wantSlug: "qa-" + testSlugSuffix},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			ownerID := uuid.New()
			created := workspace(ownerID, tc.wantSlug)
			m.ownsNone(ownerID)
			m.organisations.EXPECT().CreateWorkspace(mock.Anything, spec.NewWorkspace{OwnerID: ownerID, Slug: tc.wantSlug, Name: tc.orgName, TeamName: "Platform", MaxOwned: 1}).Return(created, nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Twice()

			// when
			got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, tc.orgName))

			// then the unusable address was never sent
			require.NoError(t, err)
			assert.Equal(t, tc.wantSlug, got.Organisation.Slug)
		})
	}
}

func TestCreateWorkspace_GivesUpOnceEveryAttemptedAddressIsTaken(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ownerID := uuid.New()
	drawn := 0
	svc.slugSuffix = func() string {
		drawn++
		return fmt.Sprintf("draw%02d", drawn)
	}
	m.ownsNone(ownerID)
	m.organisations.EXPECT().CreateWorkspace(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("create workspace: %w", dao.ErrConflict)).Times(slugAttempts)

	// when
	got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, "Acme Retros"))

	// then
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSlugTaken)
	assert.Nil(t, got)
	assert.Equal(t, slugAttempts-1, drawn)
}

func TestCreateWorkspace_ReportsAFailedRead(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, ownerID uuid.UUID)
	}{
		{name: "the allowance setting", given: func(m *testMocks, ownerID uuid.UUID) {
			m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(0, errDatabaseDown)
		}},
		{name: "the owned count", given: func(m *testMocks, ownerID uuid.UUID) {
			m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(1, nil)
			m.organisations.EXPECT().CountOwnedByUser(mock.Anything, ownerID).Return(0, errDatabaseDown)
		}},
		{name: "the write", given: func(m *testMocks, ownerID uuid.UUID) {
			m.ownsNone(ownerID)
			m.organisations.EXPECT().CreateWorkspace(mock.Anything, newWorkspace(ownerID, "acme-retros")).Return(nil, errDatabaseDown)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			ownerID := uuid.New()
			tc.given(m, ownerID)

			// when
			got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, "Acme Retros"))

			// then
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.Nil(t, got)
		})
	}
}

func TestCreateWorkspace_AnswersWithTheWorkspaceWhenAnAuditEntryFails(t *testing.T) {
	// given
	svc, m := newTestService(t)
	ownerID := uuid.New()
	created := workspace(ownerID, "acme-retros")
	m.ownsNone(ownerID)
	m.organisations.EXPECT().CreateWorkspace(mock.Anything, newWorkspace(ownerID, "acme-retros")).Return(created, nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown).Once()
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	// when
	got, err := svc.CreateWorkspace(t.Context(), workspaceRequest(ownerID, "Acme Retros"))

	// then
	assert.ErrorIs(t, err, ErrNotAudited)
	require.NotNil(t, got)
	assert.Equal(t, created.Organisation.ID, got.Organisation.ID)
}

func TestSlugBase_KeepsLowerCaseLettersAndDigitsJoinedBySingleHyphens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "words", in: "Acme Retros", want: "acme-retros"},
		{name: "punctuation and runs of spaces", in: "  Hello,   World!!  ", want: "hello-world"},
		{name: "digits", in: "Team 42", want: "team-42"},
		{name: "letters outside a to z", in: "Café Crème", want: "caf-cr-me"},
		{name: "nothing usable", in: "---", want: ""},
		{name: "cut to forty characters without a trailing hyphen", in: strings.Repeat("a", 39) + " b and more", want: strings.Repeat("a", 39)},
		{name: "cut to forty characters", in: strings.Repeat("abcd ", 20), want: "abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := slugBase(tc.in)

			// then
			assert.Equal(t, tc.want, got)
			assert.LessOrEqual(t, len(got), maxSlugLength)
		})
	}
}

func TestSlugCandidate_SuffixesEveryAttemptAfterTheFirst(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		attempt int
		want    string
	}{
		{name: "the first attempt is the base", base: "acme-retros", attempt: 0, want: "acme-retros"},
		{name: "a later attempt is suffixed", base: "acme-retros", attempt: 1, want: "acme-retros-" + testSlugSuffix},
		{name: "a long base is cut to leave space for the suffix", base: strings.Repeat("a", 40), attempt: 2, want: strings.Repeat("a", 33) + "-" + testSlugSuffix},
		{name: "a cut never leaves a double hyphen", base: strings.Repeat("a", 32) + "-" + strings.Repeat("b", 7), attempt: 1, want: strings.Repeat("a", 32) + "-" + testSlugSuffix},
		{name: "an empty base is only the suffix", base: "", attempt: 1, want: testSlugSuffix},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got := svc.slugCandidate(tc.base, tc.attempt)

			// then
			assert.Equal(t, tc.want, got)
			assert.LessOrEqual(t, len(got), maxSlugLength)
		})
	}
}

func TestNewService_DrawsLowerCaseSuffixesThatFitTheSlugFormat(t *testing.T) {
	// given
	svc := NewService(Deps{}).(*service)

	// when
	first, second := svc.slugSuffix(), svc.slugSuffix()

	// then
	assert.Len(t, first, slugSuffixLength)
	assert.NotEqual(t, first, second)
	assert.True(t, usableSlug("acme-"+first))
	assert.Equal(t, strings.ToLower(first), first)
}

func TestNormaliseSlug(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "trimmed and lower-cased", in: "  Acme-Retros ", want: "acme-retros"},
		{name: "three characters", in: "abc", want: "abc"},
		{name: "forty characters", in: strings.Repeat("a", 40), want: strings.Repeat("a", 40)},
		{name: "two characters", in: "ab", wantErr: ErrInvalidSlug},
		{name: "forty-one characters", in: strings.Repeat("a", 41), wantErr: ErrInvalidSlug},
		{name: "a leading hyphen", in: "-abc", wantErr: ErrInvalidSlug},
		{name: "a trailing hyphen", in: "abc-", wantErr: ErrInvalidSlug},
		{name: "an underscore", in: "ab_c", wantErr: ErrInvalidSlug},
		{name: "a space inside", in: "ab c", wantErr: ErrInvalidSlug},
		{name: "a reserved path segment", in: "Admin", wantErr: ErrSlugReserved},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got, err := normaliseSlug(tc.in)

			// then
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
