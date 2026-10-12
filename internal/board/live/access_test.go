package live

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"retrotide/internal/board/policy"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	liveMocks struct {
		boards   *repository.MockBoardRepository
		orgAuthz *orgauthz.MockService
	}

	teamStanding struct {
		orgMember bool
		onTeam    bool
		lead      bool
	}
)

var (
	notInOrg    = teamStanding{}
	offTeam     = teamStanding{orgMember: true}
	teamMember  = teamStanding{orgMember: true, onTeam: true}
	teamLead    = teamStanding{orgMember: true, onTeam: true, lead: true}
	allBoardCap = []model.BoardCapability{
		model.BoardCapabilityView,
		model.BoardCapabilityParticipate,
		model.BoardCapabilityFacilitate,
		model.BoardCapabilityManage,
		model.BoardCapabilityExport,
	}
)

func newTestLive(t *testing.T) (Service, *liveMocks) {
	t.Helper()

	m := &liveMocks{
		boards:   repository.NewMockBoardRepository(t),
		orgAuthz: orgauthz.NewMockService(t),
	}

	return New(Deps{Boards: m.boards, OrgAuthz: m.orgAuthz}), m
}

func liveHeader() model.BoardHeader {
	return model.BoardHeader{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		TeamID:      new(uuid.New()),
		Title:       "Sprint 42",
		LinkAccess:  model.LinkAccessOff,
		AccessEpoch: 3,
		Version:     7,
		Anonymity:   model.AnonymityNamed}
}

func user() model.Principal {
	return model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
}

func organisationHeader() model.BoardHeader {
	header := liveHeader()
	header.TeamID = nil

	return header
}

func (m *liveMocks) standing(header model.BoardHeader, principal model.Principal, standing teamStanding) {
	if header.TeamID == nil {
		m.organisationStanding(header, principal, standing)
		return
	}

	view := spec.TeamPermissionCheck{TeamID: *header.TeamID, UserID: principal.ID, Permission: orgauthz.PermTeamView}
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, view).Return(orgauthz.Decision{Member: standing.orgMember, Allowed: standing.onTeam}, nil)

	if !standing.onTeam {
		return
	}

	lead := spec.TeamPermissionCheck{TeamID: *header.TeamID, UserID: principal.ID, Permission: orgauthz.PermTeamManageBoards}
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, lead).Return(orgauthz.Decision{Member: true, Allowed: standing.lead}, nil)
}

func (m *liveMocks) organisationStanding(header model.BoardHeader, principal model.Principal, standing teamStanding) {
	view := spec.OrgPermissionCheck{OrgID: header.OrgID, UserID: principal.ID, Permission: orgauthz.PermOrgView}
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, view).Return(orgauthz.Decision{Member: standing.orgMember, Allowed: standing.orgMember}, nil)

	if !standing.orgMember {
		return
	}

	lead := spec.OrgPermissionCheck{OrgID: header.OrgID, UserID: principal.ID, Permission: orgauthz.PermOrgManageBoards}
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, lead).Return(orgauthz.Decision{Member: true, Allowed: standing.lead}, nil)
}

func (m *liveMocks) participant(header model.BoardHeader, principal model.Principal, participant *model.BoardParticipant) {
	key := spec.BoardParticipantUser{BoardID: header.ID, UserID: principal.ID}
	if participant == nil {
		m.boards.EXPECT().GetParticipantForUser(mock.Anything, key).Return(nil, errNoRow)
		return
	}

	m.boards.EXPECT().GetParticipantForUser(mock.Anything, key).Return(participant, nil)
}

func storedRow(header model.BoardHeader, principal model.Principal, role model.BoardRole) *model.BoardParticipant {
	return &model.BoardParticipant{
		ID:          uuid.New(),
		BoardID:     header.ID,
		UserID:      new(principal.ID),
		Role:        role,
		AccessEpoch: header.AccessEpoch,
		JoinedVia:   model.JoinPathMembership,
		JoinedAt:    time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC),
	}
}

