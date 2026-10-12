package ws

import (
	"encoding/json"
	"fmt"
	"testing"

	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncode_FlattensTheBoardRefIntoEachBoardFrame(t *testing.T) {
	boardID := uuid.New()
	participantID := uuid.New()
	ref := BoardRef{BoardID: boardID}

	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{
			name: "a join answers with the version, the participant and the role",
			msg:  BoardJoinedMessage(BoardJoined{BoardRef: ref, Version: 1041, ParticipantID: &participantID, Role: model.BoardRoleParticipant}),
			want: fmt.Sprintf(`{"type":"board_joined","data":{"board_id":%q,"version":1041,"participant_id":%q,"role":"participant"}}`, boardID, participantID),
		},
		{
			name: "a member who has not joined as a participant carries a null participant",
			msg:  BoardJoinedMessage(BoardJoined{BoardRef: ref, Version: 7, Role: model.BoardRoleFacilitator}),
			want: fmt.Sprintf(`{"type":"board_joined","data":{"board_id":%q,"version":7,"participant_id":null,"role":"facilitator"}}`, boardID),
		},
		{
			name: "a refused join names only the board and the reason",
			msg:  BoardJoinRejectedMessage(BoardJoinRejected{BoardRef: ref, Reason: RejectPasswordRequired}),
			want: fmt.Sprintf(`{"type":"board_join_rejected","data":{"board_id":%q,"reason":"password_required"}}`, boardID),
		},
		{
			name: "a revoked board names only the board and the reason",
			msg:  BoardAccessRevokedMessage(BoardAccessRevoked{BoardRef: ref, Reason: RejectRemoved}),
			want: fmt.Sprintf(`{"type":"board_access_revoked","data":{"board_id":%q,"reason":"removed"}}`, boardID),
		},
		{
			name: "a pong without a nonce carries only the server clock",
			msg:  Message{Type: TypePong, Data: pongData{ServerNow: "2026-09-22T10:06:02.512Z"}},
			want: `{"type":"pong","data":{"server_now":"2026-09-22T10:06:02.512Z"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, err := Encode(tt.msg)

			// then
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(got))
		})
	}
}

func TestEncode_ReportsAFrameThatCannotBeEncoded(t *testing.T) {
	// given
	msg := Message{Type: TypeBoardEvents, Data: make(chan int)}

	// when
	_, err := Encode(msg)

	// then
	var unsupported *json.UnsupportedTypeError
	require.ErrorAs(t, err, &unsupported)
	assert.Contains(t, err.Error(), TypeBoardEvents)
}

func TestBoardRef_Valid(t *testing.T) {
	tests := []struct {
		name string
		ref  BoardRef
		want bool
	}{
		{name: "a board", ref: BoardRef{BoardID: uuid.New()}, want: true},
		{name: "no board id", ref: BoardRef{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := tt.ref.valid()

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestViewerState_Valid(t *testing.T) {
	tests := []struct {
		state ViewerState
		want  bool
	}{
		{state: ViewerStateActive, want: true},
		{state: ViewerStateIdle, want: true},
		{state: "typing", want: false},
		{state: "", want: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			// when
			got := tt.state.valid()

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}
