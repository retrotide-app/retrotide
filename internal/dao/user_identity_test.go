package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func linkIdentity(t *testing.T, repos *repository.Repositories, userID uuid.UUID, subject string) *model.UserIdentity {
	t.Helper()

	linked, err := repos.UserIdentity.Create(t.Context(), spec.NewUserIdentity{UserID: userID, Provider: model.ProviderGoogle, Subject: subject, Email: subject + "@gmail.com"})
	require.NoError(t, err)

	return linked
}

func googleKey(userID uuid.UUID) spec.UserIdentityKey {
	return spec.UserIdentityKey{UserID: userID, Provider: model.ProviderGoogle}
}

func googleSubject(subject string) spec.IdentitySubject {
	return spec.IdentitySubject{Provider: model.ProviderGoogle, Subject: subject}
}

func identityRegistration(email string, subject string) spec.IdentityRegistration {
	return spec.IdentityRegistration{
		User:     spec.NewUser{Email: email, DisplayName: "Newcomer", EmailVerified: true},
		Provider: model.ProviderGoogle,
		Subject:  subject,
	}
}

func TestUserIdentityDAO_CreateAndReadBack(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())

	// when
	created, err := repos.UserIdentity.Create(t.Context(), spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: "Owner@Gmail.com"})

	// then the identity is found by its subject and by its user, with the address as Google sent it
	require.NoError(t, err)
	assert.Equal(t, model.ProviderGoogle, created.Provider)
	assert.Equal(t, "sub-1", created.Subject)
	assert.Equal(t, user.ID, created.UserID)
	assert.Equal(t, "Owner@Gmail.com", created.Email)
	assert.WithinDuration(t, time.Now(), created.CreatedAt, time.Minute)
	bySubject, err := repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-1"))
	require.NoError(t, err)
	assert.Equal(t, *created, *bySubject)
	forUser, err := repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.NoError(t, err)
	assert.Equal(t, *created, *forUser)
	listed, err := repos.UserIdentity.ListForUser(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, []model.UserIdentity{*created}, listed)
}

func TestUserIdentityDAO_GetBySubject_UnknownSubjectIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, user.ID, "sub-1")

	// when
	_, err := repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-2"))

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserIdentityDAO_ListForUser_AnAccountWithoutIdentitiesListsNone(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	listed, err := repos.UserIdentity.ListForUser(t.Context(), user.ID)

	// then
	require.NoError(t, err)
	assert.Empty(t, listed)
}

func TestUserIdentityDAO_Create_ASubjectLinkedToAnotherUserConflicts(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, owner.ID, "sub-1")

	// when
	_, err := repos.UserIdentity.Create(t.Context(), spec.NewUserIdentity{UserID: other.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: "other@gmail.com"})

	// then the identity is never moved
	require.ErrorIs(t, err, dao.ErrConflict)
	got, err := repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-1"))
	require.NoError(t, err)
	assert.Equal(t, owner.ID, got.UserID)
}

func TestUserIdentityDAO_Create_ASecondGoogleIdentityForOneUserConflicts(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, user.ID, "sub-1")

	// when
	_, err := repos.UserIdentity.Create(t.Context(), spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "sub-2", Email: "second@gmail.com"})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	listed, err := repos.UserIdentity.ListForUser(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "sub-1", listed[0].Subject)
}

func TestUserIdentityDAO_AnotherUsersIdentityCannotBeReadOrUnlinked(t *testing.T) {
	// given an identity linked to user A
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	intruder := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, owner.ID, "sub-1")

	// when user B's ID is used to reach it
	_, readErr := repos.UserIdentity.GetForUser(t.Context(), googleKey(intruder.ID))
	listed, listErr := repos.UserIdentity.ListForUser(t.Context(), intruder.ID)
	_, unlinkErr := repos.UserIdentity.Unlink(t.Context(), googleKey(intruder.ID))

	// then nothing is found, and A's identity is unchanged
	require.ErrorIs(t, readErr, dao.ErrNotFound)
	require.NoError(t, listErr)
	assert.Empty(t, listed)
	require.ErrorIs(t, unlinkErr, dao.ErrNotFound)
	got, err := repos.UserIdentity.GetForUser(t.Context(), googleKey(owner.ID))
	require.NoError(t, err)
	assert.Equal(t, "sub-1", got.Subject)
}

