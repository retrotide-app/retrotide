package dao_test

import (
	"strings"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/token"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	challengeAttemptCap = 5
)

var (
	firstSecret  = strings.Repeat("A", 32)
	secondSecret = strings.Repeat("B", 32)
)

func newChallenge(userID uuid.UUID) spec.NewTwoFactorChallenge {
	return spec.NewTwoFactorChallenge{
		TokenHash: token.Hash(uuid.NewString()),
		UserID:    userID,
		ReturnTo:  "/o/acme",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

func newTrustedDevice(userID uuid.UUID) spec.NewTwoFactorTrustedDevice {
	return spec.NewTwoFactorTrustedDevice{
		TokenHash: token.Hash(uuid.NewString()),
		UserID:    userID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
}

func turnOnTwoFactor(t *testing.T, repos *repository.Repositories, userID uuid.UUID) {
	t.Helper()

	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: userID, Secret: firstSecret})
	require.NoError(t, err)

	_, err = repos.TwoFactor.Activate(t.Context(), spec.TwoFactorActivation{UserID: userID, Step: 100})
	require.NoError(t, err)
}

func deviceLive(t *testing.T, repos *repository.Repositories, device spec.NewTwoFactorTrustedDevice) bool {
	t.Helper()

	live, err := repos.TwoFactor.TrustedDeviceLive(t.Context(), spec.TwoFactorTrustedDeviceCheck{TokenHash: device.TokenHash, UserID: device.UserID})
	require.NoError(t, err)

	return live
}

func TestTwoFactorDAO_BeginStoresAPendingSecret(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	begun, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})

	// then
	require.NoError(t, err)
	assert.Equal(t, user.ID, begun.UserID)
	assert.Equal(t, firstSecret, begun.Secret)
	assert.Nil(t, begun.ConfirmedAt)
	assert.False(t, begun.Enabled())
	assert.Zero(t, begun.LastUsedStep)

	got, err := repos.TwoFactor.GetTOTP(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, firstSecret, got.Secret)
}

func TestTwoFactorDAO_GetIsNotFoundBeforeAnyEnrolment(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	_, err := repos.TwoFactor.GetTOTP(t.Context(), user.ID)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTwoFactorDAO_BeginningAgainWhilePendingReplacesTheSecret(t *testing.T) {
	// given a pending enrolment
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)

	// when
	again, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: secondSecret})

	// then
	require.NoError(t, err)
	assert.Equal(t, secondSecret, again.Secret)
	assert.False(t, again.Enabled())
	assert.Equal(t, 1, countRows(t, repos, "user_totp"))
}

func TestTwoFactorDAO_BeginningWhileConfirmedIsAConflict(t *testing.T) {
	// given a confirmed enrolment
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)
	_, err = repos.TwoFactor.ConfirmTOTP(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 100})
	require.NoError(t, err)

	// when
	_, err = repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: secondSecret})

	// then the confirmed secret stays
	require.ErrorIs(t, err, dao.ErrConflict)
	got, err := repos.TwoFactor.GetTOTP(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, firstSecret, got.Secret)
	assert.True(t, got.Enabled())
}

func TestTwoFactorDAO_ConfirmTurnsTheSecretOnAndSpendsTheStep(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)

	// when
	confirmed, err := repos.TwoFactor.ConfirmTOTP(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 100})

	// then
	require.NoError(t, err)
	require.NotNil(t, confirmed.ConfirmedAt)
	assert.True(t, confirmed.Enabled())
	assert.Equal(t, int64(100), confirmed.LastUsedStep)
	assert.WithinDuration(t, time.Now(), *confirmed.ConfirmedAt, time.Minute)
}

