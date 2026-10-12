package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func staffEntry(actorID uuid.UUID, action audit.Action, subjectID uuid.UUID) audit.NewEntry {
	return audit.NewEntry{
		ActorID:    actorID,
		ActorKind:  audit.ActorStaff,
		Action:     action,
		TargetType: audit.TargetUser,
		TargetID:   subjectID.String(),
		SubjectID:  subjectID,
	}
}

func ageAuditLog(t *testing.T, repos *repository.Repositories, age string) {
	t.Helper()

	_, err := repos.DB().ExecContext(t.Context(), `UPDATE audit_log SET created_at = NOW() - $1::interval`, age)
	require.NoError(t, err)
}

func TestAuditLogDAO_CreateAndListSite_RoundTripsEveryField(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos, daotest.WithDisplayName("Staffer"))
	member := daotest.CreateUser(t, repos, daotest.WithDisplayName("Member"))
	entry := staffEntry(staff.ID, audit.ActionBanUser, member.ID)
	entry.Details = map[string]string{"reason": "spam"}
	entry.IP = "198.51.100.7"

	// when
	err := repos.AuditLog.Create(t.Context(), entry)

	// then
	require.NoError(t, err)
	entries, err := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	got := entries[0]
	assert.Nil(t, got.OrgID)
	require.NotNil(t, got.ActorID)
	assert.Equal(t, staff.ID, *got.ActorID)
	assert.Equal(t, audit.ActorStaff, got.ActorKind)
	assert.Equal(t, "Staffer", got.ActorName)
	assert.Equal(t, audit.ActionBanUser, got.Action)
	assert.Equal(t, audit.TargetUser, got.TargetType)
	assert.Equal(t, member.ID.String(), got.TargetID)
	require.NotNil(t, got.SubjectID)
	assert.Equal(t, member.ID, *got.SubjectID)
	assert.Equal(t, "Member", got.SubjectName)
	assert.Equal(t, map[string]string{"reason": "spam"}, got.Details)
	require.NotNil(t, got.IP)
	assert.Equal(t, "198.51.100.7", *got.IP)
	assert.WithinDuration(t, time.Now(), got.CreatedAt, time.Minute)
}

func TestAuditLogDAO_Create_ASystemEntryWithNoActorOrDetails(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)

	// when
	err := repos.AuditLog.Create(t.Context(), audit.NewEntry{ActorKind: audit.ActorSystem, Action: audit.ActionUpdateSettings, TargetType: audit.TargetSettings})

	// then
	require.NoError(t, err)
	entries, err := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Nil(t, entries[0].ActorID)
	assert.Empty(t, entries[0].ActorName)
	assert.Nil(t, entries[0].SubjectID)
	assert.Nil(t, entries[0].IP)
	assert.Empty(t, entries[0].TargetID)
	assert.NotNil(t, entries[0].Details)
	assert.Empty(t, entries[0].Details)
}

func TestAuditLogDAO_ListSite_NewestFirstFilteredAndPaged(t *testing.T) {
	// given three staff actions, the ban the most recent
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	member := daotest.CreateUser(t, repos)
	for _, action := range []audit.Action{audit.ActionLockUser, audit.ActionUnlockUser, audit.ActionBanUser} {
		require.NoError(t, repos.AuditLog.Create(t.Context(), staffEntry(staff.ID, action, member.ID)))
	}

	// when
	firstPage, firstErr := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 2})
	secondPage, secondErr := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 2, Offset: 2})
	bans, bansErr := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Action: audit.ActionBanUser, Limit: 10})
	total, totalErr := repos.AuditLog.CountSite(t.Context(), "")
	banCount, banCountErr := repos.AuditLog.CountSite(t.Context(), audit.ActionBanUser)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, bansErr)
	require.NoError(t, totalErr)
	require.NoError(t, banCountErr)
	require.Len(t, firstPage, 2)
	assert.Equal(t, audit.ActionBanUser, firstPage[0].Action)
	assert.Equal(t, audit.ActionUnlockUser, firstPage[1].Action)
	require.Len(t, secondPage, 1)
	assert.Equal(t, audit.ActionLockUser, secondPage[0].Action)
	require.Len(t, bans, 1)
	assert.Equal(t, audit.ActionBanUser, bans[0].Action)
	assert.Equal(t, 3, total)
	assert.Equal(t, 1, banCount)
}