func TestUserIdentityDAO_DeleteAllForUser_LeavesOtherUsers(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, user.ID, "sub-1")
	linkIdentity(t, repos, other.ID, "sub-2")

	// when
	removed, err := repos.UserIdentity.DeleteAllForUser(t.Context(), user.ID)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), removed)
	_, err = repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, err = repos.UserIdentity.GetForUser(t.Context(), googleKey(other.ID))
	require.NoError(t, err)
}

func TestUserIdentityDAO_DeletingTheUserRemovesTheirIdentitiesAndLinkFlows(t *testing.T) {
	// given an account with a Google identity and an unfinished link flow
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	bystander := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, user.ID, "sub-1")
	linkIdentity(t, repos, bystander.ID, "sub-2")
	session := daotest.CreateSession(t, repos, user.ID)
	require.NoError(t, repos.OIDCFlow.Create(t.Context(), linkFlow(user.ID, session)))

	// when
	require.NoError(t, repos.User.Delete(t.Context(), user.ID))

	// then
	var identities, flows int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM user_identities WHERE user_id = $1`, user.ID).Scan(&identities))
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM oidc_flows`).Scan(&flows))
	assert.Zero(t, identities)
	assert.Zero(t, flows)
	_, err := repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-2"))
	require.NoError(t, err)
}

func TestUserIdentityRepository_Unlink_BesideAPasswordRemovesTheIdentity(t *testing.T) {
	// given an account that also signs in with a password
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	linked := linkIdentity(t, repos, user.ID, "sub-1")

	// when
	removed, err := repos.UserIdentity.Unlink(t.Context(), googleKey(user.ID))

	// then
	require.NoError(t, err)
	assert.Equal(t, *linked, *removed)
	_, err = repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-1"))
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserIdentityRepository_Unlink_APasswordlessAccountKeepsItsOnlyIdentity(t *testing.T) {
	// given an account created through Google, with no password
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithoutPassword(), daotest.WithVerifiedEmail())
	linkIdentity(t, repos, user.ID, "sub-1")

	// when
	_, err := repos.UserIdentity.Unlink(t.Context(), googleKey(user.ID))

	// then
	require.ErrorIs(t, err, dao.ErrLastSignInMethod)
	got, err := repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.NoError(t, err)
	assert.Equal(t, "sub-1", got.Subject)
}

func TestUserIdentityRepository_Unlink_SucceedsOnceAPasswordIsSet(t *testing.T) {
	// given a passwordless account that has since set a password through the reset link
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithoutPassword(), daotest.WithVerifiedEmail())
	linkIdentity(t, repos, user.ID, "sub-1")
	_, err := repos.UserIdentity.Unlink(t.Context(), googleKey(user.ID))
	require.ErrorIs(t, err, dao.ErrLastSignInMethod)
	resetHash := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: resetHash, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))
	_, err = repos.User.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: resetHash, PasswordHash: "reset-hash"})
	require.NoError(t, err)

	// when
	_, err = repos.UserIdentity.Unlink(t.Context(), googleKey(user.ID))

	// then
	require.NoError(t, err)
	listed, err := repos.UserIdentity.ListForUser(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
}

func TestUserIdentityRepository_Unlink_AProviderTheAccountHasNoIdentityForIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithoutPassword())

	// when
	_, err := repos.UserIdentity.Unlink(t.Context(), googleKey(user.ID))

	// then a repeated unlink is a 404 rather than a 409
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.NotErrorIs(t, err, dao.ErrLastSignInMethod)
}

