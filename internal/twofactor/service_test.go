package twofactor

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	testMocks struct {
		twoFactor *repository.MockTwoFactorRepository
		users     *user.MockService
		settings  *settings.MockService
		auditLog  *repository.MockAuditLogRepository
		sessions  *MockSessionDisconnector
	}
)

const (
	testSiteName     = "RetroTide"
	testEmail        = "alice@example.com"
	testPassword     = "correct horse battery"
	testPasswordHash = "$2a$10$pretendhashforthetestsonlypretendhashforthetestsonly"
	testIP           = "198.51.100.7"
	testChallenge    = "5a4b3c2d1e0f5a4b3c2d1e0f5a4b3c2d1e0f5a4b3c2d1e0f5a4b3c2d1e0f5a4b"
	testUnix         = 1111111109
	testCode         = "081804"
	testWrongCode    = "123456"
	testStep         = int64(37037036)
	testReturnTo     = "/o/acme/boards"
	testRecoveryCode = "abcde-fgh23"
	testKeepToken    = "4d6f8a1c3e5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5f7a9c1e3b5d7f"
	testDeviceToken  = "7a9c1e3b5d7f4d6f8a1c3e5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5f"
)

var (
	testNow = time.Unix(testUnix, 0)

	errLookupNotFound = fmt.Errorf("lookup: %w", dao.ErrNotFound)
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		twoFactor: repository.NewMockTwoFactorRepository(t),
		users:     user.NewMockService(t),
		settings:  settings.NewMockService(t),
		auditLog:  repository.NewMockAuditLogRepository(t),
		sessions:  NewMockSessionDisconnector(t),
	}

	svc := NewService(Deps{
		TwoFactor: m.twoFactor,
		Users:     m.users,
		Settings:  m.settings,
		AuditLog:  m.auditLog,
		Sessions:  m.sessions,
	}).(*service)
	svc.now = func() time.Time { return testNow }

	return svc, m
}

func confirmedTOTP(userID uuid.UUID) *model.UserTOTP {
	confirmedAt := testNow.Add(-24 * time.Hour)

	return &model.UserTOTP{UserID: userID, Secret: rfcSecret, ConfirmedAt: &confirmedAt, LastUsedStep: testStep - 100}
}

func pendingTOTP(userID uuid.UUID) *model.UserTOTP {
	return &model.UserTOTP{UserID: userID, Secret: rfcSecret}
}

func challengeFor(userID uuid.UUID, attempts int) *model.TwoFactorChallenge {
	return &model.TwoFactorChallenge{TokenHash: token.Hash(testChallenge), UserID: userID, ReturnTo: new(testReturnTo), Attempts: attempts, ExpiresAt: testNow.Add(ChallengeTTL)}
}

func confirmationOf(userID uuid.UUID, code string) spec.TwoFactorConfirmation {
	return spec.TwoFactorConfirmation{TwoFactorCodeEntry: spec.TwoFactorCodeEntry{UserID: userID, Code: code, IP: testIP}, KeepToken: testKeepToken}
}

func attemptOf() spec.TwoFactorChallengeAttempt {
	return spec.TwoFactorChallengeAttempt{TokenHash: token.Hash(testChallenge), MaxAttempts: MaxChallengeAttempts}
}

