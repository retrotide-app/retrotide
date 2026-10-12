package admin

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/role"
	"retrotide/internal/token"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListUsers_ClampsThePageAndTrimsTheSearch(t *testing.T) {
	// given
	svc, m := newTestService(t)
	page := spec.UserListing{Search: "alice", Limit: 100, Offset: 0}
	m.users.EXPECT().ListWithRoles(mock.Anything, page).Return([]model.UserWithRole{}, nil)
	m.users.EXPECT().Count(mock.Anything, "alice").Return(0, nil)

	// when
	listing, err := svc.ListUsers(t.Context(), spec.UserListing{Search: "  alice ", Limit: 5000, Offset: -4})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.AdminUserListResponse{Users: []dto.AdminUserResponse{}, Total: 0, Limit: 100, Offset: 0}, listing)
}

func TestListUsers_MapsEachAccountWithItsRoleAndState(t *testing.T) {
	// given
	svc, m := newTestService(t)
	banned := uuid.New()
	bannedAt := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	rows := []model.UserWithRole{
		{User: *account(banned), Role: role.Support},
	}
	rows[0].BannedAt = &bannedAt
	m.users.EXPECT().ListWithRoles(mock.Anything, spec.UserListing{Limit: defaultUserPageSize}).Return(rows, nil)
	m.users.EXPECT().Count(mock.Anything, "").Return(41, nil)

	// when
	listing, err := svc.ListUsers(t.Context(), spec.UserListing{})

	// then
	require.NoError(t, err)
	assert.Equal(t, 41, listing.Total)
	assert.Equal(t, []dto.AdminUserResponse{{
		ID:            banned,
		Email:         testEmail,
		DisplayName:   testDisplayName,
		Role:          role.Support,
		EmailVerified: true,
		Banned:        true,
		Locked:        false,
		CreatedAt:     "2026-09-01T12:00:00Z",
	}}, listing.Users)
}

func TestListUsers_ReturnsAReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.users.EXPECT().ListWithRoles(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	listing, err := svc.ListUsers(t.Context(), spec.UserListing{})

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listing)
}

func TestGetUser_ShowsTheAccountWithItsRoleAndRestrictions(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID, lockedBy := uuid.New(), uuid.New()
	lockedAt := time.Date(2026, 9, 21, 9, 30, 0, 0, time.UTC)
	found := account(userID)
	found.PasswordHash = new("hash")
	found.LockedAt = &lockedAt
	found.LockedBy = &lockedBy
	found.LockReason = "spam"
	found.LastSeenIP = new("198.51.100.4")
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(found, nil)
	m.authz.EXPECT().GetRole(mock.Anything, userID).Return(role.Support, nil)

	// when
	detail, err := svc.GetUser(t.Context(), userID)

	// then
	require.NoError(t, err)
	assert.Equal(t, role.Support, detail.Role)
	assert.True(t, detail.Locked)
	assert.True(t, detail.HasPassword)
	assert.Equal(t, "spam", detail.LockReason)
	assert.Equal(t, "2026-09-21T09:30:00Z", detail.LockedAt)
	assert.Equal(t, &lockedBy, detail.LockedBy)
	assert.Equal(t, "198.51.100.4", detail.LastSeenIP)
	assert.Empty(t, detail.BannedAt)
}

func TestGetUser_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.users.EXPECT().GetByID(mock.Anything, userID).Return(nil, errRowMissing)

	// when
	detail, err := svc.GetUser(t.Context(), userID)

	// then
	assert.ErrorIs(t, err, ErrUserNotFound)
	assert.Nil(t, detail)
}

func TestSetUserRole_RefusesARoleOutsideTheStaffCatalogue(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.SetUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(uuid.New()), UserID: uuid.New(), Role: "moderator"})

	// then
	assert.ErrorIs(t, err, ErrUnknownRole)
}

func TestSetUserRole_AdminGrantsSupport(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.roles.EXPECT().SetRole(mock.Anything, spec.RoleAssignment{UserID: targetID, Role: role.Support}).Return(nil)
	m.audited(expectedUserEntry(actorID, audit.ActionSetRole, targetID, map[string]string{"role": "support"}))

	// when
	err := svc.SetUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: role.Support})

	// then
	assert.NoError(t, err)
}

