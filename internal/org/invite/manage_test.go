package invite

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/token"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (m *testMocks) createsInvite(err error) *[]spec.NewOrganisationInvite {
	created := &[]spec.NewOrganisationInvite{}

	m.orgs.EXPECT().CreateInvite(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewOrganisationInvite, _ ...*sql.Tx) (*model.OrganisationInvite, error) {
		*created = append(*created, s)
		if err != nil {
			return nil, err
		}

		return storedFrom(s), nil
	})

	return created
}

func (m *testMocks) live(orgID uuid.UUID) {
	found := &model.Organisation{ID: orgID, Name: testOrgName, Slug: testOrgSlug}

	m.orgs.EXPECT().GetByID(mock.Anything, orgID).Return(found, nil)
}

func (m *testMocks) composesLetter(inviterID uuid.UUID) {
	inviter := &model.User{ID: inviterID, DisplayName: testInviter}

	m.users.EXPECT().GetByID(mock.Anything, inviterID).Return(inviter, nil)
}

func (m *testMocks) invitable(address string) {
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.users.EXPECT().GetByEmail(mock.Anything, address).Return(nil, errMissing)
}

func (m *testMocks) nothingPending(orgID uuid.UUID, listed ...model.OrganisationInvite) {
	if listed == nil {
		listed = []model.OrganisationInvite{}
	}

	m.orgs.EXPECT().ListInvites(mock.Anything, orgID).Return(listed, nil)
}

func (m *testMocks) grants(orgID uuid.UUID, actorID uuid.UUID, allowed bool) {
	check := spec.OrgPermissionCheck{OrgID: orgID, UserID: actorID, Permission: orgauthz.PermOrgManageAdmins}

	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(orgauthz.Decision{Member: true, Allowed: allowed}, nil)
}

func storedFrom(s spec.NewOrganisationInvite) *model.OrganisationInvite {
	return &model.OrganisationInvite{
		ID:        uuid.New(),
		OrgID:     s.OrgID,
		TeamID:    s.TeamID,
		TokenHash: s.TokenHash,
		Email:     s.Email,
		Role:      s.Role,
		MaxUses:   s.MaxUses,
		InvitedBy: new(s.InvitedBy),
		ExpiresAt: s.ExpiresAt,
		CreatedAt: time.Now(),
	}
}

func emailInvite(orgID uuid.UUID) *model.OrganisationInvite {
	return &model.OrganisationInvite{
		ID:        uuid.New(),
		OrgID:     orgID,
		TokenHash: token.Hash("an-earlier-token"),
		Email:     new(testAddress),
		Role:      model.OrgRoleMember,
		MaxUses:   new(1),
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now().Add(-time.Hour),
	}
}

func linkCreation(orgID uuid.UUID, actorID uuid.UUID) spec.InviteCreation {
	return spec.InviteCreation{OrgID: orgID, ActorID: actorID, IP: testIP}
}

func emailCreation(orgID uuid.UUID, actorID uuid.UUID) spec.InviteCreation {
	return spec.InviteCreation{OrgID: orgID, ActorID: actorID, Email: testAddress, Role: model.OrgRoleMember, IP: testIP}
}

func inviteAction(orgID uuid.UUID, actorID uuid.UUID, inviteID uuid.UUID) spec.InviteAction {
	return spec.InviteAction{OrgID: orgID, InviteID: inviteID, ActorID: actorID, IP: testIP}
}

func TestManage_ADeletedOrganisationTakesNoInviteWork(t *testing.T) {
	operations := []struct {
		name string
		run  func(t *testing.T, svc *service, orgID uuid.UUID) error
	}{
		{name: "listing", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			_, err := svc.List(t.Context(), orgID)
			return err
		}},
		{name: "inviting by email", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			_, err := svc.Create(t.Context(), emailCreation(orgID, uuid.New()))
			return err
		}},
		{name: "resending", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			_, err := svc.Resend(t.Context(), inviteAction(orgID, uuid.New(), uuid.New()))
			return err
		}},
		{name: "revoking", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			return svc.Revoke(t.Context(), inviteAction(orgID, uuid.New(), uuid.New()))
		}},
	}

	states := []struct {
		name    string
		deleted bool
		err     error
		want    error
	}{
		{name: "soft-deleted", deleted: true, want: ErrOrganisationNotFound},
		{name: "purged", err: errMissing, want: ErrOrganisationNotFound},
		{name: "unreadable", err: errDatabaseDown, want: errDatabaseDown},
	}

	for _, operation := range operations {
		for _, state := range states {
			t.Run(operation.name+" in an organisation that is "+state.name, func(t *testing.T) {
				// given
				svc, m := newTestService(t)
				orgID := uuid.New()

				var found *model.Organisation
				if state.deleted {
					found = &model.Organisation{ID: orgID, Name: testOrgName, DeletedAt: new(time.Now())}
				}

				m.orgs.EXPECT().GetByID(mock.Anything, orgID).Return(found, state.err)

				// when
				err := operation.run(t, svc, orgID)

				// then the strict mocks prove nothing past the organisation was read, written or sent
				assert.ErrorIs(t, err, state.want)
			})
		}
	}
}

