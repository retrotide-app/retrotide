package ws

import (
	"context"
	"testing"
	"time"

	"retrotide/internal/model"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	guestKind model.PrincipalKind = "guest"
)

func newTestClient(t *testing.T, kind model.PrincipalKind) (*Client, context.Context) {
	t.Helper()

	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() {
		cancel(nil)
	})

	return newClient(model.Principal{Kind: kind, ID: uuid.New()}, nil, cancel, timings{}), ctx
}

func newAnonymousTestClient(t *testing.T) *Client {
	t.Helper()

	_, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() {
		cancel(nil)
	})

	return newClient(model.Principal{}, nil, cancel, timings{})
}

func drain(client *Client) []string {
	var frames []string
	for {
		select {
		case data := <-client.send:
			frames = append(frames, string(data))
		default:
			return frames
		}
	}
}

func TestHub_IsOnBoard_GatesNonMembers(t *testing.T) {
	// given
	hub := NewHub()
	boardID := uuid.New()
	member := uuid.New()
	stranger := uuid.New()
	hub.JoinBoard(boardID, member)

	// then
	assert.True(t, hub.IsOnBoard(boardID, member), "a member is on the board")
	assert.False(t, hub.IsOnBoard(boardID, stranger), "a stranger is not on the board")
	assert.False(t, hub.IsOnBoard(uuid.New(), member), "a member is not on a board they never joined")
}

func TestHub_LeaveBoard_RemovesTheMemberAndItsReverseIndexEntry(t *testing.T) {
	// given
	hub := NewHub()
	boardID := uuid.New()
	principalID := uuid.New()
	hub.JoinBoard(boardID, principalID)

	// when
	hub.LeaveBoard(boardID, principalID)

	// then
	assert.False(t, hub.IsOnBoard(boardID, principalID))
	assert.Empty(t, hub.boards, "an empty board group is dropped")
	assert.Empty(t, hub.memberships, "a principal on no board has no reverse index entry")
}

func TestHub_Unregister_DropsTheLastSocketsBoardsThroughTheReverseIndex(t *testing.T) {
	// given
	hub := NewHub()
	client, _ := newTestClient(t, model.PrincipalUser)
	require.True(t, hub.Register(client))
	firstBoard, secondBoard := uuid.New(), uuid.New()
	hub.JoinBoard(firstBoard, client.principal.ID)
	hub.JoinBoard(secondBoard, client.principal.ID)

	// when
	hub.Unregister(client)

	// then
	assert.False(t, hub.IsOnBoard(firstBoard, client.principal.ID))
	assert.False(t, hub.IsOnBoard(secondBoard, client.principal.ID))
	assert.Empty(t, hub.boards)
	assert.Empty(t, hub.memberships)
	assert.False(t, hub.IsOnline(client.principal.ID))
}

func TestHub_Unregister_KeepsTheBoardsWhileAnotherSocketOfThePrincipalIsOpen(t *testing.T) {
	// given
	hub := NewHub()
	principal := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	first := newClient(principal, nil, func(error) {}, timings{})
	second := newClient(principal, nil, func(error) {}, timings{})
	require.True(t, hub.Register(first))
	require.True(t, hub.Register(second))
	boardID := uuid.New()
	hub.JoinBoard(boardID, principal.ID)

	// when
	hub.Unregister(first)

	// then
	assert.True(t, hub.IsOnBoard(boardID, principal.ID), "the other tab still receives the board")
	assert.True(t, hub.IsOnline(principal.ID))
}

func TestHub_Unregister_IsIdempotent(t *testing.T) {
	// given
	hub := NewHub()
	client, _ := newTestClient(t, model.PrincipalUser)
	require.True(t, hub.Register(client))

	// when
	hub.Unregister(client)
	hub.Unregister(client)

	// then
	assert.Equal(t, 0, hub.OnlineCount())
}