func TestSetUserRole_SuperAdminPromotesSupportAndRecordsTheFormerRole(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.SuperAdmin, targetID, role.Support)
	m.roles.EXPECT().SetRole(mock.Anything, spec.RoleAssignment{UserID: targetID, Role: role.Admin}).Return(nil)
	m.audited(expectedUserEntry(actorID, audit.ActionSetRole, targetID, map[string]string{"role": "admin", "previous_role": "support"}))

	// when
	err := svc.SetUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: role.Admin})

	// then
	assert.NoError(t, err)
}

func TestSetUserRole_NobodyGrantsARoleAtOrAboveTheirOwn(t *testing.T) {
	tests := []struct {
		name    string
		actor   role.Role
		granted role.Role
	}{
		{name: "admin grants admin", actor: role.Admin, granted: role.Admin},
		{name: "admin grants super admin", actor: role.Admin, granted: role.SuperAdmin},
		{name: "super admin grants super admin", actor: role.SuperAdmin, granted: role.SuperAdmin},
		{name: "support grants support", actor: role.Support, granted: role.Support},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			actorID, targetID := uuid.New(), uuid.New()
			m.ranks(actorID, tc.actor, targetID, "")

			// when
			err := svc.SetUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: tc.granted})

			// then
			assert.ErrorIs(t, err, ErrRoleOutranksActor)
		})
	}
}

func TestSetUserRole_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.roles.EXPECT().SetRole(mock.Anything, mock.Anything).Return(errRowMissing)

	// when
	err := svc.SetUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: role.Support})

	// then nothing is audited
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestSetUserRole_AStaleCacheIsReportedButTheChangeIsStillAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	stale := fmt.Errorf("role of user %s %w: valkey unreachable", targetID, repository.ErrStaleStaffCache)
	m.ranks(actorID, role.Admin, targetID, "")
	m.roles.EXPECT().SetRole(mock.Anything, spec.RoleAssignment{UserID: targetID, Role: role.Support}).Return(stale)
	m.audited(expectedUserEntry(actorID, audit.ActionSetRole, targetID, map[string]string{"role": "support"}))

	// when
	err := svc.SetUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: role.Support})

	// then the role is saved, so the change is recorded, and the admin still learns of the stale cache
	assert.ErrorIs(t, err, ErrStaffCacheStale)
	assert.ErrorIs(t, err, repository.ErrStaleStaffCache)
	assert.NotErrorIs(t, err, ErrNotAudited)
}

func TestRemoveUserRole_RemovesTheHeldRole(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, role.Support)
	m.roles.EXPECT().RemoveRole(mock.Anything, spec.RoleAssignment{UserID: targetID, Role: role.Support}).Return(nil)
	m.audited(expectedUserEntry(actorID, audit.ActionRemoveRole, targetID, map[string]string{"role": "support"}))

	// when
	err := svc.RemoveUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: role.Support})

	// then
	assert.NoError(t, err)
}

func TestRemoveUserRole_AStaleCacheIsReportedButTheChangeIsStillAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	stale := fmt.Errorf("role of user %s %w: valkey unreachable", targetID, repository.ErrStaleStaffCache)
	m.ranks(actorID, role.Admin, targetID, role.Support)
	m.roles.EXPECT().RemoveRole(mock.Anything, spec.RoleAssignment{UserID: targetID, Role: role.Support}).Return(stale)
	m.audited(expectedUserEntry(actorID, audit.ActionRemoveRole, targetID, map[string]string{"role": "support"}))

	// when
	err := svc.RemoveUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: role.Support})

	// then the old role can still be served for up to a minute, and the admin is told so
	assert.ErrorIs(t, err, ErrStaffCacheStale)
	assert.ErrorIs(t, err, repository.ErrStaleStaffCache)
	assert.NotErrorIs(t, err, ErrNotAudited)
}

func TestRemoveUserRole_ARoleTheUserDoesNotHoldIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.SuperAdmin, targetID, role.Admin)
	m.roles.EXPECT().RemoveRole(mock.Anything, spec.RoleAssignment{UserID: targetID, Role: role.Support}).Return(errRowMissing)

	// when
	err := svc.RemoveUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(actorID), UserID: targetID, Role: role.Support})

	// then
	assert.ErrorIs(t, err, ErrRoleNotHeld)
}