func linkRow(header model.BoardHeader, principal model.Principal, role model.BoardRole) *model.BoardParticipant {
	row := storedRow(header, principal, role)
	row.JoinedVia = model.JoinPathLink

	return row
}

func TestResolve_DerivesTheRoleFromMembershipAndTheStoredRow(t *testing.T) {
	tests := []struct {
		name       string
		standing   teamStanding
		stored     model.BoardRole
		wantRole   model.BoardRole
		wantOrg    bool
		wantJoined bool
	}{
		{name: "a team lead with no row facilitates", standing: teamLead, wantRole: model.BoardRoleFacilitator, wantOrg: true},
		{name: "a team lead stored as participant still facilitates", standing: teamLead, stored: model.BoardRoleParticipant, wantRole: model.BoardRoleFacilitator, wantOrg: true, wantJoined: true},
		{name: "a team member with no row participates", standing: teamMember, wantRole: model.BoardRoleParticipant, wantOrg: true},
		{name: "a team member keeps a stored facilitator promotion", standing: teamMember, stored: model.BoardRoleFacilitator, wantRole: model.BoardRoleFacilitator, wantOrg: true, wantJoined: true},
		{name: "a team member stored as participant participates", standing: teamMember, stored: model.BoardRoleParticipant, wantRole: model.BoardRoleParticipant, wantOrg: true, wantJoined: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := liveHeader()
			principal := user()
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, tc.standing)

			var row *model.BoardParticipant
			if tc.stored != "" {
				row = storedRow(header, principal, tc.stored)
			}
			m.participant(header, principal, row)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.wantRole, access.Role)
			assert.Equal(t, tc.wantOrg, access.OrgMember)
			assert.Equal(t, principal, access.Principal)
			assert.Equal(t, header, access.Header)
			assert.Equal(t, policy.Capabilities(tc.wantRole, tc.wantOrg), access.Capabilities)
			if tc.wantJoined {
				require.NotNil(t, access.ParticipantID)
				assert.Equal(t, row.ID, *access.ParticipantID)
			} else {
				assert.Nil(t, access.ParticipantID)
			}
		})
	}
}

func TestResolve_AnOrganisationBoardAdmitsEveryOrganisationMember(t *testing.T) {
	tests := []struct {
		name       string
		standing   teamStanding
		stored     model.BoardRole
		wantRole   model.BoardRole
		wantJoined bool
	}{
		{name: "a plain member with no row participates", standing: teamMember, wantRole: model.BoardRoleParticipant},
		{name: "a plain member stored as participant participates", standing: teamMember, stored: model.BoardRoleParticipant, wantRole: model.BoardRoleParticipant, wantJoined: true},
		{name: "the creator, a plain member stored as facilitator, facilitates", standing: teamMember, stored: model.BoardRoleFacilitator, wantRole: model.BoardRoleFacilitator, wantJoined: true},
		{name: "an owner or admin with no row facilitates", standing: teamLead, wantRole: model.BoardRoleFacilitator},
		{name: "an owner or admin stored as participant still facilitates", standing: teamLead, stored: model.BoardRoleParticipant, wantRole: model.BoardRoleFacilitator, wantJoined: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a board that belongs to the organisation and no team
			svc, m := newTestLive(t)
			header := organisationHeader()
			principal := user()
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, tc.standing)

			var row *model.BoardParticipant
			if tc.stored != "" {
				row = storedRow(header, principal, tc.stored)
			}
			m.participant(header, principal, row)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then no team is ever asked about
			require.NoError(t, err)
			assert.Equal(t, tc.wantRole, access.Role)
			assert.True(t, access.OrgMember)
			assert.Equal(t, header, access.Header)
			assert.Equal(t, policy.Capabilities(tc.wantRole, true), access.Capabilities)
			if tc.wantJoined {
				require.NotNil(t, access.ParticipantID)
				assert.Equal(t, row.ID, *access.ParticipantID)
			} else {
				assert.Nil(t, access.ParticipantID)
			}
			m.orgAuthz.AssertNotCalled(t, "CanInTeam", mock.Anything, mock.Anything)
		})
	}
}