func TestEnabled_AnswersFromTheConfirmedSecret(t *testing.T) {
	tests := []struct {
		name    string
		current *model.UserTOTP
		err     error
		want    bool
	}{
		{name: "no secret", err: errLookupNotFound, want: false},
		{name: "an enrolment not yet confirmed", current: pendingTOTP(uuid.Nil), want: false},
		{name: "a confirmed secret", current: confirmedTOTP(uuid.Nil), want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			userID := uuid.New()
			m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(tc.current, tc.err)

			// when
			got, err := svc.Enabled(t.Context(), userID)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEnabled_PassesAStoreFailureOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(nil, errDatabaseDown)

	// when
	_, err := svc.Enabled(t.Context(), userID)

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
}

func TestChallenge_AnswersNothingForAnAccountWithoutTwoFactor(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(pendingTOTP(userID), nil)

	// when
	got, err := svc.Challenge(t.Context(), spec.TwoFactorPending{UserID: userID, ReturnTo: testReturnTo})

	// then the strict mock sees no challenge written
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestChallenge_StoresOnlyTheTokenHashForFiveMinutes(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)

	var stored spec.NewTwoFactorChallenge
	m.twoFactor.EXPECT().CreateChallenge(mock.Anything, mock.Anything).Run(func(_ context.Context, s spec.NewTwoFactorChallenge, _ ...*sql.Tx) {
		stored = s
	}).Return(nil)

	// when
	got, err := svc.Challenge(t.Context(), spec.TwoFactorPending{UserID: userID, ReturnTo: testReturnTo, Remember: true})

	// then
	require.NoError(t, err)
	assert.Len(t, got, 64)
	assert.Equal(t, token.Hash(got), stored.TokenHash)
	assert.Equal(t, userID, stored.UserID)
	assert.Equal(t, testReturnTo, stored.ReturnTo)
	assert.True(t, stored.Remember)
	assert.Equal(t, testNow.Add(ChallengeTTL), stored.ExpiresAt)
}

func TestChallenge_AnswersNothingOnADeviceTheUserTrusts(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().TrustedDeviceLive(mock.Anything, spec.TwoFactorTrustedDeviceCheck{TokenHash: token.Hash(testDeviceToken), UserID: userID}).Return(true, nil)

	// when
	got, err := svc.Challenge(t.Context(), spec.TwoFactorPending{UserID: userID, TrustedDeviceToken: testDeviceToken})

	// then the strict mock sees no challenge written
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestChallenge_StillChallengesAnUnknownExpiredOrForeignDevice(t *testing.T) {
	// given a device token the store does not hold live for this user
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().TrustedDeviceLive(mock.Anything, spec.TwoFactorTrustedDeviceCheck{TokenHash: token.Hash(testDeviceToken), UserID: userID}).Return(false, nil)
	m.twoFactor.EXPECT().CreateChallenge(mock.Anything, mock.Anything).Return(nil)

	// when
	got, err := svc.Challenge(t.Context(), spec.TwoFactorPending{UserID: userID, TrustedDeviceToken: testDeviceToken})

	// then
	require.NoError(t, err)
	assert.Len(t, got, 64)
}

func TestChallenge_FailsClosedWhenTheTrustedDeviceCannotBeChecked(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().TrustedDeviceLive(mock.Anything, mock.Anything).Return(false, errDatabaseDown)

	// when
	got, err := svc.Challenge(t.Context(), spec.TwoFactorPending{UserID: userID, TrustedDeviceToken: testDeviceToken})

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Empty(t, got)
}

func TestChallenge_PassesAStoreFailureOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().CreateChallenge(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	got, err := svc.Challenge(t.Context(), spec.TwoFactorPending{UserID: userID})

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Empty(t, got)
}

func TestStatus_ReportsWhetherTwoFactorIsOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)

	// when
	got, err := svc.Status(t.Context(), userID)

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.TwoFactorStatusResponse{Enabled: true}, got)
}

func TestBeginEnrolment_RefusesAnAccountWithoutAPassword(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail}, nil)

	// when
	got, err := svc.BeginEnrolment(t.Context(), spec.TwoFactorEnrolmentStart{UserID: userID, CurrentPassword: testPassword, IP: testIP})

	// then
	assert.ErrorIs(t, err, ErrPasswordRequired)
	assert.Nil(t, got)
}

func TestBeginEnrolment_RefusesTheWrongPassword(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail, HasPassword: true}, nil)
	m.users.EXPECT().CheckPassword(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: testPassword}).Return(user.ErrIncorrectPassword)

	// when
	got, err := svc.BeginEnrolment(t.Context(), spec.TwoFactorEnrolmentStart{UserID: userID, CurrentPassword: testPassword, IP: testIP})

	// then no secret is drawn or stored
	assert.ErrorIs(t, err, user.ErrIncorrectPassword)
	assert.Nil(t, got)
}

