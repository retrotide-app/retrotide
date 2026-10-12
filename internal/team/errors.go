package team

import "errors"

var (
	ErrTeamNotFound         = errors.New("team not found")
	ErrOrganisationNotFound = errors.New("organisation not found")
	ErrMemberNotFound       = errors.New("that person is not a member of the organisation")
	ErrTeamMemberNotFound   = errors.New("that person is not a member of the team")
	ErrAlreadyTeamMember    = errors.New("that person is already a member of the team")
	ErrInvalidRole          = errors.New("the role must be lead or member")
	ErrNameRequired         = errors.New("a team name is required")
	ErrNameTooLong          = errors.New("a team name can be at most 80 characters")
	ErrNameTaken            = errors.New("the organisation already has a team with that name")
	ErrTeamArchived         = errors.New("the team is archived, so it can no longer be changed")
	ErrNotAudited           = errors.New("the change was made, but its audit entry was not recorded")
	ErrCacheStale           = errors.New("the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply")
)