func TestList_MarksTheExpiredInvites(t *testing.T) {
	// given one invite still live and one past its expiry
	svc, m := newTestService(t)
	orgID := uuid.New()
	live := emailInvite(orgID)
	lapsed := emailInvite(orgID)
	lapsed.ExpiresAt = time.Now().Add(-time.Minute)
	m.live(orgID)
	m.orgs.EXPECT().ListInvites(mock.Anything, orgID).Return([]model.OrganisationInvite{*live, *lapsed}, nil)

	// when
	listed, err := svc.List(t.Context(), orgID)

	// then an expired invite stays listed until the sweep deletes it
	require.NoError(t, err)
	require.Len(t, listed.Invites, 2)
	assert.Equal(t, live.ID, listed.Invites[0].ID)
	assert.Equal(t, testAddress, listed.Invites[0].Email)
	assert.False(t, listed.Invites[0].Expired)
	assert.True(t, listed.Invites[1].Expired)
}

func TestList_AnOrganisationWithoutInvitesListsNone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.live(orgID)
	m.orgs.EXPECT().ListInvites(mock.Anything, orgID).Return(nil, nil)

	// when
	listed, err := svc.List(t.Context(), orgID)

	// then the list is empty, never null
	require.NoError(t, err)
	assert.Equal(t, &dto.OrganisationInviteListResponse{Invites: []dto.OrganisationInviteResponse{}}, listed)
}

func TestList_ReturnsAReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.live(orgID)
	m.orgs.EXPECT().ListInvites(mock.Anything, orgID).Return(nil, errDatabaseDown)

	// when
	listed, err := svc.List(t.Context(), orgID)

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listed)
}

func TestCreate_ALinkInviteAnswersWithItsLinkOnce(t *testing.T) {
	// given a member invite by link, with no use limit
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.readsSite()
	created := m.createsInvite(nil)
	recorded := m.audits(nil)

	// when
	response, err := svc.Create(t.Context(), linkCreation(orgID, actorID))

	// then only the hash of the token in the link is stored, and no email is read or sent
	require.NoError(t, err)
	require.Len(t, *created, 1)
	stored := (*created)[0]
	drawn := strings.TrimPrefix(response.URL, testBaseURL+"/invite#")
	assert.Regexp(t, inviteLink, response.URL)
	assert.Equal(t, token.Hash(drawn), stored.TokenHash)
	assert.NotContains(t, stored.TokenHash, drawn)
	assert.Equal(t, model.OrgRoleMember, stored.Role)
	assert.Nil(t, stored.Email)
	assert.Nil(t, stored.MaxUses)
	assert.Nil(t, stored.TeamID)
	assert.Equal(t, actorID, stored.InvitedBy)
	assert.WithinDuration(t, time.Now().Add(inviteTTL), stored.ExpiresAt, time.Minute)
	assert.Empty(t, response.Email)
	assert.Equal(t, model.OrgRoleMember, response.Role)
	require.Len(t, *recorded, 1)
	assert.Equal(t, audit.NewEntry{
		OrgID:      orgID,
		ActorID:    actorID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionMemberInvited,
		TargetType: audit.TargetInvite,
		TargetID:   response.ID.String(),
		Details:    map[string]string{"role": "member"},
		IP:         testIP,
	}, (*recorded)[0])
}