func TestBeginEnrolment_RefusesAnAccountThatAlreadyHasTwoFactorOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail, HasPassword: true}, nil)
	m.users.EXPECT().CheckPassword(mock.Anything, mock.Anything).Return(nil)
	m.twoFactor.EXPECT().BeginTOTP(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("begin: %w", dao.ErrConflict))

	// when
	got, err := svc.BeginEnrolment(t.Context(), spec.TwoFactorEnrolmentStart{UserID: userID, CurrentPassword: testPassword, IP: testIP})

	// then
	assert.ErrorIs(t, err, ErrAlreadyEnabled)
	assert.Nil(t, got)
}

func TestBeginEnrolment_StoresAFreshSecretAndAnswersItsURI(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(&dto.UserResponse{ID: userID, Email: testEmail, HasPassword: true}, nil)
	m.users.EXPECT().CheckPassword(mock.Anything, spec.UserPasswordConfirmation{UserID: userID, Password: testPassword}).Return(nil)

	var stored spec.TwoFactorEnrolment
	m.twoFactor.EXPECT().BeginTOTP(mock.Anything, mock.Anything).Run(func(_ context.Context, s spec.TwoFactorEnrolment, _ ...*sql.Tx) {
		stored = s
	}).Return(&model.UserTOTP{UserID: userID}, nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)

	// when
	got, err := svc.BeginEnrolment(t.Context(), spec.TwoFactorEnrolmentStart{UserID: userID, CurrentPassword: testPassword, IP: testIP})

	// then
	require.NoError(t, err)
	assert.Equal(t, userID, stored.UserID)
	assert.Len(t, stored.Secret, 32)
	assert.Equal(t, stored.Secret, got.Secret)
	assert.Equal(t, URI(testSiteName, testEmail, stored.Secret), got.URI)
	assert.True(t, strings.HasPrefix(got.URI, "otpauth://totp/RetroTide:alice@example.com?secret="))
}

func TestConfirmEnrolment_RefusesWithoutAPendingEnrolment(t *testing.T) {
	tests := []struct {
		name    string
		current *model.UserTOTP
		err     error
	}{
		{name: "no enrolment", err: errLookupNotFound},
		{name: "an enrolment already confirmed", current: confirmedTOTP(uuid.Nil)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			userID := uuid.New()
			m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(tc.current, tc.err)

			// when
			got, err := svc.ConfirmEnrolment(t.Context(), confirmationOf(userID, testCode))

			// then
			assert.ErrorIs(t, err, ErrNotEnrolling)
			assert.Nil(t, got)
		})
	}
}

