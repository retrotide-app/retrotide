package org

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListForUser_ListsEveryOrganisationTheStoreFindsWithTheViewersRole(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	live := organisation(uuid.New())
	deleted := deletedOrganisation(uuid.New(), testNow.Add(-24*time.Hour))
	m.organisations.EXPECT().ListForUser(mock.Anything, userID).Return([]model.UserOrganisation{
		{Organisation: *live, Role: model.OrgRoleMember},
		{Organisation: *deleted, Role: model.OrgRoleOwner},
	}, nil)

	// when
	got, err := svc.ListForUser(t.Context(), userID)

	// then
	require.NoError(t, err)
	require.Len(t, got.Organisations, 2)
	assert.Equal(t, live.ID, got.Organisations[0].ID)
	assert.Equal(t, model.OrgRoleMember, got.Organisations[0].Role)
	assert.Empty(t, got.Organisations[0].DeletedAt)
	assert.Empty(t, got.Organisations[0].RestorableUntil)
	assert.Equal(t, deleted.ID, got.Organisations[1].ID)
	assert.Equal(t, model.OrgRoleOwner, got.Organisations[1].Role)
	assert.Equal(t, "2026-09-26T12:00:00Z", got.Organisations[1].DeletedAt)
	assert.Equal(t, "2026-10-26T12:00:00Z", got.Organisations[1].RestorableUntil)
}

func TestListForUser_AnswersAnEmptyListNotNull(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.organisations.EXPECT().ListForUser(mock.Anything, userID).Return(nil, nil)

	// when
	got, err := svc.ListForUser(t.Context(), userID)

	// then
	require.NoError(t, err)
	assert.NotNil(t, got.Organisations)
	assert.Empty(t, got.Organisations)
}

func TestListForUser_ReportsAFailedRead(t *testing.T) {
	// given
	svc, m := newTestService(t)
	userID := uuid.New()
	m.organisations.EXPECT().ListForUser(mock.Anything, userID).Return(nil, errDatabaseDown)

	// when
	got, err := svc.ListForUser(t.Context(), userID)

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, got)
}

func TestGet_ShowsTheOrganisationWithTheViewersRole(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	key := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}
	m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(orgID, userID, model.OrgRoleAdmin), nil)
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(organisation(orgID), nil)

	// when
	got, err := svc.Get(t.Context(), key)

	// then
	require.NoError(t, err)
	assert.Equal(t, orgID, got.ID)
	assert.Equal(t, "acme-retros", got.Slug)
	assert.Equal(t, model.OrgRoleAdmin, got.Role)
}

func TestGet_HidesAnOrganisationTheViewerCannotSee(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, key spec.OrganisationMemberKey)
	}{
		{name: "not a member", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(nil, errMembershipMissing)
		}},
		{name: "gone", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleOwner), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, key.OrgID).Return(nil, errOrganisationMissing)
		}},
		{name: "deleted", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleOwner), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, key.OrgID).Return(deletedOrganisation(key.OrgID, testNow), nil)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			key := spec.OrganisationMemberKey{OrgID: uuid.New(), UserID: uuid.New()}
			tc.given(m, key)

			// when
			got, err := svc.Get(t.Context(), key)

			// then
			assert.ErrorIs(t, err, ErrOrganisationNotFound)
			assert.Nil(t, got)
		})
	}
}

func TestGet_ReportsAFailedRead(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, key spec.OrganisationMemberKey)
	}{
		{name: "the membership", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(nil, errDatabaseDown)
		}},
		{name: "the organisation", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleOwner), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, key.OrgID).Return(nil, errDatabaseDown)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			key := spec.OrganisationMemberKey{OrgID: uuid.New(), UserID: uuid.New()}
			tc.given(m, key)

			// when
			got, err := svc.Get(t.Context(), key)

			// then
			assert.ErrorIs(t, err, errDatabaseDown)
			assert.NotErrorIs(t, err, ErrOrganisationNotFound)
			assert.Nil(t, got)
		})
	}
}

func updateRequest(orgID uuid.UUID, actorID uuid.UUID) UpdateRequest {
	return UpdateRequest{ActorID: actorID, IP: testIP, OrgID: orgID, Name: "  Acme Platform  ", Slug: " Acme-Platform ", AllowGuests: false}
}

func (m *testMocks) updatable(orgID uuid.UUID, actorID uuid.UUID) {
	m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: actorID}).Return(membership(orgID, actorID, model.OrgRoleAdmin), nil)
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(organisation(orgID), nil)
}

