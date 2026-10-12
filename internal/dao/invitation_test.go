package dao_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/token"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	invitedAddress     = "carol@example.com"
	concurrentRedeems  = 8
	invitationLifetime = 7 * 24 * time.Hour
)

func createInvitation(t *testing.T, repos *repository.Repositories, address string, expiresAt time.Time) (*model.Invitation, string) {
	t.Helper()

	drawn, err := token.New()
	require.NoError(t, err)

	created, err := repos.Invitation.Create(t.Context(), spec.NewInvitation{Email: address, TokenHash: token.Hash(drawn), ExpiresAt: expiresAt})
	require.NoError(t, err)

	return created, drawn
}

func invitedUser() spec.NewUser {
	return spec.NewUser{Email: "ignored@example.com", PasswordHash: new(daotest.TestPasswordHash), DisplayName: "Carol"}
}

func TestInvitation_CreateStoresTheHashAndTheExpiry(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	expiresAt := time.Now().Add(invitationLifetime).UTC().Truncate(time.Microsecond)

	// when
	created, drawn := createInvitation(t, repos, invitedAddress, expiresAt)

	// then only the hash is stored, and the row can be read back by it
	assert.Equal(t, invitedAddress, created.Email)
	assert.Equal(t, token.Hash(drawn), created.TokenHash)
	assert.NotEqual(t, drawn, created.TokenHash)
	assert.True(t, expiresAt.Equal(created.ExpiresAt))
	assert.Equal(t, created.CreatedAt, created.UpdatedAt)

	found, err := repos.Invitation.GetLiveByTokenHash(t.Context(), token.Hash(drawn))
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	byID, err := repos.Invitation.GetByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byID.ID)
}

func TestInvitation_OneAddressHoldsOnePendingInvitationInAnyCase(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	createInvitation(t, repos, invitedAddress, time.Now().Add(invitationLifetime))

	// when
	_, err := repos.Invitation.Create(t.Context(), spec.NewInvitation{Email: "CAROL@example.com", TokenHash: token.Hash("another"), ExpiresAt: time.Now().Add(invitationLifetime)})

	// then
	require.ErrorIs(t, err, dao.ErrConflict)
}

func TestInvitation_AnExpiredInvitationIsNeitherShownNorRedeemed(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	_, drawn := createInvitation(t, repos, invitedAddress, time.Now().Add(-time.Minute))

	// when
	_, getErr := repos.Invitation.GetLiveByTokenHash(t.Context(), token.Hash(drawn))
	_, redeemErr := repos.Invitation.Redeem(t.Context(), spec.InvitationRedemption{TokenHash: token.Hash(drawn), User: invitedUser()})

	// then no account is created, and the row stays for staff to resend or cancel
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, redeemErr, dao.ErrNotFound)
	assert.Zero(t, countRows(t, repos, "users"))
	assert.Equal(t, 1, countRows(t, repos, "invitations"))
}

func TestInvitation_RedeemCreatesAVerifiedAccountForTheInvitedAddressAndSpendsTheInvitation(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	_, drawn := createInvitation(t, repos, invitedAddress, time.Now().Add(invitationLifetime))

	// when
	created, err := repos.Invitation.Redeem(t.Context(), spec.InvitationRedemption{TokenHash: token.Hash(drawn), User: invitedUser()})

	// then the address comes from the invitation, the link proves the mailbox, and nothing else is written
	require.NoError(t, err)
	assert.Equal(t, invitedAddress, created.Email)
	assert.True(t, created.EmailVerified)
	assert.Equal(t, "Carol", created.DisplayName)

	stored, err := repos.User.GetByEmail(t.Context(), invitedAddress)
	require.NoError(t, err)
	assert.Equal(t, created.ID, stored.ID)
	assert.True(t, stored.EmailVerified)

	assert.Zero(t, countRows(t, repos, "invitations"))
	assert.Zero(t, countRows(t, repos, "sessions"))
	assert.Zero(t, countRows(t, repos, "email_verification_tokens"))
	assert.Zero(t, countRows(t, repos, "user_roles"))
}

func TestInvitation_AnInvitationIsRedeemedOnce(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	_, drawn := createInvitation(t, repos, invitedAddress, time.Now().Add(invitationLifetime))
	_, err := repos.Invitation.Redeem(t.Context(), spec.InvitationRedemption{TokenHash: token.Hash(drawn), User: invitedUser()})
	require.NoError(t, err)

	// when
	_, again := repos.Invitation.Redeem(t.Context(), spec.InvitationRedemption{TokenHash: token.Hash(drawn), User: invitedUser()})

	// then
	require.ErrorIs(t, again, dao.ErrNotFound)
	assert.Equal(t, 1, countRows(t, repos, "users"))
}