func TestConfirmEnrolment_RefusesTheWrongCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(pendingTOTP(userID), nil)

	// when
	got, err := svc.ConfirmEnrolment(t.Context(), confirmationOf(userID, testWrongCode))

	// then nothing is confirmed, signed out or audited
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestConfirmEnrolment_RefusesARecoveryCode(t *testing.T) {
	// given an enrolment, which has no recovery codes yet
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(pendingTOTP(userID), nil)

	// when
	got, err := svc.ConfirmEnrolment(t.Context(), confirmationOf(userID, testRecoveryCode))

	// then
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestConfirmEnrolment_RefusesACodeTheStoreNoLongerAccepts(t *testing.T) {
	// given an enrolment restarted or confirmed between the read and the write
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(pendingTOTP(userID), nil)
	m.twoFactor.EXPECT().Activate(mock.Anything, mock.Anything).Return(nil, errLookupNotFound)

	// when
	got, err := svc.ConfirmEnrolment(t.Context(), confirmationOf(userID, testCode))

	// then no session is disconnected
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestConfirmEnrolment_TurnsTwoFactorOnWithRecoveryCodesSignsOutOtherDevicesAndAuditsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(pendingTOTP(userID), nil)

	var activated spec.TwoFactorActivation
	m.twoFactor.EXPECT().Activate(mock.Anything, mock.Anything).Run(func(_ context.Context, s spec.TwoFactorActivation, _ ...*sql.Tx) {
		activated = s
	}).Return(confirmedTOTP(userID), nil)
	m.sessions.EXPECT().Disconnect(userID).Return()
	m.auditLog.EXPECT().Create(mock.Anything, audit.NewEntry{
		ActorID:    userID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionTwoFactorEnabled,
		TargetType: audit.TargetUser,
		TargetID:   userID.String(),
		SubjectID:  userID,
		Details:    map[string]string{},
		IP:         testIP,
	}).Return(nil)

	// when
	got, err := svc.ConfirmEnrolment(t.Context(), confirmationOf(userID, testCode))

	// then only the hashes are stored, and only the current session is kept
	require.NoError(t, err)
	require.Len(t, got.RecoveryCodes, RecoveryCodeCount)
	assert.Equal(t, userID, activated.UserID)
	assert.Equal(t, testStep, activated.Step)
	assert.Equal(t, testKeepToken, activated.KeepToken)
	require.Len(t, activated.CodeHashes, RecoveryCodeCount)
	for i, code := range got.RecoveryCodes {
		assert.Equal(t, RecoveryCodeHash(code), activated.CodeHashes[i])
		assert.NotContains(t, activated.CodeHashes, code)
	}
}

func TestConfirmEnrolment_StillAnswersTheRecoveryCodesWhenTheAuditEntryFails(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(pendingTOTP(userID), nil)
	m.twoFactor.EXPECT().Activate(mock.Anything, mock.Anything).Return(confirmedTOTP(userID), nil)
	m.sessions.EXPECT().Disconnect(userID).Return()
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	got, err := svc.ConfirmEnrolment(t.Context(), confirmationOf(userID, testCode))

	// then two-factor is on, so its codes must not be lost
	assert.ErrorIs(t, err, ErrNotAudited)
	assert.ErrorIs(t, err, errDatabaseDown)
	require.NotNil(t, got)
	assert.Len(t, got.RecoveryCodes, RecoveryCodeCount)
}

func TestDisable_RefusesWhenTwoFactorIsNotOn(t *testing.T) {
	tests := []struct {
		name    string
		current *model.UserTOTP
		err     error
	}{
		{name: "no secret", err: errLookupNotFound},
		{name: "an enrolment never confirmed", current: pendingTOTP(uuid.Nil)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			userID := uuid.New()
			m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(tc.current, tc.err)

			// when
			err := svc.Disable(t.Context(), spec.TwoFactorCodeEntry{UserID: userID, Code: testCode, IP: testIP})

			// then
			assert.ErrorIs(t, err, ErrNotEnabled)
		})
	}
}

func TestDisable_RefusesTheWrongCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)

	// when
	err := svc.Disable(t.Context(), spec.TwoFactorCodeEntry{UserID: userID, Code: testWrongCode, IP: testIP})

	// then
	assert.ErrorIs(t, err, ErrInvalidCode)
}

func TestDisable_RefusesAReplayedCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, spec.TwoFactorStepUse{UserID: userID, Step: testStep}).Return(fmt.Errorf("use: %w", dao.ErrConflict))

	// when
	err := svc.Disable(t.Context(), spec.TwoFactorCodeEntry{UserID: userID, Code: testCode, IP: testIP})

	// then the secret stays
	assert.ErrorIs(t, err, ErrInvalidCode)
}

func TestDisable_RemovesTheSecretAndAuditsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, spec.TwoFactorStepUse{UserID: userID, Step: testStep}).Return(nil)
	m.twoFactor.EXPECT().DeleteTOTP(mock.Anything, userID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.MatchedBy(func(entry audit.NewEntry) bool {
		return entry.Action == audit.ActionTwoFactorDisabled && entry.TargetType == audit.TargetUser && entry.ActorID == userID && entry.SubjectID == userID && entry.IP == testIP
	})).Return(nil)

	// when
	err := svc.Disable(t.Context(), spec.TwoFactorCodeEntry{UserID: userID, Code: testCode, IP: testIP})

	// then
	assert.NoError(t, err)
}

