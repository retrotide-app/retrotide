package management

import (
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	strangerID = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000011")
	teamBoard  = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000012")
)

func deletedHeader(id uuid.UUID, team *uuid.UUID, createdBy uuid.UUID) model.BoardHeader {
	return model.BoardHeader{Board: model.Board{
		ID:        id,
		OrgID:     orgID,
		TeamID:    team,
		Title:     "Sprint 12",
		CreatedBy: new(createdBy),
		CreatedAt: stamp,
		DeletedAt: new(stamp.AddDate(0, 0, -3)),
	}}
}

func restoreCommand(id uuid.UUID) RestoreCommand {
	return RestoreCommand{Principal: me, OrgID: orgID, BoardID: id}
}

func restoreWrite(id uuid.UUID) spec.BoardRestore {
	return spec.BoardRestore{OrgID: orgID, BoardID: id, DeletedSince: stamp.Add(-model.BoardRestoreWindow)}
}

func (m *testMocks) decidesInOrg(permission orgauthz.Permission, decision orgauthz.Decision) {
	check := spec.OrgPermissionCheck{OrgID: orgID, UserID: userID, Permission: permission}
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(decision, nil).Once()
}

func (m *testMocks) decidesInTeam(team uuid.UUID, permission orgauthz.Permission, decision orgauthz.Decision) {
	check := spec.TeamPermissionCheck{TeamID: team, UserID: userID, Permission: permission}
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, check).Return(decision, nil).Once()
}

func restoreAudit(id uuid.UUID) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      orgID,
		ActorID:    userID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionBoardRestored,
		TargetType: audit.TargetBoard,
		TargetID:   id.String(),
		SubjectID:  userID,
	}
}

func TestRestore_TheCreatorBringsBackTheirOrganisationBoardWhileStillAMember(t *testing.T) {
	// given
	svc, m := newTestService(t)
	header := deletedHeader(boardID, nil, userID)
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(&header, nil)
	m.decidesInOrg(orgauthz.PermOrgManageBoards, orgauthz.Decision{Member: true})
	m.decidesInOrg(orgauthz.PermOrgView, orgauthz.Decision{Member: true, Allowed: true})
	m.boards.EXPECT().Restore(mock.Anything, restoreWrite(boardID)).Return(&header.Board, nil)
	m.auditLog.EXPECT().Create(mock.Anything, restoreAudit(boardID)).Return(nil).Once()

	// when
	err := svc.Restore(t.Context(), restoreCommand(boardID))

	// then
	require.NoError(t, err)
}

func TestRestore_ATeamLeadBringsBackAnyBoardOfTheTeam(t *testing.T) {
	// given a board someone else created
	svc, m := newTestService(t)
	header := deletedHeader(teamBoard, new(teamID), strangerID)
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: teamBoard}).Return(&header, nil)
	m.decidesInTeam(teamID, orgauthz.PermTeamManageBoards, orgauthz.Decision{Member: true, Allowed: true})
	m.boards.EXPECT().Restore(mock.Anything, restoreWrite(teamBoard)).Return(&header.Board, nil)
	m.auditLog.EXPECT().Create(mock.Anything, restoreAudit(teamBoard)).Return(nil).Once()

	// when
	err := svc.Restore(t.Context(), restoreCommand(teamBoard))

	// then
	require.NoError(t, err)
}

func TestRestore_IsRefusedToAMemberWhoNeitherManagesNorCreatedTheBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	header := deletedHeader(teamBoard, new(teamID), strangerID)
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: teamBoard}).Return(&header, nil)
	m.decidesInTeam(teamID, orgauthz.PermTeamManageBoards, orgauthz.Decision{Member: true})

	// when
	err := svc.Restore(t.Context(), restoreCommand(teamBoard))

	// then the strict mocks prove nothing was restored
	require.ErrorIs(t, err, ErrRestoreNotAllowed)
}

func TestRestore_IsRefusedToACreatorWhoLeftTheOrganisation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	header := deletedHeader(boardID, nil, userID)
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(&header, nil)
	m.decidesInOrg(orgauthz.PermOrgManageBoards, orgauthz.Decision{})
	m.decidesInOrg(orgauthz.PermOrgView, orgauthz.Decision{})

	// when
	err := svc.Restore(t.Context(), restoreCommand(boardID))

	// then
	require.ErrorIs(t, err, ErrRestoreNotAllowed)
}

func TestRestore_IsRefusedToAGuest(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	command := restoreCommand(boardID)
	command.Principal = model.Principal{Kind: model.PrincipalGuest, ID: userID}

	// when
	err := svc.Restore(t.Context(), command)

	// then
	require.ErrorIs(t, err, ErrRestoreNotAllowed)
}

