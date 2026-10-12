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
	userRepoMocks struct {
		users         *dao.MockUserDAO
		roles         *MockRoleRepository
		sessions      *MockSessionRepository
		verifications *MockEmailVerificationRepository
		resets        *MockPasswordResetRepository
		identities    *MockUserIdentityRepository
		organisations *dao.MockOrganisationDAO
		twoFactor     *dao.MockTwoFactorDAO
	}
)

func newMockedUserRepo(t *testing.T) (UserRepository, userRepoMocks) {
	t.Helper()

	mocks := userRepoMocks{
		users:         dao.NewMockUserDAO(t),
		roles:         NewMockRoleRepository(t),
		sessions:      NewMockSessionRepository(t),
		verifications: NewMockEmailVerificationRepository(t),
		resets:        NewMockPasswordResetRepository(t),
		identities:    NewMockUserIdentityRepository(t),
		organisations: dao.NewMockOrganisationDAO(t),
		twoFactor:     dao.NewMockTwoFactorDAO(t),
	}

	deps := UserRepoDeps{
		Roles:         mocks.roles,
		Sessions:      mocks.sessions,
		Verifications: mocks.verifications,
		Resets:        mocks.resets,
		Identities:    mocks.identities,
		Organisations: mocks.organisations,
		TwoFactor:     mocks.twoFactor,
	}

	return NewUserRepo(nil, mocks.users, deps), mocks
}

func testRegistration() spec.NewRegistration {
	return spec.NewRegistration{
		User:                  spec.NewUser{Email: "newcomer@example.com", PasswordHash: new("hash"), DisplayName: "Newcomer"},
		VerificationHash:      "verification-hash",
		VerificationExpiresAt: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
	}
}

func TestUserRepositoryRegisterAccount_WritesEveryPartInTheSameTransaction(t *testing.T) {
	// given a session mock with no expectations, so signing the new account in fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testRegistration()
	created := &model.User{ID: uuid.New(), Email: registration.User.Email}
	mocks.users.EXPECT().Create(mock.Anything, registration.User, inTx).Return(created, nil).Once()
	mocks.verifications.EXPECT().Create(mock.Anything, spec.NewEmailVerification{TokenHash: "verification-hash", UserID: created.ID, ExpiresAt: registration.VerificationExpiresAt}, inTx).Return(nil).Once()

	// when
	got, err := repo.RegisterAccount(t.Context(), registration, tx)

	// then
	require.NoError(t, err)
	assert.Same(t, created, got)
}

func TestUserRepositoryRegisterAccount_StopsAtATakenEmail(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testRegistration()
	mocks.users.EXPECT().Create(mock.Anything, registration.User, inTx).Return(nil, dao.ErrConflict).Once()

	// when
	got, err := repo.RegisterAccount(t.Context(), registration, tx)

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Nil(t, got)
}

func TestUserRepositoryRegisterAccount_TheFirstAccountClaimsSuperAdminUnderTheLock(t *testing.T) {
	// given an empty users table, and a verification mock with no expectations, so issuing a link fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testRegistration()
	registration.FirstAccount = true
	verified := registration.User
	verified.EmailVerified = true
	created := &model.User{ID: uuid.New(), EmailVerified: true}
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

	// when
	got, err := repo.RegisterAccount(t.Context(), registration, tx)

	// then the emptiness check runs only once the lock is held, and the account is created verified
	require.NoError(t, err)
	assert.Same(t, created, got)
	assert.Equal(t, []string{"lock", "check", "create"}, order)
}

func TestUserRepositoryRegisterAccount_AFirstAccountClaimLosesToAnExistingAccount(t *testing.T) {
	// given another account committed first, and no create expectation, so writing one fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testRegistration()
	registration.FirstAccount = true
	mocks.users.EXPECT().LockFirstAccountClaim(mock.Anything, inTx).Return(nil).Once()
	mocks.users.EXPECT().AnyExists(mock.Anything, inTx).Return(true, nil).Once()

	// when
	got, err := repo.RegisterAccount(t.Context(), registration, tx)

	// then
	require.ErrorIs(t, err, ErrNotFirstAccount)
	assert.Nil(t, got)
}

func TestUserRepositoryRegisterAccount_AnOrdinaryAccountNeverTakesTheLockOrARole(t *testing.T) {
	// given role mocks and lock expectations left unset, so either fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	registration := testRegistration()
	created := &model.User{ID: uuid.New()}
	mocks.users.EXPECT().Create(mock.Anything, registration.User, inTx).Return(created, nil).Once()
	mocks.verifications.EXPECT().Create(mock.Anything, mock.Anything, inTx).Return(nil).Once()

	// when
	got, err := repo.RegisterAccount(t.Context(), registration, tx)

	// then
	require.NoError(t, err)
	assert.Same(t, created, got)
}