func TestTwoFactorDAO_ConfirmNeedsAPendingEnrolment(t *testing.T) {
	tests := []struct {
		name  string
		given func(t *testing.T, repos *repository.Repositories, userID uuid.UUID)
	}{
		{
			name:  "no enrolment at all",
			given: func(*testing.T, *repository.Repositories, uuid.UUID) {},
		},
		{
			name: "an enrolment already confirmed",
			given: func(t *testing.T, repos *repository.Repositories, userID uuid.UUID) {
				_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: userID, Secret: firstSecret})
				require.NoError(t, err)
				_, err = repos.TwoFactor.ConfirmTOTP(t.Context(), spec.TwoFactorStepUse{UserID: userID, Step: 50})
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)
			user := daotest.CreateUser(t, repos)
			tc.given(t, repos, user.ID)

			// when
			_, err := repos.TwoFactor.ConfirmTOTP(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 100})

			// then
			require.ErrorIs(t, err, dao.ErrNotFound)
		})
	}
}

func TestTwoFactorDAO_AStepIsUsedOnceAndNeverAnOlderOne(t *testing.T) {
	// given a confirmed enrolment that spent step 100
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)
	_, err = repos.TwoFactor.ConfirmTOTP(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 100})
	require.NoError(t, err)

	// when
	replayed := repos.TwoFactor.UseTOTPStep(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 100})
	older := repos.TwoFactor.UseTOTPStep(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 99})
	newer := repos.TwoFactor.UseTOTPStep(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 101})
	newerAgain := repos.TwoFactor.UseTOTPStep(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 101})

	// then
	require.ErrorIs(t, replayed, dao.ErrConflict)
	require.ErrorIs(t, older, dao.ErrConflict)
	require.NoError(t, newer)
	require.ErrorIs(t, newerAgain, dao.ErrConflict)
	got, err := repos.TwoFactor.GetTOTP(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(101), got.LastUsedStep)
}

func TestTwoFactorDAO_APendingSecretSpendsNoStep(t *testing.T) {
	// given an enrolment that was never confirmed
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)

	// when
	err = repos.TwoFactor.UseTOTPStep(t.Context(), spec.TwoFactorStepUse{UserID: user.ID, Step: 100})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
}

func TestTwoFactorDAO_DeleteRemovesTheSecretOnce(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)

	// when
	first := repos.TwoFactor.DeleteTOTP(t.Context(), user.ID)
	second := repos.TwoFactor.DeleteTOTP(t.Context(), user.ID)

	// then
	require.NoError(t, first)
	require.ErrorIs(t, second, dao.ErrNotFound)
	_, err = repos.TwoFactor.GetTOTP(t.Context(), user.ID)
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTwoFactorDAO_AChallengeTakesAttemptsUpToTheCap(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	challenge := newChallenge(user.ID)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), challenge))
	attempt := spec.TwoFactorChallengeAttempt{TokenHash: challenge.TokenHash, MaxAttempts: challengeAttemptCap}

	// when
	var counted []int
	for range challengeAttemptCap {
		got, err := repos.TwoFactor.AttemptChallenge(t.Context(), attempt)
		require.NoError(t, err)
		counted = append(counted, got.Attempts)
	}
	_, spent := repos.TwoFactor.AttemptChallenge(t.Context(), attempt)

	// then
	assert.Equal(t, []int{1, 2, 3, 4, 5}, counted)
	require.ErrorIs(t, spent, dao.ErrNotFound)
}

func TestTwoFactorDAO_AnAttemptCarriesTheChallengeWhole(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	challenge := newChallenge(user.ID)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), challenge))

	// when
	got, err := repos.TwoFactor.AttemptChallenge(t.Context(), spec.TwoFactorChallengeAttempt{TokenHash: challenge.TokenHash, MaxAttempts: challengeAttemptCap})

	// then
	require.NoError(t, err)
	assert.Equal(t, challenge.TokenHash, got.TokenHash)
	assert.Equal(t, user.ID, got.UserID)
	require.NotNil(t, got.ReturnTo)
	assert.Equal(t, "/o/acme", *got.ReturnTo)
	assert.False(t, got.Remember)
	assert.WithinDuration(t, challenge.ExpiresAt, got.ExpiresAt, time.Millisecond)
}

