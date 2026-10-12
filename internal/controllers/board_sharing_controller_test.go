package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	boardSharing       = &dto.SharingView{ShareSlug: "K3V9X2QPLMNORSTUVWXYZ234", LinkAccess: model.LinkAccessGuests, HasPassword: true}
	boardSharedVersion = &dto.BoardCommandResponse{Version: 9, Sharing: boardSharing}
)

func TestBoardSharingController_GetSharingAnswersTheManagersView(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	bh.boards.EXPECT().Sharing(mock.Anything, spec.BoardAccessQuery{Principal: boardPrincipal, BoardID: boardID}).Return(boardSharing, nil)

	// when
	status, body := bh.record(http.MethodGet, boardPath("/sharing"), nil)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"share_slug":"K3V9X2QPLMNORSTUVWXYZ234","link_access":"guests","has_password":true}`, body)
}

func TestBoardSharingController_GetSharingRefusesSomeoneWhoCannotManage(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	bh.boards.EXPECT().Sharing(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("sharing: %w", policy.ErrNotAllowed))

	// when
	status, body := bh.record(http.MethodGet, boardPath("/sharing"), nil)

	// then
	assert.Equal(t, http.StatusForbidden, status)
	assert.JSONEq(t, `{"error":"your role on this board does not allow that"}`, body)
}

func TestBoardSharingController_EveryShareCommandBuildsItsCommand(t *testing.T) {
	password := "correct horse"

	tests := []struct {
		name   string
		method string
		path   string
		body   any
		expect func(bh *boardHarness)
	}{
		{
			name:   "link access",
			method: http.MethodPut,
			path:   boardPath("/sharing/link-access"),
			body:   dto.SetLinkAccessRequest{BoardCommandRequest: boardOpBody, LinkAccess: model.LinkAccessAccounts},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().SetLinkAccess(mock.Anything, board.SetLinkAccessCommand{Command: boardCommandOf, LinkAccess: model.LinkAccessAccounts}).Return(boardSharedVersion, nil)
			},
		},
		{
			name:   "a new password",
			method: http.MethodPut,
			path:   boardPath("/sharing/password"),
			body:   dto.SetBoardPasswordRequest{BoardCommandRequest: boardOpBody, Password: &password},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().SetPassword(mock.Anything, board.SetPasswordCommand{Command: boardCommandOf, Password: &password}).Return(boardSharedVersion, nil)
			},
		},
		{
			name:   "a cleared password",
			method: http.MethodPut,
			path:   boardPath("/sharing/password"),
			body:   map[string]any{"op_id": boardOp, "password": nil},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().SetPassword(mock.Anything, board.SetPasswordCommand{Command: boardCommandOf}).Return(boardSharedVersion, nil)
			},
		},
		{
			name:   "a rotated link",
			method: http.MethodPost,
			path:   boardPath("/sharing/rotate"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RotateLink(mock.Anything, boardCommandOf).Return(boardSharedVersion, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, mustJSON(t, boardSharedVersion), body)
		})
	}
}

func TestBoardSharingController_AnUnauditedChangeStillAnswersTheChange(t *testing.T) {
	// given the change committed but its audit entry failed
	bh := newBoardHarness(t)
	bh.boards.EXPECT().RotateLink(mock.Anything, boardCommandOf).Return(boardSharedVersion, fmt.Errorf("%w: %w", board.ErrNotAudited, errors.New("audit store down")))

	// when
	status, body := bh.record(http.MethodPost, boardPath("/sharing/rotate"), boardOpBody)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, boardSharedVersion), body)
}

func TestBoardSharingController_ShareCommandsMapTheRefusals(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: board.ErrInvalidLinkAccess, status: http.StatusBadRequest, body: `{"error":"link access must be off, guests or accounts","code":"invalid_link_access"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			bh.boards.EXPECT().SetLinkAccess(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("share: %w", tc.err))

			// when
			status, body := bh.record(http.MethodPut, boardPath("/sharing/link-access"), dto.SetLinkAccessRequest{BoardCommandRequest: boardOpBody, LinkAccess: "public"})

			// then
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardSharingController_AGuestCannotReachTheSharingButIsAskedAsItself(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	cookie := bh.SignInAsGuest(linkGuest)
	bh.boards.EXPECT().Sharing(mock.Anything, spec.BoardAccessQuery{Principal: linkGuestPerson, BoardID: boardID}).Return(nil, policy.ErrNotAllowed)

	// when
	rec := bh.NewRequest(http.MethodGet, boardPath("/sharing")).WithCookie(cookie).Record()

	// then the board service, which owns board access, refuses a guest
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