func TestResolve_AnOrganisationBoardIsClosedToEveryoneOutsideTheOrganisation(t *testing.T) {
	tests := []struct {
		name   string
		member bool
		noRow  bool
		link   model.LinkAccess
		want   error
	}{
		{name: "an outsider with no row", noRow: true, link: model.LinkAccessOff, want: ErrNoAccess},
		{name: "a former member whose row came from joining as a member", member: true, link: model.LinkAccessAccounts, want: ErrNoAccess},
		{name: "a link participant once the link is off", link: model.LinkAccessOff, want: ErrLinkDisabled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := organisationHeader()
			header.LinkAccess = tc.link
			principal := user()

			var row *model.BoardParticipant
			if !tc.noRow {
				row = linkRow(header, principal, model.BoardRoleParticipant)
			}
			if tc.member {
				row = storedRow(header, principal, model.BoardRoleFacilitator)
			}
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, notInOrg)
			m.participant(header, principal, row)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, access)
		})
	}
}

func TestResolve_AnOrganisationBoardStillAdmitsAnOutsiderThroughTheLink(t *testing.T) {
	// given a stored facilitator promotion that only the link still backs
	svc, m := newTestLive(t)
	header := organisationHeader()
	header.LinkAccess = model.LinkAccessAccounts
	principal := user()
	row := linkRow(header, principal, model.BoardRoleFacilitator)
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.standing(header, principal, notInOrg)
	m.participant(header, principal, row)

	// when
	access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.BoardRoleParticipant, access.Role)
	assert.False(t, access.OrgMember)
	assert.Equal(t, policy.Capabilities(model.BoardRoleParticipant, false), access.Capabilities)
}

func TestResolve_AnOrganisationBoardReportsAFailedMembershipRead(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *liveMocks, header model.BoardHeader, principal model.Principal)
	}{
		{name: "the organisation membership", given: func(m *liveMocks, header model.BoardHeader, principal model.Principal) {
			m.orgAuthz.EXPECT().CanInOrg(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabaseDown)
		}},
		{name: "the owner or admin check", given: func(m *liveMocks, header model.BoardHeader, principal model.Principal) {
			view := spec.OrgPermissionCheck{OrgID: header.OrgID, UserID: principal.ID, Permission: orgauthz.PermOrgView}
			m.orgAuthz.EXPECT().CanInOrg(mock.Anything, view).Return(orgauthz.Decision{Member: true, Allowed: true}, nil)
			m.orgAuthz.EXPECT().CanInOrg(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabaseDown)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := organisationHeader()
			principal := user()
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			tc.given(m, header, principal)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.ErrorIs(t, err, errDatabaseDown)
			require.NotErrorIs(t, err, ErrNoAccess)
			assert.Nil(t, access)
		})
	}
}

func TestResolve_CapsLinkOnlyAccessAtParticipant(t *testing.T) {
	for _, standing := range []teamStanding{notInOrg, offTeam} {
		t.Run(fmt.Sprintf("organisation member %t", standing.orgMember), func(t *testing.T) {
			// given a stored facilitator promotion that only the link still backs
			svc, m := newTestLive(t)
			header := liveHeader()
			header.LinkAccess = model.LinkAccessAccounts
			principal := user()
			row := linkRow(header, principal, model.BoardRoleFacilitator)
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, standing)
			m.participant(header, principal, row)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.NoError(t, err)
			assert.Equal(t, model.BoardRoleParticipant, access.Role)
			assert.Equal(t, standing.orgMember, access.OrgMember)
			require.NotNil(t, access.ParticipantID)
			assert.Equal(t, row.ID, *access.ParticipantID)
			assert.Equal(t, policy.Capabilities(model.BoardRoleParticipant, standing.orgMember), access.Capabilities)
		})
	}
}