func TestTwoFactorDAO_GetLiveChallengeReadsWithoutSpendingAnAttempt(t *testing.T) {
	// given a live challenge and an expired one
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	live := newChallenge(user.ID)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), live))
	expired := newChallenge(user.ID)
	expired.ExpiresAt = time.Now().Add(-time.Second)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), expired))

	// when
	got, err := repos.TwoFactor.GetLiveChallenge(t.Context(), live.TokenHash)
	_, expiredErr := repos.TwoFactor.GetLiveChallenge(t.Context(), expired.TokenHash)

	// then
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.UserID)
	assert.Zero(t, got.Attempts)
	require.ErrorIs(t, expiredErr, dao.ErrNotFound)
}

func TestTwoFactorDAO_AChallengeKeepsTheRememberMeChoice(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	challenge := newChallenge(user.ID)
	challenge.Remember = true
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), challenge))

	// when
	got, err := repos.TwoFactor.ConsumeChallenge(t.Context(), challenge.TokenHash)

	// then
	require.NoError(t, err)
	assert.True(t, got.Remember)
}

func TestTwoFactorDAO_AChallengeWithNoReturnPathStoresNull(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	challenge := newChallenge(user.ID)
	challenge.ReturnTo = ""
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), challenge))

	// when
	got, err := repos.TwoFactor.ConsumeChallenge(t.Context(), challenge.TokenHash)

	// then
	require.NoError(t, err)
	assert.Nil(t, got.ReturnTo)
}

func TestTwoFactorDAO_OneTokenHashIsOneChallenge(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	challenge := newChallenge(user.ID)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), challenge))

	// when
	err := repos.TwoFactor.CreateChallenge(t.Context(), challenge)

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
}

func TestTwoFactorDAO_AChallengeIsConsumedOnce(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	challenge := newChallenge(user.ID)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), challenge))

	// when
	first, firstErr := repos.TwoFactor.ConsumeChallenge(t.Context(), challenge.TokenHash)
	_, secondErr := repos.TwoFactor.ConsumeChallenge(t.Context(), challenge.TokenHash)
	_, attemptErr := repos.TwoFactor.AttemptChallenge(t.Context(), spec.TwoFactorChallengeAttempt{TokenHash: challenge.TokenHash, MaxAttempts: challengeAttemptCap})

	// then
	require.NoError(t, firstErr)
	assert.Equal(t, user.ID, first.UserID)
	require.ErrorIs(t, secondErr, dao.ErrNotFound)
	require.ErrorIs(t, attemptErr, dao.ErrNotFound)
}

func TestTwoFactorDAO_AnExpiredChallengeIsNeitherAttemptedNorConsumed(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	challenge := newChallenge(user.ID)
	challenge.ExpiresAt = time.Now().Add(-time.Second)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), challenge))

	// when
	_, attemptErr := repos.TwoFactor.AttemptChallenge(t.Context(), spec.TwoFactorChallengeAttempt{TokenHash: challenge.TokenHash, MaxAttempts: challengeAttemptCap})
	_, consumeErr := repos.TwoFactor.ConsumeChallenge(t.Context(), challenge.TokenHash)

	// then it is refused, and left for the sweep
	require.ErrorIs(t, attemptErr, dao.ErrNotFound)
	require.ErrorIs(t, consumeErr, dao.ErrNotFound)
	assert.Equal(t, 1, countRows(t, repos, "two_factor_challenges"))
}

func TestTwoFactorDAO_CleanExpiredChallengesRemovesOnlyExpiredOnes(t *testing.T) {
	// given one live challenge and two expired ones
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	live := newChallenge(user.ID)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), live))
	for range 2 {
		expired := newChallenge(user.ID)
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), expired))
	}

	// when
	removed, err := repos.TwoFactor.CleanExpiredChallenges(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)
	_, err = repos.TwoFactor.ConsumeChallenge(t.Context(), live.TokenHash)
	require.NoError(t, err)
}

func TestTwoFactorDAO_DeletingTheUserRemovesTheirSecretAndChallenges(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)
	require.NoError(t, repos.TwoFactor.CreateChallenge(t.Context(), newChallenge(user.ID)))

	// when
	_, err = repos.DB().ExecContext(t.Context(), `DELETE FROM users WHERE id = $1`, user.ID)

	// then
	require.NoError(t, err)
	assert.Zero(t, countRows(t, repos, "user_totp"))
	assert.Zero(t, countRows(t, repos, "two_factor_challenges"))
}

