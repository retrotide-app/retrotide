package dao

import "errors"

var (
	ErrNotFound              = errors.New("not found")
	ErrConflict              = errors.New("conflict")
	ErrTxRequired            = errors.New("needs the caller's transaction")
	ErrLastSignInMethod      = errors.New("it is the account's last way to sign in")
	ErrQuotaExceeded         = errors.New("the quota is already used")
	ErrVersionConflict       = errors.New("the board's version is no longer the one read")
	ErrDuplicateOp           = errors.New("the operation was already applied")
	ErrOnePrincipal          = errors.New("a board participant belongs to exactly one user or one guest device")
	ErrVoteBudgetSpent       = errors.New("the participant has spent every vote the board allows")
	ErrCardVoteCapReached    = errors.New("the participant already holds as many votes on the card as the board allows")
	ErrStageMoved            = errors.New("the board is no longer open in the stage the change started from")
	ErrTimerMoved            = errors.New("the board's timer no longer ends when the lock was scheduled, or the board can no longer lock")
	ErrColumnVoteBudgetSpent = errors.New("the participant has spent every vote the column allows")
)
