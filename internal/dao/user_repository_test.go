package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRegistration(email string) spec.NewRegistration {
	return spec.NewRegistration{
		User:                  spec.NewUser{Email: email, PasswordHash: new(daotest.TestPasswordHash), DisplayName: "Newcomer"},
		VerificationHash:      uuid.NewString(),
		VerificationExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func TestUserRepository_RegisterAccount_WritesTheWholeAccount(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	registration := newRegistration("newcomer@example.com")

	// when
	created, err := repos.User.RegisterAccount(t.Context(), registration)

	// then the account exists and can verify its address, but is not signed in
	require.NoError(t, err)
	assert.Equal(t, "newcomer@example.com", created.Email)
	assert.False(t, created.EmailVerified)
	got, err := repos.User.GetByEmail(t.Context(), "newcomer@example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Zero(t, countRows(t, repos, "sessions"))
	consumed, err := repos.User.ConfirmEmailVerification(t.Context(), registration.VerificationHash)
	require.NoError(t, err)
	assert.Equal(t, created.ID, consumed.UserID)
}

func TestUserRepository_RegisterAccount_ATakenEmailWritesNothing(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos, daotest.WithEmail("taken@example.com"))

	// when
	_, err := repos.User.RegisterAccount(t.Context(), newRegistration("TAKEN@example.com"))

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Equal(t, 1, countRows(t, repos, "users"))
	assert.Zero(t, countRows(t, repos, "email_verification_tokens"))
}

func TestUserRepository_ResetPassword_SetsThePasswordAndEndsEverySession(t *testing.T) {
	// given a user signed in on two browsers who asked for a reset
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	first := daotest.CreateSession(t, repos, user.ID)
	second := daotest.CreateSession(t, repos, user.ID)
	tokenHash := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	userID, err := repos.User.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: tokenHash, PasswordHash: "reset-hash"})

	// then
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.PasswordHash)
	assert.Equal(t, "reset-hash", *got.PasswordHash)
	_, firstErr := repos.Session.Get(t.Context(), first)
	_, secondErr := repos.Session.Get(t.Context(), second)
	require.ErrorIs(t, firstErr, dao.ErrNotFound)
	require.ErrorIs(t, secondErr, dao.ErrNotFound)
}

func issuePendingChange(t *testing.T, repos *repository.Repositories, userID uuid.UUID, address string) string {
	t.Helper()

	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{
		TokenHash:    tokenHash,
		UserID:       userID,
		PendingEmail: new(address),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}))

	return tokenHash
}

func assertAddressKept(t *testing.T, repos *repository.Repositories, userID uuid.UUID, address string) {
	t.Helper()

	got, err := repos.User.GetByID(t.Context(), userID)
	require.NoError(t, err)
	assert.Equal(t, address, got.Email)
	assert.True(t, got.EmailVerified)
}

func TestUserRepository_ResetPassword_CancelsAPendingChangeOfAddress(t *testing.T) {
	// given someone else queued a move to their own address before the owner reset the password
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("owner@example.com"), daotest.WithVerifiedEmail())
	pending := issuePendingChange(t, repos, user.ID, "attacker@example.com")
	resetHash := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: resetHash, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	_, err := repos.User.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: resetHash, PasswordHash: "reset-hash"})

	// then the queued link no longer swaps the address
	require.NoError(t, err)
	_, confirmErr := repos.User.ConfirmEmailVerification(t.Context(), pending)
	require.ErrorIs(t, confirmErr, dao.ErrNotFound)
	assertAddressKept(t, repos, user.ID, "owner@example.com")
}

func TestUserRepository_ResetPassword_AUsedOrExpiredTokenChangesNothing(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn time.Duration
		useFirst  bool
	}{
		{name: "a token already used", expiresIn: time.Hour, useFirst: true},
		{name: "a token past its hour", expiresIn: -time.Minute, useFirst: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)
			user := daotest.CreateUser(t, repos)
			session := daotest.CreateSession(t, repos, user.ID)
			tokenHash := uuid.NewString()
			require.NoError(t, repos.PasswordReset.Create(t.Context(), spec.NewPasswordReset{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(tc.expiresIn)}))
			if tc.useFirst {
				_, err := repos.PasswordReset.Consume(t.Context(), tokenHash)
				require.NoError(t, err)
			}

			// when
			_, err := repos.User.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: tokenHash, PasswordHash: "attacker-hash"})

			// then
			require.ErrorIs(t, err, dao.ErrNotFound)
			got, getErr := repos.User.GetByID(t.Context(), user.ID)
			require.NoError(t, getErr)
			require.NotNil(t, got.PasswordHash)
			assert.Equal(t, daotest.TestPasswordHash, *got.PasswordHash)
			_, sessionErr := repos.Session.Get(t.Context(), session)
			require.NoError(t, sessionErr)
		})
	}
}

