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

func TestPasswordResetRepositoryIssue_ReplacesTheUnusedTokens(t *testing.T) {
	// given
	resetDAO := dao.NewMockPasswordResetDAO(t)
	repo := NewPasswordResetRepo(nil, resetDAO)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	issued := spec.NewPasswordReset{TokenHash: "reset-hash", UserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}
	resetDAO.EXPECT().DeleteUnusedForUser(mock.Anything, issued.UserID, inTx).Return(nil).Once()
	resetDAO.EXPECT().Create(mock.Anything, issued, inTx).Return(nil).Once()

	// when
	err := repo.Issue(t.Context(), issued, tx)

	// then
	require.NoError(t, err)
}

func TestPasswordResetRepositoryIssue_StopsWhenTheOldTokensCannotBeCleared(t *testing.T) {
	// given no Create expectation, so issuing beside a live token fails the test
	resetDAO := dao.NewMockPasswordResetDAO(t)
	repo := NewPasswordResetRepo(nil, resetDAO)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	clearErr := errors.New("db down")
	issued := spec.NewPasswordReset{TokenHash: "reset-hash", UserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}
	resetDAO.EXPECT().DeleteUnusedForUser(mock.Anything, issued.UserID, inTx).Return(clearErr).Once()

	// when
	err := repo.Issue(t.Context(), issued, tx)

	// then
	require.ErrorIs(t, err, clearErr)
}
