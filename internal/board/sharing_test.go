package board

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"retrotide/internal/audit"
	"retrotide/internal/board/policy"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const (
	sharedSlug = "K3V9X2QPLMNORSTUVWXYZ234"
)

func sharedBoard(version int64, change func(board *model.Board)) *model.Versioned[model.Board] {
	board := boardAccess(host).Header.Board
	board.ShareSlug = sharedSlug
	board.Version = version
	change(&board)

	return &model.Versioned[model.Board]{Value: board, Version: version}
}

func sharingAudit(action audit.Action, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      orgID,
		ActorID:    userID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetBoard,
		TargetID:   boardID.String(),
		SubjectID:  userID,
		Details:    details,
	}
}

func TestSharing_AnswersTheFacilitatorsView(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host, func(a *model.BoardAccess) {
		a.Header.ShareSlug = sharedSlug
		a.Header.LinkAccess = model.LinkAccessGuests
		a.Header.PasswordHash = new("hash")
	})
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil)

	// when
	sharing, err := svc.Sharing(t.Context(), spec.BoardAccessQuery{Principal: me, BoardID: boardID})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.SharingView{ShareSlug: sharedSlug, LinkAccess: model.LinkAccessGuests, HasPassword: true}, sharing)
}

func TestSharing_RefusesAParticipant(t *testing.T) {
	// given a participant, a guest included, who may not manage the board
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, mock.Anything).Return(boardAccess(writer), nil)

	// when
	sharing, err := svc.Sharing(t.Context(), spec.BoardAccessQuery{Principal: me, BoardID: boardID})

	// then the slug never reaches someone who cannot rotate it
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Nil(t, sharing)
}

func TestSetLinkAccess_RefusesAnUnknownLevelBeforeTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	_, err := svc.SetLinkAccess(t.Context(), SetLinkAccessCommand{Command: baseCommand(), LinkAccess: "public"})

	// then
	require.ErrorIs(t, err, ErrInvalidLinkAccess)
}

func TestSetLinkAccess_WritesPublishesReevaluatesAndAudits(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	written := sharedBoard(8, func(board *model.Board) { board.LinkAccess = model.LinkAccessAccounts })
	m.boards.EXPECT().SetBoardLinkAccess(mock.Anything, spec.BoardLinkAccessWrite{BoardWrite: target(), LinkAccess: model.LinkAccessAccounts}).Return(written, nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()
	m.auditLog.EXPECT().Create(mock.Anything, sharingAudit(audit.ActionBoardSharingChanged, map[string]string{"link_access": "accounts"})).Return(nil).Once()

	// when
	response, err := svc.SetLinkAccess(t.Context(), SetLinkAccessCommand{Command: baseCommand(), LinkAccess: model.LinkAccessAccounts})

	// then the live board holds the new header before any viewer is resolved again
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8, Sharing: &dto.SharingView{ShareSlug: sharedSlug, LinkAccess: model.LinkAccessAccounts}}, response)
	require.Len(t, m.events.headers, 1)
	assert.Equal(t, model.LinkAccessAccounts, m.events.headers[0].LinkAccess)
	assert.Empty(t, m.events.published, "a sharing change sends viewers no event")
}

func TestShareCommands_RefuseSomeoneWhoCannotManageWithoutWriting(t *testing.T) {
	tests := []struct {
		name string
		run  func(svc *service) error
	}{
		{name: "link access", run: func(svc *service) error {
			_, err := svc.SetLinkAccess(context.Background(), SetLinkAccessCommand{Command: baseCommand(), LinkAccess: model.LinkAccessGuests})
			return err
		}},
		{name: "password", run: func(svc *service) error {
			_, err := svc.SetPassword(context.Background(), SetPasswordCommand{Command: baseCommand()})
			return err
		}},
		{name: "rotation", run: func(svc *service) error {
			_, err := svc.RotateLink(context.Background(), baseCommand())
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a participant, whom the strict repository and audit mocks prove write nothing
			svc, m := newTestService(t)
			m.entered(boardAccess(writer))

			// when
			err := tc.run(svc)

			// then
			require.ErrorIs(t, err, policy.ErrNotAllowed)
		})
	}
}