func TestResolve_AdmitsAUserThroughEitherLinkSetting(t *testing.T) {
	for _, link := range []model.LinkAccess{model.LinkAccessGuests, model.LinkAccessAccounts} {
		t.Run(string(link), func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := liveHeader()
			header.LinkAccess = link
			principal := user()
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, notInOrg)
			m.participant(header, principal, linkRow(header, principal, model.BoardRoleParticipant))

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.NoError(t, err)
			assert.Equal(t, model.BoardRoleParticipant, access.Role)
		})
	}
}

func TestResolve_RefusesAPrincipalWithNoPathIn(t *testing.T) {
	tests := []struct {
		name     string
		standing teamStanding
		edit     func(header *model.BoardHeader, row *model.BoardParticipant)
		noRow    bool
		member   bool
		want     error
	}{
		{name: "an outsider with no row", standing: notInOrg, noRow: true, want: ErrNoAccess},
		{name: "an organisation member off the team with no row", standing: offTeam, noRow: true, want: ErrNoAccess},
		{name: "a former organisation member whose row came from joining as a member", standing: notInOrg, member: true, want: ErrNoAccess},
		{name: "a former team member whose row came from joining as a member", standing: offTeam, member: true, want: ErrNoAccess},
		{name: "a former member whose member row matches the epoch once the link is on", standing: notInOrg, member: true, edit: func(header *model.BoardHeader, _ *model.BoardParticipant) {
			header.LinkAccess = model.LinkAccessAccounts
		}, want: ErrNoAccess},
		{name: "a former member whose member row matches the epoch of a guest link", standing: offTeam, member: true, edit: func(header *model.BoardHeader, _ *model.BoardParticipant) {
			header.LinkAccess = model.LinkAccessGuests
		}, want: ErrNoAccess},
		{name: "a link participant once the link is off", standing: notInOrg, want: ErrLinkDisabled},
		{name: "a link participant after the password changed", standing: notInOrg, edit: func(header *model.BoardHeader, row *model.BoardParticipant) {
			header.LinkAccess = model.LinkAccessGuests
			header.PasswordHash = new("hash")
			row.AccessEpoch = header.AccessEpoch - 1
		}, want: ErrPasswordRequired},
		{name: "a link participant after the link was rotated", standing: offTeam, edit: func(header *model.BoardHeader, row *model.BoardParticipant) {
			header.LinkAccess = model.LinkAccessAccounts
			row.AccessEpoch = header.AccessEpoch - 1
		}, want: ErrNoAccess},
		{name: "a removed link participant", standing: notInOrg, edit: func(header *model.BoardHeader, row *model.BoardParticipant) {
			header.LinkAccess = model.LinkAccessGuests
			row.RemovedAt = new(time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC))
		}, want: ErrRemoved},
		{name: "a removed team member, whom membership would admit", standing: teamMember, edit: func(_ *model.BoardHeader, row *model.BoardParticipant) {
			row.RemovedAt = new(time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC))
		}, want: ErrRemoved},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := liveHeader()
			principal := user()

			var row *model.BoardParticipant
			if !tc.noRow {
				row = linkRow(header, principal, model.BoardRoleParticipant)
			}
			if tc.member {
				row = storedRow(header, principal, model.BoardRoleParticipant)
			}
			if tc.edit != nil {
				tc.edit(&header, row)
			}
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, tc.standing)
			m.participant(header, principal, row)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, access)
		})
	}
}

func TestResolve_ClosesAMissingOrDeletedBoardToEveryone(t *testing.T) {
	deleted := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		edit func(header *model.BoardHeader)
	}{
		{name: "a deleted board", edit: func(header *model.BoardHeader) {
			header.DeletedAt = new(deleted)
		}},
		{name: "a board of a deleted organisation", edit: func(header *model.BoardHeader) {
			header.OrgDeletedAt = new(deleted)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := liveHeader()
			tc.edit(&header)
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: user(), BoardID: header.ID})

			// then
			require.ErrorIs(t, err, ErrNoAccess)
			assert.Nil(t, access)
		})
	}

	t.Run("a missing board", func(t *testing.T) {
		// given
		svc, m := newTestLive(t)
		missingID := uuid.New()
		m.boards.EXPECT().GetHeader(mock.Anything, missingID).Return(nil, errNoRow)

		// when
		access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: user(), BoardID: missingID})

		// then
		require.ErrorIs(t, err, ErrNoAccess)
		require.ErrorIs(t, err, dao.ErrNotFound)
		assert.Nil(t, access)
	})
}

