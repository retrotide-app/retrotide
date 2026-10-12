package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEmailVerificationRepositoryIssue_ReplacesTheUnusedTokens(t *testing.T) {
	// given
	verificationDAO := dao.NewMockEmailVerificationDAO(t)
	repo := NewEmailVerificationRepo(nil, verificationDAO)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	issued := spec.NewEmailVerification{TokenHash: "verify-hash", UserID: uuid.New(), PendingEmail: new("next@example.com"), ExpiresAt: time.Now().Add(time.Hour)}
	verificationDAO.EXPECT().DeleteUnusedForUser(mock.Anything, issued.UserID, inTx).Return(nil).Once()
	verificationDAO.EXPECT().Create(mock.Anything, issued, inTx).Return(nil).Once()

	// when
	err := repo.Issue(t.Context(), issued, tx)

	// then
	require.NoError(t, err)
}

func TestEmailVerificationRepositoryIssue_StopsWhenTheOldTokensCannotBeCleared(t *testing.T) {
	// given no Create expectation, so issuing beside a live token fails the test
	verificationDAO := dao.NewMockEmailVerificationDAO(t)
	repo := NewEmailVerificationRepo(nil, verificationDAO)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	clearErr := errors.New("db down")
	issued := spec.NewEmailVerification{TokenHash: "verify-hash", UserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}
	verificationDAO.EXPECT().DeleteUnusedForUser(mock.Anything, issued.UserID, inTx).Return(clearErr).Once()

	// when
	err := repo.Issue(t.Context(), issued, tx)

	// then
	require.ErrorIs(t, err, clearErr)
}
