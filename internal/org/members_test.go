package org

import (
	"fmt"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	owner       = hierarchy.Holder{Org: model.OrgRoleOwner}
	orgAdmin    = hierarchy.Holder{Org: model.OrgRoleAdmin}
	orgMember   = hierarchy.Holder{Org: model.OrgRoleMember}
	staffAdmin  = hierarchy.Holder{Site: role.Support, Org: model.OrgRoleAdmin}
	staffMember = hierarchy.Holder{Site: role.Support, Org: model.OrgRoleMember}
)

func (m *testMocks) acts(action MemberCommand, actor hierarchy.Holder, target hierarchy.Holder) {
	standing := hierarchy.Standing{Actor: actor, Target: target}

	m.live(action.OrgID)
	m.hierarchy.EXPECT().ActOnMember(mock.Anything, spec.MemberActionCheck{OrgID: action.OrgID, ActorID: action.ActorID, TargetID: action.UserID}).Return(&standing, nil)
}

func roleChange(r model.OrgRole) RoleChange {
	return RoleChange{ActorID: uuid.New(), IP: testIP, OrgID: uuid.New(), UserID: uuid.New(), Role: r}
}

func TestMembers_ADeletedOrganisationTakesNoMemberWork(t *testing.T) {
	operations := []struct {
		name string
		run  func(t *testing.T, svc *service, orgID uuid.UUID) error
	}{
		{name: "listing", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			_, err := svc.ListMembers(t.Context(), orgID)
			return err
		}},
		{name: "changing a role", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			change := roleChange(model.OrgRoleAdmin)
			change.OrgID = orgID

			return svc.ChangeMemberRole(t.Context(), change)
		}},
		{name: "removing", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			return svc.RemoveMember(t.Context(), memberCommand(orgID, uuid.New(), uuid.New()))
		}},
		{name: "leaving", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			return svc.Leave(t.Context(), command(orgID, uuid.New()))
		}},
		{name: "transferring ownership", run: func(t *testing.T, svc *service, orgID uuid.UUID) error {
			return svc.TransferOwnership(t.Context(), memberCommand(orgID, uuid.New(), uuid.New()))
		}},
	}

	states := []struct {
		name  string
		found *model.Organisation
		err   error
		want  error
	}{
		{name: "soft-deleted", found: deletedOrganisation(uuid.New(), testNow), want: ErrOrganisationNotFound},
		{name: "purged", err: errOrganisationMissing, want: ErrOrganisationNotFound},
		{name: "unreadable", err: errDatabaseDown, want: errDatabaseDown},
	}

	for _, operation := range operations {
		for _, state := range states {
			t.Run(operation.name+" in an organisation that is "+state.name, func(t *testing.T) {
				// given
				svc, m := newTestService(t)
				orgID := uuid.New()
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(state.found, state.err)

				// when
				err := operation.run(t, svc, orgID)

				// then the strict mocks prove the hierarchy was never asked and nothing was written
				assert.ErrorIs(t, err, state.want)
			})
		}
	}
}

func TestListMembers_ListsEveryMemberWithTheirProfile(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	joined := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	m.live(orgID)
	m.organisations.EXPECT().ListMembers(mock.Anything, orgID).Return([]model.OrganisationMemberProfile{
		{OrgID: orgID, UserID: userID, Role: model.OrgRoleAdmin, JoinedAt: joined, Email: "alice@example.com", DisplayName: "Alice", SiteRole: role.Support},
		{OrgID: orgID, UserID: uuid.New(), Role: model.OrgRoleMember, JoinedAt: joined, Email: "bob@example.com", DisplayName: "Bob"},
	}, nil)

	// when
	got, err := svc.ListMembers(t.Context(), orgID)

	// then the site staff role is shown only for staff, so the page can apply the hierarchy
	require.NoError(t, err)
	require.Len(t, got.Members, 2)
	assert.Equal(t, userID, got.Members[0].UserID)
	assert.Equal(t, "alice@example.com", got.Members[0].Email)
	assert.Equal(t, "Alice", got.Members[0].DisplayName)
	assert.Equal(t, model.OrgRoleAdmin, got.Members[0].Role)
	assert.Equal(t, role.Support, got.Members[0].SiteRole)
	assert.Equal(t, "2026-09-02T12:00:00Z", got.Members[0].JoinedAt)
	assert.Empty(t, got.Members[1].SiteRole)
}

