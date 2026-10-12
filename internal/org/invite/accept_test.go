package invite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testOtherAddress = "dave@example.com"
)

func (m *testMocks) arrives(details *model.InviteDetails, account *model.User) {
	m.finds(details)
	m.users.EXPECT().GetByID(mock.Anything, account.ID).Return(account, nil)
}

func (m *testMocks) buildsAccount(address string) {
	built := spec.NewUser{Email: address, PasswordHash: new("a bcrypt hash"), DisplayName: testName}

	m.accounts.EXPECT().NewAccountSpec(mock.Anything, spec.NewAccount{Email: address, Password: testPassword, DisplayName: testName}).Return(built, nil)
}

func (m *testMocks) registers(err error) *[]spec.NewRegistration {
	registered := &[]spec.NewRegistration{}

	m.users.EXPECT().RegisterAccount(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewRegistration, _ ...*sql.Tx) (*model.User, error) {
		*registered = append(*registered, s)
		if err != nil {
			return nil, err
		}

		return &model.User{ID: uuid.New(), Email: s.User.Email, PasswordHash: s.User.PasswordHash, DisplayName: s.User.DisplayName, EmailVerified: s.User.EmailVerified}, nil
	})

	return registered
}

func (m *testMocks) passwordSignIn(allowed bool) {
	switched := m.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn)

	switched.Return(allowed, nil)
}

func (m *testMocks) addressFree(address string) {
	filter := spec.UserEmailFilter{Email: address}

	m.users.EXPECT().EmailInUse(mock.Anything, filter).Return(false, nil)
}

func member(account *model.User) *model.User {
	account.ID = uuid.New()
	account.EmailVerified = true

	return account
}

func join(userID uuid.UUID) spec.InviteJoin {
	return spec.InviteJoin{Token: testToken, UserID: userID, IP: testIP}
}

func registration(address string) spec.InviteRegistration {
	return spec.InviteRegistration{Token: testToken, Email: address, DisplayName: testName, Password: testPassword, IP: testIP}
}

func TestLookup_FindsTheInviteByTheHashOfItsToken(t *testing.T) {
	// given an email invite to a team
	svc, m := newTestService(t)
	details := boundDetails(uuid.New(), testAddress)
	details.Role = model.OrgRoleAdmin
	details.TeamName = new("Platform")
	m.finds(details)

	// when
	found, err := svc.Lookup(t.Context(), testToken)

	// then the page can show what joining means, and the address it is for
	require.NoError(t, err)
	assert.Equal(t, &dto.InviteDetailsResponse{
		OrgName:     testOrgName,
		OrgSlug:     testOrgSlug,
		TeamName:    "Platform",
		InviterName: testInviter,
		Role:        model.OrgRoleAdmin,
		Email:       testAddress,
		ExpiresAt:   details.ExpiresAt.UTC().Format(time.RFC3339),
	}, found)
}

func TestLookup_RefusesAnInviteThatCannotBeUsed(t *testing.T) {
	cases := []struct {
		name     string
		token    string
		lookedUp bool
		err      error
		want     error
	}{
		{name: "an empty token, before any lookup", token: "", want: ErrInvalidInvite},
		{name: "an unknown, used, revoked or expired token, or a deleted organisation", token: testToken, lookedUp: true, err: errMissing, want: ErrInvalidInvite},
		{name: "a lookup that fails", token: testToken, lookedUp: true, err: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			if tc.lookedUp {
				m.orgs.EXPECT().GetLiveInviteByTokenHash(mock.Anything, token.Hash(tc.token)).Return(nil, tc.err)
			}

			// when
			found, err := svc.Lookup(t.Context(), tc.token)

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, found)
		})
	}
}

