package policy

import "errors"

var (
	ErrNotAllowed         = errors.New("your role on this board does not allow that")
	ErrReadOnly           = errors.New("the board is read-only")
	ErrCardsLocked        = errors.New("cards are locked on this board")
	ErrMovesLocked        = errors.New("moving cards is locked on this board")
	ErrCardNotFound       = errors.New("card not found")
	ErrNotParticipant     = errors.New("join the board before writing on it")
	ErrVotingClosed       = errors.New("voting is closed on this board")
	ErrCommentsDisabled   = errors.New("comments are switched off on this board")
	ErrCommentNotFound    = errors.New("comment not found")
	ErrMergingDisabled    = errors.New("merging cards is switched off on this board")
	ErrCardsHidden        = errors.New("cards are hidden, so they cannot be merged or unmerged until they are revealed")
	ErrMergeIntoSelf      = errors.New("a card cannot be merged into itself")
	ErrNotMerged          = errors.New("the card is not part of a merged stack")
	ErrCardsOutOfStage    = errors.New("cards can be added, edited, moved and deleted only in the Write stage")
	ErrVotesOutOfStage    = errors.New("votes can be cast or taken back only in the Vote stage")
	ErrCommentsOutOfStage = errors.New("comments can be written, edited and deleted only in the Discuss stage")
	ErrMergesOutOfStage   = errors.New("cards can be merged or unmerged only in the Discuss stage")
	ErrHideOutOfStage     = errors.New("cards can be hidden only in the Write stage")

	ErrActionItemsOutOfStage = errors.New("action items can be added, edited and marked only in the Actions stage")
	ErrActionItemNotFound    = errors.New("action item not found")
	ErrMembersOnly           = errors.New("only members of the board's organisation can do that")

	ErrColumnHidden     = errors.New("the column is hidden from participants, so nothing in it can change until it is shown")
	ErrMoveAcrossReveal = errors.New("a card cannot move between a revealed column and one whose cards are still hidden")
	ErrCardNotShown     = errors.New("only a card everyone can see can be presented or highlighted")

	ErrCardNotPublic  = errors.New("only a card everyone can see can be reacted to or pinned")
	ErrSurveyClosed   = errors.New("the survey is closed, so it takes no more answers")
	ErrSurveyNotFound = errors.New("survey not found")
)
