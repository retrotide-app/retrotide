package actionitems

import (
	"errors"
)

var (
	ErrTitleRequired        = errors.New("an action item needs a title")
	ErrTitleTooLong         = errors.New("an action item title can be at most 500 characters")
	ErrInvalidDueDate       = errors.New("a due date must be a calendar date written as YYYY-MM-DD")
	ErrInvalidStatus        = errors.New("an action item's status must be open, done or dropped")
	ErrInvalidReviewOutcome = errors.New("a review must mark the action done, carried or dropped")
	ErrCardRequired         = errors.New("an action item is made from a card")
)