func TestRemoveUserRole_RefusesARoleOutsideTheStaffCatalogue(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.RemoveUserRole(t.Context(), spec.StaffRoleChange{StaffAction: staffAction(uuid.New()), UserID: uuid.New(), Role: "owner"})

	// then
	assert.ErrorIs(t, err, ErrUnknownRole)
}

func TestBanUser_BansDeletesEverySessionAndDisconnects(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().BanAccount(mock.Anything, spec.UserBan{UserID: targetID, BannedBy: actorID, Reason: "spam"}).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(targetID).Return(2)
	m.audited(expectedUserEntry(actorID, audit.ActionBanUser, targetID, map[string]string{"reason": "spam"}))

	// when
	err := svc.BanUser(t.Context(), spec.StaffUserRestriction{StaffAction: staffAction(actorID), UserID: targetID, Reason: "  spam  "})

	// then
	assert.NoError(t, err)
}

func TestBanUser_AndLockUser_RequireAReason(t *testing.T) {
	tests := []struct {
		name string
		act  func(ctx context.Context, svc *service, restriction spec.StaffUserRestriction) error
	}{
		{name: "ban", act: func(ctx context.Context, svc *service, restriction spec.StaffUserRestriction) error {
			return svc.BanUser(ctx, restriction)
		}},
		{name: "lock", act: func(ctx context.Context, svc *service, restriction spec.StaffUserRestriction) error {
			return svc.LockUser(ctx, restriction)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			err := tc.act(t.Context(), svc, spec.StaffUserRestriction{StaffAction: staffAction(uuid.New()), UserID: uuid.New(), Reason: " \t "})

			// then nothing is read or written
			assert.ErrorIs(t, err, ErrReasonRequired)
		})
	}
}

func TestBanUser_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.users.EXPECT().BanAccount(mock.Anything, mock.Anything).Return(errRowMissing)

	// when
	err := svc.BanUser(t.Context(), spec.StaffUserRestriction{StaffAction: staffAction(actorID), UserID: targetID, Reason: "spam"})

	// then no socket is closed and nothing is audited
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestBanUser_ReturnsAWriteFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.users.EXPECT().BanAccount(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	err := svc.BanUser(t.Context(), spec.StaffUserRestriction{StaffAction: staffAction(actorID), UserID: targetID, Reason: "spam"})

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.NotErrorIs(t, err, ErrUserNotFound)
}

func TestLockUser_LocksWithoutSigningTheUserOut(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().Lock(mock.Anything, spec.UserLock{UserID: targetID, LockedBy: actorID, Reason: "cooling off"}).Return(nil)
	m.audited(expectedUserEntry(actorID, audit.ActionLockUser, targetID, map[string]string{"reason": "cooling off"}))

	// when
	err := svc.LockUser(t.Context(), spec.StaffUserRestriction{StaffAction: staffAction(actorID), UserID: targetID, Reason: "cooling off"})

	// then the strict disconnector proves no socket was closed
	assert.NoError(t, err)
}

func TestUnlockUser(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().Unlock(mock.Anything, targetID).Return(nil)
	m.audited(expectedUserEntry(actorID, audit.ActionUnlockUser, targetID, nil))

	// when
	err := svc.UnlockUser(t.Context(), userAction(actorID, targetID))

	// then
	assert.NoError(t, err)
}

func TestUnbanUser_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().Unban(mock.Anything, targetID).Return(errRowMissing)

	// when
	err := svc.UnbanUser(t.Context(), userAction(actorID, targetID))

	// then
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestForceLogout_DeletesEverySessionAndDisconnects(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.sessionRepo.EXPECT().DeleteAllForUser(mock.Anything, targetID).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(targetID).Return(1)
	m.audited(expectedUserEntry(actorID, audit.ActionForceLogout, targetID, nil))

	// when
	err := svc.ForceLogout(t.Context(), userAction(actorID, targetID))

	// then
	assert.NoError(t, err)
}

