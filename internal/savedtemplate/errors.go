package savedtemplate

import "errors"

var (
	ErrNameRequired         = errors.New("a template name is required")
	ErrNameTooLong          = errors.New("a template name can be at most 80 characters")
	ErrDescriptionTooLong   = errors.New("a template description can be at most 300 characters")
	ErrInvalidScope         = errors.New("a template is saved for yourself or for the board's team")
	ErrNotTeamBoard         = errors.New("only a team board can be saved as a team template")
	ErrNoColumns            = errors.New("add a column to the board before saving it as a template")
	ErrNotAllowed           = errors.New("you cannot save this board as a template")
	ErrDeleteNotAllowed     = errors.New("you cannot delete this template")
	ErrTemplateNotFound     = errors.New("template not found")
	ErrOrganisationNotFound = errors.New("organisation not found")
)