func TestTwoFactorRepository_ActivateStoresTheCodesAndSignsOutEveryOtherDevice(t *testing.T) {
	// given a pending enrolment, the session turning it on, and two other devices
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)
	current := daotest.CreateSession(t, repos, user.ID)
	otherDevices := []string{daotest.CreateSession(t, repos, user.ID), daotest.CreateSession(t, repos, user.ID)}
	hashes := []string{token.Hash("first-code"), token.Hash("second-code")}

	// when
	activated, err := repos.TwoFactor.Activate(t.Context(), spec.TwoFactorActivation{UserID: user.ID, Step: 100, CodeHashes: hashes, KeepToken: current})

	// then
	require.NoError(t, err)
	assert.True(t, activated.Enabled())
	assert.Equal(t, 2, countRows(t, repos, "two_factor_recovery_codes"))
	_, err = repos.Session.Get(t.Context(), current)
	require.NoError(t, err)
	for _, other := range otherDevices {
		_, err = repos.Session.Get(t.Context(), other)
		require.ErrorIs(t, err, dao.ErrNotFound)
	}
}

func TestTwoFactorRepository_ActivateLeavesEverythingAsItWasWhenTheEnrolmentIsGone(t *testing.T) {
	// given no pending enrolment
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	current := daotest.CreateSession(t, repos, user.ID)
	other := daotest.CreateSession(t, repos, user.ID)

	// when
	_, err := repos.TwoFactor.Activate(t.Context(), spec.TwoFactorActivation{UserID: user.ID, Step: 100, CodeHashes: []string{token.Hash("code")}, KeepToken: current})

	// then no device is signed out and no code is stored
	require.ErrorIs(t, err, dao.ErrNotFound)
	_, err = repos.Session.Get(t.Context(), other)
	require.NoError(t, err)
	assert.Zero(t, countRows(t, repos, "two_factor_recovery_codes"))
}

func TestTwoFactorDAO_ARecoveryCodeWorksOnceAndOnlyForItsOwner(t *testing.T) {
	// given two accounts with two-factor on, one holding the code
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	for _, userID := range []uuid.UUID{owner.ID, other.ID} {
		_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: userID, Secret: firstSecret})
		require.NoError(t, err)
	}
	code := spec.TwoFactorRecoveryCodeUse{UserID: owner.ID, CodeHash: token.Hash("the-code")}
	_, err := repos.TwoFactor.Activate(t.Context(), spec.TwoFactorActivation{UserID: owner.ID, Step: 100, CodeHashes: []string{code.CodeHash}})
	require.NoError(t, err)

	// when
	strangerErr := repos.TwoFactor.UseRecoveryCode(t.Context(), spec.TwoFactorRecoveryCodeUse{UserID: other.ID, CodeHash: code.CodeHash})
	firstErr := repos.TwoFactor.UseRecoveryCode(t.Context(), code)
	secondErr := repos.TwoFactor.UseRecoveryCode(t.Context(), code)

	// then
	require.ErrorIs(t, strangerErr, dao.ErrNotFound)
	require.NoError(t, firstErr)
	require.ErrorIs(t, secondErr, dao.ErrNotFound)
}

func TestTwoFactorDAO_TurningTwoFactorOffRemovesTheRecoveryCodes(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	_, err := repos.TwoFactor.BeginTOTP(t.Context(), spec.TwoFactorEnrolment{UserID: user.ID, Secret: firstSecret})
	require.NoError(t, err)
	_, err = repos.TwoFactor.Activate(t.Context(), spec.TwoFactorActivation{UserID: user.ID, Step: 100, CodeHashes: []string{token.Hash("one"), token.Hash("two")}})
	require.NoError(t, err)

	// when
	err = repos.TwoFactor.DeleteTOTP(t.Context(), user.ID)

	// then a later enrolment starts with no old codes
	require.NoError(t, err)
	assert.Zero(t, countRows(t, repos, "two_factor_recovery_codes"))
}

