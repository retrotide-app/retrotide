package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	identityRepoMocks struct {
		identities *dao.MockUserIdentityDAO
		users      *dao.MockUserDAO
		roles      *MockRoleRepository
		sessions   *MockSessionRepository
	}
)

const (
	googleSubject = "108234567890123456789"
)

func newMockedIdentityRepo(t *testing.T) (UserIdentityRepository, identityRepoMocks) {
	t.Helper()

	mocks := identityRepoMocks{
		identities: dao.NewMockUserIdentityDAO(t),
		users:      dao.NewMockUserDAO(t),
		roles:      NewMockRoleRepository(t),
		sessions:   NewMockSessionRepository(t),
	}

	deps := UserIdentityRepoDeps{
		Users:    mocks.users,
		Roles:    mocks.roles,
		Sessions: mocks.sessions,
	}

	return NewUserIdentityRepo(nil, mocks.identities, deps), mocks
}

func testIdentityRegistration() spec.IdentityRegistration {
	return spec.IdentityRegistration{
		User:     spec.NewUser{Email: "newcomer@gmail.com", DisplayName: "Newcomer", EmailVerified: true},
		Provider: model.ProviderGoogle,
		Subject:  googleSubject,
	}
}

func testLink(userID uuid.UUID, automatic bool) spec.IdentityLink {
	return spec.IdentityLink{
		Identity:     spec.NewUserIdentity{UserID: userID, Provider: model.ProviderGoogle, Subject: googleSubject, Email: "owner@gmail.com"},
		Automatic:    automatic,
		MatchedEmail: "owner@gmail.com",
	}
}

func TestUserIdentityRepositoryRegisterWithIdentity_WritesTheAccountAndItsIdentityTogether(t *testing.T) {
	// given a role mock with no expectations, so an ordinary sign-up that grants a role fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testIdentityRegistration()
	created := &model.User{ID: uuid.New(), Email: "newcomer@gmail.com", EmailVerified: true}
	mocks.users.EXPECT().Create(mock.Anything, registration.User, inTx).Return(created, nil).Once()
	mocks.identities.EXPECT().Create(mock.Anything, spec.NewUserIdentity{UserID: created.ID, Provider: model.ProviderGoogle, Subject: googleSubject, Email: "newcomer@gmail.com"}, inTx).Return(&model.UserIdentity{UserID: created.ID}, nil).Once()

	// when
	got, err := repo.RegisterWithIdentity(t.Context(), registration, tx)

	// then
	require.NoError(t, err)
	assert.Same(t, created, got)
}

func TestUserIdentityRepositoryRegisterWithIdentity_TheFirstAccountClaimsSuperAdminUnderTheLock(t *testing.T) {
	// given an empty users table, and a caller that did not ask for a verified address
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testIdentityRegistration()
	registration.FirstAccount = true
	registration.User.EmailVerified = false
	verified := registration.User
	verified.EmailVerified = true
	created := &model.User{ID: uuid.New(), Email: "newcomer@gmail.com", EmailVerified: true}
	var order []string
	mocks.users.EXPECT().LockFirstAccountClaim(mock.Anything, inTx).RunAndReturn(func(context.Context, ...*sql.Tx) error {
		order = append(order, "lock")
		return nil
	}).Once()
	mocks.users.EXPECT().AnyExists(mock.Anything, inTx).RunAndReturn(func(context.Context, ...*sql.Tx) (bool, error) {
		order = append(order, "check")
		return false, nil
	}).Once()
	mocks.users.EXPECT().Create(mock.Anything, verified, inTx).RunAndReturn(func(context.Context, spec.NewUser, ...*sql.Tx) (*model.User, error) {
		order = append(order, "create")
		return created, nil
	}).Once()
	mocks.roles.EXPECT().AssignToNewAccount(mock.Anything, spec.RoleAssignment{UserID: created.ID, Role: role.SuperAdmin}, inTx).Return(nil).Once()
	mocks.identities.EXPECT().Create(mock.Anything, mock.Anything, inTx).Return(&model.UserIdentity{}, nil).Once()

	// when
	got, err := repo.RegisterWithIdentity(t.Context(), registration, tx)

	// then the emptiness check runs only once the lock is held, and the super admin is created verified
	require.NoError(t, err)
	assert.Same(t, created, got)
	assert.Equal(t, []string{"lock", "check", "create"}, order)
}

