package repository

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func invitationRedemption() spec.InvitationRedemption {
	return spec.InvitationRedemption{
		TokenHash: "invitation-hash",
		User:      spec.NewUser{Email: "someone-else@example.com", PasswordHash: new("password-hash"), DisplayName: "Carol"},
	}
}

func TestInvitationRepositoryRedeem_CreatesAVerifiedAccountForTheInvitedAddressInTheSameTransaction(t *testing.T) {
	// given
	invitationDAO := dao.NewMockInvitationDAO(t)
	userDAO := dao.NewMockUserDAO(t)
	repo := NewInvitationRepo(nil, invitationDAO, userDAO)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	consumed := &model.Invitation{ID: uuid.New(), Email: "carol@example.com", TokenHash: "invitation-hash", ExpiresAt: time.Now().Add(time.Hour)}
	created := &model.User{ID: uuid.New(), Email: "carol@example.com", EmailVerified: true}
	invitationDAO.EXPECT().Consume(mock.Anything, "invitation-hash", inTx).Return(consumed, nil).Once()
	userDAO.EXPECT().Create(mock.Anything, spec.NewUser{Email: "carol@example.com", PasswordHash: new("password-hash"), DisplayName: "Carol", EmailVerified: true}, inTx).Return(created, nil).Once()

	// when
	user, err := repo.Redeem(t.Context(), invitationRedemption(), tx)

	// then the address comes from the invitation, never from the caller, and the link proves the mailbox
	require.NoError(t, err)
	assert.Equal(t, created, user)
}

func TestInvitationRepositoryRedeem_WritesNoAccountForAnInvitationThatIsGone(t *testing.T) {
	// given no Create expectation, so an account written for a used or expired invitation fails the test
	invitationDAO := dao.NewMockInvitationDAO(t)
	repo := NewInvitationRepo(nil, invitationDAO, dao.NewMockUserDAO(t))
	tx := new(sql.Tx)
	invitationDAO.EXPECT().Consume(mock.Anything, "invitation-hash", []*sql.Tx{tx}).Return(nil, fmt.Errorf("consume invitation: %w", dao.ErrNotFound)).Once()

	// when
	user, err := repo.Redeem(t.Context(), invitationRedemption(), tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
	assert.Nil(t, user)
}

func TestInvitationRepositoryRedeem_ReturnsATakenAddress(t *testing.T) {
	// given
	invitationDAO := dao.NewMockInvitationDAO(t)
	userDAO := dao.NewMockUserDAO(t)
	repo := NewInvitationRepo(nil, invitationDAO, userDAO)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	invitationDAO.EXPECT().Consume(mock.Anything, "invitation-hash", inTx).Return(&model.Invitation{ID: uuid.New(), Email: "carol@example.com"}, nil).Once()
	userDAO.EXPECT().Create(mock.Anything, mock.Anything, inTx).Return(nil, fmt.Errorf("create user: %w", dao.ErrConflict)).Once()

	// when
	user, err := repo.Redeem(t.Context(), invitationRedemption(), tx)

	// then the caller's transaction rolls the consumption back with it
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Nil(t, user)
}
