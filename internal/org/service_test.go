package org

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	testMocks struct {
		organisations *repository.MockOrganisationRepository
		users         *repository.MockUserRepository
		auditLog      *repository.MockAuditLogRepository
		hierarchy     *hierarchy.MockService
		settings      *settings.MockService
	}
)

const (
	testIP         = "203.0.113.9"
	testSlugSuffix = "x7k2qd"
)

var (
	testMaxOwned = &config.SiteSettingDef{Key: "max_free_orgs_per_owner", Default: "1", Type: config.TypeInt}
	testNow      = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		organisations: repository.NewMockOrganisationRepository(t),
		users:         repository.NewMockUserRepository(t),
		auditLog:      repository.NewMockAuditLogRepository(t),
		hierarchy:     hierarchy.NewMockService(t),
		settings:      settings.NewMockService(t),
	}

	svc := NewService(Deps{
		Organisations: m.organisations,
		Users:         m.users,
		AuditLog:      m.auditLog,
		Hierarchy:     m.hierarchy,
		Settings:      m.settings,
		MaxOwned:      testMaxOwned,
	}).(*service)

	svc.now = func() time.Time {
		return testNow
	}
	svc.slugSuffix = func() string {
		return testSlugSuffix
	}

	return svc, m
}

func (m *testMocks) audited(entries ...audit.NewEntry) {
	for _, entry := range entries {
		m.auditLog.EXPECT().Create(mock.Anything, entry).Return(nil).Once()
	}
}

func (m *testMocks) live(orgID uuid.UUID) {
	found := organisation(orgID)

	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(found, nil)
}

func command(orgID uuid.UUID, actorID uuid.UUID) Command {
	return Command{ActorID: actorID, IP: testIP, OrgID: orgID}
}

func memberCommand(orgID uuid.UUID, actorID uuid.UUID, userID uuid.UUID) MemberCommand {
	return MemberCommand{ActorID: actorID, IP: testIP, OrgID: orgID, UserID: userID}
}

func organisation(orgID uuid.UUID) *model.Organisation {
	return &model.Organisation{
		ID:          orgID,
		Slug:        "acme-retros",
		Name:        "Acme Retros",
		AllowGuests: true,
		CreatedAt:   time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
}

func deletedOrganisation(orgID uuid.UUID, deletedAt time.Time) *model.Organisation {
	found := organisation(orgID)
	found.DeletedAt = &deletedAt

	return found
}

func membership(orgID uuid.UUID, userID uuid.UUID, r model.OrgRole) *model.OrganisationMember {
	return &model.OrganisationMember{OrgID: orgID, UserID: userID, Role: r, JoinedAt: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)}
}

func expectedOrgEntry(orgID uuid.UUID, actorID uuid.UUID, action audit.Action, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      orgID,
		ActorID:    actorID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetOrganisation,
		TargetID:   orgID.String(),
		Details:    details,
		IP:         testIP,
	}
}

func expectedMemberEntry(orgID uuid.UUID, actorID uuid.UUID, action audit.Action, userID uuid.UUID, details map[string]string) audit.NewEntry {
	entry := expectedOrgEntry(orgID, actorID, action, details)
	entry.SubjectID = userID

	return entry
}

func TestRecord_JoinsEveryFailedEntryUnderNotAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, actorID := uuid.New(), uuid.New()
	first := expectedOrgEntry(orgID, actorID, audit.ActionOrgCreated, nil)
	second := expectedOrgEntry(orgID, actorID, audit.ActionTeamCreated, nil)
	third := expectedOrgEntry(orgID, actorID, audit.ActionOrgUpdated, nil)
	m.auditLog.EXPECT().Create(mock.Anything, first).Return(errAuditDown)
	m.auditLog.EXPECT().Create(mock.Anything, second).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, third).Return(errDatabaseDown)

	// when
	err := svc.record(t.Context(), first, second, third)

	// then every entry was tried, and both failures are kept
	assert.ErrorIs(t, err, ErrNotAudited)
	assert.ErrorIs(t, err, errAuditDown)
	assert.ErrorIs(t, err, errDatabaseDown)
}

func TestSettle_AStaleCacheOutranksTheAuditOutcome(t *testing.T) {
	tests := []struct {
		name      string
		writeErr  error
		auditErr  error
		wantStale bool
	}{
		{name: "both succeeded", writeErr: nil, auditErr: nil, wantStale: false},
		{name: "only the audit failed", writeErr: nil, auditErr: fmt.Errorf("%w: %w", ErrNotAudited, errAuditDown), wantStale: false},
		{name: "only the cache failed", writeErr: errCacheDown, auditErr: nil, wantStale: true},
		{name: "both failed", writeErr: errCacheDown, auditErr: fmt.Errorf("%w: %w", ErrNotAudited, errAuditDown), wantStale: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := settle(tc.writeErr, tc.auditErr)

			// then
			if tc.writeErr == nil && tc.auditErr == nil {
				assert.NoError(t, err)
				return
			}

			assert.Equal(t, tc.wantStale, errors.Is(err, ErrCacheStale))
			assert.Equal(t, tc.auditErr != nil, errors.Is(err, ErrNotAudited))
			assert.Equal(t, tc.writeErr != nil, errors.Is(err, repository.ErrStaleTenantCache))
		})
	}
}