func TestForceLogout_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(nil, errRowMissing)

	// when
	err := svc.ForceLogout(t.Context(), userAction(actorID, targetID))

	// then no session is touched and nothing is audited
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestDeleteUser_DeletesDisconnectsAndRecordsTheAddress(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.users.EXPECT().DeleteAccount(mock.Anything, targetID).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(targetID).Return(0)
	m.audited(audit.NewEntry{
		ActorID:    actorID,
		Action:     audit.ActionDeleteUser,
		TargetType: audit.TargetUser,
		TargetID:   targetID.String(),
		Details:    map[string]string{"email": testEmail},
	})

	// when
	err := svc.DeleteUser(t.Context(), userAction(actorID, targetID))

	// then the entry names no subject, because the account row it would reference is gone
	assert.NoError(t, err)
}

func TestDeleteUser_TheSoleOwnerOfASharedOrganisationIsRefused(t *testing.T) {
	// given the account deletion path, which refuses while the user owns an organisation with other members
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.users.EXPECT().DeleteAccount(mock.Anything, targetID).Return(fmt.Errorf("delete account %s: %w: acme-retros", targetID, repository.ErrOwnsSharedOrganisation))

	// when
	err := svc.DeleteUser(t.Context(), userAction(actorID, targetID))

	// then the strict mocks prove no socket was dropped and nothing was audited
	assert.ErrorIs(t, err, ErrSoleOwner)
	assert.NotErrorIs(t, err, ErrUserNotFound)
}

func TestDeleteUser_AnAccountRemovedMeanwhileIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.users.EXPECT().DeleteAccount(mock.Anything, targetID).Return(errRowMissing)

	// when
	err := svc.DeleteUser(t.Context(), userAction(actorID, targetID))

	// then
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestDeleteUser_AProtectedAccountIsNeverRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, role.SuperAdmin)

	// when
	err := svc.DeleteUser(t.Context(), userAction(actorID, targetID))

	// then
	assert.ErrorIs(t, err, ErrProtectedUser)
}

func TestDeleteUser_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(nil, errRowMissing)

	// when
	err := svc.DeleteUser(t.Context(), userAction(actorID, targetID))

	// then
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestResetUserPassword_SetsAFreshPasswordAndSignsTheUserOutEverywhere(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingMinPasswordLength).Return(8, nil)

	var hashed string
	m.accounts.EXPECT().HashPassword(mock.Anything, mock.AnythingOfType("string")).RunAndReturn(func(_ context.Context, password string) (string, error) {
		hashed = password
		return "new-hash", nil
	})
	m.users.EXPECT().ChangePassword(mock.Anything, spec.PasswordChange{UserID: targetID, PasswordHash: "new-hash"}).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(targetID).Return(1)
	m.audited(expectedUserEntry(actorID, audit.ActionResetPassword, targetID, nil))

	// when
	reset, err := svc.ResetUserPassword(t.Context(), userAction(actorID, targetID))

	// then the change keeps no session, since KeepToken is empty, and cancels pending email changes with it
	require.NoError(t, err)
	assert.Equal(t, hashed, reset.Password)
	assert.Len(t, reset.Password, 26)
}

func TestResetUserPassword_MeetsAMinimumLengthAboveTheGeneratedText(t *testing.T) {
	// given a site whose minimum is longer than one rand.Text
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingMinPasswordLength).Return(30, nil)

	var hashed string
	m.accounts.EXPECT().HashPassword(mock.Anything, mock.AnythingOfType("string")).RunAndReturn(func(_ context.Context, password string) (string, error) {
		hashed = password
		return "new-hash", nil
	})
	m.users.EXPECT().ChangePassword(mock.Anything, spec.PasswordChange{UserID: targetID, PasswordHash: "new-hash"}).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(targetID).Return(0)
	m.audited(expectedUserEntry(actorID, audit.ActionResetPassword, targetID, nil))

	// when
	reset, err := svc.ResetUserPassword(t.Context(), userAction(actorID, targetID))

	// then
	require.NoError(t, err)
	assert.Equal(t, hashed, reset.Password)
	assert.GreaterOrEqual(t, len(reset.Password), 30)
}