func TestAccept_ALinkInviteJoinsTheOrganisation(t *testing.T) {
	// given a signed-in user who is not yet a member
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := liveDetails(orgID)
	account := member(&model.User{Email: testAddress})
	m.arrives(details, account)
	m.notAMember(orgID, account.ID)
	m.hasSpace(t, orgID, 3)
	acceptance := m.accepts(details, nil)
	recorded := m.audits(nil)

	// when
	joined, err := svc.Accept(t.Context(), join(account.ID))

	// then the invite is spent by the hash of its token, within the free plan's ten members
	require.NoError(t, err)
	assert.Equal(t, spec.InviteAcceptance{TokenHash: token.Hash(testToken), UserID: account.ID, Email: testAddress, MaxMembers: freeMembers}, *acceptance)
	assert.Equal(t, &dto.JoinedOrganisationResponse{OrgID: orgID, OrgSlug: testOrgSlug, OrgName: testOrgName, Role: model.OrgRoleMember}, joined)
	assert.Equal(t, []audit.NewEntry{{
		OrgID:      orgID,
		ActorID:    account.ID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionMemberJoined,
		TargetType: audit.TargetInvite,
		TargetID:   details.ID.String(),
		SubjectID:  account.ID,
		Details:    map[string]string{"role": "member"},
		IP:         testIP,
	}}, *recorded)
}

func TestAccept_ATeamInviteJoinsTheTeamToo(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, teamID := uuid.New(), uuid.New()
	details := liveDetails(orgID)
	details.TeamID = new(teamID)
	account := member(&model.User{Email: testAddress})
	m.arrives(details, account)
	m.notAMember(orgID, account.ID)
	m.hasSpace(t, orgID, 3)
	m.accepts(details, nil)
	recorded := m.audits(nil)

	// when
	joined, err := svc.Accept(t.Context(), join(account.ID))

	// then
	require.NoError(t, err)
	assert.Equal(t, new(teamID), joined.TeamID)
	assert.Equal(t, map[string]string{"role": "member", "team_id": teamID.String()}, (*recorded)[0].Details)
}

func TestAccept_AnEmailInviteForTheVerifiedAddressInAnyCase(t *testing.T) {
	// given an invite typed with capitals, and the account's own address
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := boundDetails(orgID, "Carol@Example.com")
	details.Role = model.OrgRoleAdmin
	account := member(&model.User{Email: testAddress})
	m.arrives(details, account)
	m.notAMember(orgID, account.ID)
	m.hasSpace(t, orgID, 3)
	acceptance := m.accepts(details, nil)
	m.audits(nil)

	// when
	joined, err := svc.Accept(t.Context(), join(account.ID))

	// then the account's address is the one the statement matches against
	require.NoError(t, err)
	assert.Equal(t, testAddress, acceptance.Email)
	assert.Equal(t, model.OrgRoleAdmin, joined.Role)
}

func TestAccept_AnEmailInviteIsOnlyForItsVerifiedAddress(t *testing.T) {
	cases := []struct {
		name     string
		address  string
		verified bool
		want     error
	}{
		{name: "another address", address: testOtherAddress, verified: true, want: ErrEmailMismatch},
		{name: "another address, unverified", address: testOtherAddress, verified: false, want: ErrEmailMismatch},
		{name: "the address, unverified, as only the super admin can sign in with", address: testAddress, verified: false, want: ErrEmailUnverified},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			details := boundDetails(uuid.New(), testAddress)
			account := &model.User{ID: uuid.New(), Email: tc.address, EmailVerified: tc.verified}
			m.arrives(details, account)

			// when
			joined, err := svc.Accept(t.Context(), join(account.ID))

			// then the invite is left unspent
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, joined)
		})
	}
}

func TestAccept_AnExistingMemberSpendsNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := liveDetails(orgID)
	account := member(&model.User{Email: testAddress})
	m.arrives(details, account)
	m.orgs.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: account.ID}).Return(&model.OrganisationMember{OrgID: orgID, UserID: account.ID, Role: model.OrgRoleMember}, nil)

	// when
	joined, err := svc.Accept(t.Context(), join(account.ID))

	// then even a full organisation answers that they are already in it
	assert.ErrorIs(t, err, ErrAlreadyMember)
	assert.Nil(t, joined)
}

