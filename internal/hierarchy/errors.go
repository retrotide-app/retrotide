package hierarchy

import "errors"

var (
	ErrOutranked = errors.New("nobody may act on someone at their own level or above")
	ErrNotMember = errors.New("not a member of the organisation")
)