func TestUserIdentityRepositoryRegisterWithIdentity_AFirstAccountClaimLosesToAnExistingAccount(t *testing.T) {
	// given another account committed first, and no create expectation, so writing one fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testIdentityRegistration()
	registration.FirstAccount = true
	mocks.users.EXPECT().LockFirstAccountClaim(mock.Anything, inTx).Return(nil).Once()
	mocks.users.EXPECT().AnyExists(mock.Anything, inTx).Return(true, nil).Once()

	// when
	got, err := repo.RegisterWithIdentity(t.Context(), registration, tx)

	// then
	require.ErrorIs(t, err, ErrNotFirstAccount)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryRegisterWithIdentity_ATakenAddressWritesNoIdentity(t *testing.T) {
	// given an identity mock with no expectations, so linking one fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testIdentityRegistration()
	mocks.users.EXPECT().Create(mock.Anything, registration.User, inTx).Return(nil, dao.ErrConflict).Once()

	// when
	got, err := repo.RegisterWithIdentity(t.Context(), registration, tx)

	// then the address was taken, which is not the identity being in use
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.NotErrorIs(t, err, ErrIdentityInUse)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryRegisterWithIdentity_AnIdentityLinkedMeanwhileIsInUse(t *testing.T) {
	// given
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testIdentityRegistration()
	mocks.users.EXPECT().Create(mock.Anything, registration.User, inTx).Return(&model.User{ID: uuid.New()}, nil).Once()
	mocks.identities.EXPECT().Create(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrConflict).Once()

	// when
	got, err := repo.RegisterWithIdentity(t.Context(), registration, tx)

	// then
	require.ErrorIs(t, err, ErrIdentityInUse)
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryLinkToAccount_AnAutomaticLinkEndsEverySessionOfTheAccount(t *testing.T) {
	// given
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	link := testLink(userID, true)
	linked := &model.UserIdentity{UserID: userID, Provider: model.ProviderGoogle, Subject: googleSubject}
	var order []string
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (*model.User, error) {
		order = append(order, "lock")
		return &model.User{ID: userID, Email: "owner@gmail.com", EmailVerified: true}, nil
	}).Once()
	mocks.identities.EXPECT().GetForUser(mock.Anything, spec.UserIdentityKey{UserID: userID, Provider: model.ProviderGoogle}, inTx).Return(nil, dao.ErrNotFound).Once()
	mocks.identities.EXPECT().Create(mock.Anything, link.Identity, inTx).RunAndReturn(func(context.Context, spec.NewUserIdentity, ...*sql.Tx) (*model.UserIdentity, error) {
		order = append(order, "link")
		return linked, nil
	}).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) error {
		order = append(order, "end sessions")
		return nil
	}).Once()

	// when
	got, err := repo.LinkToAccount(t.Context(), link, tx)

	// then
	require.NoError(t, err)
	assert.Same(t, linked, got)
	assert.Equal(t, []string{"lock", "link", "end sessions"}, order)
}

func TestUserIdentityRepositoryLinkToAccount_ALinkFromSettingsKeepsTheSessionsAndIgnoresTheAddress(t *testing.T) {
	// given a Google address unlike the account's, and a session mock with no expectations, so ending a session fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	link := testLink(userID, false)
	link.MatchedEmail = ""
	linked := &model.UserIdentity{UserID: userID}
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, Email: "work@example.com", EmailVerified: true}, nil).Once()
	mocks.identities.EXPECT().GetForUser(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()
	mocks.identities.EXPECT().Create(mock.Anything, link.Identity, inTx).Return(linked, nil).Once()

	// when
	got, err := repo.LinkToAccount(t.Context(), link, tx)

	// then
	require.NoError(t, err)
	assert.Same(t, linked, got)
}