func TestAccept_AFullOrganisationRefusesBeforeSpendingTheInvite(t *testing.T) {
	// given ten members on the free plan
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := liveDetails(orgID)
	account := member(&model.User{Email: testAddress})
	m.arrives(details, account)
	m.notAMember(orgID, account.ID)
	m.isFull(orgID, freeMembers)

	// when
	joined, err := svc.Accept(t.Context(), join(account.ID))

	// then the refusal carries what the upgrade prompt needs
	denied, ok := errors.AsType[*entitlements.DeniedError](err)
	require.True(t, ok, "%v", err)
	assert.ErrorIs(t, err, entitlements.ErrUpgradeRequired)
	assert.Equal(t, entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitMembers, Allowed: freeMembers, Current: freeMembers}, *denied)
	assert.Nil(t, joined)
}

func TestAccept_WhatTheJoinCanRefuse(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		check func(t *testing.T, err error)
	}{
		{
			name: "the organisation filled up meanwhile",
			err:  fmt.Errorf("add member: %w", dao.ErrQuotaExceeded),
			check: func(t *testing.T, err error) {
				denied, ok := errors.AsType[*entitlements.DeniedError](err)
				require.True(t, ok, "%v", err)
				assert.Equal(t, entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitMembers, Allowed: freeMembers, Current: freeMembers}, *denied)
			},
		},
		{
			name: "a membership that appeared meanwhile",
			err:  fmt.Errorf("add member: %w", dao.ErrConflict),
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrAlreadyMember)
			},
		},
		{
			name: "an invite spent, revoked or expired meanwhile, or an organisation deleted",
			err:  errMissing,
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidInvite)
			},
		},
		{
			name: "a join that fails",
			err:  errDatabaseDown,
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, errDatabaseDown)
				assert.NotErrorIs(t, err, ErrInvalidInvite)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			details := liveDetails(orgID)
			account := member(&model.User{Email: testAddress})
			m.arrives(details, account)
			m.notAMember(orgID, account.ID)
			m.hasSpace(t, orgID, 9)
			m.orgs.EXPECT().AcceptInvite(mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			joined, err := svc.Accept(t.Context(), join(account.ID))

			// then nothing is audited
			tc.check(t, err)
			assert.Nil(t, joined)
		})
	}
}

func TestAccept_APlanThatDoesNotNameTheMemberLimitIsAnError(t *testing.T) {
	// given a plan with no member limit
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := liveDetails(orgID)
	account := member(&model.User{Email: testAddress})
	m.arrives(details, account)
	m.notAMember(orgID, account.ID)
	m.orgs.EXPECT().CountMembers(mock.Anything, orgID).Return(250, nil)
	m.entitlements.EXPECT().CheckWithin(mock.Anything, mock.Anything).Return(nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(&entitlements.Plan{Code: "unlimited", Limits: map[model.Limit]int{model.LimitTeams: 5}}, nil)

	// when
	joined, err := svc.Accept(t.Context(), join(account.ID))

	// then a limit the plan does not name is never unlimited, and the strict mocks prove nothing was written
	assert.ErrorIs(t, err, entitlements.ErrUnknownLimit)
	assert.NotErrorIs(t, err, entitlements.ErrUpgradeRequired)
	assert.Nil(t, joined)
}

func TestAccept_JoinsDespiteWhatFailsAfterTheCommit(t *testing.T) {
	cases := []struct {
		name     string
		joinErr  error
		auditErr error
		want     []error
	}{
		{name: "a cached membership that could not be cleared", joinErr: errStaleCache, want: []error{ErrCacheNotCleared, repository.ErrStaleTenantCache}},
		{name: "a lost audit entry", auditErr: errDatabaseDown, want: []error{ErrNotAudited, errDatabaseDown}},
		{name: "both", joinErr: errStaleCache, auditErr: errDatabaseDown, want: []error{ErrCacheNotCleared, ErrNotAudited}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			details := liveDetails(orgID)
			account := member(&model.User{Email: testAddress})
			m.arrives(details, account)
			m.notAMember(orgID, account.ID)
			m.hasSpace(t, orgID, 3)
			m.accepts(details, tc.joinErr)
			recorded := m.audits(tc.auditErr)

			// when
			joined, err := svc.Accept(t.Context(), join(account.ID))

			// then the member is in, and the cause is reported beside the answer
			require.NotNil(t, joined)
			assert.Equal(t, orgID, joined.OrgID)
			assert.Len(t, *recorded, 1)
			for _, want := range tc.want {
				assert.ErrorIs(t, err, want)
			}
		})
	}
}

