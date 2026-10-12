package db

import "errors"

var (
	ErrInboundTx = errors.New("inbound transaction not allowed")
)