func TestCreate_ALinkInviteKeepsItsUseLimitAndTeam(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID, teamID := uuid.New(), uuid.New(), uuid.New()
	m.live(orgID)
	m.teams.EXPECT().GetByID(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return(&model.Team{ID: teamID, OrgID: orgID, Name: "Platform"}, nil)
	m.readsSite()
	created := m.createsInvite(nil)
	recorded := m.audits(nil)

	creation := linkCreation(orgID, actorID)
	creation.TeamID = new(teamID)
	creation.MaxUses = new(5)

	// when
	response, err := svc.Create(t.Context(), creation)

	// then
	require.NoError(t, err)
	stored := (*created)[0]
	assert.Equal(t, new(teamID), stored.TeamID)
	assert.Equal(t, new(5), stored.MaxUses)
	assert.Equal(t, new(teamID), response.TeamID)
	assert.Equal(t, map[string]string{"role": "member", "team_id": teamID.String(), "max_uses": "5"}, (*recorded)[0].Details)
}

func TestCreate_AnEmailInviteSendsItsLinkToTheAddress(t *testing.T) {
	// given an address typed with spaces around it
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.invitable(testAddress)
	m.nothingPending(orgID)
	m.readsSite()
	m.composesLetter(actorID)
	created := m.createsInvite(nil)
	recorded := m.audits(nil)
	sent := m.sends(nil)

	creation := emailCreation(orgID, actorID)
	creation.Email = "  " + testAddress + " "

	// when
	response, err := svc.Create(t.Context(), creation)

	// then the email carries the only copy of the token, and the answer carries no link
	require.NoError(t, err)
	stored := (*created)[0]
	assert.Equal(t, new(testAddress), stored.Email)
	assert.Equal(t, new(emailInviteUses), stored.MaxUses)
	require.Len(t, *sent, 1)
	letter := (*sent)[0]
	assert.Equal(t, testAddress, letter.To)
	assert.Equal(t, "You are invited to join Acme Retros on RetroTide", letter.Subject)
	assert.Contains(t, letter.HTML, "<strong><bdi>"+testInviter+"</bdi></strong> invited you")
	assert.Equal(t, token.Hash(linkedToken(t, inviteLink, letter)), stored.TokenHash)
	assert.Empty(t, response.URL)
	assert.Equal(t, testAddress, response.Email)
	assert.Equal(t, map[string]string{"role": "member", "email": testAddress, "max_uses": "1"}, (*recorded)[0].Details)
}

func TestCreate_AnEmailInviteToAnAccountThatIsNotAMember(t *testing.T) {
	// given the address already has an account on the site
	svc, m := newTestService(t)
	orgID, actorID, accountID := uuid.New(), uuid.New(), uuid.New()
	m.live(orgID)
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.users.EXPECT().GetByEmail(mock.Anything, testAddress).Return(&model.User{ID: accountID, Email: testAddress}, nil)
	m.notAMember(orgID, accountID)
	m.nothingPending(orgID)
	m.readsSite()
	m.composesLetter(actorID)
	created := m.createsInvite(nil)
	m.audits(nil)
	m.sends(nil)

	// when
	_, err := svc.Create(t.Context(), emailCreation(orgID, actorID))

	// then it is invited like any other address
	require.NoError(t, err)
	assert.Len(t, *created, 1)
}

func TestCreate_AnExpiredInviteForTheAddressDoesNotBlockANewOne(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	lapsed := emailInvite(orgID)
	lapsed.ExpiresAt = time.Now().Add(-time.Minute)
	other := emailInvite(orgID)
	other.Email = new("dave@example.com")
	m.live(orgID)
	m.invitable(testAddress)
	m.nothingPending(orgID, *lapsed, *other)
	m.readsSite()
	m.composesLetter(actorID)
	created := m.createsInvite(nil)
	m.audits(nil)
	m.sends(nil)

	// when
	_, err := svc.Create(t.Context(), emailCreation(orgID, actorID))

	// then
	require.NoError(t, err)
	assert.Len(t, *created, 1)
}

func TestCreate_RefusesAMalformedInviteBeforeAnyLookup(t *testing.T) {
	cases := []struct {
		name  string
		shape func(creation *spec.InviteCreation)
		want  error
	}{
		{name: "an owner invite", shape: func(creation *spec.InviteCreation) { creation.Role = model.OrgRoleOwner }, want: ErrInvalidRole},
		{name: "an unknown role", shape: func(creation *spec.InviteCreation) { creation.Role = "guest" }, want: ErrInvalidRole},
		{name: "a malformed address", shape: func(creation *spec.InviteCreation) { creation.Email = "not an address" }, want: ErrInvalidEmail},
		{name: "an address of spaces", shape: func(creation *spec.InviteCreation) { creation.Email = "   " }, want: ErrInvalidEmail},
		{name: "an address with a display name", shape: func(creation *spec.InviteCreation) { creation.Email = "Carol <carol@example.com>" }, want: ErrInvalidEmail},
		{name: "an email invite that works twice", shape: func(creation *spec.InviteCreation) { creation.Email, creation.MaxUses = testAddress, new(2) }, want: ErrInvalidMaxUses},
		{name: "a link that never works", shape: func(creation *spec.InviteCreation) { creation.MaxUses = new(0) }, want: ErrInvalidMaxUses},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given the strict mocks, which fail on any lookup or write
			svc, _ := newTestService(t)
			creation := linkCreation(uuid.New(), uuid.New())
			tc.shape(&creation)

			// when
			response, err := svc.Create(t.Context(), creation)

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, response)
		})
	}
}

func TestCreate_AnAdminInviteFromSomeoneWhoMakesAdmins(t *testing.T) {
	// given an owner
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.grants(orgID, actorID, true)
	m.readsSite()
	created := m.createsInvite(nil)
	m.audits(nil)

	creation := linkCreation(orgID, actorID)
	creation.Role = model.OrgRoleAdmin

	// when
	_, err := svc.Create(t.Context(), creation)

	// then
	require.NoError(t, err)
	assert.Equal(t, model.OrgRoleAdmin, (*created)[0].Role)
}

