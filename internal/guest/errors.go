package guest

import "errors"

var (
	ErrInvalidGuest   = errors.New("invalid or expired guest device")
	ErrNotAudited     = errors.New("the boards were claimed, but their audit entries were not recorded")
	ErrNothingClaimed = errors.New("choose at least one board to claim")
)
