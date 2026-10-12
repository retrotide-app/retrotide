package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTx_WithTx_HandsFnTheInboundTransaction(t *testing.T) {
	// given
	inbound := new(sql.Tx)
	var received *sql.Tx

	// when
	err := WithTx(t.Context(), nil, []*sql.Tx{inbound}, func(tx *sql.Tx) error {
		received = tx

		return nil
	})

	// then
	require.NoError(t, err)
	assert.Same(t, inbound, received)
}

func TestTx_WithReadTx_RefusesAnInboundTransaction(t *testing.T) {
	// given
	called := false

	// when
	err := WithReadTx(t.Context(), nil, []*sql.Tx{new(sql.Tx)}, func(*sql.Tx) error {
		called = true

		return nil
	})

	// then
	require.ErrorIs(t, err, ErrInboundTx)
	assert.False(t, called)
}

func TestTxOrDB_PrefersTheTransaction(t *testing.T) {
	// given
	database := new(sql.DB)
	inbound := new(sql.Tx)

	// when
	handle := TxOrDB(database, []*sql.Tx{inbound})

	// then
	assert.Same(t, inbound, handle)
}

func TestTxOrDB_FallsBackToThePool(t *testing.T) {
	// given
	database := new(sql.DB)

	// when
	handle := TxOrDB(database, nil)

	// then
	assert.Same(t, database, handle)
}

func TestTx_Joined_IgnoresANilEntry(t *testing.T) {
	// given
	tx := []*sql.Tx{nil}

	// when
	joined := Joined(tx)

	// then
	assert.Nil(t, joined)
}
