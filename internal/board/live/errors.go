package live

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type (
	RetryError struct {
		err        error
		RetryAfter time.Duration
	}
)

var (
	ErrGuestsNotAllowed    = errors.New("this organisation does not let guests join its boards")
	ErrWrongPassword       = errors.New("that is not the board's password")
	ErrPasswordAttempts    = errors.New("too many wrong passwords for this board, so try again later")
	ErrBoardFull           = errors.New("the board already holds as many participants as it can")
	ErrBusy                = errors.New("the board is busy, so try again in a moment")
	ErrNoAccess            = errors.New("board not found")
	ErrRemoved             = errors.New("you were removed from this board")
	ErrLinkDisabled        = errors.New("the board's link is switched off")
	ErrPasswordRequired    = errors.New("the board needs its password")
	ErrAccountRequired     = errors.New("the board needs you to sign in")
	ErrArchived            = errors.New("the board is archived")
	ErrParticipantUnlisted = errors.New("the participant who just joined is missing from the board's list")

	errDraining      = errors.New("the live board service is draining for shutdown")
	errNoGuestIssuer = errors.New("the live board service has no guest issuer, so it cannot let a visitor in as a guest")
)

func NewPasswordAttemptsError(boardID uuid.UUID, retryAfter time.Duration) *RetryError {
	return &RetryError{err: fmt.Errorf("%w: board %s", ErrPasswordAttempts, boardID), RetryAfter: retryAfter}
}

func (e *RetryError) Error() string {
	return e.err.Error()
}

func (e *RetryError) Unwrap() error {
	return e.err
}
