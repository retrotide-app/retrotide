package user

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type (
	serviceMocks struct {
		users    *repository.MockUserRepository
		settings *settings.MockService
	}
)

const (
	testPassword          = "correct horse battery"
	testMinPasswordLength = 8
)

func newTestService(t *testing.T) (Service, serviceMocks) {
	t.Helper()

	mocks := serviceMocks{
		users:    repository.NewMockUserRepository(t),
		settings: settings.NewMockService(t),
	}

	return NewService(mocks.users, mocks.settings), mocks
}

func newPasswordTestService(t *testing.T) Service {
	t.Helper()

	svc, mocks := newTestService(t)
	mocks.settings.EXPECT().GetInt(mock.Anything, config.SettingMinPasswordLength).Return(testMinPasswordLength, nil)

	return svc
}

func userWithPassword(t *testing.T, password string) *model.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	return &model.User{
		ID:           uuid.New(),
		Email:        "sam@example.com",
		PasswordHash: new(string(hash)),
		DisplayName:  "Sam",
		CreatedAt:    time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC),
	}
}

func passwordlessUser() *model.User {
	return &model.User{ID: uuid.New(), Email: "google@example.com", DisplayName: "Google User", EmailVerified: true}
}

func TestNewAccountSpec_HashesThePasswordAndClampsTheDisplayName(t *testing.T) {
	// given
	svc := newPasswordTestService(t)

	// when
	created, err := svc.NewAccountSpec(t.Context(), spec.NewAccount{Email: "sam@example.com", Password: testPassword, DisplayName: "  <b>Sam</b>  Okafor "})

	// then
	require.NoError(t, err)
	assert.Equal(t, "sam@example.com", created.Email)
	assert.Equal(t, "Sam Okafor", created.DisplayName)
	assert.False(t, created.EmailVerified)
	require.NotNil(t, created.PasswordHash)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*created.PasswordHash), []byte(testPassword)))
}

func TestNewAccountSpec_RefusesAShortPassword(t *testing.T) {
	// given
	svc := newPasswordTestService(t)

	// when
	_, err := svc.NewAccountSpec(t.Context(), spec.NewAccount{Email: "sam@example.com", Password: "short", DisplayName: "Sam"})

	// then
	require.ErrorIs(t, err, ErrPasswordTooShort)
}

func TestHashPassword_CountsCharactersNotBytes(t *testing.T) {
	// given seven two-byte characters, which is fourteen bytes
	svc := newPasswordTestService(t)

	// when
	_, err := svc.HashPassword(t.Context(), strings.Repeat("é", testMinPasswordLength-1))

	// then
	require.ErrorIs(t, err, ErrPasswordTooShort)
}

func TestHashPassword_AcceptsExactlyTheMinimumLength(t *testing.T) {
	// given
	svc := newPasswordTestService(t)
	password := strings.Repeat("p", testMinPasswordLength)

	// when
	hash, err := svc.HashPassword(t.Context(), password)

	// then
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)))
}

func TestHashPassword_RefusesAPasswordBcryptWouldTruncate(t *testing.T) {
	// given
	svc := newPasswordTestService(t)

	// when
	_, err := svc.HashPassword(t.Context(), strings.Repeat("p", 73))

	// then
	require.ErrorIs(t, err, ErrPasswordTooLong)
}

func TestHashPassword_ReturnsTheSettingsReadError(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	readErr := errors.New("settings unavailable")
	mocks.settings.EXPECT().GetInt(mock.Anything, config.SettingMinPasswordLength).Return(0, readErr)

	// when
	_, err := svc.HashPassword(t.Context(), testPassword)

	// then
	require.ErrorIs(t, err, readErr)
}

func TestGetByID_ReturnsTheUser(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	found := userWithPassword(t, testPassword)
	mocks.users.EXPECT().GetByID(mock.Anything, found.ID).Return(found, nil)

	// when
	got, err := svc.GetByID(t.Context(), found.ID)

	// then
	require.NoError(t, err)
	assert.Equal(t, new(dto.NewUserResponse(found)), got)
}

func TestGetByID_MapsAMissingUser(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	id := uuid.New()
	mocks.users.EXPECT().GetByID(mock.Anything, id).Return(nil, fmt.Errorf("get user: %w", dao.ErrNotFound))

	// when
	_, err := svc.GetByID(t.Context(), id)

	// then
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestValidateCredentials(t *testing.T) {
	// given
	withPassword := userWithPassword(t, testPassword)
	lookupErr := errors.New("database unavailable")

	cases := []struct {
		name     string
		found    *model.User
		findErr  error
		password string
		wantErr  error
	}{
		{name: "the right password signs in", found: withPassword, password: testPassword},
		{name: "a wrong password is refused", found: withPassword, password: "wrong password", wantErr: ErrInvalidCredentials},
		{name: "an unknown email is refused the same way", findErr: fmt.Errorf("get user: %w", dao.ErrNotFound), password: testPassword, wantErr: ErrInvalidCredentials},
		{name: "an account without a password is refused the same way", found: passwordlessUser(), password: "", wantErr: ErrInvalidCredentials},
		{name: "a lookup failure is not a refusal", findErr: lookupErr, password: testPassword, wantErr: lookupErr},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mocks := newTestService(t)
			mocks.users.EXPECT().GetByEmail(mock.Anything, "sam@example.com").Return(tc.found, tc.findErr)

			// when
			got, err := svc.ValidateCredentials(t.Context(), spec.UserCredentials{Email: "sam@example.com", Password: tc.password})

			// then
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, new(dto.NewUserResponse(tc.found)), got)
		})
	}
}