func TestCreate_AnAdminInviteFromSomeoneWhoCannotMakeAdminsIsRefused(t *testing.T) {
	// given an admin, who manages members but never makes another admin
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.grants(orgID, actorID, false)

	creation := emailCreation(orgID, actorID)
	creation.Role = model.OrgRoleAdmin

	// when
	response, err := svc.Create(t.Context(), creation)

	// then nothing past the organisation is looked up, written or sent
	assert.ErrorIs(t, err, ErrAdminInviteForbidden)
	assert.Nil(t, response)
}

func TestCreate_AnAdminInviteWhosePermissionCannotBeCheckedIsRefused(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.live(orgID)
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabaseDown)

	creation := linkCreation(orgID, uuid.New())
	creation.Role = model.OrgRoleAdmin

	// when
	response, err := svc.Create(t.Context(), creation)

	// then a check that fails is never a pass
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, response)
}

func TestCreate_ATeamInviteNeedsALiveTeamOfTheOrganisation(t *testing.T) {
	cases := []struct {
		name string
		team *model.Team
		err  error
		want error
	}{
		{name: "a team of another organisation, or none", err: errMissing, want: ErrTeamNotFound},
		{name: "an archived team", team: &model.Team{Name: "Old", ArchivedAt: new(time.Now())}, want: ErrTeamArchived},
		{name: "a lookup that fails", err: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, teamID := uuid.New(), uuid.New()
			m.live(orgID)
			m.teams.EXPECT().GetByID(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return(tc.team, tc.err)

			creation := linkCreation(orgID, uuid.New())
			creation.TeamID = new(teamID)

			// when
			response, err := svc.Create(t.Context(), creation)

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, response)
		})
	}
}

func TestCreate_AnEmailInviteIsRefusedBeforeWriting(t *testing.T) {
	cases := []struct {
		name  string
		given func(m *testMocks, orgID uuid.UUID)
		want  error
	}{
		{
			name: "no email provider",
			given: func(m *testMocks, _ uuid.UUID) {
				m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)
			},
			want: ErrEmailUnavailable,
		},
		{
			name: "an email provider that cannot be checked",
			given: func(m *testMocks, _ uuid.UUID) {
				m.email.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)
			},
			want: errDatabaseDown,
		},
		{
			name: "an address that is already a member",
			given: func(m *testMocks, orgID uuid.UUID) {
				accountID := uuid.New()
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().GetByEmail(mock.Anything, testAddress).Return(&model.User{ID: accountID}, nil)
				m.orgs.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: accountID}).Return(&model.OrganisationMember{OrgID: orgID, UserID: accountID}, nil)
			},
			want: ErrAlreadyMember,
		},
		{
			name: "an account lookup that fails",
			given: func(m *testMocks, _ uuid.UUID) {
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().GetByEmail(mock.Anything, testAddress).Return(nil, errDatabaseDown)
			},
			want: errDatabaseDown,
		},
		{
			name: "a membership check that fails",
			given: func(m *testMocks, _ uuid.UUID) {
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().GetByEmail(mock.Anything, testAddress).Return(&model.User{ID: uuid.New()}, nil)
				m.orgs.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
			},
			want: errDatabaseDown,
		},
		{
			name: "a pending invite for the address in another case",
			given: func(m *testMocks, orgID uuid.UUID) {
				pending := emailInvite(orgID)
				pending.Email = new("Carol@Example.com")
				m.invitable(testAddress)
				m.nothingPending(orgID, *pending)
			},
			want: ErrInvitePending,
		},
		{
			name: "a pending list that cannot be read",
			given: func(m *testMocks, orgID uuid.UUID) {
				m.invitable(testAddress)
				m.orgs.EXPECT().ListInvites(mock.Anything, orgID).Return(nil, errDatabaseDown)
			},
			want: errDatabaseDown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			m.live(orgID)
			tc.given(m, orgID)

			// when
			response, err := svc.Create(t.Context(), emailCreation(orgID, uuid.New()))

			// then a check that fails is never a pass, and nothing is written
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, response)
		})
	}
}

func TestCreate_TheSiteIsReadBeforeWriting(t *testing.T) {
	cases := []struct {
		name  string
		given func(m *testMocks)
	}{
		{
			name: "a base url that cannot be read",
			given: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)
			},
		},
		{
			name: "a site name that cannot be read",
			given: func(m *testMocks) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)
				m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return("", errDatabaseDown)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			m.live(orgID)
			tc.given(m)

			// when
			response, err := svc.Create(t.Context(), linkCreation(orgID, uuid.New()))

			// then no invite exists whose link could not be given
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.Nil(t, response)
		})
	}
}

