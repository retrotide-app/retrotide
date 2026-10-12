package management

import (
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func moveCommand(team *uuid.UUID) MoveCommand {
	return MoveCommand{Command: baseCommand(), TeamID: team}
}

func TestMove_MovesTheBoardToAnotherTeamTheManagerMayCreateBoardsOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.teams.EXPECT().GetHeader(mock.Anything, otherTeam).Return(&model.Team{ID: otherTeam, OrgID: orgID, Name: "Platform"}, nil)
	m.decidesInTeam(otherTeam, orgauthz.PermTeamCreateBoard, orgauthz.Decision{Member: true, Allowed: true})
	moved := access.Header.Board
	moved.TeamID = new(otherTeam)
	m.boards.EXPECT().MoveBoard(mock.Anything, spec.BoardTeamWrite{BoardWrite: target(), TeamID: new(otherTeam)}).Return(versioned(moved, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)

	// when
	got, err := svc.Move(t.Context(), moveCommand(new(otherTeam)))

	// then the old team's viewers are re-evaluated against the new home
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, resyncAt(8), m.events.published[0].event.Payload(projection.ClassOthers))
	assert.Equal(t, new(otherTeam), m.events.published[0].access.Header.TeamID)
	assert.Equal(t, []uuid.UUID{teamID, otherTeam}, m.events.announced, "both teams' action item lists refresh, since the board's items moved with it")
}

func TestMove_MovesATeamBoardToTheOrganisation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.decidesInOrg(orgauthz.PermOrgCreateBoard, orgauthz.Decision{Member: true, Allowed: true})
	moved := access.Header.Board
	moved.TeamID = nil
	m.boards.EXPECT().MoveBoard(mock.Anything, spec.BoardTeamWrite{BoardWrite: target()}).Return(versioned(moved, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)

	// when
	got, err := svc.Move(t.Context(), moveCommand(nil))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	assert.Equal(t, []uuid.UUID{teamID}, m.events.announced)
}

func TestMove_RefusesTheTeamTheBoardIsAlreadyOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(&model.Team{ID: teamID, OrgID: orgID}, nil)
	m.decidesInTeam(teamID, orgauthz.PermTeamCreateBoard, orgauthz.Decision{Member: true, Allowed: true})

	// when
	_, err := svc.Move(t.Context(), moveCommand(new(teamID)))

	// then
	require.ErrorIs(t, err, ErrSameTeam)
}

func TestMove_RefusesAnOrganisationBoardMovedToTheOrganisation(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	access.Header.TeamID = nil
	m.entered(access)
	m.decidesInOrg(orgauthz.PermOrgCreateBoard, orgauthz.Decision{Member: true, Allowed: true})

	// when
	_, err := svc.Move(t.Context(), moveCommand(nil))

	// then
	require.ErrorIs(t, err, ErrSameTeam)
}

func TestMove_RefusesATeamOfAnotherOrganisationAsNotFound(t *testing.T) {
	// given a team ID taken from another tenant
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.teams.EXPECT().GetHeader(mock.Anything, otherTeam).Return(&model.Team{ID: otherTeam, OrgID: otherOrgID}, nil)

	// when
	_, err := svc.Move(t.Context(), moveCommand(new(otherTeam)))

	// then the strict mocks prove nothing was written
	require.ErrorIs(t, err, ErrTeamNotFound)
}

func TestMove_RefusesAMissingTeamAsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.teams.EXPECT().GetHeader(mock.Anything, otherTeam).Return(nil, errNotFound)

	// when
	_, err := svc.Move(t.Context(), moveCommand(new(otherTeam)))

	// then
	require.ErrorIs(t, err, ErrTeamNotFound)
}

func TestMove_RefusesAnArchivedTeam(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.teams.EXPECT().GetHeader(mock.Anything, otherTeam).Return(&model.Team{ID: otherTeam, OrgID: orgID, ArchivedAt: new(stamp)}, nil)

	// when
	_, err := svc.Move(t.Context(), moveCommand(new(otherTeam)))

	// then
	require.ErrorIs(t, err, ErrTeamArchived)
}

func TestMove_RefusesATeamTheManagerMayNotCreateBoardsOn(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.teams.EXPECT().GetHeader(mock.Anything, otherTeam).Return(&model.Team{ID: otherTeam, OrgID: orgID}, nil)
	m.decidesInTeam(otherTeam, orgauthz.PermTeamCreateBoard, orgauthz.Decision{Member: true})

	// when
	_, err := svc.Move(t.Context(), moveCommand(new(otherTeam)))

	// then
	require.ErrorIs(t, err, ErrMoveNotAllowed)
}

func TestMove_RefusesTheOrganisationToAManagerWhoMayNotCreateOrganisationBoards(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.decidesInOrg(orgauthz.PermOrgCreateBoard, orgauthz.Decision{Member: true})

	// when
	_, err := svc.Move(t.Context(), moveCommand(nil))

	// then
	require.ErrorIs(t, err, ErrMoveNotAllowed)
}

func TestMove_IsRefusedWithoutBoardManage(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	_, err := svc.Move(t.Context(), moveCommand(new(otherTeam)))

	// then
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Empty(t, m.events.published)
}