func TestDisable_AcceptsAnUnusedRecoveryCode(t *testing.T) {
	// given someone who lost their phone and signed in with a recovery code
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseRecoveryCode(mock.Anything, spec.TwoFactorRecoveryCodeUse{UserID: userID, CodeHash: RecoveryCodeHash(testRecoveryCode)}).Return(nil)
	m.twoFactor.EXPECT().DeleteTOTP(mock.Anything, userID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	// when
	err := svc.Disable(t.Context(), spec.TwoFactorCodeEntry{UserID: userID, Code: testRecoveryCode, IP: testIP})

	// then
	assert.NoError(t, err)
}

func TestDisable_RefusesAUsedOrUnknownRecoveryCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseRecoveryCode(mock.Anything, mock.Anything).Return(errLookupNotFound)

	// when
	err := svc.Disable(t.Context(), spec.TwoFactorCodeEntry{UserID: userID, Code: testRecoveryCode, IP: testIP})

	// then the secret stays
	assert.ErrorIs(t, err, ErrInvalidCode)
}

func TestChallengeOwner_AnswersWhoseSignInTheChallengeHolds(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().GetLiveChallenge(mock.Anything, token.Hash(testChallenge)).Return(challengeFor(userID, 2), nil)

	// when
	got, err := svc.ChallengeOwner(t.Context(), testChallenge)

	// then no attempt is spent, which the strict mock would report
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestChallengeOwner_AnUnknownOrExpiredChallengeHasExpired(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.twoFactor.EXPECT().GetLiveChallenge(mock.Anything, mock.Anything).Return(nil, errLookupNotFound)

	// when
	got, err := svc.ChallengeOwner(t.Context(), testChallenge)

	// then
	require.ErrorIs(t, err, ErrChallengeExpired)
	assert.Equal(t, uuid.Nil, got)
}

func TestChallengeOwner_PassesAStoreFailureOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.twoFactor.EXPECT().GetLiveChallenge(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	_, err := svc.ChallengeOwner(t.Context(), testChallenge)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrChallengeExpired)
}

func TestVerify_RefusesAnUnknownExpiredOrUsedUpChallenge(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(nil, errLookupNotFound)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode})

	// then
	assert.ErrorIs(t, err, ErrChallengeExpired)
	assert.Nil(t, got)
}

func TestVerify_SpendsTheChallengeWhenTwoFactorWasTurnedOffMeanwhile(t *testing.T) {
	tests := []struct {
		name    string
		current *model.UserTOTP
		err     error
	}{
		{name: "the secret is gone", err: errLookupNotFound},
		{name: "only an unconfirmed enrolment is left", current: pendingTOTP(uuid.Nil)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			userID := uuid.New()
			m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
			m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(tc.current, tc.err)
			m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, token.Hash(testChallenge)).Return(challengeFor(userID, 1), nil)

			// when
			got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode})

			// then
			assert.ErrorIs(t, err, ErrChallengeExpired)
			assert.Nil(t, got)
		})
	}
}

func TestVerify_KeepsTheChallengeAfterAWrongCodeWhileAttemptsRemain(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, MaxChallengeAttempts-1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testWrongCode})

	// then the strict mock sees no consume and no session
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestVerify_SpendsTheChallengeOnTheLastWrongCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, MaxChallengeAttempts), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, token.Hash(testChallenge)).Return(challengeFor(userID, MaxChallengeAttempts), nil)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testWrongCode})

	// then
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestVerify_RefusesAReplayedCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, spec.TwoFactorStepUse{UserID: userID, Step: testStep}).Return(fmt.Errorf("use: %w", dao.ErrConflict))

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode})

	// then
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestVerify_PassesAStoreFailureOnWhenUsingTheCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode})

	// then the failure is reported, not dressed up as a wrong code
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestVerify_RefusesWhenAnotherRequestSpentTheChallengeFirst(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, mock.Anything).Return(nil)
	m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, token.Hash(testChallenge)).Return(nil, errLookupNotFound)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode})

	// then
	assert.ErrorIs(t, err, ErrChallengeExpired)
	assert.Nil(t, got)
}

func TestVerify_SpendsTheChallengeAndAnswersWhoPassedAndWhereToLand(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 2), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, spec.TwoFactorStepUse{UserID: userID, Step: testStep}).Return(nil)
	m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, token.Hash(testChallenge)).Return(challengeFor(userID, 2), nil)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode})

	// then the strict mocks see no session, which auth creates, and no device trusted
	require.NoError(t, err)
	assert.Equal(t, &Passed{UserID: userID, ReturnTo: testReturnTo}, got)
}

