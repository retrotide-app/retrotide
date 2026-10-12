package ws

import (
	"testing"

	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestInboundLabel_KeepsTheLabelSetBounded(t *testing.T) {
	tests := []struct {
		msgType string
		want    string
	}{
		{msgType: TypePing, want: TypePing},
		{msgType: TypeBoardJoin, want: TypeBoardJoin},
		{msgType: TypeBoardLeave, want: TypeBoardLeave},
		{msgType: TypeBoardViewerState, want: TypeBoardViewerState},
		{msgType: "card_text_typed_by_a_client", want: unknownMessageType},
		{msgType: "", want: unknownMessageType},
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			// when
			got := inboundLabel(tt.msgType)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		name      string
		principal model.Principal
		want      string
	}{
		{name: "a user", principal: model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}, want: "user"},
		{name: "a guest", principal: model.Principal{Kind: guestKind, ID: uuid.New()}, want: "guest"},
		{name: "no principal", principal: model.Principal{}, want: anonymousKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got := kindOf(tt.principal)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHub_Register_CountsOpenSocketsByKind(t *testing.T) {
	// given
	hub := NewHub()
	client, _ := newTestClient(t, model.PrincipalUser)
	openBefore := testutil.ToFloat64(wsConnections.WithLabelValues("user"))
	totalBefore := testutil.ToFloat64(wsConnectionsTotal.WithLabelValues("user"))

	// when
	hub.Register(client)
	openWhileRegistered := testutil.ToFloat64(wsConnections.WithLabelValues("user"))
	hub.Unregister(client)

	// then
	assert.Equal(t, openBefore+1, openWhileRegistered)
	assert.Equal(t, openBefore, testutil.ToFloat64(wsConnections.WithLabelValues("user")))
	assert.Equal(t, totalBefore+1, testutil.ToFloat64(wsConnectionsTotal.WithLabelValues("user")))
}

func TestRecordInbound_CountsTheFrameByLabel(t *testing.T) {
	// given
	before := testutil.ToFloat64(wsInboundMessages.WithLabelValues(TypeBoardJoin))

	// when
	recordInbound(TypeBoardJoin, 12)

	// then
	assert.Equal(t, before+1, testutil.ToFloat64(wsInboundMessages.WithLabelValues(TypeBoardJoin)))
}