func TestValidateCredentials_ComparesAgainstTheTimingHashWhenNoPasswordCanMatch(t *testing.T) {
	tests := []struct {
		name    string
		found   *model.User
		findErr error
	}{
		{name: "an unknown email", findErr: fmt.Errorf("get user: %w", dao.ErrNotFound)},
		{name: "an account without a password", found: passwordlessUser()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, mocks := newTestService(t)
			mocks.users.EXPECT().GetByEmail(mock.Anything, "sam@example.com").Return(tc.found, tc.findErr)

			var compared [][]byte
			svc.(*service).compare = func(hash []byte, password []byte) error {
				compared = append(compared, hash)

				return bcrypt.CompareHashAndPassword(hash, password)
			}

			// when
			got, err := svc.ValidateCredentials(t.Context(), spec.UserCredentials{Email: "sam@example.com", Password: testPassword})

			// then one comparison of the cost a real password costs runs before the same refusal
			require.ErrorIs(t, err, ErrInvalidCredentials)
			assert.Nil(t, got)
			require.Len(t, compared, 1)
			cost, costErr := bcrypt.Cost(compared[0])
			require.NoError(t, costErr)
			assert.Equal(t, bcrypt.DefaultCost, cost)
		})
	}
}

func TestValidateCredentials_TheTimingHashIsBuiltOnce(t *testing.T) {
	// given
	first, firstErr := timingHash()
	require.NoError(t, firstErr)

	// when
	second, secondErr := timingHash()

	// then
	require.NoError(t, secondErr)
	assert.Same(t, &first[0], &second[0])
}

func TestValidateCredentials_AFailedTimingComparisonIsAnErrorNotARefusal(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	mocks.users.EXPECT().GetByEmail(mock.Anything, "sam@example.com").Return(nil, fmt.Errorf("get user: %w", dao.ErrNotFound))
	broken := errors.New("comparison broke")
	svc.(*service).compare = func([]byte, []byte) error {
		return broken
	}

	// when
	_, err := svc.ValidateCredentials(t.Context(), spec.UserCredentials{Email: "sam@example.com", Password: testPassword})

	// then
	require.ErrorIs(t, err, broken)
	assert.NotErrorIs(t, err, ErrInvalidCredentials)
}

func TestValidateCredentials_ACorruptHashIsAnErrorNotARefusal(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	corrupt := &model.User{ID: uuid.New(), Email: "sam@example.com", PasswordHash: new("not-a-bcrypt-hash")}
	mocks.users.EXPECT().GetByEmail(mock.Anything, "sam@example.com").Return(corrupt, nil)

	// when
	_, err := svc.ValidateCredentials(t.Context(), spec.UserCredentials{Email: "sam@example.com", Password: testPassword})

	// then
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidCredentials)
}

func TestCheckPassword(t *testing.T) {
	// given
	withPassword := userWithPassword(t, testPassword)

	cases := []struct {
		name     string
		found    *model.User
		findErr  error
		password string
		wantErr  error
	}{
		{name: "the right password passes", found: withPassword, password: testPassword},
		{name: "a wrong password is incorrect", found: withPassword, password: "wrong password", wantErr: ErrIncorrectPassword},
		{name: "an account without a password never passes", found: passwordlessUser(), password: "", wantErr: ErrIncorrectPassword},
		{name: "a missing user is not found", findErr: fmt.Errorf("get user: %w", dao.ErrNotFound), password: testPassword, wantErr: ErrUserNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mocks := newTestService(t)
			userID := uuid.New()
			mocks.users.EXPECT().GetByID(mock.Anything, userID).Return(tc.found, tc.findErr)

			// when
			err := svc.CheckPassword(t.Context(), spec.UserPasswordConfirmation{UserID: userID, Password: tc.password})

			// then
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestUpdateDisplayName_WritesTheClampedName(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	updated := userWithPassword(t, testPassword)
	updated.DisplayName = "Sam & Co"
	mocks.users.EXPECT().SetDisplayName(mock.Anything, spec.UserDisplayNameUpdate{UserID: updated.ID, DisplayName: "Sam & Co"}).Return(nil)
	mocks.users.EXPECT().GetByID(mock.Anything, updated.ID).Return(updated, nil)

	// when
	got, err := svc.UpdateDisplayName(t.Context(), spec.UserDisplayNameUpdate{UserID: updated.ID, DisplayName: "  <i>Sam</i>   & Co  "})

	// then
	require.NoError(t, err)
	assert.Equal(t, "Sam & Co", got.DisplayName)
}

func TestUpdateDisplayName_RefusesANameThatClampsToNothing(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	_, err := svc.UpdateDisplayName(t.Context(), spec.UserDisplayNameUpdate{UserID: uuid.New(), DisplayName: " <b></b> "})

	// then nothing is written
	require.ErrorIs(t, err, ErrDisplayNameRequired)
}

func TestUpdateDisplayName_MapsAMissingUser(t *testing.T) {
	// given
	svc, mocks := newTestService(t)
	userID := uuid.New()
	mocks.users.EXPECT().SetDisplayName(mock.Anything, spec.UserDisplayNameUpdate{UserID: userID, DisplayName: "Sam"}).Return(fmt.Errorf("set display name: %w", dao.ErrNotFound))

	// when
	_, err := svc.UpdateDisplayName(t.Context(), spec.UserDisplayNameUpdate{UserID: userID, DisplayName: "Sam"})

	// then
	require.ErrorIs(t, err, ErrUserNotFound)
}