func TestUserIdentityRepositoryLinkToAccount_RefusesBeforeWritingAnything(t *testing.T) {
	tests := []struct {
		name      string
		automatic bool
		account   model.User
		role      role.Role
		held      bool
		want      error
	}{
		{name: "a banned account", automatic: true, account: model.User{Email: "owner@gmail.com", EmailVerified: true, BannedAt: new(time.Now())}, want: ErrAccountBanned},
		{name: "a banned admin linking from settings", automatic: false, account: model.User{Email: "owner@gmail.com", EmailVerified: true, BannedAt: new(time.Now())}, role: role.Admin, want: ErrAccountBanned},
		{name: "an address changed since the lookup", automatic: true, account: model.User{Email: "moved@gmail.com", EmailVerified: true}, want: ErrAccountChanged},
		{name: "an address no longer verified", automatic: true, account: model.User{Email: "owner@gmail.com", EmailVerified: false}, want: ErrAccountChanged},
		{name: "an account that already holds a Google identity", automatic: true, account: model.User{Email: "owner@gmail.com", EmailVerified: true}, held: true, want: ErrAlreadyLinked},
		{name: "a settings link beside a Google identity already held", automatic: false, account: model.User{Email: "owner@gmail.com", EmailVerified: true}, held: true, want: ErrAlreadyLinked},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given no Create or session expectations, so any write fails the test
			repo, mocks := newMockedIdentityRepo(t)
			tx := new(sql.Tx)
			inTx := []*sql.Tx{tx}
			userID := uuid.New()
			account := tc.account
			account.ID = userID
			mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&account, nil).Once()
			if account.BannedAt != nil {
				mocks.roles.EXPECT().GetRole(mock.Anything, userID, inTx).Return(tc.role, nil).Once()
			}
			if tc.held {
				mocks.identities.EXPECT().GetForUser(mock.Anything, spec.UserIdentityKey{UserID: userID, Provider: model.ProviderGoogle}, inTx).Return(&model.UserIdentity{UserID: userID, Subject: "an-older-google-account"}, nil).Once()
			}

			// when
			got, err := repo.LinkToAccount(t.Context(), testLink(userID, tc.automatic), tx)

			// then
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestUserIdentityRepositoryLinkToAccount_ABanDoesNotHoldTheSuperAdmin(t *testing.T) {
	tests := []struct {
		name      string
		automatic bool
	}{
		{name: "an automatic link", automatic: true},
		{name: "a link from settings", automatic: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a super admin whose account row carries a ban
			repo, mocks := newMockedIdentityRepo(t)
			tx := new(sql.Tx)
			inTx := []*sql.Tx{tx}
			userID := uuid.New()
			link := testLink(userID, tc.automatic)
			linked := &model.UserIdentity{UserID: userID, Provider: model.ProviderGoogle, Subject: googleSubject}
			mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, Email: "owner@gmail.com", EmailVerified: true, BannedAt: new(time.Now())}, nil).Once()
			mocks.roles.EXPECT().GetRole(mock.Anything, userID, inTx).Return(role.SuperAdmin, nil).Once()
			mocks.identities.EXPECT().GetForUser(mock.Anything, spec.UserIdentityKey{UserID: userID, Provider: model.ProviderGoogle}, inTx).Return(nil, dao.ErrNotFound).Once()
			mocks.identities.EXPECT().Create(mock.Anything, link.Identity, inTx).Return(linked, nil).Once()
			if tc.automatic {
				mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(nil).Once()
			}

			// when
			got, err := repo.LinkToAccount(t.Context(), link, tx)

			// then
			require.NoError(t, err)
			assert.Same(t, linked, got)
		})
	}
}