func TestUserRepositoryUnverifyAccount_MarksTheAddressUnverifiedAndEndsEverySessionTogether(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.users.EXPECT().MarkEmailUnverified(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(nil).Once()

	// when
	err := repo.UnverifyAccount(t.Context(), userID, tx)

	// then
	require.NoError(t, err)
}

func TestUserRepositoryUnverifyAccount_EndsNoSessionForAnUnknownUser(t *testing.T) {
	// given a session mock with no expectations, so deleting sessions fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.users.EXPECT().MarkEmailUnverified(mock.Anything, userID, inTx).Return(dao.ErrNotFound).Once()

	// when
	err := repo.UnverifyAccount(t.Context(), userID, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserRepositoryUnverifyAccount_ReportsAFailedSessionPurge(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	purgeErr := errors.New("db down")
	mocks.users.EXPECT().MarkEmailUnverified(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(purgeErr).Once()

	// when
	err := repo.UnverifyAccount(t.Context(), userID, tx)

	// then an unverified account that could still hold a session is not reported as done
	require.ErrorIs(t, err, purgeErr)
}

func TestUserRepositoryBanAccount_BansAndEndsEverySessionTogether(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	ban := spec.UserBan{UserID: uuid.New(), BannedBy: uuid.New(), Reason: "spam"}
	mocks.users.EXPECT().Ban(mock.Anything, ban, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, ban.UserID, inTx).Return(nil).Once()

	// when
	err := repo.BanAccount(t.Context(), ban, tx)

	// then
	require.NoError(t, err)
}

func TestUserRepositoryBanAccount_EndsNoSessionForAnUnknownUser(t *testing.T) {
	// given a session mock with no expectations, so deleting sessions fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	ban := spec.UserBan{UserID: uuid.New(), Reason: "spam"}
	mocks.users.EXPECT().Ban(mock.Anything, ban, inTx).Return(dao.ErrNotFound).Once()

	// when
	err := repo.BanAccount(t.Context(), ban, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserRepositoryResetPassword_SetsThePasswordEndsEverySessionForgetsTrustedDevicesAndCancelsPendingEmailChanges(t *testing.T) {
	// given a verified account, and an identity mock with no expectations, so removing its links fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.resets.EXPECT().Consume(mock.Anything, "reset-hash", inTx).Return(&model.PasswordResetToken{TokenHash: "reset-hash", UserID: userID}, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, EmailVerified: true}, nil).Once()
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, spec.UserPasswordHashUpdate{UserID: userID, PasswordHash: "new-hash"}, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.twoFactor.EXPECT().DeleteTrustedDevicesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID, inTx).Return(nil).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "reset-hash", PasswordHash: "new-hash"}, tx)

	// then
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestUserRepositoryResetPassword_AnAccountNeverVerifiedLosesItsLinkedIdentities(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.resets.EXPECT().Consume(mock.Anything, "reset-hash", inTx).Return(&model.PasswordResetToken{TokenHash: "reset-hash", UserID: userID}, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, EmailVerified: false}, nil).Once()
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, spec.UserPasswordHashUpdate{UserID: userID, PasswordHash: "new-hash"}, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.twoFactor.EXPECT().DeleteTrustedDevicesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.identities.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(1, nil).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "reset-hash", PasswordHash: "new-hash"}, tx)

	// then the reset is the first proof of the mailbox, so no link made before it survives
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestUserRepositoryResetPassword_ReportsAFailedIdentityRemoval(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	removeErr := errors.New("db down")
	mocks.resets.EXPECT().Consume(mock.Anything, "reset-hash", inTx).Return(&model.PasswordResetToken{UserID: userID}, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID}, nil).Once()
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.twoFactor.EXPECT().DeleteTrustedDevicesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.identities.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(0, removeErr).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "reset-hash", PasswordHash: "new-hash"}, tx)

	// then a reset that leaves an untrusted link in place is not reported as done
	require.ErrorIs(t, err, removeErr)
	assert.Equal(t, uuid.Nil, got)
}

func TestUserRepositoryResetPassword_StopsWhenTheAccountCannotBeLocked(t *testing.T) {
	// given no password, session or identity expectations, so any write fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.resets.EXPECT().Consume(mock.Anything, "reset-hash", inTx).Return(&model.PasswordResetToken{UserID: userID}, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "reset-hash", PasswordHash: "new-hash"}, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, uuid.Nil, got)
}