func TestCreate_TheEmailIsComposedBeforeWriting(t *testing.T) {
	// given an inviter who cannot be read
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.invitable(testAddress)
	m.nothingPending(orgID)
	m.readsSite()
	m.users.EXPECT().GetByID(mock.Anything, actorID).Return(nil, errDatabaseDown)

	// when
	response, err := svc.Create(t.Context(), emailCreation(orgID, actorID))

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, response)
}

func TestCreate_ATeamDeletedMeanwhileIsNotFound(t *testing.T) {
	// given the team is deleted between the check and the insert
	svc, m := newTestService(t)
	orgID, teamID := uuid.New(), uuid.New()
	m.live(orgID)
	m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(&model.Team{ID: teamID, OrgID: orgID, Name: "Platform"}, nil)
	m.readsSite()
	m.createsInvite(errMissing)

	creation := linkCreation(orgID, uuid.New())
	creation.TeamID = new(teamID)

	// when
	response, err := svc.Create(t.Context(), creation)

	// then
	assert.ErrorIs(t, err, ErrTeamNotFound)
	assert.Nil(t, response)
}

func TestCreate_AFailedWriteIsReturned(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.live(orgID)
	m.readsSite()
	m.createsInvite(errDatabaseDown)

	// when
	response, err := svc.Create(t.Context(), linkCreation(orgID, uuid.New()))

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrTeamNotFound)
	assert.Nil(t, response)
}

func TestCreate_AnEmailThatFailsAfterTheWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.invitable(testAddress)
	m.nothingPending(orgID)
	m.readsSite()
	m.composesLetter(actorID)
	m.createsInvite(nil)
	recorded := m.audits(nil)
	m.sends(errDatabaseDown)

	// when
	response, err := svc.Create(t.Context(), emailCreation(orgID, actorID))

	// then the invite is saved and audited, and the caller is told to resend it
	assert.ErrorIs(t, err, ErrInviteNotSent)
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrNotAudited)
	require.NotNil(t, response)
	assert.Equal(t, testAddress, response.Email)
	assert.Len(t, *recorded, 1)
}

func TestCreate_ALostAuditEntryStillAnswersWithTheInvite(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.live(orgID)
	m.readsSite()
	m.createsInvite(nil)
	m.audits(errDatabaseDown)

	// when
	response, err := svc.Create(t.Context(), linkCreation(orgID, uuid.New()))

	// then
	assert.ErrorIs(t, err, ErrNotAudited)
	assert.ErrorIs(t, err, errDatabaseDown)
	require.NotNil(t, response)
	assert.Regexp(t, inviteLink, response.URL)
}

func TestCreate_AnEmailThatFailsAfterALostAuditEntryReportsBoth(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.invitable(testAddress)
	m.nothingPending(orgID)
	m.readsSite()
	m.composesLetter(actorID)
	m.createsInvite(nil)
	m.audits(errDatabaseDown)
	m.sends(errDatabaseDown)

	// when
	response, err := svc.Create(t.Context(), emailCreation(orgID, actorID))

	// then the unsent email is what the caller must act on
	assert.ErrorIs(t, err, ErrInviteNotSent)
	assert.ErrorIs(t, err, ErrNotAudited)
	assert.NotNil(t, response)
}

func TestResend_ReplacesTheInviteWithAFreshLinkAndSendsIt(t *testing.T) {
	// given an email invite past its expiry
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	previous := emailInvite(orgID)
	previous.ExpiresAt = time.Now().Add(-time.Hour)
	key := spec.OrganisationInviteKey{OrgID: orgID, InviteID: previous.ID}
	m.live(orgID)
	m.orgs.EXPECT().GetInvite(mock.Anything, key).Return(previous, nil)
	m.invitable(testAddress)
	m.readsSite()
	m.composesLetter(actorID)
	m.orgs.EXPECT().RevokeInvite(mock.Anything, key).Return(previous, nil)
	created := m.createsInvite(nil)
	recorded := m.audits(nil)
	sent := m.sends(nil)

	// when
	response, err := svc.Resend(t.Context(), inviteAction(orgID, actorID, previous.ID))

	// then the earlier link stops working, and only the new token reaches the address
	require.NoError(t, err)
	require.Len(t, *created, 1)
	replacement := (*created)[0]
	assert.Equal(t, previous.Email, replacement.Email)
	assert.Equal(t, previous.Role, replacement.Role)
	assert.Nil(t, replacement.TeamID)
	assert.Equal(t, new(emailInviteUses), replacement.MaxUses)
	assert.Equal(t, actorID, replacement.InvitedBy)
	assert.NotEqual(t, previous.TokenHash, replacement.TokenHash)
	assert.WithinDuration(t, time.Now().Add(inviteTTL), replacement.ExpiresAt, time.Minute)
	require.Len(t, *sent, 1)
	assert.Equal(t, testAddress, (*sent)[0].To)
	assert.Equal(t, token.Hash(linkedToken(t, inviteLink, (*sent)[0])), replacement.TokenHash)
	assert.NotEqual(t, previous.ID, response.ID)
	assert.False(t, response.Expired)
	require.Len(t, *recorded, 1)
	assert.Equal(t, audit.ActionMemberInvited, (*recorded)[0].Action)
	assert.Equal(t, response.ID.String(), (*recorded)[0].TargetID)
	assert.Equal(t, map[string]string{"role": "member", "email": testAddress, "max_uses": "1", "replaces": previous.ID.String()}, (*recorded)[0].Details)
}