func TestUpdate_SavesTheChangesAndAuditsWhatChangedWithItsPreviousValue(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	updated := organisation(orgID)
	updated.Name, updated.Slug, updated.AllowGuests = "Acme Platform", "acme-platform", false
	m.updatable(orgID, actorID)
	m.organisations.EXPECT().Update(mock.Anything, spec.OrganisationUpdate{OrgID: orgID, Slug: "acme-platform", Name: "Acme Platform", AllowGuests: false}).Return(updated, nil)
	m.audited(expectedOrgEntry(orgID, actorID, audit.ActionOrgUpdated, map[string]string{
		"name":          "Acme Platform",
		"previous_name": "Acme Retros",
		"slug":          "acme-platform",
		"previous_slug": "acme-retros",
		"allow_guests":  "false",
	}))

	// when
	got, err := svc.Update(t.Context(), updateRequest(orgID, actorID))

	// then
	require.NoError(t, err)
	assert.Equal(t, "acme-platform", got.Slug)
	assert.Equal(t, "Acme Platform", got.Name)
	assert.False(t, got.AllowGuests)
	assert.Equal(t, model.OrgRoleAdmin, got.Role)
}

func TestUpdate_AuditsOnlyTheFieldsThatChanged(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	updated := organisation(orgID)
	updated.Name = "Acme Platform"
	m.updatable(orgID, actorID)
	m.organisations.EXPECT().Update(mock.Anything, spec.OrganisationUpdate{OrgID: orgID, Slug: "acme-retros", Name: "Acme Platform", AllowGuests: true}).Return(updated, nil)
	m.audited(expectedOrgEntry(orgID, actorID, audit.ActionOrgUpdated, map[string]string{"name": "Acme Platform", "previous_name": "Acme Retros"}))

	// when
	_, err := svc.Update(t.Context(), UpdateRequest{ActorID: actorID, IP: testIP, OrgID: orgID, Name: "Acme Platform", Slug: "acme-retros", AllowGuests: true})

	// then
	assert.NoError(t, err)
}

func TestUpdate_RefusesAnInvalidNameOrAddressBeforeReadingAnything(t *testing.T) {
	tests := []struct {
		name    string
		orgName string
		slug    string
		want    error
	}{
		{name: "no name", orgName: " ", slug: "acme-retros", want: ErrNameRequired},
		{name: "a name over 80 characters", orgName: strings.Repeat("n", 81), slug: "acme-retros", want: ErrNameTooLong},
		{name: "a malformed address", orgName: "Acme", slug: "a", want: ErrInvalidSlug},
		{name: "a reserved address", orgName: "Acme", slug: "boards", want: ErrSlugReserved},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.Update(t.Context(), UpdateRequest{ActorID: uuid.New(), IP: testIP, OrgID: uuid.New(), Name: tc.orgName, Slug: tc.slug})

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestUpdate_RefusesAnAddressAnotherOrganisationHolds(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.updatable(orgID, actorID)
	m.organisations.EXPECT().Update(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("update: %w", dao.ErrConflict))

	// when
	got, err := svc.Update(t.Context(), updateRequest(orgID, actorID))

	// then
	assert.ErrorIs(t, err, ErrSlugTaken)
	assert.Nil(t, got)
}

func TestUpdate_ADeletedOrganisationIsNotFound(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID)
	}{
		{name: "deleted before the read", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: actorID}).Return(membership(orgID, actorID, model.OrgRoleOwner), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(deletedOrganisation(orgID, testNow), nil)
		}},
		{name: "deleted before the write", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.updatable(orgID, actorID)
			m.organisations.EXPECT().Update(mock.Anything, mock.Anything).Return(nil, errOrganisationMissing)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			tc.given(m, orgID, actorID)

			// when
			got, err := svc.Update(t.Context(), updateRequest(orgID, actorID))

			// then
			assert.ErrorIs(t, err, ErrOrganisationNotFound)
			assert.Nil(t, got)
		})
	}
}