func TestUserRepositoryResetPassword_ReportsAFailedPendingEmailCancellation(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	cancelErr := errors.New("db down")
	mocks.resets.EXPECT().Consume(mock.Anything, "reset-hash", inTx).Return(&model.PasswordResetToken{UserID: userID}, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, EmailVerified: true}, nil).Once()
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.twoFactor.EXPECT().DeleteTrustedDevicesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID, inTx).Return(cancelErr).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "reset-hash", PasswordHash: "new-hash"}, tx)

	// then a reset that leaves someone else's change of address alive is not reported as done
	require.ErrorIs(t, err, cancelErr)
	assert.Equal(t, uuid.Nil, got)
}

func TestUserRepositoryResetPassword_StopsAtATokenThatCannotBeUsed(t *testing.T) {
	// given user and session mocks with no expectations, so any write fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	mocks.resets.EXPECT().Consume(mock.Anything, "expired-hash", inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "expired-hash", PasswordHash: "new-hash"}, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Equal(t, uuid.Nil, got)
}

func TestUserRepositoryResetPassword_ReportsAFailedSessionPurge(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	purgeErr := errors.New("db down")
	mocks.resets.EXPECT().Consume(mock.Anything, "reset-hash", inTx).Return(&model.PasswordResetToken{UserID: userID}, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, EmailVerified: true}, nil).Once()
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(purgeErr).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "reset-hash", PasswordHash: "new-hash"}, tx)

	// then a reset that could not end the other sessions is not reported as done
	require.ErrorIs(t, err, purgeErr)
	assert.Equal(t, uuid.Nil, got)
}

func TestUserRepositoryResetPassword_ReportsAFailureToForgetTrustedDevices(t *testing.T) {
	// given a verification mock with no expectations, so going on past the failure fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	forgetErr := errors.New("db down")
	mocks.resets.EXPECT().Consume(mock.Anything, "reset-hash", inTx).Return(&model.PasswordResetToken{UserID: userID}, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID, EmailVerified: true}, nil).Once()
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.twoFactor.EXPECT().DeleteTrustedDevicesForUser(mock.Anything, userID, inTx).Return(forgetErr).Once()

	// when
	got, err := repo.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: "reset-hash", PasswordHash: "new-hash"}, tx)

	// then a reset that leaves a device able to skip the second factor is not reported as done
	require.ErrorIs(t, err, forgetErr)
	assert.Equal(t, uuid.Nil, got)
}

func TestUserRepositoryChangePassword_KeepsOnlyThePresentedSessionForgetsTrustedDevicesAndCancelsPendingEmailChanges(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, spec.UserPasswordHashUpdate{UserID: userID, PasswordHash: "changed-hash"}, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUserExcept(mock.Anything, spec.SessionDeletionExcept{UserID: userID, KeepToken: "current-token"}, inTx).Return(nil).Once()
	mocks.twoFactor.EXPECT().DeleteTrustedDevicesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID, inTx).Return(nil).Once()

	// when
	err := repo.ChangePassword(t.Context(), spec.PasswordChange{UserID: userID, PasswordHash: "changed-hash", KeepToken: "current-token"}, tx)

	// then
	require.NoError(t, err)
}

func TestUserRepositoryChangePassword_ReportsAFailedPendingEmailCancellation(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	cancelErr := errors.New("db down")
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	mocks.sessions.EXPECT().DeleteAllForUserExcept(mock.Anything, mock.Anything, inTx).Return(nil).Once()
	mocks.twoFactor.EXPECT().DeleteTrustedDevicesForUser(mock.Anything, userID, inTx).Return(nil).Once()
	mocks.verifications.EXPECT().DeletePendingEmailChangesForUser(mock.Anything, userID, inTx).Return(cancelErr).Once()

	// when
	err := repo.ChangePassword(t.Context(), spec.PasswordChange{UserID: userID, PasswordHash: "changed-hash", KeepToken: "current-token"}, tx)

	// then
	require.ErrorIs(t, err, cancelErr)
}

