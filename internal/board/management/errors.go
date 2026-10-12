package management

import (
	"errors"
)

var (
	ErrSameTeam          = errors.New("the board is already there")
	ErrMoveNotAllowed    = errors.New("you cannot move a board there")
	ErrTeamNotFound      = errors.New("team not found")
	ErrTeamArchived      = errors.New("the team is archived, so it can no longer hold new boards")
	ErrCSVEmpty          = errors.New("the CSV holds no cards to import")
	ErrCSVInvalid        = errors.New("the CSV could not be read")
	ErrCSVTooManyCards   = errors.New("a CSV import can add at most 500 cards")
	ErrCSVTooManyColumns = errors.New("a board can have at most 20 columns")
	ErrRestoreNotAllowed = errors.New("you cannot restore that board")
	ErrBoardNotFound     = errors.New("board not found")
	ErrTooManyBoards     = errors.New("a bulk action takes at most 100 boards")
	ErrBoardsRequired    = errors.New("a bulk action needs at least one board")
	ErrInvalidBulkAction = errors.New("a bulk action must be archive, unarchive or delete")
	ErrNotAudited        = errors.New("the board changed, but its audit entry was not recorded")
)