func TestUserIdentityRepository_Unlink_AnUnknownAccountIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	_, err := repos.UserIdentity.Unlink(t.Context(), googleKey(uuid.New()))

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserIdentityRepository_RegisterWithIdentity_CreatesAVerifiedPasswordlessAccount(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos)

	// when
	created, err := repos.UserIdentity.RegisterWithIdentity(t.Context(), identityRegistration("newcomer@gmail.com", "sub-1"))

	// then the account and its identity were written together, with no role
	require.NoError(t, err)
	got, err := repos.User.GetByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "newcomer@gmail.com", got.Email)
	assert.True(t, got.EmailVerified)
	assert.False(t, got.HasPassword())
	linked, err := repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-1"))
	require.NoError(t, err)
	assert.Equal(t, created.ID, linked.UserID)
	assert.Equal(t, "newcomer@gmail.com", linked.Email)
	granted, err := repos.Role.GetRole(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Empty(t, granted)
}

func TestUserIdentityRepository_RegisterWithIdentity_AnIdentityAlreadyLinkedWritesNoAccount(t *testing.T) {
	// given the Google identity already signs in another account
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, owner.ID, "sub-1")

	// when
	_, err := repos.UserIdentity.RegisterWithIdentity(t.Context(), identityRegistration("second@gmail.com", "sub-1"))

	// then
	require.ErrorIs(t, err, repository.ErrIdentityInUse)
	_, err = repos.User.GetByEmail(t.Context(), "second@gmail.com")
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserIdentityRepository_RegisterWithIdentity_ATakenAddressConflicts(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos, daotest.WithEmail("taken@gmail.com"))

	// when
	_, err := repos.UserIdentity.RegisterWithIdentity(t.Context(), identityRegistration("TAKEN@gmail.com", "sub-1"))

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.NotErrorIs(t, err, repository.ErrIdentityInUse)
	_, err = repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-1"))
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestUserIdentityRepository_RegisterWithIdentity_TheFirstAccountBecomesSuperAdmin(t *testing.T) {
	// given an empty users table
	repos := daotest.NewRepos(t)
	registration := identityRegistration("founder@gmail.com", "sub-1")
	registration.FirstAccount = true

	// when
	created, err := repos.UserIdentity.RegisterWithIdentity(t.Context(), registration)

	// then
	require.NoError(t, err)
	got, err := repos.Role.GetRole(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, role.SuperAdmin, got)
	stored, err := repos.User.GetByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.True(t, stored.EmailVerified)
}

func TestUserIdentityRepository_RegisterWithIdentity_AFirstAccountClaimAfterAnyAccountWritesNothing(t *testing.T) {
	// given an account already exists
	repos := daotest.NewRepos(t)
	daotest.CreateUser(t, repos)
	registration := identityRegistration("latecomer@gmail.com", "sub-1")
	registration.FirstAccount = true

	// when
	_, err := repos.UserIdentity.RegisterWithIdentity(t.Context(), registration)

	// then
	require.ErrorIs(t, err, repository.ErrNotFirstAccount)
	_, err = repos.User.GetByEmail(t.Context(), "latecomer@gmail.com")
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Zero(t, countRows(t, repos, "user_identities"))
	assert.Zero(t, countRows(t, repos, "user_roles"))
}