func TestNewClient_GivesAPrincipalTheBoardBufferAndAnAnonymousSocketTheSmallOne(t *testing.T) {
	// given
	user, _ := newTestClient(t, model.PrincipalUser)
	guest, _ := newTestClient(t, guestKind)
	anonymous := newAnonymousTestClient(t)

	// then
	assert.Equal(t, boardSendBufferSize, cap(user.send))
	assert.Equal(t, boardSendBufferSize, cap(guest.send))
	assert.Equal(t, sendBufferSize, cap(anonymous.send))
}

func TestHub_SendToBoardClassified_SendsEachRecipientItsOwnClassPayload(t *testing.T) {
	// given
	hub := NewHub()
	boardID := uuid.New()
	author, _ := newTestClient(t, model.PrincipalUser)
	authorSecondTab := newClient(author.principal, nil, func(error) {}, timings{})
	viewer, _ := newTestClient(t, model.PrincipalUser)
	removed, _ := newTestClient(t, model.PrincipalUser)
	for _, client := range []*Client{author, authorSecondTab, viewer, removed} {
		require.True(t, hub.Register(client))
	}
	hub.JoinBoard(boardID, author.principal.ID)
	hub.JoinBoard(boardID, viewer.principal.ID)
	hub.JoinBoard(boardID, removed.principal.ID)
	payloads := [][]byte{[]byte(`{"class":"author"}`), []byte(`{"class":"others"}`)}
	classify := func(principalID uuid.UUID) int {
		switch principalID {
		case author.principal.ID:
			return 0
		case removed.principal.ID:
			return -1
		default:
			return 1
		}
	}

	// when
	sent := hub.SendToBoardClassified(boardID, classify, payloads)

	// then
	assert.Equal(t, 3, sent)
	assert.Equal(t, []string{`{"class":"author"}`}, drain(author))
	assert.Equal(t, []string{`{"class":"author"}`}, drain(authorSecondTab), "every tab of the author gets the author's payload")
	assert.Equal(t, []string{`{"class":"others"}`}, drain(viewer))
	assert.Empty(t, drain(removed), "a payload index of -1 skips the recipient")
}

func TestHub_SendToBoardClassified_NeverReachesASocketOutsideTheBoard(t *testing.T) {
	// given
	hub := NewHub()
	boardID := uuid.New()
	member, _ := newTestClient(t, model.PrincipalUser)
	outsider, _ := newTestClient(t, model.PrincipalUser)
	anonymous := newAnonymousTestClient(t)
	require.True(t, hub.Register(member))
	require.True(t, hub.Register(outsider))
	require.True(t, hub.Register(anonymous))
	hub.JoinBoard(boardID, member.principal.ID)
	hub.JoinBoard(uuid.New(), outsider.principal.ID)

	// when
	sent := hub.SendToBoardClassified(boardID, func(uuid.UUID) int {
		return 0
	}, [][]byte{[]byte(`{"type":"board_events"}`)})

	// then
	assert.Equal(t, 1, sent)
	assert.Len(t, drain(member), 1)
	assert.Empty(t, drain(outsider), "a member of another board hears nothing of this one")
	assert.Empty(t, drain(anonymous), "an anonymous socket is never on a board")
}

func TestHub_SendToBoardClassified_SkipsAnIndexWithNoPayload(t *testing.T) {
	// given
	hub := NewHub()
	boardID := uuid.New()
	viewer, _ := newTestClient(t, model.PrincipalUser)
	require.True(t, hub.Register(viewer))
	hub.JoinBoard(boardID, viewer.principal.ID)

	// when
	sent := hub.SendToBoardClassified(boardID, func(uuid.UUID) int {
		return 3
	}, [][]byte{[]byte(`{}`)})

	// then
	assert.Equal(t, 0, sent)
	assert.Empty(t, drain(viewer))
}