func TestAccept_AnInvalidTokenIsRefused(t *testing.T) {
	// given the strict mocks, which fail on any lookup
	svc, _ := newTestService(t)

	// when
	joined, err := svc.Accept(t.Context(), spec.InviteJoin{UserID: uuid.New()})

	// then
	assert.ErrorIs(t, err, ErrInvalidInvite)
	assert.Nil(t, joined)
}

func TestAccept_WhatCannotBeRead(t *testing.T) {
	cases := []struct {
		name  string
		given func(t *testing.T, m *testMocks, details *model.InviteDetails, account *model.User)
	}{
		{
			name: "the invite",
			given: func(_ *testing.T, m *testMocks, _ *model.InviteDetails, _ *model.User) {
				m.orgs.EXPECT().GetLiveInviteByTokenHash(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the account",
			given: func(_ *testing.T, m *testMocks, details *model.InviteDetails, account *model.User) {
				m.finds(details)
				m.users.EXPECT().GetByID(mock.Anything, account.ID).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the membership",
			given: func(_ *testing.T, m *testMocks, details *model.InviteDetails, account *model.User) {
				m.arrives(details, account)
				m.orgs.EXPECT().GetMember(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
			},
		},
		{
			name: "the member count",
			given: func(_ *testing.T, m *testMocks, details *model.InviteDetails, account *model.User) {
				m.arrives(details, account)
				m.notAMember(details.OrgID, account.ID)
				m.orgs.EXPECT().CountMembers(mock.Anything, details.OrgID).Return(0, errDatabaseDown)
			},
		},
		{
			name: "the allowance",
			given: func(_ *testing.T, m *testMocks, details *model.InviteDetails, account *model.User) {
				m.arrives(details, account)
				m.notAMember(details.OrgID, account.ID)
				m.orgs.EXPECT().CountMembers(mock.Anything, details.OrgID).Return(3, nil)
				m.entitlements.EXPECT().CheckWithin(mock.Anything, mock.Anything).Return(errDatabaseDown)
			},
		},
		{
			name: "the plan",
			given: func(_ *testing.T, m *testMocks, details *model.InviteDetails, account *model.User) {
				m.arrives(details, account)
				m.notAMember(details.OrgID, account.ID)
				m.orgs.EXPECT().CountMembers(mock.Anything, details.OrgID).Return(3, nil)
				m.entitlements.EXPECT().CheckWithin(mock.Anything, mock.Anything).Return(nil)
				m.entitlements.EXPECT().For(mock.Anything, details.OrgID).Return(nil, errDatabaseDown)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			details := liveDetails(uuid.New())
			account := member(&model.User{Email: testAddress})
			tc.given(t, m, details, account)

			// when
			joined, err := svc.Accept(t.Context(), join(account.ID))

			// then a read that fails is never a pass, and the invite is left unspent
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.NotErrorIs(t, err, entitlements.ErrUpgradeRequired)
			assert.Nil(t, joined)
		})
	}
}

func TestRegister_AnEmailInviteCreatesAVerifiedAccountThatJoins(t *testing.T) {
	// given someone with no account, on a site of either mode
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := boundDetails(orgID, testAddress)
	m.passwordSignIn(true)
	m.finds(details)
	m.addressFree(testAddress)
	m.hasSpace(t, orgID, 3)
	m.buildsAccount(testAddress)
	registered := m.registers(nil)
	acceptance := m.accepts(details, nil)
	recorded := m.audits(nil)

	// when
	created, err := svc.Register(t.Context(), registration(""))

	// then the link proved the mailbox, so no verification email is sent and no site mode or first-account claim is read
	require.NoError(t, err)
	require.Len(t, *registered, 1)
	pending := (*registered)[0]
	assert.Equal(t, testAddress, pending.User.Email)
	assert.True(t, pending.User.EmailVerified)
	assert.False(t, pending.FirstAccount)
	assert.Empty(t, pending.VerificationHash)
	assert.Equal(t, testAddress, acceptance.Email)
	assert.Equal(t, token.Hash(testToken), acceptance.TokenHash)
	assert.Equal(t, freeMembers, acceptance.MaxMembers)
	assert.Equal(t, &dto.InviteRegistrationResponse{
		Email:                testAddress,
		VerificationRequired: false,
		Joined:               dto.JoinedOrganisationResponse{OrgID: orgID, OrgSlug: testOrgSlug, OrgName: testOrgName, Role: model.OrgRoleMember},
	}, created)
	require.Len(t, *recorded, 1)
	assert.Equal(t, audit.ActionMemberJoined, (*recorded)[0].Action)
	assert.Equal(t, acceptance.UserID, (*recorded)[0].ActorID)
	assert.Equal(t, acceptance.UserID, (*recorded)[0].SubjectID)
	assert.Equal(t, testIP, (*recorded)[0].IP)
}

func TestRegister_AnEmailInviteKeepsItsAddressInAnyCase(t *testing.T) {
	// given the page sends the address back with different capitals
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := boundDetails(orgID, testAddress)
	m.passwordSignIn(true)
	m.finds(details)
	m.addressFree(testAddress)
	m.hasSpace(t, orgID, 3)
	m.buildsAccount(testAddress)
	registered := m.registers(nil)
	m.accepts(details, nil)
	m.audits(nil)

	// when
	_, err := svc.Register(t.Context(), registration(" CAROL@example.com "))

	// then the account takes the invited address as it was stored
	require.NoError(t, err)
	assert.Equal(t, testAddress, (*registered)[0].User.Email)
}

func TestRegister_ALinkInviteCreatesAnAccountThatVerifiesBeforeSigningIn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	details := liveDetails(orgID)
	m.passwordSignIn(true)
	m.finds(details)
	m.addressFree(testOtherAddress)
	m.hasSpace(t, orgID, 3)
	m.buildsAccount(testOtherAddress)
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.readsSite()
	registered := m.registers(nil)
	acceptance := m.accepts(details, nil)
	m.audits(nil)
	sent := m.sends(nil)

	// when
	created, err := svc.Register(t.Context(), registration("  "+testOtherAddress+" "))

	// then the account joins at once, and signs in once its address is verified
	require.NoError(t, err)
	pending := (*registered)[0]
	assert.Equal(t, testOtherAddress, pending.User.Email)
	assert.False(t, pending.User.EmailVerified)
	assert.False(t, pending.FirstAccount)
	assert.WithinDuration(t, time.Now().Add(verificationTTL), pending.VerificationExpiresAt, time.Minute)
	require.Len(t, *sent, 1)
	letter := (*sent)[0]
	assert.Equal(t, testOtherAddress, letter.To)
	assert.Equal(t, "Confirm your email address for RetroTide", letter.Subject)
	assert.Equal(t, token.Hash(linkedToken(t, verificationLink, letter)), pending.VerificationHash)
	assert.Equal(t, testOtherAddress, acceptance.Email)
	assert.True(t, created.VerificationRequired)
	assert.Equal(t, testOtherAddress, created.Email)
	assert.Equal(t, orgID, created.Joined.OrgID)
}

func TestRegister_RefusesBeforeCreatingAnAccount(t *testing.T) {
	cases := []struct {
		name  string
		shape func(registration *spec.InviteRegistration)
		given func(t *testing.T, m *testMocks, orgID uuid.UUID)
		want  error
	}{
		{
			name:  "an empty token, before any lookup",
			shape: func(registration *spec.InviteRegistration) { registration.Token = "" },
			given: func(*testing.T, *testMocks, uuid.UUID) {},
			want:  ErrInvalidInvite,
		},
		{
			name: "an unknown, used, revoked or expired invite",
			given: func(_ *testing.T, m *testMocks, _ uuid.UUID) {
				m.orgs.EXPECT().GetLiveInviteByTokenHash(mock.Anything, token.Hash(testToken)).Return(nil, errMissing)
			},
			want: ErrInvalidInvite,
		},
		{
			name:  "a link invite with no address",
			shape: func(registration *spec.InviteRegistration) { registration.Email = "" },
			given: func(_ *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
			},
			want: ErrInvalidEmail,
		},
		{
			name:  "a link invite with a malformed address",
			shape: func(registration *spec.InviteRegistration) { registration.Email = "dave at example" },
			given: func(_ *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
			},
			want: ErrInvalidEmail,
		},
		{
			name:  "an email invite sent back with another address",
			shape: func(registration *spec.InviteRegistration) { registration.Email = testOtherAddress },
			given: func(_ *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(boundDetails(orgID, testAddress))
			},
			want: ErrEmailMismatch,
		},
		{
			name:  "a display name that is only markup and spaces",
			shape: func(registration *spec.InviteRegistration) { registration.DisplayName = "  <b></b> " },
			given: func(_ *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
			},
			want: user.ErrDisplayNameRequired,
		},
		{
			name: "an address that already has an account",
			given: func(_ *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
				m.users.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testOtherAddress}).Return(true, nil)
			},
			want: ErrEmailTaken,
		},
		{
			name: "an address check that fails",
			given: func(_ *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
				m.users.EXPECT().EmailInUse(mock.Anything, mock.Anything).Return(false, errDatabaseDown)
			},
			want: errDatabaseDown,
		},
		{
			name: "a full organisation",
			given: func(_ *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
				m.addressFree(testOtherAddress)
				m.isFull(orgID, freeMembers)
			},
			want: entitlements.ErrUpgradeRequired,
		},
		{
			name: "a password too short",
			given: func(t *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
				m.addressFree(testOtherAddress)
				m.hasSpace(t, orgID, 3)
				m.accounts.EXPECT().NewAccountSpec(mock.Anything, mock.Anything).Return(spec.NewUser{}, user.ErrPasswordTooShort)
			},
			want: user.ErrPasswordTooShort,
		},
		{
			name: "a verification link whose site cannot be read",
			given: func(t *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
				m.addressFree(testOtherAddress)
				m.hasSpace(t, orgID, 3)
				m.buildsAccount(testOtherAddress)
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return("", errDatabaseDown)
			},
			want: errDatabaseDown,
		},
		{
			name: "a link invite while the site cannot send its verification email",
			given: func(t *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
				m.addressFree(testOtherAddress)
				m.hasSpace(t, orgID, 3)
				m.buildsAccount(testOtherAddress)
				m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)
			},
			want: ErrEmailUnavailable,
		},
		{
			name: "an email provider check that fails",
			given: func(t *testing.T, m *testMocks, orgID uuid.UUID) {
				m.finds(liveDetails(orgID))
				m.addressFree(testOtherAddress)
				m.hasSpace(t, orgID, 3)
				m.buildsAccount(testOtherAddress)
				m.email.EXPECT().Enabled(mock.Anything).Return(false, errDatabaseDown)
			},
			want: errDatabaseDown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.passwordSignIn(true)
			tc.given(t, m, uuid.New())

			request := registration(testOtherAddress)
			if tc.shape != nil {
				tc.shape(&request)
			}

			// when
			created, err := svc.Register(t.Context(), request)

			// then no account is created and the invite is left unspent
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, created)
		})
	}
}