func TestUserIdentityRepository_LinkToAccount_AnAutomaticLinkEndsEveryOtherSession(t *testing.T) {
	// given a verified account signed in on two browsers
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("owner@gmail.com"), daotest.WithVerifiedEmail())
	first := daotest.CreateSession(t, repos, user.ID)
	second := daotest.CreateSession(t, repos, user.ID)
	bystander := daotest.CreateUser(t, repos)
	foreign := daotest.CreateSession(t, repos, bystander.ID)
	link := spec.IdentityLink{
		Identity:     spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: "Owner@gmail.com"},
		Automatic:    true,
		MatchedEmail: user.Email,
	}

	// when
	linked, err := repos.UserIdentity.LinkToAccount(t.Context(), link)

	// then
	require.NoError(t, err)
	assert.Equal(t, user.ID, linked.UserID)
	assert.Equal(t, "Owner@gmail.com", linked.Email)
	_, firstErr := repos.Session.Get(t.Context(), first)
	_, secondErr := repos.Session.Get(t.Context(), second)
	_, foreignErr := repos.Session.Get(t.Context(), foreign)
	require.ErrorIs(t, firstErr, dao.ErrNotFound)
	require.ErrorIs(t, secondErr, dao.ErrNotFound)
	require.NoError(t, foreignErr)
}

func TestUserIdentityRepository_LinkToAccount_ALinkFromSettingsKeepsTheSession(t *testing.T) {
	// given a signed-in account linking a Google account on another address
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("me@work.example"), daotest.WithVerifiedEmail())
	session := daotest.CreateSession(t, repos, user.ID)
	link := spec.IdentityLink{Identity: spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: "me@gmail.com"}}

	// when
	_, err := repos.UserIdentity.LinkToAccount(t.Context(), link)

	// then
	require.NoError(t, err)
	_, err = repos.Session.Get(t.Context(), session)
	require.NoError(t, err)
	got, err := repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.NoError(t, err)
	assert.Equal(t, "me@gmail.com", got.Email)
}

func TestUserIdentityRepository_LinkToAccount_AnAccountChangedSinceTheLookupIsNotLinked(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, repos *repository.Repositories, userID uuid.UUID)
	}{
		{
			name: "its address moved",
			change: func(t *testing.T, repos *repository.Repositories, userID uuid.UUID) {
				require.NoError(t, repos.User.SetVerifiedEmail(t.Context(), spec.UserVerifiedEmailUpdate{UserID: userID, Email: "moved@gmail.com"}))
			},
		},
		{
			name: "its address is no longer verified",
			change: func(t *testing.T, repos *repository.Repositories, userID uuid.UUID) {
				require.NoError(t, repos.User.MarkEmailUnverified(t.Context(), userID))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given the service matched a verified account, which then changed
			repos := daotest.NewRepos(t)
			user := daotest.CreateUser(t, repos, daotest.WithEmail("owner@gmail.com"), daotest.WithVerifiedEmail())
			session := daotest.CreateSession(t, repos, user.ID)
			tc.change(t, repos, user.ID)
			link := spec.IdentityLink{
				Identity:     spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: "owner@gmail.com"},
				Automatic:    true,
				MatchedEmail: user.Email,
			}

			// when
			_, err := repos.UserIdentity.LinkToAccount(t.Context(), link)

			// then nothing was linked and no session ended
			require.ErrorIs(t, err, repository.ErrAccountChanged)
			assert.Zero(t, countRows(t, repos, "user_identities"))
			_, err = repos.Session.Get(t.Context(), session)
			require.NoError(t, err)
		})
	}
}

func TestUserIdentityRepository_LinkToAccount_ABannedAccountIsNotLinked(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("owner@gmail.com"), daotest.WithVerifiedEmail())
	require.NoError(t, repos.User.Ban(t.Context(), spec.UserBan{UserID: user.ID, Reason: "spam"}))
	link := spec.IdentityLink{
		Identity:     spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: "owner@gmail.com"},
		Automatic:    true,
		MatchedEmail: user.Email,
	}

	// when
	_, err := repos.UserIdentity.LinkToAccount(t.Context(), link)

	// then
	require.ErrorIs(t, err, repository.ErrAccountBanned)
	assert.Zero(t, countRows(t, repos, "user_identities"))
}