func TestTwoFactorDAO_ATrustedDeviceIsLiveOnlyForItsOwnerUntilItExpires(t *testing.T) {
	// given two accounts with two-factor on, one trusting a device, and an expired device
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	turnOnTwoFactor(t, repos, owner.ID)
	turnOnTwoFactor(t, repos, other.ID)
	device := newTrustedDevice(owner.ID)
	expired := newTrustedDevice(owner.ID)
	expired.ExpiresAt = time.Now().Add(-time.Second)

	// when
	createErr := repos.TwoFactor.CreateTrustedDevice(t.Context(), device)
	expiredErr := repos.TwoFactor.CreateTrustedDevice(t.Context(), expired)

	// then
	require.NoError(t, createErr)
	require.NoError(t, expiredErr)
	assert.True(t, deviceLive(t, repos, device))
	assert.False(t, deviceLive(t, repos, spec.NewTwoFactorTrustedDevice{TokenHash: device.TokenHash, UserID: other.ID}))
	assert.False(t, deviceLive(t, repos, expired))
	assert.False(t, deviceLive(t, repos, newTrustedDevice(owner.ID)))
}

func TestTwoFactorDAO_ADeviceCannotBeTrustedWithoutTwoFactor(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)

	// when
	err := repos.TwoFactor.CreateTrustedDevice(t.Context(), newTrustedDevice(user.ID))

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestTwoFactorDAO_TurningTwoFactorOffForgetsTheTrustedDevices(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	turnOnTwoFactor(t, repos, user.ID)
	require.NoError(t, repos.TwoFactor.CreateTrustedDevice(t.Context(), newTrustedDevice(user.ID)))

	// when
	err := repos.TwoFactor.DeleteTOTP(t.Context(), user.ID)

	// then turning it on again asks every device for a code
	require.NoError(t, err)
	assert.Zero(t, countRows(t, repos, "two_factor_trusted_devices"))
}

func TestTwoFactorDAO_DeleteTrustedDevicesForUserLeavesOtherAccountsAlone(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	other := daotest.CreateUser(t, repos)
	turnOnTwoFactor(t, repos, owner.ID)
	turnOnTwoFactor(t, repos, other.ID)
	require.NoError(t, repos.TwoFactor.CreateTrustedDevice(t.Context(), newTrustedDevice(owner.ID)))
	kept := newTrustedDevice(other.ID)
	require.NoError(t, repos.TwoFactor.CreateTrustedDevice(t.Context(), kept))

	// when
	err := repos.TwoFactor.DeleteTrustedDevicesForUser(t.Context(), owner.ID)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, repos, "two_factor_trusted_devices"))
	assert.True(t, deviceLive(t, repos, kept))
}

func TestTwoFactorDAO_CleanExpiredTrustedDevicesRemovesOnlyExpiredOnes(t *testing.T) {
	// given one live device and two expired ones
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	turnOnTwoFactor(t, repos, user.ID)
	live := newTrustedDevice(user.ID)
	require.NoError(t, repos.TwoFactor.CreateTrustedDevice(t.Context(), live))
	for range 2 {
		expired := newTrustedDevice(user.ID)
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		require.NoError(t, repos.TwoFactor.CreateTrustedDevice(t.Context(), expired))
	}

	// when
	removed, err := repos.TwoFactor.CleanExpiredTrustedDevices(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)
	assert.True(t, deviceLive(t, repos, live))
}

func TestUserRepository_ChangingThePasswordForgetsTheTrustedDevices(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	turnOnTwoFactor(t, repos, user.ID)
	current := daotest.CreateSession(t, repos, user.ID)
	require.NoError(t, repos.TwoFactor.CreateTrustedDevice(t.Context(), newTrustedDevice(user.ID)))

	// when
	err := repos.User.ChangePassword(t.Context(), spec.PasswordChange{UserID: user.ID, PasswordHash: "changed-hash", KeepToken: current})

	// then
	require.NoError(t, err)
	assert.Zero(t, countRows(t, repos, "two_factor_trusted_devices"))
}