func TestRegister_IsRefusedWhilePasswordSignInIsOff(t *testing.T) {
	// given password sign-in is switched off
	svc, m := newTestService(t)
	m.passwordSignIn(false)

	// when
	created, err := svc.Register(t.Context(), registration(testOtherAddress))

	// then the invite is never looked up, so nothing about it is revealed
	assert.ErrorIs(t, err, ErrPasswordSignInDisabled)
	assert.EqualError(t, err, "password sign-in is turned off on this site")
	assert.Nil(t, created)
}

func TestRegister_ReportsAPasswordSwitchThatCannotBeRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.settings.EXPECT().GetBool(mock.Anything, config.SettingAllowPasswordSignIn).Return(false, errDatabaseDown)

	// when
	created, err := svc.Register(t.Context(), registration(testOtherAddress))

	// then a failed read is never taken as either answer
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrPasswordSignInDisabled)
	assert.Nil(t, created)
}

func TestRegister_WhatTheRegistrationCanRefuse(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{name: "an address taken meanwhile", err: fmt.Errorf("create user: %w", dao.ErrConflict), want: ErrEmailTaken},
		{name: "a registration that fails", err: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			m.passwordSignIn(true)
			m.finds(boundDetails(orgID, testAddress))
			m.addressFree(testAddress)
			m.hasSpace(t, orgID, 3)
			m.buildsAccount(testAddress)
			m.registers(tc.err)

			// when
			created, err := svc.Register(t.Context(), registration(""))

			// then the invite is left unspent
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, created)
		})
	}
}