func TestUserRepository_ChangePassword_KeepsOnlyTheCurrentSession(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	current := daotest.CreateSession(t, repos, user.ID)
	other := daotest.CreateSession(t, repos, user.ID)

	// when
	err := repos.User.ChangePassword(t.Context(), spec.PasswordChange{UserID: user.ID, PasswordHash: "changed-hash", KeepToken: current})

	// then
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.PasswordHash)
	assert.Equal(t, "changed-hash", *got.PasswordHash)
	_, currentErr := repos.Session.Get(t.Context(), current)
	_, otherErr := repos.Session.Get(t.Context(), other)
	require.NoError(t, currentErr)
	require.ErrorIs(t, otherErr, dao.ErrNotFound)
}

func TestUserRepository_ChangePassword_CancelsAPendingChangeOfAddress(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("owner@example.com"), daotest.WithVerifiedEmail())
	current := daotest.CreateSession(t, repos, user.ID)
	pending := issuePendingChange(t, repos, user.ID, "attacker@example.com")

	// when
	err := repos.User.ChangePassword(t.Context(), spec.PasswordChange{UserID: user.ID, PasswordHash: "changed-hash", KeepToken: current})

	// then
	require.NoError(t, err)
	_, confirmErr := repos.User.ConfirmEmailVerification(t.Context(), pending)
	require.ErrorIs(t, confirmErr, dao.ErrNotFound)
	assertAddressKept(t, repos, user.ID, "owner@example.com")
}

func TestUserRepository_ChangePassword_AMissingUserKeepsEverySession(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	bystander := daotest.CreateUser(t, repos)
	session := daotest.CreateSession(t, repos, bystander.ID)

	// when
	err := repos.User.ChangePassword(t.Context(), spec.PasswordChange{UserID: uuid.New(), PasswordHash: "changed-hash", KeepToken: "none"})

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, sessionErr := repos.Session.Get(t.Context(), session)
	require.NoError(t, sessionErr)
}

func TestUserRepository_ConfirmEmailVerification_VerifiesTheRegisteredAddress(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("fresh@example.com"))
	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(24 * time.Hour)}))

	// when
	consumed, err := repos.User.ConfirmEmailVerification(t.Context(), tokenHash)

	// then
	require.NoError(t, err)
	assert.False(t, consumed.ChangesEmail())
	got, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	assert.True(t, got.EmailVerified)
	assert.Equal(t, "fresh@example.com", got.Email)
}

func TestUserRepository_ConfirmEmailVerification_SwapsInThePendingAddress(t *testing.T) {
	// given a verified account that asked to move to a new address
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("old@example.com"), daotest.WithVerifiedEmail())
	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{
		TokenHash:    tokenHash,
		UserID:       user.ID,
		PendingEmail: new("new@example.com"),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}))
	before, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	require.Equal(t, "old@example.com", before.Email)
	require.True(t, before.EmailVerified)

	// when
	consumed, err := repos.User.ConfirmEmailVerification(t.Context(), tokenHash)

	// then the new address replaces the old one, verified, and the old one is free
	require.NoError(t, err)
	assert.True(t, consumed.ChangesEmail())
	after, err := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", after.Email)
	assert.True(t, after.EmailVerified)
	_, err = repos.User.GetByEmail(t.Context(), "old@example.com")
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserRepository_ConfirmEmailVerification_AnAddressTakenMeanwhileConflicts(t *testing.T) {
	// given a pending change to an address that another account registered before the link was used
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("mine@example.com"), daotest.WithVerifiedEmail())
	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{
		TokenHash:    tokenHash,
		UserID:       user.ID,
		PendingEmail: new("contested@example.com"),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}))
	daotest.CreateUser(t, repos, daotest.WithEmail("Contested@example.com"))

	// when
	_, err := repos.User.ConfirmEmailVerification(t.Context(), tokenHash)

	// then the account keeps its old address, and the token is not spent
	require.ErrorIs(t, err, dao.ErrConflict)
	got, getErr := repos.User.GetByID(t.Context(), user.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "mine@example.com", got.Email)
	assert.True(t, got.EmailVerified)
	_, retryErr := repos.User.ConfirmEmailVerification(t.Context(), tokenHash)
	require.ErrorIs(t, retryErr, dao.ErrConflict)
}

func TestUserRepository_ConfirmEmailVerification_AUsedTokenIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	tokenHash := uuid.NewString()
	require.NoError(t, repos.EmailVerification.Issue(t.Context(), spec.NewEmailVerification{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: time.Now().Add(24 * time.Hour)}))
	_, err := repos.User.ConfirmEmailVerification(t.Context(), tokenHash)
	require.NoError(t, err)

	// when
	_, err = repos.User.ConfirmEmailVerification(t.Context(), tokenHash)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}
