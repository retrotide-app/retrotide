package user

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"unicode/utf8"

	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type (
	Service interface {
		NewAccountSpec(ctx context.Context, account spec.NewAccount) (spec.NewUser, error)
		GetByID(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error)
		ValidateCredentials(ctx context.Context, credentials spec.UserCredentials) (*dto.UserResponse, error)
		CheckPassword(ctx context.Context, confirmation spec.UserPasswordConfirmation) error
		HashPassword(ctx context.Context, password string) (string, error)
		UpdateDisplayName(ctx context.Context, update spec.UserDisplayNameUpdate) (*dto.UserResponse, error)
	}

	service struct {
		repo     repository.UserRepository
		settings settings.Service
		compare  func(hashedPassword []byte, password []byte) error
	}
)

const (
	timingPassword = "retrotide compares every sign-in against a bcrypt hash"
)

var (
	timingHash = sync.OnceValues(func() ([]byte, error) {
		hash, err := bcrypt.GenerateFromPassword([]byte(timingPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("build the timing hash: %w", err)
		}

		return hash, nil
	})
)

func NewService(repo repository.UserRepository, settingsSvc settings.Service) Service {
	return &service{repo: repo, settings: settingsSvc, compare: bcrypt.CompareHashAndPassword}
}

func (s *service) NewAccountSpec(ctx context.Context, account spec.NewAccount) (spec.NewUser, error) {
	hash, err := s.HashPassword(ctx, account.Password)
	if err != nil {
		return spec.NewUser{}, err
	}

	return spec.NewUser{
		Email:        account.Email,
		PasswordHash: new(hash),
		DisplayName:  ClampDisplayName(account.DisplayName),
	}, nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error) {
	found, err := s.lookUp(ctx, id)
	if err != nil {
		return nil, err
	}

	return new(dto.NewUserResponse(found)), nil
}

func (s *service) ValidateCredentials(ctx context.Context, credentials spec.UserCredentials) (*dto.UserResponse, error) {
	found, err := s.repo.GetByEmail(ctx, credentials.Email)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, s.refuseAfterComparing(credentials.Password)
	}
	if err != nil {
		return nil, fmt.Errorf("look up the account by email: %w", err)
	}

	if !found.HasPassword() {
		return nil, s.refuseAfterComparing(credentials.Password)
	}

	matches, err := s.passwordMatches(found, credentials.Password)
	if err != nil {
		return nil, fmt.Errorf("check the password of user %s: %w", found.ID, err)
	}

	if !matches {
		return nil, ErrInvalidCredentials
	}

	return new(dto.NewUserResponse(found)), nil
}

func (s *service) CheckPassword(ctx context.Context, confirmation spec.UserPasswordConfirmation) error {
	found, err := s.lookUp(ctx, confirmation.UserID)
	if err != nil {
		return err
	}

	matches, err := s.passwordMatches(found, confirmation.Password)
	if err != nil {
		return fmt.Errorf("check the password of user %s: %w", found.ID, err)
	}

	if !matches {
		return ErrIncorrectPassword
	}

	return nil
}

func (s *service) HashPassword(ctx context.Context, password string) (string, error) {
	minLength, err := s.settings.GetInt(ctx, config.SettingMinPasswordLength)
	if err != nil {
		return "", fmt.Errorf("read the minimum password length: %w", err)
	}

	if utf8.RuneCountInString(password) < minLength {
		return "", ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return "", ErrPasswordTooLong
	}
	if err != nil {
		return "", fmt.Errorf("hash the password: %w", err)
	}

	return string(hash), nil
}

func (s *service) UpdateDisplayName(ctx context.Context, update spec.UserDisplayNameUpdate) (*dto.UserResponse, error) {
	displayName := ClampDisplayName(update.DisplayName)
	if displayName == "" {
		return nil, ErrDisplayNameRequired
	}

	err := s.repo.SetDisplayName(ctx, spec.UserDisplayNameUpdate{UserID: update.UserID, DisplayName: displayName})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set the display name of user %s: %w", update.UserID, err)
	}

	return s.GetByID(ctx, update.UserID)
}

func (s *service) lookUp(ctx context.Context, id uuid.UUID) (*model.User, error) {
	found, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("look up user %s: %w", id, err)
	}

	return found, nil
}

func (s *service) passwordMatches(u *model.User, password string) (bool, error) {
	if !u.HasPassword() {
		return false, nil
	}

	return s.hashMatches([]byte(*u.PasswordHash), password)
}

func (s *service) refuseAfterComparing(password string) error {
	hash, err := timingHash()
	if err != nil {
		return err
	}

	if _, err := s.hashMatches(hash, password); err != nil {
		return fmt.Errorf("compare against the timing hash: %w", err)
	}

	return ErrInvalidCredentials
}

func (s *service) hashMatches(hash []byte, password string) (bool, error) {
	err := s.compare(hash, []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}