func TestRegister_AJoinRefusedAfterTheAccountWasCreatedRemovesIt(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{name: "an invite spent or revoked meanwhile", err: errMissing, want: ErrInvalidInvite},
		{name: "an organisation that filled up meanwhile", err: dao.ErrQuotaExceeded, want: entitlements.ErrUpgradeRequired},
		{name: "a join that fails", err: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			m.passwordSignIn(true)
			m.finds(boundDetails(orgID, testAddress))
			m.addressFree(testAddress)
			m.hasSpace(t, orgID, 9)
			m.buildsAccount(testAddress)
			m.registers(nil)

			var joined uuid.UUID
			m.orgs.EXPECT().AcceptInvite(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.InviteAcceptance, _ ...*sql.Tx) (*model.AcceptedInvite, error) {
				joined = s.UserID

				return nil, tc.err
			})

			var removed uuid.UUID
			m.users.EXPECT().Delete(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, userID uuid.UUID, _ ...*sql.Tx) error {
				removed = userID

				return nil
			})

			// when
			created, err := svc.Register(t.Context(), registration(""))

			// then no account outlives an invite it could not accept
			assert.ErrorIs(t, err, tc.want)
			assert.NotErrorIs(t, err, ErrAccountLeftBehind)
			assert.Nil(t, created)
			assert.NotEqual(t, uuid.Nil, removed)
			assert.Equal(t, joined, removed)
		})
	}
}

