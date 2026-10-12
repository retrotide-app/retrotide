package admin

import (
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetAuditLog_FiltersByActionAndClampsThePage(t *testing.T) {
	// given
	svc, m := newTestService(t)
	actorID, subjectID := uuid.New(), uuid.New()
	createdAt := time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)
	m.auditLog.EXPECT().ListSite(mock.Anything, spec.AuditLogListing{Action: audit.ActionBanUser, Limit: maxPageSize, Offset: 10}).Return([]audit.Entry{
		{ID: 7, ActorID: &actorID, ActorKind: audit.ActorStaff, ActorName: "Staff", Action: audit.ActionBanUser, TargetType: audit.TargetUser, TargetID: subjectID.String(), SubjectID: &subjectID, SubjectName: "Alice", Details: map[string]string{"reason": "spam"}, IP: new(testIP), CreatedAt: createdAt},
	}, nil)
	m.auditLog.EXPECT().CountSite(mock.Anything, audit.ActionBanUser).Return(1, nil)

	// when
	listing, err := svc.GetAuditLog(t.Context(), spec.AuditLogListing{Action: audit.ActionBanUser, Limit: 1000, Offset: 10})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.AuditLogListResponse{
		Entries: []dto.AuditLogEntryResponse{{
			ID:          7,
			ActorID:     &actorID,
			ActorKind:   audit.ActorStaff,
			ActorName:   "Staff",
			Action:      audit.ActionBanUser,
			TargetType:  audit.TargetUser,
			TargetID:    subjectID.String(),
			SubjectID:   &subjectID,
			SubjectName: "Alice",
			Details:     map[string]string{"reason": "spam"},
			CreatedAt:   "2026-09-25T11:00:00Z",
		}},
		Total:  1,
		Limit:  maxPageSize,
		Offset: 10,
	}, listing)
}

func TestGetAuditLog_EntriesWithoutDetailsCarryAnEmptyObject(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.auditLog.EXPECT().ListSite(mock.Anything, spec.AuditLogListing{Limit: defaultAuditPageSize}).Return([]audit.Entry{{ID: 1, Action: audit.ActionUnbanUser}}, nil)
	m.auditLog.EXPECT().CountSite(mock.Anything, audit.Action("")).Return(1, nil)

	// when
	listing, err := svc.GetAuditLog(t.Context(), spec.AuditLogListing{})

	// then
	require.NoError(t, err)
	require.Len(t, listing.Entries, 1)
	assert.Equal(t, map[string]string{}, listing.Entries[0].Details)
}

func TestGetAuditLog_ReturnsAReadFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.auditLog.EXPECT().ListSite(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	listing, err := svc.GetAuditLog(t.Context(), spec.AuditLogListing{})

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listing)
}

func TestGetUserAuditLog_ListsTheEntriesAboutOneUser(t *testing.T) {
	// given
	svc, m := newTestService(t)
	subjectID := uuid.New()
	m.auditLog.EXPECT().ListForSubject(mock.Anything, spec.AuditLogSubjectListing{SubjectID: subjectID, Limit: 10, Offset: 0}).Return([]audit.Entry{}, nil)
	m.auditLog.EXPECT().CountForSubject(mock.Anything, subjectID).Return(0, nil)

	// when
	listing, err := svc.GetUserAuditLog(t.Context(), spec.AuditLogSubjectListing{SubjectID: subjectID, Limit: 10})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.AuditLogListResponse{Entries: []dto.AuditLogEntryResponse{}, Total: 0, Limit: 10, Offset: 0}, listing)
}

func TestGetUserAuditLog_ReturnsACountFailure(t *testing.T) {
	// given
	svc, m := newTestService(t)
	subjectID := uuid.New()
	m.auditLog.EXPECT().ListForSubject(mock.Anything, mock.Anything).Return([]audit.Entry{}, nil)
	m.auditLog.EXPECT().CountForSubject(mock.Anything, subjectID).Return(0, errDatabaseDown)

	// when
	listing, err := svc.GetUserAuditLog(t.Context(), spec.AuditLogSubjectListing{SubjectID: subjectID})

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, listing)
}