func TestResolve_RefusesAPrincipalThatIsNotAUser(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	header := liveHeader()
	header.LinkAccess = model.LinkAccessGuests
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)

	// when
	access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: model.Principal{ID: uuid.New()}, BoardID: header.ID})

	// then
	require.ErrorIs(t, err, ErrNoAccess)
	assert.Nil(t, access)
}

func TestResolve_ReportsAFailedRead(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *liveMocks, header model.BoardHeader, principal model.Principal)
	}{
		{name: "the header", given: func(m *liveMocks, header model.BoardHeader, _ model.Principal) {
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(nil, errDatabaseDown)
		}},
		{name: "the team membership", given: func(m *liveMocks, header model.BoardHeader, principal model.Principal) {
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.orgAuthz.EXPECT().CanInTeam(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabaseDown)
		}},
		{name: "the team lead check", given: func(m *liveMocks, header model.BoardHeader, principal model.Principal) {
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			view := spec.TeamPermissionCheck{TeamID: *header.TeamID, UserID: principal.ID, Permission: orgauthz.PermTeamView}
			m.orgAuthz.EXPECT().CanInTeam(mock.Anything, view).Return(orgauthz.Decision{Member: true, Allowed: true}, nil)
			m.orgAuthz.EXPECT().CanInTeam(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabaseDown)
		}},
		{name: "the participant row", given: func(m *liveMocks, header model.BoardHeader, principal model.Principal) {
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, teamLead)
			m.boards.EXPECT().GetParticipantForUser(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := liveHeader()
			principal := user()
			tc.given(m, header, principal)

			// when
			access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.ErrorIs(t, err, errDatabaseDown)
			require.NotErrorIs(t, err, ErrNoAccess)
			assert.Nil(t, access)
		})
	}
}

func TestResolve_CarriesTheSettingsAndNarrowsAClosedAnonymousBoard(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	header := liveHeader()
	header.ClosedAt = new(time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))
	header.Anonymity = model.AnonymityAnonymous
	header.CardsHidden = true
	principal := user()
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.standing(header, principal, teamLead)
	m.participant(header, principal, nil)

	// when
	access, err := svc.Resolve(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

	// then
	require.NoError(t, err)
	assert.Equal(t, header, access.Header)
	assert.True(t, access.Header.Anonymous())
	assert.True(t, access.Header.CardsHidden)
	assert.Equal(t, model.BoardRoleFacilitator, access.Role)
	assert.NotContains(t, access.Capabilities, model.BoardCapabilityFacilitate)
	assert.NotContains(t, access.Capabilities, model.BoardCapabilityParticipate)
}

func TestNarrow_LeavesOnlyWhatTheBoardsStateAllows(t *testing.T) {
	at := new(time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))
	tests := []struct {
		name      string
		edit      func(header *model.BoardHeader)
		anonymous bool
		want      []model.BoardCapability
	}{
		{name: "a live board", edit: func(*model.BoardHeader) {}, want: allBoardCap},
		{name: "a live anonymous board", edit: func(*model.BoardHeader) {}, anonymous: true, want: allBoardCap},
		{name: "a locked board keeps facilitating", edit: func(header *model.BoardHeader) {
			header.LockedAt = at
		}, want: []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityFacilitate, model.BoardCapabilityManage, model.BoardCapabilityExport}},
		{name: "a closed named board may be reopened", edit: func(header *model.BoardHeader) {
			header.ClosedAt = at
		}, want: []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityFacilitate, model.BoardCapabilityManage, model.BoardCapabilityExport}},
		{name: "a closed anonymous board stays closed", edit: func(header *model.BoardHeader) {
			header.ClosedAt = at
		}, anonymous: true, want: []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityManage, model.BoardCapabilityExport}},
		{name: "an archived board may only be managed", edit: func(header *model.BoardHeader) {
			header.ArchivedAt = at
		}, want: []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityManage, model.BoardCapabilityExport}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			header := liveHeader()
			tc.edit(&header)
			if tc.anonymous {
				header.Anonymity = model.AnonymityAnonymous
			}
			full := slices.Clone(allBoardCap)

			// when
			got := narrow(full, header)

			// then
			assert.Equal(t, tc.want, got)
			assert.Equal(t, allBoardCap, full)
		})
	}
}

