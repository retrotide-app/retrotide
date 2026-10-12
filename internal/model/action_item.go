package model

import (
	"time"

	"github.com/google/uuid"
)

type (
	ActionItemStatus string

	ReviewOutcome string

	ActionItemPerson struct {
		UserID      uuid.UUID
		DisplayName string
	}

	ActionItem struct {
		ID             uuid.UUID
		OrgID          uuid.UUID
		TeamID         *uuid.UUID
		BoardID        *uuid.UUID
		SourceCardID   *uuid.UUID
		Title          string
		AssigneeUserID *uuid.UUID
		AssigneeName   string
		DueOn          *time.Time
		Status         ActionItemStatus
		CompletedAt    *time.Time
		CreatedBy      *uuid.UUID
		CreatedAt      time.Time
		UpdatedAt      time.Time
		Mentions       []ActionItemPerson
	}

	PreviousAction struct {
		ActionItem

		Review *ReviewOutcome
	}

	ActionItemListing struct {
		ActionItem

		BoardTitle     string
		BoardAnonymity Anonymity
		TeamName       string
	}

	ActionItemCounts struct {
		Open    int
		Done    int
		Dropped int
	}

	ActionItemPage struct {
		Items  []ActionItemListing
		Counts ActionItemCounts
	}

	ActionItemReviewed struct {
		Item             PreviousAction
		ItemBoardVersion *int64
	}

	MentionKind string

	Mention struct {
		Kind         MentionKind
		ActionItemID uuid.UUID
		CardID       uuid.UUID
		CommentID    uuid.UUID
		OrgID        uuid.UUID
		OrgSlug      string
		OrgName      string
		TeamID       *uuid.UUID
		BoardID      *uuid.UUID
		BoardTitle   string
		Title        string
		Status       ActionItemStatus
		DueOn        *time.Time
		Body         string
		CardsHidden  bool
		OwnCard      bool
		MentionedAt  time.Time
	}
)

const (
	MentionActionItem MentionKind = "action_item"
	MentionCard       MentionKind = "card"
	MentionComment    MentionKind = "comment"
)

const (
	ActionItemOpen    ActionItemStatus = "open"
	ActionItemDone    ActionItemStatus = "done"
	ActionItemDropped ActionItemStatus = "dropped"
)

const (
	ReviewDone    ReviewOutcome = "done"
	ReviewCarried ReviewOutcome = "carried"
	ReviewDropped ReviewOutcome = "dropped"
)

const (
	ActionItemTitleMaxRunes = 500
	ActionItemMaxMentions   = 20
	MaxMentions             = ActionItemMaxMentions
	DueDateLayout           = "2006-01-02"
)

var (
	ActionItemStatuses = []ActionItemStatus{ActionItemOpen, ActionItemDone, ActionItemDropped}

	ReviewOutcomes = []ReviewOutcome{ReviewDone, ReviewCarried, ReviewDropped}
)

func (o ReviewOutcome) Status() ActionItemStatus {
	switch o {
	case ReviewDone:
		return ActionItemDone
	case ReviewDropped:
		return ActionItemDropped
	default:
		return ActionItemOpen
	}
}

func (c ActionItemCounts) Of(status ActionItemStatus) int {
	switch status {
	case ActionItemOpen:
		return c.Open
	case ActionItemDone:
		return c.Done
	case ActionItemDropped:
		return c.Dropped
	default:
		return 0
	}
}

func (c ActionItemCounts) Total() int {
	return c.Open + c.Done + c.Dropped
}

func (c ActionItemCounts) CompletionRate() *float64 {
	counted := c.Open + c.Done
	if counted == 0 {
		return nil
	}

	return new(float64(c.Done) / float64(counted))
}