func TestListMembers_ReportsAFailedRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.live(orgID)
	m.organisations.EXPECT().ListMembers(mock.Anything, orgID).Return(nil, errDatabaseDown)

	// when
	got, err := svc.ListMembers(t.Context(), orgID)

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, got)
}

func TestChangeMemberRole_TheOwnerMakesAndUnmakesAdmins(t *testing.T) {
	tests := []struct {
		name   string
		target hierarchy.Holder
		role   model.OrgRole
	}{
		{name: "a member becomes an admin", target: orgMember, role: model.OrgRoleAdmin},
		{name: "an admin becomes a member", target: orgAdmin, role: model.OrgRoleMember},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			change := roleChange(tc.role)
			m.acts(change.MemberCommand, owner, tc.target)
			m.organisations.EXPECT().SetMemberRole(mock.Anything, spec.OrganisationMemberRoleChange{OrgID: change.OrgID, UserID: change.UserID, Role: tc.role}).Return(membership(change.OrgID, change.UserID, tc.role), nil)
			m.audited(expectedMemberEntry(change.OrgID, change.ActorID, audit.ActionMemberRoleChanged, change.UserID, map[string]string{"role": string(tc.role), "previous_role": string(tc.target.Org)}))

			// when
			err := svc.ChangeMemberRole(t.Context(), change)

			// then
			assert.NoError(t, err)
		})
	}
}

func TestChangeMemberRole_RefusesARoleOtherThanAdminOrMember(t *testing.T) {
	for _, r := range []model.OrgRole{model.OrgRoleOwner, "", "lead"} {
		t.Run(string(r), func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			err := svc.ChangeMemberRole(t.Context(), roleChange(r))

			// then ownership only moves through a transfer, and nothing was asked
			assert.ErrorIs(t, err, ErrInvalidRole)
		})
	}
}