func TestResend_KeepsTheTeamOfTheInvite(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID, teamID := uuid.New(), uuid.New(), uuid.New()
	previous := emailInvite(orgID)
	previous.TeamID = new(teamID)
	m.live(orgID)
	m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(previous, nil)
	m.teams.EXPECT().GetByID(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return(&model.Team{ID: teamID, OrgID: orgID, Name: "Platform"}, nil)
	m.invitable(testAddress)
	m.readsSite()
	m.composesLetter(actorID)
	m.orgs.EXPECT().RevokeInvite(mock.Anything, mock.Anything).Return(previous, nil)
	created := m.createsInvite(nil)
	m.audits(nil)
	sent := m.sends(nil)

	// when
	_, err := svc.Resend(t.Context(), inviteAction(orgID, actorID, previous.ID))

	// then
	require.NoError(t, err)
	assert.Equal(t, new(teamID), (*created)[0].TeamID)
	assert.Contains(t, (*sent)[0].HTML, "in the team <strong><bdi>Platform</bdi></strong>")
}

func TestResend_RefusesAnInviteItCannotResend(t *testing.T) {
	cases := []struct {
		name   string
		invite func(orgID uuid.UUID) *model.OrganisationInvite
		err    error
		want   error
	}{
		{name: "an unknown invite, or one of another organisation", err: errMissing, want: ErrInviteNotFound},
		{name: "a lookup that fails", err: errDatabaseDown, want: errDatabaseDown},
		{
			name: "a revoked invite",
			invite: func(orgID uuid.UUID) *model.OrganisationInvite {
				revoked := emailInvite(orgID)
				revoked.RevokedAt = new(time.Now())

				return revoked
			},
			want: ErrInviteNotFound,
		},
		{
			name: "an invite already accepted",
			invite: func(orgID uuid.UUID) *model.OrganisationInvite {
				accepted := emailInvite(orgID)
				accepted.UseCount = 1

				return accepted
			},
			want: ErrInviteNotFound,
		},
		{
			name: "a link invite, which has no address",
			invite: func(orgID uuid.UUID) *model.OrganisationInvite {
				link := emailInvite(orgID)
				link.Email, link.MaxUses = nil, nil

				return link
			},
			want: ErrNotResendable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()

			var stored *model.OrganisationInvite
			if tc.invite != nil {
				stored = tc.invite(orgID)
			}

			m.live(orgID)
			m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(stored, tc.err)

			// when
			response, err := svc.Resend(t.Context(), inviteAction(orgID, uuid.New(), uuid.New()))

			// then nothing is revoked, written or sent
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, response)
		})
	}
}

func TestResend_AnAdminInviteNeedsThePermissionToMakeAdmins(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	previous := emailInvite(orgID)
	previous.Role = model.OrgRoleAdmin
	m.live(orgID)
	m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(previous, nil)
	m.grants(orgID, actorID, false)

	// when
	response, err := svc.Resend(t.Context(), inviteAction(orgID, actorID, previous.ID))

	// then
	assert.ErrorIs(t, err, ErrAdminInviteForbidden)
	assert.Nil(t, response)
}

func TestResend_RefusesWhenTheAddressCannotBeInvited(t *testing.T) {
	cases := []struct {
		name  string
		given func(m *testMocks, orgID uuid.UUID)
		want  error
	}{
		{
			name: "no email provider",
			given: func(m *testMocks, _ uuid.UUID) {
				m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)
			},
			want: ErrEmailUnavailable,
		},
		{
			name: "an address that has joined since",
			given: func(m *testMocks, orgID uuid.UUID) {
				accountID := uuid.New()
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().GetByEmail(mock.Anything, testAddress).Return(&model.User{ID: accountID}, nil)
				m.orgs.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: accountID}).Return(&model.OrganisationMember{}, nil)
			},
			want: ErrAlreadyMember,
		},
		{
			name: "a team archived since",
			given: func(m *testMocks, _ uuid.UUID) {
				m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(&model.Team{ArchivedAt: new(time.Now())}, nil)
			},
			want: ErrTeamArchived,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			previous := emailInvite(orgID)
			previous.TeamID = new(uuid.New())
			m.live(orgID)
			m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(previous, nil)
			tc.given(m, orgID)
			m.teams.EXPECT().GetByID(mock.Anything, mock.Anything).Return(&model.Team{Name: "Platform"}, nil).Maybe()

			// when
			response, err := svc.Resend(t.Context(), inviteAction(orgID, uuid.New(), previous.ID))

			// then the earlier invite is left as it was
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, response)
		})
	}
}