func TestUserIdentityRepository_LinkToAccount_ABannedSuperAdminIsStillLinked(t *testing.T) {
	// given the super admin, banned by hand in the database
	repos := daotest.NewRepos(t)
	registration := newRegistration("founder@gmail.com")
	registration.FirstAccount = true
	founder, err := repos.User.RegisterAccount(t.Context(), registration)
	require.NoError(t, err)
	require.NoError(t, repos.User.Ban(t.Context(), spec.UserBan{UserID: founder.ID, Reason: "set by hand"}))
	link := spec.IdentityLink{
		Identity:     spec.NewUserIdentity{UserID: founder.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: founder.Email},
		Automatic:    true,
		MatchedEmail: founder.Email,
	}

	// when
	_, err = repos.UserIdentity.LinkToAccount(t.Context(), link)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, repos, "user_identities"))
}

func TestUserIdentityRepository_LinkToAccount_AnAccountWithAGoogleIdentityCannotLinkASecond(t *testing.T) {
	// given an account whose address a new Google account now uses
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithEmail("owner@work.example"), daotest.WithVerifiedEmail())
	session := daotest.CreateSession(t, repos, user.ID)
	linkIdentity(t, repos, user.ID, "old-sub")
	link := spec.IdentityLink{
		Identity:     spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "new-sub", Email: "owner@work.example"},
		Automatic:    true,
		MatchedEmail: user.Email,
	}

	// when
	_, err := repos.UserIdentity.LinkToAccount(t.Context(), link)

	// then the old identity is untouched and the account keeps its session
	require.ErrorIs(t, err, repository.ErrAlreadyLinked)
	got, err := repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.NoError(t, err)
	assert.Equal(t, "old-sub", got.Subject)
	_, err = repos.Session.Get(t.Context(), session)
	require.NoError(t, err)
}

func TestUserIdentityRepository_LinkToAccount_AnIdentityLinkedElsewhereIsNeverMoved(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	session := daotest.CreateSession(t, repos, user.ID)
	linkIdentity(t, repos, owner.ID, "sub-1")
	link := spec.IdentityLink{Identity: spec.NewUserIdentity{UserID: user.ID, Provider: model.ProviderGoogle, Subject: "sub-1", Email: "sub-1@gmail.com"}}

	// when
	_, err := repos.UserIdentity.LinkToAccount(t.Context(), link)

	// then
	require.ErrorIs(t, err, repository.ErrIdentityInUse)
	got, err := repos.UserIdentity.GetBySubject(t.Context(), googleSubject("sub-1"))
	require.NoError(t, err)
	assert.Equal(t, owner.ID, got.UserID)
	_, err = repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, err = repos.Session.Get(t.Context(), session)
	require.NoError(t, err)
}

func TestUserRepository_ResetPassword_AnAccountNeverVerifiedLosesItsLinkedIdentities(t *testing.T) {
	// given an unverified account holding an identity linked before anyone proved the mailbox
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	bystander := daotest.CreateUser(t, repos)
	linkIdentity(t, repos, user.ID, "sub-1")
	linkIdentity(t, repos, bystander.ID, "sub-2")
	resetHash := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: resetHash, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	_, err := repos.User.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: resetHash, PasswordHash: "reset-hash"})

	// then
	require.NoError(t, err)
	_, err = repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, err = repos.UserIdentity.GetForUser(t.Context(), googleKey(bystander.ID))
	require.NoError(t, err)
}

func TestUserRepository_ResetPassword_AVerifiedAccountKeepsItsLinkedIdentities(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	linkIdentity(t, repos, user.ID, "sub-1")
	resetHash := uuid.NewString()
	require.NoError(t, repos.PasswordReset.Issue(t.Context(), spec.NewPasswordReset{TokenHash: resetHash, UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}))

	// when
	_, err := repos.User.ResetPassword(t.Context(), spec.PasswordReset{TokenHash: resetHash, PasswordHash: "reset-hash"})

	// then the confirmation can list the link with a Remove action
	require.NoError(t, err)
	got, err := repos.UserIdentity.GetForUser(t.Context(), googleKey(user.ID))
	require.NoError(t, err)
	assert.Equal(t, "sub-1", got.Subject)
}
