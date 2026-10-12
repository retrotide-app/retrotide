package ws

import "errors"

var (
	errSlowConsumer    = errors.New("the client's send buffer is full")
	errWriterStopped   = errors.New("the socket's write loop stopped")
	errPongTimeout     = errors.New("the pong never arrived")
	errShutdownExpired = errors.New("the socket outlasted the shutdown drain")
	errConnectionEnded = errors.New("the connection ended")

	errMissingHub      = errors.New("the socket handler needs the hub")
	errMissingSettings = errors.New("the socket handler needs the settings reader")
	errMissingUsers    = errors.New("the socket handler needs the user session validator")
	errMissingBans     = errors.New("the socket handler needs the ban checker")
	errMissingBoards   = errors.New("the socket handler needs the board presence service")

	errOriginRefused      = errors.New("the upgrade carried an empty or foreign Origin")
	errBanned             = errors.New("the account is banned")
	errPrincipalChanged   = errors.New("the credential now belongs to another principal")
	errRevalidationFailed = errors.New("the socket's access no longer holds")
)