func TestResend_TheEmailIsComposedBeforeTheEarlierInviteIsRevoked(t *testing.T) {
	cases := []struct {
		name  string
		given func(m *testMocks, actorID uuid.UUID)
	}{
		{
			name: "a site that cannot be read",
			given: func(m *testMocks, _ uuid.UUID) {
				m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)
			},
		},
		{
			name: "an inviter who cannot be read",
			given: func(m *testMocks, actorID uuid.UUID) {
				m.readsSite()
				m.users.EXPECT().GetByID(mock.Anything, actorID).Return(nil, errDatabaseDown)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			previous := emailInvite(orgID)
			m.live(orgID)
			m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(previous, nil)
			m.invitable(testAddress)
			tc.given(m, actorID)

			// when
			response, err := svc.Resend(t.Context(), inviteAction(orgID, actorID, previous.ID))

			// then the earlier invite still works
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.Nil(t, response)
		})
	}
}

func TestResend_WhatFailsAroundTheReplacement(t *testing.T) {
	cases := []struct {
		name      string
		revokeErr error
		createErr error
		want      error
	}{
		{name: "an invite revoked or accepted meanwhile", revokeErr: errMissing, want: ErrInviteNotFound},
		{name: "a revoke that fails", revokeErr: errDatabaseDown, want: errDatabaseDown},
		{name: "a replacement that cannot be created", createErr: errDatabaseDown, want: ErrInviteNotReissued},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			previous := emailInvite(orgID)
			m.live(orgID)
			m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(previous, nil)
			m.invitable(testAddress)
			m.readsSite()
			m.composesLetter(actorID)

			var revoked *model.OrganisationInvite
			if tc.revokeErr == nil {
				revoked = previous
				m.createsInvite(tc.createErr)
			}

			m.orgs.EXPECT().RevokeInvite(mock.Anything, mock.Anything).Return(revoked, tc.revokeErr)

			// when
			response, err := svc.Resend(t.Context(), inviteAction(orgID, actorID, previous.ID))

			// then nothing is sent
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, response)
		})
	}
}

func TestResend_AnEmailThatFailsIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	previous := emailInvite(orgID)
	m.live(orgID)
	m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(previous, nil)
	m.invitable(testAddress)
	m.readsSite()
	m.composesLetter(actorID)
	m.orgs.EXPECT().RevokeInvite(mock.Anything, mock.Anything).Return(previous, nil)
	m.createsInvite(nil)
	recorded := m.audits(nil)
	m.sends(errDatabaseDown)

	// when
	response, err := svc.Resend(t.Context(), inviteAction(orgID, actorID, previous.ID))

	// then
	assert.ErrorIs(t, err, ErrInviteNotSent)
	require.NotNil(t, response)
	assert.Len(t, *recorded, 1)
}

func TestRevoke_RevokesAndAudits(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID, teamID := uuid.New(), uuid.New(), uuid.New()
	pending := emailInvite(orgID)
	pending.TeamID = new(teamID)
	revoked := *pending
	revoked.RevokedAt = new(time.Now())
	key := spec.OrganisationInviteKey{OrgID: orgID, InviteID: revoked.ID}
	m.live(orgID)
	m.orgs.EXPECT().GetInvite(mock.Anything, key).Return(pending, nil)
	m.orgs.EXPECT().RevokeInvite(mock.Anything, key).Return(&revoked, nil)
	recorded := m.audits(nil)

	// when
	err := svc.Revoke(t.Context(), inviteAction(orgID, actorID, revoked.ID))

	// then a member invite needs no grant beyond the route's own
	require.NoError(t, err)
	assert.Equal(t, []audit.NewEntry{{
		OrgID:      orgID,
		ActorID:    actorID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionInviteRevoked,
		TargetType: audit.TargetInvite,
		TargetID:   revoked.ID.String(),
		Details:    map[string]string{"role": "member", "email": testAddress, "team_id": teamID.String(), "max_uses": "1"},
		IP:         testIP,
	}}, *recorded)
}