func TestChangeMemberRole_HandsOnTheHierarchysAnswer(t *testing.T) {
	tests := []struct {
		name    string
		refusal error
		want    error
	}{
		{name: "a target outside the organisation", refusal: errNotMember, want: ErrMemberNotFound},
		{name: "a target at the actor's level or above", refusal: errOutranked, want: hierarchy.ErrOutranked},
		{name: "a failed read", refusal: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			change := roleChange(model.OrgRoleMember)
			m.live(change.OrgID)
			m.hierarchy.EXPECT().ActOnMember(mock.Anything, spec.MemberActionCheck{OrgID: change.OrgID, ActorID: change.ActorID, TargetID: change.UserID}).Return(nil, tc.refusal)

			// when
			err := svc.ChangeMemberRole(t.Context(), change)

			// then the strict mocks prove nothing was written
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestChangeMemberRole_NobodyChangesTheOwnersRole(t *testing.T) {
	// given site staff can outrank the owner, so the owner is protected on its own
	svc, m := newTestService(t)
	change := roleChange(model.OrgRoleAdmin)
	m.acts(change.MemberCommand, hierarchy.Holder{Site: role.Admin, Org: model.OrgRoleAdmin}, owner)

	// when
	err := svc.ChangeMemberRole(t.Context(), change)

	// then
	assert.ErrorIs(t, err, ErrOwnerProtected)
}

func TestChangeMemberRole_OnlyTheOwnerMakesOrUnmakesAnAdmin(t *testing.T) {
	tests := []struct {
		name   string
		target hierarchy.Holder
		role   model.OrgRole
	}{
		{name: "promoting a member", target: orgMember, role: model.OrgRoleAdmin},
		{name: "demoting an admin", target: orgAdmin, role: model.OrgRoleMember},
		{name: "keeping an admin an admin", target: orgAdmin, role: model.OrgRoleAdmin},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given an admin who is site staff, whom the ladder alone would let through
			svc, m := newTestService(t)
			change := roleChange(tc.role)
			m.acts(change.MemberCommand, staffAdmin, tc.target)

			// when
			err := svc.ChangeMemberRole(t.Context(), change)

			// then
			assert.ErrorIs(t, err, ErrAdminsNeedOwner)
		})
	}
}

func TestChangeMemberRole_AnAdminMayLeaveAMemberAMember(t *testing.T) {
	// given
	svc, m := newTestService(t)
	change := roleChange(model.OrgRoleMember)
	m.acts(change.MemberCommand, orgAdmin, orgMember)
	m.organisations.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(membership(change.OrgID, change.UserID, model.OrgRoleMember), nil)
	m.audited(expectedMemberEntry(change.OrgID, change.ActorID, audit.ActionMemberRoleChanged, change.UserID, map[string]string{"role": "member", "previous_role": "member"}))

	// when
	err := svc.ChangeMemberRole(t.Context(), change)

	// then
	assert.NoError(t, err)
}

func TestChangeMemberRole_RefusesAChangeThatWouldNotLeaveTheTargetBelowTheActor(t *testing.T) {
	// given a standing the ladder does not permit, so no change can be allowed either
	svc, m := newTestService(t)
	change := roleChange(model.OrgRoleAdmin)
	m.acts(change.MemberCommand, owner, staffMember)

	// when
	err := svc.ChangeMemberRole(t.Context(), change)

	// then
	assert.ErrorIs(t, err, hierarchy.ErrOutranked)
}

func TestChangeMemberRole_AMemberRemovedMeanwhileIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	change := roleChange(model.OrgRoleAdmin)
	m.acts(change.MemberCommand, owner, orgMember)
	m.organisations.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(nil, errMembershipMissing)

	// when
	err := svc.ChangeMemberRole(t.Context(), change)

	// then
	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestChangeMemberRole_AuditsAChangeWhoseCacheWasNotCleared(t *testing.T) {
	// given
	svc, m := newTestService(t)
	change := roleChange(model.OrgRoleAdmin)
	m.acts(change.MemberCommand, owner, orgMember)
	m.organisations.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(membership(change.OrgID, change.UserID, model.OrgRoleAdmin), errCacheDown)
	m.audited(expectedMemberEntry(change.OrgID, change.ActorID, audit.ActionMemberRoleChanged, change.UserID, map[string]string{"role": "admin", "previous_role": "member"}))

	// when
	err := svc.ChangeMemberRole(t.Context(), change)

	// then
	assert.ErrorIs(t, err, ErrCacheStale)
	assert.NotErrorIs(t, err, ErrNotAudited)
}

func TestChangeMemberRole_ReportsAFailedWriteOrAudit(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, change RoleChange)
		want  error
	}{
		{name: "the write", given: func(m *testMocks, change RoleChange) {
			m.organisations.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "the audit entry", given: func(m *testMocks, change RoleChange) {
			m.organisations.EXPECT().SetMemberRole(mock.Anything, mock.Anything).Return(membership(change.OrgID, change.UserID, model.OrgRoleAdmin), nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
		}, want: ErrNotAudited},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			change := roleChange(model.OrgRoleAdmin)
			m.acts(change.MemberCommand, owner, orgMember)
			tc.given(m, change)

			// when
			err := svc.ChangeMemberRole(t.Context(), change)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRemoveMember_RemovesAndAuditsTheirRole(t *testing.T) {
	tests := []struct {
		name   string
		actor  hierarchy.Holder
		target hierarchy.Holder
	}{
		{name: "an admin removes a member", actor: orgAdmin, target: orgMember},
		{name: "the owner removes an admin", actor: owner, target: orgAdmin},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			action := memberCommand(uuid.New(), uuid.New(), uuid.New())
			m.acts(action, tc.actor, tc.target)
			m.organisations.EXPECT().RemoveMember(mock.Anything, spec.OrganisationMemberKey{OrgID: action.OrgID, UserID: action.UserID}).Return(membership(action.OrgID, action.UserID, tc.target.Org), nil)
			m.audited(expectedMemberEntry(action.OrgID, action.ActorID, audit.ActionMemberRemoved, action.UserID, map[string]string{"role": string(tc.target.Org)}))

			// when
			err := svc.RemoveMember(t.Context(), action)

			// then
			assert.NoError(t, err)
		})
	}
}

func TestRemoveMember_RefusesWhatTheActorMayNotDo(t *testing.T) {
	tests := []struct {
		name   string
		actor  hierarchy.Holder
		target hierarchy.Holder
		want   error
	}{
		{name: "removing the owner", actor: hierarchy.Holder{Site: role.Admin, Org: model.OrgRoleMember}, target: owner, want: ErrOwnerProtected},
		{name: "an admin who is site staff removing an admin", actor: staffAdmin, target: orgAdmin, want: ErrAdminsNeedOwner},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			action := memberCommand(uuid.New(), uuid.New(), uuid.New())
			m.acts(action, tc.actor, tc.target)

			// when
			err := svc.RemoveMember(t.Context(), action)

			// then the strict mocks prove nothing was removed
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRemoveMember_HandsOnTheHierarchysAnswer(t *testing.T) {
	tests := []struct {
		name    string
		refusal error
		want    error
	}{
		{name: "a target outside the organisation", refusal: errNotMember, want: ErrMemberNotFound},
		{name: "a target at the actor's level or above", refusal: errOutranked, want: hierarchy.ErrOutranked},
		{name: "a failed read", refusal: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			action := memberCommand(uuid.New(), uuid.New(), uuid.New())
			m.live(action.OrgID)
			m.hierarchy.EXPECT().ActOnMember(mock.Anything, mock.Anything).Return(nil, tc.refusal)

			// when
			err := svc.RemoveMember(t.Context(), action)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRemoveMember_ReportsWhatHappenedToTheWrite(t *testing.T) {
	tests := []struct {
		name     string
		removed  *model.OrganisationMember
		writeErr error
		audited  bool
		want     error
	}{
		{name: "removed meanwhile", writeErr: errMembershipMissing, want: ErrMemberNotFound},
		{name: "a failed write", writeErr: errDatabaseDown, want: errDatabaseDown},
		{name: "a cache that was not cleared", removed: &model.OrganisationMember{Role: model.OrgRoleMember}, writeErr: errCacheDown, audited: true, want: ErrCacheStale},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			action := memberCommand(uuid.New(), uuid.New(), uuid.New())
			m.acts(action, orgAdmin, orgMember)
			m.organisations.EXPECT().RemoveMember(mock.Anything, mock.Anything).Return(tc.removed, tc.writeErr)
			if tc.audited {
				m.audited(expectedMemberEntry(action.OrgID, action.ActorID, audit.ActionMemberRemoved, action.UserID, map[string]string{"role": "member"}))
			}

			// when
			err := svc.RemoveMember(t.Context(), action)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRemoveMember_AFailedAuditEntryIsReportedAfterTheRemoval(t *testing.T) {
	// given
	svc, m := newTestService(t)
	action := memberCommand(uuid.New(), uuid.New(), uuid.New())
	m.acts(action, orgAdmin, orgMember)
	m.organisations.EXPECT().RemoveMember(mock.Anything, mock.Anything).Return(membership(action.OrgID, action.UserID, model.OrgRoleMember), nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	err := svc.RemoveMember(t.Context(), action)

	// then
	assert.ErrorIs(t, err, ErrNotAudited)
}

func TestLeave_AMemberOrAdminLeaves(t *testing.T) {
	for _, r := range []model.OrgRole{model.OrgRoleMember, model.OrgRoleAdmin} {
		t.Run(string(r), func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, userID := uuid.New(), uuid.New()
			key := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}
			m.live(orgID)
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(orgID, userID, r), nil)
			m.organisations.EXPECT().RemoveMember(mock.Anything, key).Return(membership(orgID, userID, r), nil)
			m.audited(expectedMemberEntry(orgID, userID, audit.ActionMemberRemoved, userID, map[string]string{"role": string(r), "left": "true"}))

			// when
			err := svc.Leave(t.Context(), command(orgID, userID))

			// then
			assert.NoError(t, err)
		})
	}
}

func TestLeave_TheOwnerCannotLeave(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID := uuid.New(), uuid.New()
	m.live(orgID)
	m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: ownerID}).Return(membership(orgID, ownerID, model.OrgRoleOwner), nil)

	// when
	err := svc.Leave(t.Context(), command(orgID, ownerID))

	// then the strict mocks prove nothing was removed
	assert.ErrorIs(t, err, ErrOwnerCannotLeave)
}

func TestLeave_ReportsWhatHappened(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, key spec.OrganisationMemberKey)
		want  error
	}{
		{name: "not a member", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(nil, errMembershipMissing)
		}, want: ErrOrganisationNotFound},
		{name: "a failed membership read", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "removed meanwhile", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleMember), nil)
			m.organisations.EXPECT().RemoveMember(mock.Anything, key).Return(nil, errMembershipMissing)
		}, want: ErrMemberNotFound},
		{name: "a failed write", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleMember), nil)
			m.organisations.EXPECT().RemoveMember(mock.Anything, key).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "a cache that was not cleared", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleMember), nil)
			m.organisations.EXPECT().RemoveMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleMember), errCacheDown)
			m.audited(expectedMemberEntry(key.OrgID, key.UserID, audit.ActionMemberRemoved, key.UserID, map[string]string{"role": "member", "left": "true"}))
		}, want: ErrCacheStale},
		{name: "a failed audit entry", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleMember), nil)
			m.organisations.EXPECT().RemoveMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleMember), nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
		}, want: ErrNotAudited},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			key := spec.OrganisationMemberKey{OrgID: uuid.New(), UserID: uuid.New()}
			m.live(key.OrgID)
			tc.given(m, key)

			// when
			err := svc.Leave(t.Context(), command(key.OrgID, key.UserID))

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestTransferOwnership_TheOwnerHandsOverAndIsAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	action := memberCommand(uuid.New(), uuid.New(), uuid.New())
	m.acts(action, owner, orgAdmin)
	m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(3, nil)
	m.organisations.EXPECT().TransferOwnership(mock.Anything, spec.OwnershipTransfer{OrgID: action.OrgID, FromUserID: action.ActorID, ToUserID: action.UserID, MaxOwned: 3}).Return(nil)
	m.audited(expectedMemberEntry(action.OrgID, action.ActorID, audit.ActionOrgOwnershipTransferred, action.UserID, nil))

	// when
	err := svc.TransferOwnership(t.Context(), action)

	// then the new owner's allowance is handed to the write, which checks it under their row lock
	assert.NoError(t, err)
}