func TestHub_SendToBoardClassified_KillsAndReapsASlowConsumer(t *testing.T) {
	// given
	hub := NewHub()
	boardID := uuid.New()
	slow, ctx := newTestClient(t, model.PrincipalUser)
	require.True(t, hub.Register(slow))
	hub.JoinBoard(boardID, slow.principal.ID)
	for range cap(slow.send) {
		slow.send <- []byte("backlog")
	}
	killedBefore := testutil.ToFloat64(wsClientsKilled.WithLabelValues(killSlowConsumer))

	// when
	sent := hub.SendToBoardClassified(boardID, func(uuid.UUID) int {
		return 0
	}, [][]byte{[]byte(`{"type":"board_events"}`)})

	// then
	assert.Equal(t, 0, sent)
	assert.ErrorIs(t, context.Cause(ctx), errSlowConsumer, "the connection's context is cancelled")
	assert.True(t, slow.isClosing())
	assert.False(t, hub.IsOnline(slow.principal.ID), "the killed client is reaped")
	assert.False(t, hub.IsOnBoard(boardID, slow.principal.ID))
	assert.Equal(t, killedBefore+1, testutil.ToFloat64(wsClientsKilled.WithLabelValues(killSlowConsumer)))
}

func TestHub_SendToBoardClassified_LeavesAClosingClientToItsOwnHandler(t *testing.T) {
	// given
	hub := NewHub()
	boardID := uuid.New()
	closing, _ := newTestClient(t, model.PrincipalUser)
	require.True(t, hub.Register(closing))
	hub.JoinBoard(boardID, closing.principal.ID)
	closing.requestClose(closeRequest{code: websocket.StatusServiceRestart, flush: true})

	// when
	sent := hub.SendToBoardClassified(boardID, func(uuid.UUID) int {
		return 0
	}, [][]byte{[]byte(`{"type":"board_events"}`)})

	// then
	assert.Equal(t, 0, sent)
	assert.True(t, hub.IsOnline(closing.principal.ID), "a closing client is unregistered by its handler, never reaped")
	assert.True(t, hub.IsOnBoard(boardID, closing.principal.ID))
}

func TestHub_SendToBoardLossy_LeavesHeadroomForBoardEvents(t *testing.T) {
	tests := []struct {
		name       string
		backlog    int
		wantSent   int
		wantBuffer int
	}{
		{name: "an idle buffer takes the frame", backlog: 0, wantSent: 1, wantBuffer: 1},
		{name: "just under the watermark takes the frame", backlog: boardSendBufferSize/2 - 1, wantSent: 1, wantBuffer: boardSendBufferSize / 2},
		{name: "at the watermark the frame is skipped", backlog: boardSendBufferSize / 2, wantSent: 0, wantBuffer: boardSendBufferSize / 2},
		{name: "over the watermark the frame is skipped", backlog: boardSendBufferSize/2 + 5, wantSent: 0, wantBuffer: boardSendBufferSize/2 + 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			hub := NewHub()
			boardID := uuid.New()
			viewer, ctx := newTestClient(t, model.PrincipalUser)
			require.True(t, hub.Register(viewer))
			hub.JoinBoard(boardID, viewer.principal.ID)
			for range tt.backlog {
				viewer.send <- []byte("backlog")
			}

			// when
			sent := hub.SendToBoardLossy(boardID, func(uuid.UUID) int {
				return 0
			}, [][]byte{[]byte(`{"type":"board_presence"}`)})

			// then
			assert.Equal(t, tt.wantSent, sent)
			assert.Len(t, viewer.send, tt.wantBuffer)
			assert.NoError(t, ctx.Err(), "a skipped presence frame never costs the client its connection")
		})
	}
}

func TestHub_Broadcast_ReachesUsersOnly(t *testing.T) {
	// given
	hub := NewHub()
	user, _ := newTestClient(t, model.PrincipalUser)
	guest, _ := newTestClient(t, guestKind)
	anonymous := newAnonymousTestClient(t)
	require.True(t, hub.Register(user))
	require.True(t, hub.Register(guest))
	require.True(t, hub.Register(anonymous))

	// when
	hub.Broadcast(Message{Type: TypeNotification})

	// then
	assert.Len(t, drain(user), 1)
	assert.Empty(t, drain(guest), "a guest never receives a user broadcast")
	assert.Empty(t, drain(anonymous))
}

