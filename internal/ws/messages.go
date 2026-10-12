package ws

import (
	"encoding/json"
	"fmt"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	Message struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}

	BoardRef struct {
		BoardID uuid.UUID `json:"board_id"`
	}

	ViewerState string

	RejectReason string

	BoardJoined struct {
		BoardRef

		Version       int64           `json:"version"`
		ParticipantID *uuid.UUID      `json:"participant_id"`
		Role          model.BoardRole `json:"role"`
	}

	BoardJoinRejected struct {
		BoardRef

		Reason RejectReason `json:"reason"`
	}

	BoardAccessRevoked struct {
		BoardRef

		Reason RejectReason `json:"reason"`
	}

	ViewerStateChange struct {
		BoardRef

		State ViewerState `json:"state"`
	}

	TeamActionItemsChanged struct {
		TeamID uuid.UUID `json:"team_id"`
	}

	incomingMessage struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	pingData struct {
		Nonce *int64 `json:"nonce,omitempty"`
		RTT   int    `json:"rtt,omitempty"`
	}

	pongData struct {
		Nonce     *int64 `json:"nonce,omitempty"`
		ServerNow string `json:"server_now"`
	}

	errorData struct {
		Error string `json:"error"`
	}
)

const (
	TypePing             = "ping"
	TypeBoardJoin        = "board_join"
	TypeBoardLeave       = "board_leave"
	TypeBoardViewerState = "board_viewer_state"

	TypePong                   = "pong"
	TypeError                  = "error"
	TypeNotification           = "notification"
	TypeBoardJoined            = "board_joined"
	TypeBoardJoinRejected      = "board_join_rejected"
	TypeBoardAccessRevoked     = "board_access_revoked"
	TypeBoardPresence          = "board_presence"
	TypeBoardEvents            = "board_events"
	TypeBoardMe                = "board_me"
	TypeTeamActionItemsChanged = "team_action_items_changed"
	TypeOrgEntitlementsChanged = "org_entitlements_changed"
)

const (
	ViewerStateActive ViewerState = "active"
	ViewerStateIdle   ViewerState = "idle"
)

const (
	RejectNotParticipant   RejectReason = "not_participant"
	RejectPasswordRequired RejectReason = "password_required"
	RejectAccountRequired  RejectReason = "account_required"
	RejectLinkDisabled     RejectReason = "link_disabled"
	RejectRemoved          RejectReason = "removed"
	RejectArchived         RejectReason = "archived"
)

const (
	serverNowLayout = "2006-01-02T15:04:05.000Z"
)

var (
	viewerStates = map[ViewerState]struct{}{
		ViewerStateActive: {},
		ViewerStateIdle:   {},
	}
)

func Encode(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode the %s frame: %w", msg.Type, err)
	}

	return data, nil
}

func BoardJoinedMessage(joined BoardJoined) Message {
	return Message{Type: TypeBoardJoined, Data: joined}
}

func BoardJoinRejectedMessage(rejected BoardJoinRejected) Message {
	return Message{Type: TypeBoardJoinRejected, Data: rejected}
}

func BoardAccessRevokedMessage(revoked BoardAccessRevoked) Message {
	return Message{Type: TypeBoardAccessRevoked, Data: revoked}
}

func (r BoardRef) valid() bool {
	return r.BoardID != uuid.Nil
}

func (s ViewerState) valid() bool {
	_, known := viewerStates[s]

	return known
}