func TestResetUserPassword_ReturnsAMinimumLengthReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingMinPasswordLength).Return(0, errDatabaseDown)

	// when
	reset, err := svc.ResetUserPassword(t.Context(), userAction(actorID, targetID))

	// then the strict mocks prove that no password is set
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, reset)
}

func TestResetUserPassword_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingMinPasswordLength).Return(8, nil)
	m.accounts.EXPECT().HashPassword(mock.Anything, mock.Anything).Return("new-hash", nil)
	m.users.EXPECT().ChangePassword(mock.Anything, mock.Anything).Return(errRowMissing)

	// when
	reset, err := svc.ResetUserPassword(t.Context(), userAction(actorID, targetID))

	// then
	assert.ErrorIs(t, err, ErrUserNotFound)
	assert.Nil(t, reset)
}

func TestResetUserPassword_StillHandsBackThePasswordWhenTheAuditFails(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.settings.EXPECT().GetInt(mock.Anything, config.SettingMinPasswordLength).Return(8, nil)
	m.accounts.EXPECT().HashPassword(mock.Anything, mock.Anything).Return("new-hash", nil)
	m.users.EXPECT().ChangePassword(mock.Anything, mock.Anything).Return(nil)
	m.disconnector.EXPECT().DisconnectUser(targetID).Return(0)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	reset, err := svc.ResetUserPassword(t.Context(), userAction(actorID, targetID))

	// then the password is already set, so losing it would lock the user out
	assert.ErrorIs(t, err, ErrNotAudited)
	require.NotNil(t, reset)
	assert.NotEmpty(t, reset.Password)
}

func TestSetUserEmail_HoldsTheNewAddressUntilItsLinkIsUsed(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.users.EXPECT().EmailInUse(mock.Anything, spec.UserEmailFilter{Email: testNewEmail, ExcludeUserID: targetID}).Return(false, nil)

	var pending spec.NewEmailVerification
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, issued spec.NewEmailVerification, _ ...*sql.Tx) error {
		pending = issued
		return nil
	})
	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)

	var sent []email.Message
	m.email.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, message email.Message) error {
		sent = append(sent, message)
		return nil
	}).Times(2)
	m.audited(expectedUserEntry(actorID, audit.ActionSetUserEmail, targetID, map[string]string{"previous_email": testEmail, "pending_email": testNewEmail}))

	// when
	err := svc.SetUserEmail(t.Context(), spec.StaffEmailChange{StaffAction: staffAction(actorID), UserID: targetID, Email: " " + testNewEmail + " "})

	// then users.email is never written; the address waits beside its token
	require.NoError(t, err)
	assert.Equal(t, targetID, pending.UserID)
	assert.Equal(t, new(testNewEmail), pending.PendingEmail)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), pending.ExpiresAt, time.Minute)

	require.Len(t, sent, 2)
	assert.Equal(t, testEmail, sent[0].To)
	assert.Contains(t, sent[0].HTML, "https://retrotide.test/forgot-password")
	assert.Equal(t, testNewEmail, sent[1].To)

	link := regexp.MustCompile(`https://retrotide\.test/verify-email#([0-9a-f]{64})`).FindStringSubmatch(sent[1].HTML)
	require.Len(t, link, 2)
	assert.Equal(t, pending.TokenHash, token.Hash(link[1]))
}