func TestSetPassword_StoresOnlyABcryptHashAndAuditsThatOneWasSet(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	password := "correct horse"
	var stored *string
	m.boards.EXPECT().SetBoardPassword(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, write spec.BoardPasswordWrite, _ ...*sql.Tx) (*model.Versioned[model.Board], error) {
		stored = write.PasswordHash

		return sharedBoard(8, func(board *model.Board) { board.PasswordHash = write.PasswordHash }), nil
	})
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()
	m.auditLog.EXPECT().Create(mock.Anything, sharingAudit(audit.ActionBoardPasswordChanged, map[string]string{"password": "set"})).Return(nil).Once()

	// when
	response, err := svc.SetPassword(t.Context(), SetPasswordCommand{Command: baseCommand(), Password: &password})

	// then neither the response nor the audit entry carries the password
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.NotEqual(t, password, *stored)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(*stored), []byte(password)))
	assert.True(t, response.Sharing.HasPassword)
}

func TestSetPassword_ClearsThePasswordAndAuditsThatItWasCleared(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().SetBoardPassword(mock.Anything, spec.BoardPasswordWrite{BoardWrite: target()}).Return(sharedBoard(8, func(*model.Board) {}), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()
	m.auditLog.EXPECT().Create(mock.Anything, sharingAudit(audit.ActionBoardPasswordChanged, map[string]string{"password": "cleared"})).Return(nil).Once()

	// when
	response, err := svc.SetPassword(t.Context(), SetPasswordCommand{Command: baseCommand()})

	// then
	require.NoError(t, err)
	assert.False(t, response.Sharing.HasPassword)
}

func TestSetPassword_RefusesABadPasswordBeforeTheGate(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     error
	}{
		{name: "blank", password: "   ", want: ErrPasswordBlank},
		{name: "empty", password: "", want: ErrPasswordBlank},
		{name: "over 72 bytes", password: strings.Repeat("é", 37), want: ErrPasswordTooLong},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			_, err := svc.SetPassword(t.Context(), SetPasswordCommand{Command: baseCommand(), Password: &tc.password})

			// then
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRotateLink_DrawsANewSlugAndNeverAuditsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	var drawn string
	m.boards.EXPECT().RotateBoardLink(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, write spec.BoardLinkRotationWrite, _ ...*sql.Tx) (*model.Versioned[model.Board], error) {
		drawn = write.ShareSlug

		return sharedBoard(8, func(board *model.Board) { board.ShareSlug = write.ShareSlug }), nil
	})
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()
	m.auditLog.EXPECT().Create(mock.Anything, sharingAudit(audit.ActionBoardLinkRotated, nil)).Return(nil).Once()

	// when
	response, err := svc.RotateLink(t.Context(), baseCommand())

	// then the slug is a fresh crypto/rand text of at least 128 bits, and only the manager's response carries it
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(drawn), 26)
	assert.NotEqual(t, sharedSlug, drawn)
	assert.Equal(t, drawn, response.Sharing.ShareSlug)
}

func TestShareCommands_AFailedAuditStillAnswersTheChange(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().RotateBoardLink(mock.Anything, mock.Anything).Return(sharedBoard(8, func(*model.Board) {}), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil).Once()
	errAudit := errors.New("audit store down")
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errAudit)

	// when
	response, err := svc.RotateLink(t.Context(), baseCommand())

	// then
	require.ErrorIs(t, err, ErrNotAudited)
	require.ErrorIs(t, err, errAudit)
	require.NotNil(t, response)
	assert.Equal(t, int64(8), response.Version)
}

func TestShareCommands_AFailedReevaluationIsReportedAndNotAudited(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().RotateBoardLink(mock.Anything, mock.Anything).Return(sharedBoard(8, func(*model.Board) {}), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(errDatabase)

	// when
	response, err := svc.RotateLink(t.Context(), baseCommand())

	// then the client retries with the same op and is answered from the replay
	require.ErrorIs(t, err, errDatabase)
	assert.Nil(t, response)
}

func TestShareCommands_AReplayAnswersOnlyTheVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 8)

	// when
	response, err := svc.RotateLink(t.Context(), baseCommand())

	// then a replay never carries the slug, so the dialog reads the sharing afresh
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8, Replayed: true}, response)
}
