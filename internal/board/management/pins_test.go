package management

import (
	"fmt"
	"testing"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func pinCommand(pinned bool) PinCommand {
	return PinCommand{Principal: me, BoardID: boardID, Pinned: pinned}
}

func pinKey() spec.BoardPinKey {
	return spec.BoardPinKey{OrgID: orgID, BoardID: boardID, UserID: userID}
}

func (m *testMocks) resolvedForPin(access *model.BoardAccess) {
	query := spec.BoardAccessQuery{Principal: me, BoardID: boardID}
	m.live.EXPECT().Resolve(mock.Anything, query).Return(access, nil).Once()
}

func TestPin_PinsTheBoardForAnOrganisationMember(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.resolvedForPin(boardAccess(writer))
	m.boards.EXPECT().Pin(mock.Anything, pinKey()).Return(nil)

	// when
	err := svc.Pin(t.Context(), pinCommand(true))

	// then
	require.NoError(t, err)
}

func TestPin_UnpinsTheBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.resolvedForPin(boardAccess(writer))
	m.boards.EXPECT().Unpin(mock.Anything, pinKey()).Return(nil)

	// when
	err := svc.Pin(t.Context(), pinCommand(false))

	// then
	require.NoError(t, err)
}

func TestPin_PinsAnArchivedBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.resolvedForPin(boardAccess(host, archived))
	m.boards.EXPECT().Pin(mock.Anything, pinKey()).Return(nil)

	// when
	err := svc.Pin(t.Context(), pinCommand(true))

	// then
	require.NoError(t, err)
}

func TestPin_IsRefusedToAGuestWithoutResolvingTheBoard(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	command := pinCommand(true)
	command.Principal = model.Principal{Kind: model.PrincipalGuest, ID: userID}

	// when
	err := svc.Pin(t.Context(), command)

	// then the strict mocks prove the board was never read
	require.ErrorIs(t, err, policy.ErrMembersOnly)
}

func TestPin_IsRefusedToAnAccountThatJoinedThroughTheLinkWithoutMembership(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(writer)
	access.OrgMember = false
	m.resolvedForPin(access)

	// when
	err := svc.Pin(t.Context(), pinCommand(true))

	// then
	require.ErrorIs(t, err, policy.ErrMembersOnly)
}

func TestPin_PassesOnABoardTheViewerCannotOpen(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(nil, fmt.Errorf("resolve: %w", live.ErrNoAccess)).Once()

	// when
	err := svc.Pin(t.Context(), pinCommand(true))

	// then
	require.ErrorIs(t, err, live.ErrNoAccess)
}

func TestPin_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.resolvedForPin(boardAccess(writer))
	m.boards.EXPECT().Pin(mock.Anything, pinKey()).Return(errDatabase)

	// when
	err := svc.Pin(t.Context(), pinCommand(true))

	// then
	require.ErrorIs(t, err, errDatabase)
}