func TestUpdate_ReportsAFailure(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID)
		want  error
	}{
		{name: "an actor outside the organisation", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: actorID}).Return(nil, errMembershipMissing)
		}, want: ErrOrganisationNotFound},
		{name: "reading the membership", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: actorID}).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "reading the organisation", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: actorID}).Return(membership(orgID, actorID, model.OrgRoleAdmin), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "the write", given: func(m *testMocks, orgID uuid.UUID, actorID uuid.UUID) {
			m.updatable(orgID, actorID)
			m.organisations.EXPECT().Update(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, actorID := uuid.New(), uuid.New()
			tc.given(m, orgID, actorID)

			// when
			got, err := svc.Update(t.Context(), updateRequest(orgID, actorID))

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestUpdate_AnswersWithTheChangeWhenItsAuditEntryFails(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.updatable(orgID, actorID)
	m.organisations.EXPECT().Update(mock.Anything, mock.Anything).Return(organisation(orgID), nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	got, err := svc.Update(t.Context(), updateRequest(orgID, actorID))

	// then
	assert.ErrorIs(t, err, ErrNotAudited)
	require.NotNil(t, got)
	assert.Equal(t, orgID, got.ID)
}

func TestDelete_SoftDeletesAndAudits(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	m.organisations.EXPECT().SoftDelete(mock.Anything, orgID).Return(deletedOrganisation(orgID, testNow), nil)
	m.audited(expectedOrgEntry(orgID, actorID, audit.ActionOrgDeleted, map[string]string{"name": "Acme Retros", "slug": "acme-retros"}))

	// when
	err := svc.Delete(t.Context(), command(orgID, actorID))

	// then
	assert.NoError(t, err)
}

func TestDelete_AnUnknownOrAlreadyDeletedOrganisationIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.organisations.EXPECT().SoftDelete(mock.Anything, orgID).Return(nil, errOrganisationMissing)

	// when
	err := svc.Delete(t.Context(), command(orgID, uuid.New()))

	// then
	assert.ErrorIs(t, err, ErrOrganisationNotFound)
}

func TestDelete_ReportsAFailedWriteOrAudit(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, orgID uuid.UUID)
		want  error
	}{
		{name: "the write", given: func(m *testMocks, orgID uuid.UUID) {
			m.organisations.EXPECT().SoftDelete(mock.Anything, orgID).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "the audit entry", given: func(m *testMocks, orgID uuid.UUID) {
			m.organisations.EXPECT().SoftDelete(mock.Anything, orgID).Return(deletedOrganisation(orgID, testNow), nil)
			m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)
		}, want: ErrNotAudited},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID := uuid.New()
			tc.given(m, orgID)

			// when
			err := svc.Delete(t.Context(), command(orgID, uuid.New()))

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func (m *testMocks) restorable(orgID uuid.UUID, ownerID uuid.UUID, deletedAt time.Time) {
	m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: ownerID}).Return(membership(orgID, ownerID, model.OrgRoleOwner), nil)
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(deletedOrganisation(orgID, deletedAt), nil)
}

func TestRestore_TheOwnerRestoresWithinThirtyDays(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID := uuid.New(), uuid.New()
	m.restorable(orgID, ownerID, testNow.Add(-29*24*time.Hour))
	m.ownsNone(ownerID)
	m.organisations.EXPECT().Restore(mock.Anything, spec.OrganisationRestore{OrgID: orgID, OwnerID: ownerID, DeletedAfter: testNow.Add(-model.OrganisationRestoreWindow), MaxOwned: 1}).Return(organisation(orgID), nil)
	m.audited(expectedOrgEntry(orgID, ownerID, audit.ActionOrgRestored, map[string]string{"name": "Acme Retros", "slug": "acme-retros"}))

	// when
	got, err := svc.Restore(t.Context(), command(orgID, ownerID))

	// then
	require.NoError(t, err)
	assert.Equal(t, orgID, got.ID)
	assert.Equal(t, model.OrgRoleOwner, got.Role)
	assert.Empty(t, got.DeletedAt)
	assert.Empty(t, got.RestorableUntil)
}

func TestRestore_OnlyTheOwnerMayRestore(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, key spec.OrganisationMemberKey)
		want  error
	}{
		{name: "not a member", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(nil, errMembershipMissing)
		}, want: ErrOrganisationNotFound},
		{name: "an admin", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleAdmin), nil)
		}, want: ErrNotOwner},
		{name: "a member", given: func(m *testMocks, key spec.OrganisationMemberKey) {
			m.organisations.EXPECT().GetMember(mock.Anything, key).Return(membership(key.OrgID, key.UserID, model.OrgRoleMember), nil)
		}, want: ErrNotOwner},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			key := spec.OrganisationMemberKey{OrgID: uuid.New(), UserID: uuid.New()}
			tc.given(m, key)

			// when
			got, err := svc.Restore(t.Context(), command(key.OrgID, key.UserID))

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestRestore_RefusesAnOrganisationThatCannotBeRestored(t *testing.T) {
	tests := []struct {
		name      string
		deletedAt *time.Time
		want      error
	}{
		{name: "not deleted", deletedAt: nil, want: ErrOrganisationNotDeleted},
		{name: "deleted exactly thirty days ago", deletedAt: new(testNow.Add(-model.OrganisationRestoreWindow)), want: ErrNotRestorable},
		{name: "deleted thirty-one days ago", deletedAt: new(testNow.Add(-31 * 24 * time.Hour)), want: ErrNotRestorable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, ownerID := uuid.New(), uuid.New()
			found := organisation(orgID)
			found.DeletedAt = tc.deletedAt
			m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: ownerID}).Return(membership(orgID, ownerID, model.OrgRoleOwner), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(found, nil)

			// when
			got, err := svc.Restore(t.Context(), command(orgID, ownerID))

			// then the strict mocks prove nothing was written
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestRestore_RefusesAnOwnerWhoAlreadyOwnsTheirAllowance(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID := uuid.New(), uuid.New()
	m.restorable(orgID, ownerID, testNow.Add(-time.Hour))
	m.settings.EXPECT().GetInt(mock.Anything, testMaxOwned).Return(1, nil)
	m.organisations.EXPECT().CountOwnedByUser(mock.Anything, ownerID).Return(1, nil)

	// when
	got, err := svc.Restore(t.Context(), command(orgID, ownerID))

	// then
	assert.ErrorIs(t, err, ErrOwnedLimitReached)
	assert.Nil(t, got)
}

func TestRestore_AnAllowanceTakenMeanwhileIsRefusedUnderTheLock(t *testing.T) {
	// given the count passed, but another workspace or restore took the last place before the locked restore
	svc, m := newTestService(t)
	orgID, ownerID := uuid.New(), uuid.New()
	m.restorable(orgID, ownerID, testNow.Add(-time.Hour))
	m.ownsNone(ownerID)
	m.organisations.EXPECT().Restore(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("restore organisation: %w", dao.ErrQuotaExceeded))

	// when
	got, err := svc.Restore(t.Context(), command(orgID, ownerID))

	// then
	assert.ErrorIs(t, err, ErrOwnedLimitReached)
	assert.NotErrorIs(t, err, ErrNotRestorable)
	assert.Nil(t, got)
}

func TestRestore_AnOrganisationRestoredOrPurgedMeanwhileCannotBeRestored(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID := uuid.New(), uuid.New()
	m.restorable(orgID, ownerID, testNow.Add(-time.Hour))
	m.ownsNone(ownerID)
	m.organisations.EXPECT().Restore(mock.Anything, mock.Anything).Return(nil, errOrganisationMissing)

	// when
	got, err := svc.Restore(t.Context(), command(orgID, ownerID))

	// then
	assert.ErrorIs(t, err, ErrNotRestorable)
	assert.Nil(t, got)
}

func TestRestore_ReportsAFailure(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks, orgID uuid.UUID, ownerID uuid.UUID)
		want  error
	}{
		{name: "reading the organisation", given: func(m *testMocks, orgID uuid.UUID, ownerID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: ownerID}).Return(membership(orgID, ownerID, model.OrgRoleOwner), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
		{name: "an organisation purged before the read", given: func(m *testMocks, orgID uuid.UUID, ownerID uuid.UUID) {
			m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: ownerID}).Return(membership(orgID, ownerID, model.OrgRoleOwner), nil)
			m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errOrganisationMissing)
		}, want: ErrOrganisationNotFound},
		{name: "the write", given: func(m *testMocks, orgID uuid.UUID, ownerID uuid.UUID) {
			m.restorable(orgID, ownerID, testNow.Add(-time.Hour))
			m.ownsNone(ownerID)
			m.organisations.EXPECT().Restore(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)
		}, want: errDatabaseDown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, ownerID := uuid.New(), uuid.New()
			tc.given(m, orgID, ownerID)

			// when
			got, err := svc.Restore(t.Context(), command(orgID, ownerID))

			// then
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestRestore_AnswersWithTheOrganisationWhenItsAuditEntryFails(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, ownerID := uuid.New(), uuid.New()
	m.restorable(orgID, ownerID, testNow.Add(-time.Hour))
	m.ownsNone(ownerID)
	m.organisations.EXPECT().Restore(mock.Anything, mock.Anything).Return(organisation(orgID), nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditDown)

	// when
	got, err := svc.Restore(t.Context(), command(orgID, ownerID))

	// then
	assert.ErrorIs(t, err, ErrNotAudited)
	require.NotNil(t, got)
	assert.Equal(t, orgID, got.ID)
}