func TestRegister_AnAccountThatCannotBeRemovedIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.passwordSignIn(true)
	m.finds(boundDetails(orgID, testAddress))
	m.addressFree(testAddress)
	m.hasSpace(t, orgID, 3)
	m.buildsAccount(testAddress)
	m.registers(nil)
	m.orgs.EXPECT().AcceptInvite(mock.Anything, mock.Anything).Return(nil, errMissing)
	m.users.EXPECT().Delete(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	created, err := svc.Register(t.Context(), registration(""))

	// then both causes are kept
	assert.ErrorIs(t, err, ErrAccountLeftBehind)
	assert.ErrorIs(t, err, ErrInvalidInvite)
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, created)
}

func TestRegister_AnswersDespiteWhatFailsAfterTheCommit(t *testing.T) {
	cases := []struct {
		name     string
		joinErr  error
		auditErr error
		sendErr  error
		want     []error
	}{
		{name: "a verification email that was not sent", sendErr: errDatabaseDown, want: []error{ErrVerificationNotSent, errDatabaseDown}},
		{name: "a lost audit entry", auditErr: errDatabaseDown, want: []error{ErrNotAudited}},
		{name: "a cached membership that could not be cleared", joinErr: errStaleCache, want: []error{ErrCacheNotCleared}},
		{name: "all three", joinErr: errStaleCache, auditErr: errDatabaseDown, sendErr: errDatabaseDown, want: []error{ErrVerificationNotSent, ErrNotAudited, ErrCacheNotCleared}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given a link invite, whose account is sent a verification email
			svc, m := newTestService(t)
			orgID := uuid.New()
			details := liveDetails(orgID)
			m.passwordSignIn(true)
			m.finds(details)
			m.addressFree(testOtherAddress)
			m.hasSpace(t, orgID, 3)
			m.buildsAccount(testOtherAddress)
			m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
			m.readsSite()
			m.registers(nil)
			m.accepts(details, tc.joinErr)
			m.audits(tc.auditErr)
			m.sends(tc.sendErr)

			// when
			created, err := svc.Register(t.Context(), registration(testOtherAddress))

			// then the account exists and has joined, so the answer still comes back
			require.NotNil(t, created)
			assert.True(t, created.VerificationRequired)
			assert.NotErrorIs(t, err, ErrAccountLeftBehind)
			for _, want := range tc.want {
				assert.ErrorIs(t, err, want)
			}
		})
	}
}