func TestJoinAsMember_AddsAParticipantRowUnderTheGate(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	header := liveHeader()
	principal := user()
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.standing(header, principal, teamLead)
	m.participant(header, principal, nil)

	joined := storedRow(header, principal, model.BoardRoleParticipant)
	want := spec.BoardParticipantJoin{
		OrgID: header.OrgID, BoardID: header.ID, Version: header.Version,
		UserID: principal.ID,
		Role:   model.BoardRoleParticipant,
	}
	m.boards.EXPECT().JoinParticipant(mock.Anything, want).Return(&model.Versioned[model.BoardParticipant]{Value: *joined, Version: header.Version + 1}, nil)

	// when
	access, err := svc.JoinAsMember(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

	// then
	require.NoError(t, err)
	require.NotNil(t, access.ParticipantID)
	assert.Equal(t, joined.ID, *access.ParticipantID)
	assert.Equal(t, header.Version+1, access.Header.Version)
	assert.Equal(t, model.BoardRoleFacilitator, access.Role)
	assert.Empty(t, svc.(*service).gates.byID)
}

func TestJoinAsMember_IsIdempotent(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	header := liveHeader()
	principal := user()
	row := storedRow(header, principal, model.BoardRoleParticipant)
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.standing(header, principal, teamMember)
	m.participant(header, principal, row)

	// when
	access, err := svc.JoinAsMember(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

	// then
	require.NoError(t, err)
	require.NotNil(t, access.ParticipantID)
	assert.Equal(t, row.ID, *access.ParticipantID)
	assert.Equal(t, header.Version, access.Header.Version)
}

func TestJoinAsMember_RefusesWithoutWriting(t *testing.T) {
	tests := []struct {
		name     string
		standing teamStanding
		edit     func(header *model.BoardHeader)
		want     error
	}{
		{name: "an organisation member off the team", standing: offTeam, edit: func(*model.BoardHeader) {}, want: ErrNoAccess},
		{name: "an archived board", standing: teamMember, edit: func(header *model.BoardHeader) {
			header.ArchivedAt = new(time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))
		}, want: ErrArchived},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestLive(t)
			header := liveHeader()
			tc.edit(&header)
			principal := user()
			m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
			m.standing(header, principal, tc.standing)
			m.participant(header, principal, nil)

			// when
			access, err := svc.JoinAsMember(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, access)
			assert.Empty(t, svc.(*service).gates.byID)
		})
	}
}

func TestJoinAsMember_ReportsAFailedWrite(t *testing.T) {
	// given
	svc, m := newTestLive(t)
	header := liveHeader()
	principal := user()
	m.boards.EXPECT().GetHeader(mock.Anything, header.ID).Return(&header, nil)
	m.standing(header, principal, teamMember)
	m.participant(header, principal, nil)
	m.boards.EXPECT().JoinParticipant(mock.Anything, mock.Anything).Return(nil, dao.ErrVersionConflict)

	// when
	access, err := svc.JoinAsMember(t.Context(), spec.BoardAccessQuery{Principal: principal, BoardID: header.ID})

	// then
	require.ErrorIs(t, err, dao.ErrVersionConflict)
	assert.Nil(t, access)
}