func TestSetUserEmail_RefusesBeforeHoldingAnything(t *testing.T) {
	tests := []struct {
		name    string
		address string
		arrange func(m *testMocks, actorID uuid.UUID, targetID uuid.UUID)
		want    error
	}{
		{
			name:    "an invalid address",
			address: "not an address",
			arrange: func(*testMocks, uuid.UUID, uuid.UUID) {},
			want:    ErrInvalidEmail,
		},
		{
			name:    "a protected account",
			address: testNewEmail,
			arrange: func(m *testMocks, actorID uuid.UUID, targetID uuid.UUID) {
				m.ranks(actorID, role.Admin, targetID, role.Admin)
			},
			want: ErrProtectedUser,
		},
		{
			name:    "no email provider",
			address: testNewEmail,
			arrange: func(m *testMocks, actorID uuid.UUID, targetID uuid.UUID) {
				m.ranks(actorID, role.Admin, targetID, "")
				m.email.EXPECT().Enabled(mock.Anything).Return(false, nil)
			},
			want: ErrEmailUnavailable,
		},
		{
			name:    "the address the account already has",
			address: "ALICE@example.com",
			arrange: func(m *testMocks, actorID uuid.UUID, targetID uuid.UUID) {
				m.ranks(actorID, role.Admin, targetID, "")
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
			},
			want: ErrEmailUnchanged,
		},
		{
			name:    "an address another account holds",
			address: testNewEmail,
			arrange: func(m *testMocks, actorID uuid.UUID, targetID uuid.UUID) {
				m.ranks(actorID, role.Admin, targetID, "")
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
				m.users.EXPECT().EmailInUse(mock.Anything, mock.Anything).Return(true, nil)
			},
			want: ErrEmailTaken,
		},
		{
			name:    "an unknown user",
			address: testNewEmail,
			arrange: func(m *testMocks, actorID uuid.UUID, targetID uuid.UUID) {
				m.ranks(actorID, role.Admin, targetID, "")
				m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
				m.users.EXPECT().GetByID(mock.Anything, targetID).Return(nil, errRowMissing)
			},
			want: ErrUserNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			actorID, targetID := uuid.New(), uuid.New()
			tc.arrange(m, actorID, targetID)

			// when
			err := svc.SetUserEmail(t.Context(), spec.StaffEmailChange{StaffAction: staffAction(actorID), UserID: targetID, Email: tc.address})

			// then no token is issued, no email is sent and nothing is audited
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestSetUserEmail_AuditsTheHeldChangeEvenWhenASendFails(t *testing.T) {
	tests := []struct {
		name  string
		sends func(m *testMocks)
	}{
		{
			name: "the notice to the previous address fails",
			sends: func(m *testMocks) {
				m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errDatabaseDown).Once()
			},
		},
		{
			name: "the verification link to the new address fails",
			sends: func(m *testMocks) {
				m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(nil).Once()
				m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errDatabaseDown).Once()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			actorID, targetID := uuid.New(), uuid.New()
			m.ranks(actorID, role.Admin, targetID, "")
			m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
			m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
			m.users.EXPECT().EmailInUse(mock.Anything, mock.Anything).Return(false, nil)
			m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).Return(nil)
			m.audited(expectedUserEntry(actorID, audit.ActionSetUserEmail, targetID, map[string]string{"previous_email": testEmail, "pending_email": testNewEmail}))
			m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)
			m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
			tc.sends(m)

			// when
			err := svc.SetUserEmail(t.Context(), spec.StaffEmailChange{StaffAction: staffAction(actorID), UserID: targetID, Email: testNewEmail})

			// then the held change names the staff member, and the admin still sees the failed send
			assert.ErrorIs(t, err, ErrEmailNotSent)
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.NotErrorIs(t, err, ErrNotAudited)
		})
	}
}

func TestSetUserEmail_AFailedSendAndAFailedAuditAreBothReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.users.EXPECT().EmailInUse(mock.Anything, mock.Anything).Return(false, nil)
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(errDatabaseDown)

	// when
	err := svc.SetUserEmail(t.Context(), spec.StaffEmailChange{StaffAction: staffAction(actorID), UserID: targetID, Email: testNewEmail})

	// then
	assert.ErrorIs(t, err, ErrEmailNotSent)
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.ErrorIs(t, err, ErrNotAudited)
	assert.ErrorIs(t, err, errAuditDown)
}

func TestSetUserEmail_AFailedAuditAloneIsReportedAfterTheSends(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, "")
	m.email.EXPECT().Enabled(mock.Anything).Return(true, nil)
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.users.EXPECT().EmailInUse(mock.Anything, mock.Anything).Return(false, nil)
	m.verifications.EXPECT().Issue(mock.Anything, mock.Anything).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL, nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
	m.email.EXPECT().Send(mock.Anything, mock.Anything).Return(nil).Times(2)

	// when
	err := svc.SetUserEmail(t.Context(), spec.StaffEmailChange{StaffAction: staffAction(actorID), UserID: targetID, Email: testNewEmail})

	// then both messages still go out, and the caller learns the change went unrecorded
	assert.ErrorIs(t, err, ErrNotAudited)
	assert.NotErrorIs(t, err, ErrEmailNotSent)
}