func TestUserIdentityRepositoryLinkToAccount_ABanWhoseRoleCannotBeReadStillHolds(t *testing.T) {
	// given no Create or session expectations, so any write fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	readErr := errors.New("db down")
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, Email: "owner@gmail.com", EmailVerified: true, BannedAt: new(time.Now())}, nil).Once()
	mocks.roles.EXPECT().GetRole(mock.Anything, userID, inTx).Return("", readErr).Once()

	// when
	got, err := repo.LinkToAccount(t.Context(), testLink(userID, true), tx)

	// then the cause is reported and nothing is linked
	require.ErrorIs(t, err, readErr)
	assert.NotErrorIs(t, err, ErrAccountBanned)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryLinkToAccount_AnIdentityHeldByAnotherAccountIsInUse(t *testing.T) {
	// given a session mock with no expectations, so ending a session fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, Email: "owner@gmail.com", EmailVerified: true}, nil).Once()
	mocks.identities.EXPECT().GetForUser(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()
	mocks.identities.EXPECT().Create(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrConflict).Once()

	// when
	got, err := repo.LinkToAccount(t.Context(), testLink(userID, true), tx)

	// then
	require.ErrorIs(t, err, ErrIdentityInUse)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryLinkToAccount_ReportsAFailedIdentityRead(t *testing.T) {
	// given
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	readErr := errors.New("db down")
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, Email: "owner@gmail.com", EmailVerified: true}, nil).Once()
	mocks.identities.EXPECT().GetForUser(mock.Anything, mock.Anything, inTx).Return(nil, readErr).Once()

	// when
	got, err := repo.LinkToAccount(t.Context(), testLink(userID, true), tx)

	// then a failed read is never taken for an account without an identity
	require.ErrorIs(t, err, readErr)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryLinkToAccount_ReportsAFailedSessionPurge(t *testing.T) {
	// given
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	purgeErr := errors.New("db down")
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, Email: "owner@gmail.com", EmailVerified: true}, nil).Once()
	mocks.identities.EXPECT().GetForUser(mock.Anything, mock.Anything, inTx).Return(nil, dao.ErrNotFound).Once()
	mocks.identities.EXPECT().Create(mock.Anything, mock.Anything, inTx).Return(&model.UserIdentity{}, nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(purgeErr).Once()

	// when
	got, err := repo.LinkToAccount(t.Context(), testLink(userID, true), tx)

	// then an automatic link that leaves the squatter's session alive is not reported as done
	require.ErrorIs(t, err, purgeErr)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryLinkToAccount_AnUnknownAccountIsNotFound(t *testing.T) {
	// given
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	got, err := repo.LinkToAccount(t.Context(), testLink(userID, true), tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryUnlink_RemovesTheIdentityUnderTheAccountLock(t *testing.T) {
	// given
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	key := spec.UserIdentityKey{UserID: uuid.New(), Provider: model.ProviderGoogle}
	removal := spec.UserIdentityRemoval{UserIdentityKey: key, PasswordCounts: true}
	held := &model.UserIdentity{UserID: key.UserID, Provider: model.ProviderGoogle, Subject: googleSubject, Email: "owner@gmail.com"}
	var order []string
	mocks.users.EXPECT().GetForUpdate(mock.Anything, key.UserID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (*model.User, error) {
		order = append(order, "lock")
		return &model.User{ID: key.UserID}, nil
	}).Once()
	mocks.identities.EXPECT().GetForUser(mock.Anything, key, inTx).RunAndReturn(func(context.Context, spec.UserIdentityKey, ...*sql.Tx) (*model.UserIdentity, error) {
		order = append(order, "read")
		return held, nil
	}).Once()
	mocks.identities.EXPECT().DeleteUnlessLast(mock.Anything, removal, inTx).RunAndReturn(func(context.Context, spec.UserIdentityRemoval, ...*sql.Tx) error {
		order = append(order, "delete")
		return nil
	}).Once()

	// when
	got, err := repo.Unlink(t.Context(), removal, tx)

	// then
	require.NoError(t, err)
	assert.Same(t, held, got)
	assert.Equal(t, []string{"lock", "read", "delete"}, order)
}

func TestUserIdentityRepositoryUnlink_AnAccountWithoutTheIdentityIsNotFound(t *testing.T) {
	// given no DeleteUnlessLast expectation, so a delete fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	key := spec.UserIdentityKey{UserID: uuid.New(), Provider: model.ProviderGoogle}
	mocks.users.EXPECT().GetForUpdate(mock.Anything, key.UserID, inTx).Return(&model.User{ID: key.UserID}, nil).Once()
	mocks.identities.EXPECT().GetForUser(mock.Anything, key, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	got, err := repo.Unlink(t.Context(), spec.UserIdentityRemoval{UserIdentityKey: key, PasswordCounts: true}, tx)

	// then a repeated unlink is a 404 rather than a 409
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.NotErrorIs(t, err, dao.ErrLastSignInMethod)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryUnlink_RefusesTheLastWayToSignIn(t *testing.T) {
	// given
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	key := spec.UserIdentityKey{UserID: uuid.New(), Provider: model.ProviderGoogle}
	mocks.users.EXPECT().GetForUpdate(mock.Anything, key.UserID, inTx).Return(&model.User{ID: key.UserID}, nil).Once()
	removal := spec.UserIdentityRemoval{UserIdentityKey: key, PasswordCounts: true}
	mocks.identities.EXPECT().GetForUser(mock.Anything, key, inTx).Return(&model.UserIdentity{UserID: key.UserID}, nil).Once()
	mocks.identities.EXPECT().DeleteUnlessLast(mock.Anything, removal, inTx).Return(dao.ErrLastSignInMethod).Once()

	// when
	got, err := repo.Unlink(t.Context(), removal, tx)

	// then
	require.ErrorIs(t, err, dao.ErrLastSignInMethod)
	assert.Nil(t, got)
}

func TestUserIdentityRepositoryUnlink_AnUnknownAccountIsNotFound(t *testing.T) {
	// given no identity expectations, so reading or deleting one fails the test
	repo, mocks := newMockedIdentityRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	key := spec.UserIdentityKey{UserID: uuid.New(), Provider: model.ProviderGoogle}
	mocks.users.EXPECT().GetForUpdate(mock.Anything, key.UserID, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	got, err := repo.Unlink(t.Context(), spec.UserIdentityRemoval{UserIdentityKey: key, PasswordCounts: true}, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Nil(t, got)
}