func TestVerify_CarriesTheRememberMeChoiceAndTrustsTheDeviceForThirtyDays(t *testing.T) {
	// given a challenge made by a sign-in that asked to be remembered
	svc, m := newTestService(t)
	userID := uuid.New()
	challenge := challengeFor(userID, 1)
	challenge.Remember = true
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challenge, nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, mock.Anything).Return(nil)
	m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, token.Hash(testChallenge)).Return(challenge, nil)

	var trusted spec.NewTwoFactorTrustedDevice
	m.twoFactor.EXPECT().CreateTrustedDevice(mock.Anything, mock.Anything).Run(func(_ context.Context, s spec.NewTwoFactorTrustedDevice, _ ...*sql.Tx) {
		trusted = s
	}).Return(nil)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode, TrustDevice: true})

	// then only the hash of the device token is stored
	require.NoError(t, err)
	assert.True(t, got.Remember)
	assert.Len(t, got.TrustedDeviceToken, 64)
	assert.Equal(t, token.Hash(got.TrustedDeviceToken), trusted.TokenHash)
	assert.Equal(t, userID, trusted.UserID)
	assert.Equal(t, testNow.Add(TrustedDeviceTTL), trusted.ExpiresAt)
}

func TestVerify_TrustingADeviceAfterTwoFactorWasTurnedOffHasExpired(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, mock.Anything).Return(nil)
	m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, mock.Anything).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().CreateTrustedDevice(mock.Anything, mock.Anything).Return(fmt.Errorf("trust: %w", dao.ErrNotFound))

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode, TrustDevice: true})

	// then
	assert.ErrorIs(t, err, ErrChallengeExpired)
	assert.Nil(t, got)
}

func TestVerify_AnswersNoLandingPathForAPasswordSignIn(t *testing.T) {
	// given a challenge stored without a return path
	svc, m := newTestService(t)
	userID := uuid.New()
	challenge := challengeFor(userID, 1)
	challenge.ReturnTo = nil
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challenge, nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseTOTPStep(mock.Anything, mock.Anything).Return(nil)
	m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, token.Hash(testChallenge)).Return(challenge, nil)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testCode})

	// then the SPA falls back to its own return path
	require.NoError(t, err)
	assert.Empty(t, got.ReturnTo)
}

func TestVerify_AcceptsARecoveryCodeInsteadOfTheAppCode(t *testing.T) {
	// given a recovery code typed in capitals, with spaces
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseRecoveryCode(mock.Anything, spec.TwoFactorRecoveryCodeUse{UserID: userID, CodeHash: RecoveryCodeHash(testRecoveryCode)}).Return(nil)
	m.twoFactor.EXPECT().ConsumeChallenge(mock.Anything, token.Hash(testChallenge)).Return(challengeFor(userID, 1), nil)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: " " + strings.ToUpper(testRecoveryCode) + " "})

	// then
	require.NoError(t, err)
	assert.Equal(t, userID, got.UserID)
}

func TestVerify_ARecoveryCodeWorksOnlyOnce(t *testing.T) {
	// given a code already spent, which the store no longer matches
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseRecoveryCode(mock.Anything, mock.Anything).Return(errLookupNotFound)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testRecoveryCode})

	// then it counts as a wrong attempt
	assert.ErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}

func TestVerify_PassesAStoreFailureOnWhenUsingARecoveryCode(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.twoFactor.EXPECT().AttemptChallenge(mock.Anything, attemptOf()).Return(challengeFor(userID, 1), nil)
	m.twoFactor.EXPECT().GetTOTP(mock.Anything, userID).Return(confirmedTOTP(userID), nil)
	m.twoFactor.EXPECT().UseRecoveryCode(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	got, err := svc.Verify(t.Context(), spec.TwoFactorVerification{ChallengeToken: testChallenge, Code: testRecoveryCode})

	// then the failure is reported, not dressed up as a wrong code
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrInvalidCode)
	assert.Nil(t, got)
}