func TestRevoke_AnAdminInviteNeedsTheGrantToMakeAdmins(t *testing.T) {
	cases := []struct {
		name    string
		allowed bool
		want    error
	}{
		{name: "an owner", allowed: true},
		{name: "an admin", allowed: false, want: ErrAdminRevokeForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			pending := emailInvite(orgID)
			pending.Role = model.OrgRoleAdmin
			m.live(orgID)
			m.orgs.EXPECT().GetInvite(mock.Anything, spec.OrganisationInviteKey{OrgID: orgID, InviteID: pending.ID}).Return(pending, nil)
			m.grants(orgID, actorID, tc.allowed)
			if tc.allowed {
				m.orgs.EXPECT().RevokeInvite(mock.Anything, mock.Anything).Return(pending, nil)
				m.audits(nil)
			}

			// when
			err := svc.Revoke(t.Context(), inviteAction(orgID, actorID, pending.ID))

			// then the strict mocks prove a refused admin invite is left in place
			if tc.want == nil {
				require.NoError(t, err)
				return
			}

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRevoke_AFailedGrantCheckRevokesNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	pending := emailInvite(orgID)
	pending.Role = model.OrgRoleAdmin
	m.live(orgID)
	m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(pending, nil)
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabaseDown)

	// when
	err := svc.Revoke(t.Context(), inviteAction(orgID, actorID, pending.ID))

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
}

func TestRevoke_RefusesAnInviteItCannotFind(t *testing.T) {
	cases := []struct {
		name   string
		invite func(orgID uuid.UUID) *model.OrganisationInvite
		err    error
		want   error
	}{
		{name: "an unknown invite, or one of another organisation", err: errMissing, want: ErrInviteNotFound},
		{name: "a lookup that fails", err: errDatabaseDown, want: errDatabaseDown},
		{
			name: "an admin invite already revoked",
			invite: func(orgID uuid.UUID) *model.OrganisationInvite {
				revoked := emailInvite(orgID)
				revoked.Role = model.OrgRoleAdmin
				revoked.RevokedAt = new(time.Now())

				return revoked
			},
			want: ErrInviteNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, inviteID := uuid.New(), uuid.New()

			var found *model.OrganisationInvite
			if tc.invite != nil {
				found = tc.invite(orgID)
			}

			m.live(orgID)
			m.orgs.EXPECT().GetInvite(mock.Anything, spec.OrganisationInviteKey{OrgID: orgID, InviteID: inviteID}).Return(found, tc.err)

			// when
			err := svc.Revoke(t.Context(), inviteAction(orgID, uuid.New(), inviteID))

			// then the invite is looked up only within the path's organisation, and the strict mocks prove neither the grant nor the write was asked
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRevoke_WhatCanGoWrong(t *testing.T) {
	cases := []struct {
		name      string
		revokeErr error
		auditErr  error
		want      error
	}{
		{name: "an invite revoked meanwhile", revokeErr: errMissing, want: ErrInviteNotFound},
		{name: "a revoke that fails", revokeErr: errDatabaseDown, want: errDatabaseDown},
		{name: "a lost audit entry", auditErr: errDatabaseDown, want: ErrNotAudited},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()

			var revoked *model.OrganisationInvite
			if tc.revokeErr == nil {
				revoked = emailInvite(orgID)
				m.audits(tc.auditErr)
			}

			m.live(orgID)
			m.orgs.EXPECT().GetInvite(mock.Anything, mock.Anything).Return(emailInvite(orgID), nil)
			m.orgs.EXPECT().RevokeInvite(mock.Anything, mock.Anything).Return(revoked, tc.revokeErr)

			// when
			err := svc.Revoke(t.Context(), inviteAction(orgID, uuid.New(), uuid.New()))

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestDeleteExpired_ReportsHowManyWentAway(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.orgs.EXPECT().DeleteExpiredInvites(mock.Anything).Return(3, nil)

	// when
	deleted, err := svc.DeleteExpired(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)
}

func TestDeleteExpired_ReturnsAFailedSweep(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.orgs.EXPECT().DeleteExpiredInvites(mock.Anything).Return(0, errDatabaseDown)

	// when
	deleted, err := svc.DeleteExpired(t.Context())

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Zero(t, deleted)
}

func TestInviteRole_DefaultsToMember(t *testing.T) {
	cases := []struct {
		requested model.OrgRole
		want      model.OrgRole
	}{
		{requested: "", want: model.OrgRoleMember},
		{requested: model.OrgRoleMember, want: model.OrgRoleMember},
		{requested: model.OrgRoleAdmin, want: model.OrgRoleAdmin},
	}

	for _, tc := range cases {
		t.Run(string(tc.requested), func(t *testing.T) {
			// when
			role, err := inviteRole(tc.requested)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, role)
		})
	}
}

func TestCreate_AnAdminInviteByEmailNamesTheRole(t *testing.T) {
	// given an owner inviting an admin by email
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.live(orgID)
	m.grants(orgID, actorID, true)
	m.invitable(testAddress)
	m.nothingPending(orgID)
	m.readsSite()
	m.composesLetter(actorID)
	m.createsInvite(nil)
	m.audits(nil)
	sent := m.sends(nil)

	creation := emailCreation(orgID, actorID)
	creation.Role = model.OrgRoleAdmin

	// when
	_, err := svc.Create(t.Context(), creation)

	// then
	require.NoError(t, err)
	assert.Contains(t, (*sent)[0].HTML, " as an admin")
}