func TestHub_BroadcastPublic_ReachesEverySocket(t *testing.T) {
	// given
	hub := NewHub()
	user, _ := newTestClient(t, model.PrincipalUser)
	guest, _ := newTestClient(t, guestKind)
	anonymous := newAnonymousTestClient(t)
	require.True(t, hub.Register(user))
	require.True(t, hub.Register(guest))
	require.True(t, hub.Register(anonymous))

	// when
	hub.BroadcastPublic(Message{Type: "site_state"})

	// then
	assert.Len(t, drain(user), 1)
	assert.Len(t, drain(guest), 1)
	assert.Len(t, drain(anonymous), 1)
}

func TestHub_SendToUser_NeverReachesAGuest(t *testing.T) {
	// given
	hub := NewHub()
	guest, _ := newTestClient(t, guestKind)
	require.True(t, hub.Register(guest))

	// when
	hub.SendToUser(guest.principal.ID, Message{Type: TypeNotification})

	// then
	assert.Empty(t, drain(guest))
}

func TestHub_SendToPrincipal_ReachesEveryTabOfThePrincipal(t *testing.T) {
	// given
	hub := NewHub()
	principal := model.Principal{Kind: guestKind, ID: uuid.New()}
	first := newClient(principal, nil, func(error) {}, timings{})
	second := newClient(principal, nil, func(error) {}, timings{})
	require.True(t, hub.Register(first))
	require.True(t, hub.Register(second))

	// when
	hub.SendToPrincipal(principal.ID, Message{Type: TypeBoardMe, Data: map[string]int{"votes_remaining": 3}})

	// then
	assert.Equal(t, []string{`{"type":"board_me","data":{"votes_remaining":3}}`}, drain(first))
	assert.Equal(t, []string{`{"type":"board_me","data":{"votes_remaining":3}}`}, drain(second))
}

func TestHub_DisconnectUser_AsksEverySocketOfTheUserToClose(t *testing.T) {
	// given
	hub := NewHub()
	principal := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	first := newClient(principal, nil, func(error) {}, timings{})
	second := newClient(principal, nil, func(error) {}, timings{})
	bystander, _ := newTestClient(t, model.PrincipalUser)
	for _, client := range []*Client{first, second, bystander} {
		require.True(t, hub.Register(client))
	}

	// when
	closed := hub.DisconnectUser(principal.ID)

	// then
	assert.Equal(t, 2, closed)
	assert.True(t, first.isClosing())
	assert.True(t, second.isClosing())
	assert.Equal(t, websocket.StatusPolicyViolation, first.closeWith.code)
	assert.False(t, bystander.isClosing())
}

func TestHub_Shutdown_ReturnsAtOnceWithNoSockets(t *testing.T) {
	// given
	hub := NewHub()

	// when
	err := hub.Shutdown(t.Context())

	// then
	require.NoError(t, err)
}

func TestHub_Shutdown_RefusesNewSockets(t *testing.T) {
	// given
	hub := NewHub()
	require.NoError(t, hub.Shutdown(t.Context()))
	client, _ := newTestClient(t, model.PrincipalUser)

	// when
	registered := hub.Register(client)

	// then
	assert.False(t, registered)
}

func TestHub_Shutdown_AsksEverySocketToCloseWithServiceRestart(t *testing.T) {
	// given
	hub := NewHub()
	client, _ := newTestClient(t, model.PrincipalUser)
	require.True(t, hub.Register(client))
	done := make(chan error, 1)

	// when
	go func() {
		done <- hub.Shutdown(t.Context())
	}()
	<-client.closing
	hub.Unregister(client)

	// then
	require.NoError(t, <-done)
	assert.Equal(t, closeRequest{code: websocket.StatusServiceRestart, reason: closeReasonRestart, flush: true}, client.closeWith)
}

func TestHub_Shutdown_DropsASocketThatOutlastsTheDrain(t *testing.T) {
	// given
	hub := NewHub()
	stuck, ctx := newTestClient(t, model.PrincipalUser)
	require.True(t, hub.Register(stuck))
	drainCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	// when
	err := hub.Shutdown(drainCtx)

	// then
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorIs(t, context.Cause(ctx), errShutdownExpired, "the stuck socket's context is cancelled")
}