func TestAuditLogDAO_ListSite_LeavesOutOrganisationEntries(t *testing.T) {
	// given one site entry and one entry that belongs to an organisation's own log
	repos := daotest.NewRepos(t)
	owner := daotest.CreateUser(t, repos)
	require.NoError(t, repos.AuditLog.Create(t.Context(), audit.NewEntry{ActorID: owner.ID, ActorKind: audit.ActorUser, Action: audit.ActionIdentityLinked, TargetType: audit.TargetIdentity, SubjectID: owner.ID}))
	require.NoError(t, repos.AuditLog.Create(t.Context(), audit.NewEntry{OrgID: uuid.New(), ActorID: owner.ID, ActorKind: audit.ActorUser, Action: audit.ActionOrgUpdated, TargetType: "organisation", SubjectID: owner.ID}))

	// when
	site, siteErr := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 10})
	siteCount, siteCountErr := repos.AuditLog.CountSite(t.Context(), "")
	subject, subjectErr := repos.AuditLog.ListForSubject(t.Context(), spec.AuditLogSubjectListing{SubjectID: owner.ID, Limit: 10})
	subjectCount, subjectCountErr := repos.AuditLog.CountForSubject(t.Context(), owner.ID)

	// then staff reading the site log never see a tenant's own audit trail
	require.NoError(t, siteErr)
	require.NoError(t, siteCountErr)
	require.NoError(t, subjectErr)
	require.NoError(t, subjectCountErr)
	require.Len(t, site, 1)
	assert.Equal(t, audit.ActionIdentityLinked, site[0].Action)
	assert.Equal(t, 1, siteCount)
	require.Len(t, subject, 1)
	assert.Equal(t, audit.ActionIdentityLinked, subject[0].Action)
	assert.Equal(t, 1, subjectCount)
}

func TestAuditLogDAO_ListForSubject_OnlyThatUsersEntries(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	first := daotest.CreateUser(t, repos)
	second := daotest.CreateUser(t, repos)
	require.NoError(t, repos.AuditLog.Create(t.Context(), staffEntry(staff.ID, audit.ActionLockUser, first.ID)))
	require.NoError(t, repos.AuditLog.Create(t.Context(), staffEntry(staff.ID, audit.ActionBanUser, second.ID)))
	require.NoError(t, repos.AuditLog.Create(t.Context(), staffEntry(staff.ID, audit.ActionUnlockUser, first.ID)))

	// when
	entries, err := repos.AuditLog.ListForSubject(t.Context(), spec.AuditLogSubjectListing{SubjectID: first.ID, Limit: 10})
	count, countErr := repos.AuditLog.CountForSubject(t.Context(), first.ID)

	// then
	require.NoError(t, err)
	require.NoError(t, countErr)
	require.Len(t, entries, 2)
	assert.Equal(t, audit.ActionUnlockUser, entries[0].Action)
	assert.Equal(t, audit.ActionLockUser, entries[1].Action)
	assert.Equal(t, 2, count)
}

func TestAuditLogDAO_DeletingTheActorKeepsTheEntry(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	member := daotest.CreateUser(t, repos)
	require.NoError(t, repos.AuditLog.Create(t.Context(), staffEntry(staff.ID, audit.ActionDeleteUser, member.ID)))

	// when
	require.NoError(t, repos.User.Delete(t.Context(), staff.ID))
	require.NoError(t, repos.User.Delete(t.Context(), member.ID))

	// then the record of what happened outlives both accounts
	entries, err := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Nil(t, entries[0].ActorID)
	assert.Empty(t, entries[0].ActorName)
	assert.Nil(t, entries[0].SubjectID)
	assert.Equal(t, member.ID.String(), entries[0].TargetID)
}

func TestAuditLogDAO_ClearIPsBefore_NullsOnlyStaleAddresses(t *testing.T) {
	// given one entry recorded 100 days ago and one recorded now
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	old := staffEntry(staff.ID, audit.ActionLockUser, staff.ID)
	old.IP = "198.51.100.7"
	require.NoError(t, repos.AuditLog.Create(t.Context(), old))
	ageAuditLog(t, repos, "100 days")
	fresh := staffEntry(staff.ID, audit.ActionUnlockUser, staff.ID)
	fresh.IP = "203.0.113.9"
	require.NoError(t, repos.AuditLog.Create(t.Context(), fresh))

	// when
	cleared, err := repos.AuditLog.ClearIPsBefore(t.Context(), time.Now().Add(-90*24*time.Hour))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleared)
	entries, err := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.NotNil(t, entries[0].IP)
	assert.Equal(t, "203.0.113.9", *entries[0].IP)
	assert.Nil(t, entries[1].IP)
}

func TestAuditLogDAO_DeleteCreatedBefore_PrunesOnlyOldEntries(t *testing.T) {
	// given one entry recorded 401 days ago and one recorded now
	repos := daotest.NewRepos(t)
	staff := daotest.CreateUser(t, repos)
	require.NoError(t, repos.AuditLog.Create(t.Context(), staffEntry(staff.ID, audit.ActionLockUser, staff.ID)))
	ageAuditLog(t, repos, "401 days")
	require.NoError(t, repos.AuditLog.Create(t.Context(), staffEntry(staff.ID, audit.ActionUnlockUser, staff.ID)))

	// when
	deleted, err := repos.AuditLog.DeleteCreatedBefore(t.Context(), time.Now().Add(-400*24*time.Hour))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	entries, err := repos.AuditLog.ListSite(t.Context(), spec.AuditLogListing{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, audit.ActionUnlockUser, entries[0].Action)
}