func TestUserRepositoryChangePassword_StopsWhenThePasswordCannotBeSet(t *testing.T) {
	// given a session mock with no expectations, so deleting sessions fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	mocks.users.EXPECT().SetPasswordHash(mock.Anything, mock.Anything, inTx).Return(dao.ErrNotFound).Once()

	// when
	err := repo.ChangePassword(t.Context(), spec.PasswordChange{UserID: uuid.New(), PasswordHash: "changed-hash", KeepToken: "current-token"}, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserRepositoryConfirmEmailVerification_VerifiesTheRegisteredAddress(t *testing.T) {
	// given a registration token, which carries no pending address
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	token := &model.EmailVerificationToken{TokenHash: "verify-hash", UserID: uuid.New()}
	mocks.verifications.EXPECT().Consume(mock.Anything, "verify-hash", inTx).Return(token, nil).Once()
	mocks.users.EXPECT().MarkEmailVerified(mock.Anything, token.UserID, inTx).Return(nil).Once()

	// when
	got, err := repo.ConfirmEmailVerification(t.Context(), "verify-hash", tx)

	// then
	require.NoError(t, err)
	assert.Same(t, token, got)
}

func TestUserRepositoryConfirmEmailVerification_SwapsInThePendingAddress(t *testing.T) {
	// given a change-of-address token, and no MarkEmailVerified expectation
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	token := &model.EmailVerificationToken{TokenHash: "change-hash", UserID: uuid.New(), PendingEmail: new("next@example.com")}
	mocks.verifications.EXPECT().Consume(mock.Anything, "change-hash", inTx).Return(token, nil).Once()
	mocks.users.EXPECT().SetVerifiedEmail(mock.Anything, spec.UserVerifiedEmailUpdate{UserID: token.UserID, Email: "next@example.com"}, inTx).Return(nil).Once()

	// when
	got, err := repo.ConfirmEmailVerification(t.Context(), "change-hash", tx)

	// then
	require.NoError(t, err)
	assert.Same(t, token, got)
}

func TestUserRepositoryConfirmEmailVerification_ReportsAnAddressTakenMeanwhile(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	token := &model.EmailVerificationToken{UserID: uuid.New(), PendingEmail: new("contested@example.com")}
	mocks.verifications.EXPECT().Consume(mock.Anything, "change-hash", inTx).Return(token, nil).Once()
	mocks.users.EXPECT().SetVerifiedEmail(mock.Anything, mock.Anything, inTx).Return(dao.ErrConflict).Once()

	// when
	got, err := repo.ConfirmEmailVerification(t.Context(), "change-hash", tx)

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Nil(t, got)
}

func TestUserRepositoryConfirmEmailVerification_StopsAtATokenThatCannotBeUsed(t *testing.T) {
	// given a user mock with no expectations, so any write fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	mocks.verifications.EXPECT().Consume(mock.Anything, "used-hash", inTx).Return(nil, dao.ErrNotFound).Once()

	// when
	got, err := repo.ConfirmEmailVerification(t.Context(), "used-hash", tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Nil(t, got)
}

func TestUserRepositoryDeleteAccount_LocksChecksAndReleasesTheOrganisationsBeforeDeleting(t *testing.T) {
	// given
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	var order []string
	mocks.organisations.EXPECT().LockAllOfMember(mock.Anything, userID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) ([]uuid.UUID, error) {
		order = append(order, "lock the organisations")
		return []uuid.UUID{uuid.New()}, nil
	}).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (*model.User, error) {
		order = append(order, "lock the account")
		return &model.User{ID: userID}, nil
	}).Once()
	mocks.organisations.EXPECT().ListOwnedWithOtherMembers(mock.Anything, userID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) ([]model.Organisation, error) {
		order = append(order, "check for shared organisations")
		return []model.Organisation{}, nil
	}).Once()
	mocks.organisations.EXPECT().SoftDeleteOwnedAlone(mock.Anything, userID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) (int64, error) {
		order = append(order, "delete the organisations owned alone")
		return 1, nil
	}).Once()
	mocks.users.EXPECT().Delete(mock.Anything, userID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) error {
		order = append(order, "delete the account")
		return nil
	}).Once()

	// when
	err := repo.DeleteAccount(t.Context(), userID, tx)

	// then the organisations are locked before the account, in the order an invite acceptance takes them
	require.NoError(t, err)
	assert.Equal(t, []string{"lock the organisations", "lock the account", "check for shared organisations", "delete the organisations owned alone", "delete the account"}, order)
}

func TestUserRepositoryDeleteAccount_RefusesTheOwnerOfASharedOrganisation(t *testing.T) {
	// given mocks with no delete expectations, so deleting anything fails the test
	repo, mocks := newMockedUserRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	userID := uuid.New()
	mocks.organisations.EXPECT().LockAllOfMember(mock.Anything, userID, inTx).Return(nil, nil).Once()
	mocks.users.EXPECT().GetForUpdate(mock.Anything, userID, inTx).Return(&model.User{ID: userID}, nil).Once()
	mocks.organisations.EXPECT().ListOwnedWithOtherMembers(mock.Anything, userID, inTx).Return([]model.Organisation{{ID: uuid.New(), Slug: "acme"}}, nil).Once()

	// when
	err := repo.DeleteAccount(t.Context(), userID, tx)

	// then
	require.ErrorIs(t, err, ErrOwnsSharedOrganisation)
	assert.ErrorContains(t, err, "acme")
}