func TestRestore_ABoardThatIsNotDeletedIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	header := deletedHeader(boardID, nil, userID)
	header.DeletedAt = nil
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(&header, nil)

	// when
	err := svc.Restore(t.Context(), restoreCommand(boardID))

	// then
	require.ErrorIs(t, err, ErrBoardNotFound)
}

func TestRestore_ABoardOfAnotherOrganisationIsNotFound(t *testing.T) {
	// given the board lives in another organisation, so the organisation-scoped read finds nothing
	svc, m := newTestService(t)
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(nil, errNotFound)

	// when
	err := svc.Restore(t.Context(), restoreCommand(boardID))

	// then
	require.ErrorIs(t, err, ErrBoardNotFound)
}

func TestRestore_ABoardDeletedBeforeTheWindowIsNotFound(t *testing.T) {
	// given a board whose 30 days ran out between the read and the write
	svc, m := newTestService(t)
	header := deletedHeader(boardID, nil, userID)
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(&header, nil)
	m.decidesInOrg(orgauthz.PermOrgManageBoards, orgauthz.Decision{Member: true, Allowed: true})
	m.boards.EXPECT().Restore(mock.Anything, restoreWrite(boardID)).Return(nil, errNotFound)

	// when
	err := svc.Restore(t.Context(), restoreCommand(boardID))

	// then
	require.ErrorIs(t, err, ErrBoardNotFound)
}

func TestRestore_AMissingAuditEntryIsReportedAfterTheRestore(t *testing.T) {
	// given
	svc, m := newTestService(t)
	header := deletedHeader(boardID, nil, userID)
	m.boards.EXPECT().GetByID(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(&header, nil)
	m.decidesInOrg(orgauthz.PermOrgManageBoards, orgauthz.Decision{Member: true, Allowed: true})
	m.boards.EXPECT().Restore(mock.Anything, restoreWrite(boardID)).Return(&header.Board, nil)
	m.auditLog.EXPECT().Create(mock.Anything, restoreAudit(boardID)).Return(errDatabase).Once()

	// when
	err := svc.Restore(t.Context(), restoreCommand(boardID))

	// then
	require.ErrorIs(t, err, ErrNotAudited)
}

func TestListDeleted_ListsOnlyTheBoardsTheViewerCouldRestore(t *testing.T) {
	// given three deleted boards: one the viewer created, one on a team they lead, one they cannot touch
	svc, m := newTestService(t)
	mine := deletedHeader(boardID, nil, userID)
	led := deletedHeader(teamBoard, new(teamID), strangerID)
	theirs := deletedHeader(uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000013"), new(otherTeam), strangerID)
	query := spec.DeletedBoardsQuery{OrgID: orgID, DeletedSince: stamp.Add(-model.BoardRestoreWindow)}
	m.boards.EXPECT().ListDeletedForOrganisation(mock.Anything, query).Return([]model.BoardHeader{mine, led, theirs}, nil)
	m.decidesInOrg(orgauthz.PermOrgManageBoards, orgauthz.Decision{Member: true})
	m.decidesInOrg(orgauthz.PermOrgView, orgauthz.Decision{Member: true, Allowed: true})
	m.decidesInTeam(teamID, orgauthz.PermTeamManageBoards, orgauthz.Decision{Member: true, Allowed: true})
	m.decidesInTeam(otherTeam, orgauthz.PermTeamManageBoards, orgauthz.Decision{Member: true})
	m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return([]model.Team{{ID: teamID, OrgID: orgID, Name: "Payments"}}, nil)

	// when
	got, err := svc.ListDeleted(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.DeletedBoardListResponse{Boards: []dto.DeletedBoardItem{
		{ID: boardID, Title: "Sprint 12", DeletedAt: "2026-10-09T09:00:00.000Z", RestoreUntil: "2026-11-08T09:00:00.000Z"},
		{ID: teamBoard, TeamID: new(teamID), TeamName: "Payments", Title: "Sprint 12", DeletedAt: "2026-10-09T09:00:00.000Z", RestoreUntil: "2026-11-08T09:00:00.000Z"},
	}}, got)
}

func TestListDeleted_LeavesOutABoardOfAnotherOrganisation(t *testing.T) {
	// given a row that is not the viewer's organisation's
	svc, m := newTestService(t)
	stray := deletedHeader(boardID, nil, userID)
	stray.OrgID = otherOrgID
	m.boards.EXPECT().ListDeletedForOrganisation(mock.Anything, mock.Anything).Return([]model.BoardHeader{stray}, nil)
	m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return(nil, nil)

	// when
	got, err := svc.ListDeleted(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	require.NoError(t, err)
	assert.Empty(t, got.Boards)
}

func TestListDeleted_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.boards.EXPECT().ListDeletedForOrganisation(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	_, err := svc.ListDeleted(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	require.ErrorIs(t, err, errDatabase)
}