func TestTransferOwnership_AnAllowanceThatCannotBeReadTransfersNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	action := memberCommand(uuid.New(), uuid.New(), uuid.New())
	m.acts(action, owner, orgMember)
	m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(0, errDatabaseDown)

	// when
	err := svc.TransferOwnership(t.Context(), action)

	// then the strict mocks prove nothing was transferred
	assert.ErrorIs(t, err, errDatabaseDown)
}

func TestTransferOwnership_HandsOnTheHierarchysAnswer(t *testing.T) {
	tests := []struct {
		name    string
		refusal error
		want    error
	}{
		{name: "a target outside the organisation", refusal: errNotMember, want: ErrMemberNotFound},
		{name: "a target at the owner's level or above, the owner included", refusal: errOutranked, want: hierarchy.ErrOutranked},
		{name: "a failed read", refusal: errDatabaseDown, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			action := memberCommand(uuid.New(), uuid.New(), uuid.New())
			m.live(action.OrgID)
			m.hierarchy.EXPECT().ActOnMember(mock.Anything, mock.Anything).Return(nil, tc.refusal)

			// when
			err := svc.TransferOwnership(t.Context(), action)

			// then the strict mocks prove nothing was transferred
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestTransferOwnership_ReportsWhatHappenedToTheWrite(t *testing.T) {
	tests := []struct {
		name     string
		writeErr error
		audited  bool
		want     error
	}{
		{name: "the organisation, its owner or the new owner changed meanwhile", writeErr: fmt.Errorf("transfer: %w", dao.ErrNotFound), want: ErrOwnershipNotTransferred},
		{name: "the new owner already owns their allowance", writeErr: fmt.Errorf("transfer: %w", dao.ErrQuotaExceeded), want: ErrNewOwnerLimitReached},
		{name: "a failed write", writeErr: errDatabaseDown, want: errDatabaseDown},
		{name: "a cache that was not cleared", writeErr: errCacheDown, audited: true, want: ErrCacheStale},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			action := memberCommand(uuid.New(), uuid.New(), uuid.New())
			m.acts(action, owner, orgMember)
			m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(1, nil)
			m.organisations.EXPECT().TransferOwnership(mock.Anything, mock.Anything).Return(tc.writeErr)
			if tc.audited {
				m.audited(expectedMemberEntry(action.OrgID, action.ActorID, audit.ActionOrgOwnershipTransferred, action.UserID, nil))
			}

			// when
			err := svc.TransferOwnership(t.Context(), action)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestTransferOwnership_AFailedAuditEntryIsReportedAfterTheTransfer(t *testing.T) {
	// given
	svc, m := newTestService(t)
	action := memberCommand(uuid.New(), uuid.New(), uuid.New())
	m.acts(action, owner, orgMember)
	m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(1, nil)
	m.organisations.EXPECT().TransferOwnership(mock.Anything, mock.Anything).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	err := svc.TransferOwnership(t.Context(), action)

	// then
	assert.ErrorIs(t, err, ErrNotAudited)
}