func TestVerifyUserEmail_AndUnverifyUserEmail(t *testing.T) {
	tests := []struct {
		name   string
		action audit.Action
		expect func(m *testMocks, userID uuid.UUID)
		act    func(ctx context.Context, svc *service, action spec.StaffUserAction) error
	}{
		{
			name:   "verify",
			action: audit.ActionVerifyUserEmail,
			expect: func(m *testMocks, userID uuid.UUID) {
				m.users.EXPECT().MarkEmailVerified(mock.Anything, userID).Return(nil)
			},
			act: func(ctx context.Context, svc *service, action spec.StaffUserAction) error {
				return svc.VerifyUserEmail(ctx, action)
			},
		},
		{
			name:   "unverify, which also revokes every session and disconnects",
			action: audit.ActionUnverifyUserEmail,
			expect: func(m *testMocks, userID uuid.UUID) {
				m.users.EXPECT().UnverifyAccount(mock.Anything, userID).Return(nil)
				m.disconnector.EXPECT().DisconnectUser(userID).Return(2)
			},
			act: func(ctx context.Context, svc *service, action spec.StaffUserAction) error {
				return svc.UnverifyUserEmail(ctx, action)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			actorID, targetID := uuid.New(), uuid.New()
			m.ranks(actorID, role.Support, targetID, "")
			tc.expect(m, targetID)
			m.audited(expectedUserEntry(actorID, tc.action, targetID, nil))

			// when
			err := tc.act(t.Context(), svc, userAction(actorID, targetID))

			// then
			assert.NoError(t, err)
		})
	}
}

func TestVerifyUserEmail_AnUnknownUserIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().MarkEmailVerified(mock.Anything, targetID).Return(errRowMissing)

	// when
	err := svc.VerifyUserEmail(t.Context(), userAction(actorID, targetID))

	// then
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestUnverifyUserEmail_AFailedWriteDisconnectsNobody(t *testing.T) {
	tests := []struct {
		name    string
		failure error
		want    error
	}{
		{name: "an unknown user", failure: errRowMissing, want: ErrUserNotFound},
		{name: "a failed write", failure: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			actorID, targetID := uuid.New(), uuid.New()
			m.ranks(actorID, role.Support, targetID, "")
			m.users.EXPECT().UnverifyAccount(mock.Anything, targetID).Return(tc.failure)

			// when
			err := svc.UnverifyUserEmail(t.Context(), userAction(actorID, targetID))

			// then nothing is disconnected or audited, which the strict mocks would report
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestUnverifyUserEmail_NeverTouchesTheSuperAdmin(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Admin, targetID, role.SuperAdmin)

	// when
	err := svc.UnverifyUserEmail(t.Context(), userAction(actorID, targetID))

	// then the account is never written, which the strict user repository mock would report
	assert.ErrorIs(t, err, ErrProtectedUser)
}

func TestSetUserDisplayName_ClampsAndRecordsTheChange(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, targetID := uuid.New(), uuid.New()
	m.ranks(actorID, role.Support, targetID, "")
	m.users.EXPECT().GetByID(mock.Anything, targetID).Return(account(targetID), nil)
	m.users.EXPECT().SetDisplayName(mock.Anything, spec.UserDisplayNameUpdate{UserID: targetID, DisplayName: "Renamed Person"}).Return(nil)
	m.audited(expectedUserEntry(actorID, audit.ActionSetDisplayName, targetID, map[string]string{"previous_display_name": testDisplayName, "display_name": "Renamed Person"}))

	// when
	err := svc.SetUserDisplayName(t.Context(), spec.StaffDisplayNameChange{StaffAction: staffAction(actorID), UserID: targetID, DisplayName: "  <b>Renamed</b>   Person "})

	// then
	assert.NoError(t, err)
}

func TestSetUserDisplayName_RefusesABlankName(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	err := svc.SetUserDisplayName(t.Context(), spec.StaffDisplayNameChange{StaffAction: staffAction(uuid.New()), UserID: uuid.New(), DisplayName: "  <i></i> "})

	// then
	assert.ErrorIs(t, err, ErrDisplayNameRequired)
}