func TestInvitation_ConcurrentRedemptionsCreateOneAccount(t *testing.T) {
	// given one link opened in several tabs at once
	repos := daotest.NewRepos(t)
	_, drawn := createInvitation(t, repos, invitedAddress, time.Now().Add(invitationLifetime))

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		failures  []error
	)

	// when
	for range concurrentRedeems {
		wg.Go(func() {
			_, err := repos.Invitation.Redeem(t.Context(), spec.InvitationRedemption{TokenHash: token.Hash(drawn), User: invitedUser()})

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				succeeded++
				return
			}

			failures = append(failures, err)
		})
	}
	wg.Wait()

	// then exactly one wins, and every other attempt finds the invitation gone
	assert.Equal(t, 1, succeeded)
	for _, failure := range failures {
		assert.ErrorIs(t, failure, dao.ErrNotFound)
	}
	assert.Equal(t, 1, countRows(t, repos, "users"))
	assert.Zero(t, countRows(t, repos, "invitations"))
}

func TestInvitation_ARedemptionForAnAddressTakenMeanwhileLeavesTheInvitation(t *testing.T) {
	// given an account took the invited address after the invitation was sent
	repos := daotest.NewRepos(t)
	_, drawn := createInvitation(t, repos, invitedAddress, time.Now().Add(invitationLifetime))
	daotest.CreateUser(t, repos, daotest.WithEmail("Carol@example.com"))

	// when
	_, err := repos.Invitation.Redeem(t.Context(), spec.InvitationRedemption{TokenHash: token.Hash(drawn), User: invitedUser()})

	// then the consumption rolls back with the failed insert
	require.ErrorIs(t, err, dao.ErrConflict)
	assert.Equal(t, 1, countRows(t, repos, "invitations"))
	assert.Equal(t, 1, countRows(t, repos, "users"))
}

func TestInvitation_RenewReplacesTheHashAndTheExpiry(t *testing.T) {
	// given an invitation that lapsed
	repos := daotest.NewRepos(t)
	created, earlier := createInvitation(t, repos, invitedAddress, time.Now().Add(-time.Hour))
	later, err := token.New()
	require.NoError(t, err)

	// when
	renewed, err := repos.Invitation.Renew(t.Context(), spec.InvitationRenewal{ID: created.ID, TokenHash: token.Hash(later), ExpiresAt: time.Now().Add(invitationLifetime)})

	// then only the new link works, and the resend moves the sent time on
	require.NoError(t, err)
	assert.Equal(t, created.ID, renewed.ID)
	assert.Equal(t, invitedAddress, renewed.Email)
	assert.True(t, renewed.UpdatedAt.After(created.UpdatedAt) || renewed.UpdatedAt.Equal(created.UpdatedAt))
	assert.Equal(t, created.CreatedAt, renewed.CreatedAt)

	_, earlierErr := repos.Invitation.GetLiveByTokenHash(t.Context(), token.Hash(earlier))
	require.ErrorIs(t, earlierErr, dao.ErrNotFound)
	_, laterErr := repos.Invitation.GetLiveByTokenHash(t.Context(), token.Hash(later))
	require.NoError(t, laterErr)
}

func TestInvitation_DeleteRemovesItAndAnUnknownIDIsNotFound(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	created, drawn := createInvitation(t, repos, invitedAddress, time.Now().Add(invitationLifetime))

	// when
	removed, err := repos.Invitation.Delete(t.Context(), created.ID)
	_, again := repos.Invitation.Delete(t.Context(), created.ID)
	_, renewErr := repos.Invitation.Renew(t.Context(), spec.InvitationRenewal{ID: created.ID, TokenHash: token.Hash("x"), ExpiresAt: time.Now().Add(time.Hour)})
	_, getErr := repos.Invitation.GetByID(t.Context(), created.ID)

	// then its link stops working at once
	require.NoError(t, err)
	assert.Equal(t, invitedAddress, removed.Email)
	require.ErrorIs(t, again, dao.ErrNotFound)
	require.ErrorIs(t, renewErr, dao.ErrNotFound)
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	_, liveErr := repos.Invitation.GetLiveByTokenHash(t.Context(), token.Hash(drawn))
	require.ErrorIs(t, liveErr, dao.ErrNotFound)
}

func TestInvitation_ListsNewestFirstByPageWithTheTotal(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	var created []*model.Invitation
	for i := range 3 {
		invitation, _ := createInvitation(t, repos, fmt.Sprintf("invitee-%d@example.com", i), time.Now().Add(invitationLifetime))
		created = append(created, invitation)
	}

	// when
	page, err := repos.Invitation.List(t.Context(), spec.InvitationListing{Limit: 2, Offset: 0})
	rest, restErr := repos.Invitation.List(t.Context(), spec.InvitationListing{Limit: 2, Offset: 2})
	total, countErr := repos.Invitation.Count(t.Context())

	// then
	require.NoError(t, err)
	require.NoError(t, restErr)
	require.NoError(t, countErr)
	assert.Equal(t, 3, total)
	require.Len(t, page, 2)
	require.Len(t, rest, 1)

	listed := []uuid.UUID{page[0].ID, page[1].ID, rest[0].ID}
	assert.ElementsMatch(t, []uuid.UUID{created[0].ID, created[1].ID, created[2].ID}, listed)
	assert.False(t, page[0].CreatedAt.Before(page[1].CreatedAt))
	assert.False(t, page[1].CreatedAt.Before(rest[0].CreatedAt))
}
