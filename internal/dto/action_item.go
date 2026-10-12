package dto

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	ActionItemFieldsRequest struct {
		BoardCommandRequest

		Title          string      `json:"title"`
		AssigneeUserID *uuid.UUID  `json:"assignee_user_id"`
		DueOn          *string     `json:"due_on"`
		Mentions       []uuid.UUID `json:"mentions"`
	}

	CreateActionItemRequest struct {
		ActionItemFieldsRequest

		ID           uuid.UUID  `json:"id"`
		SourceCardID *uuid.UUID `json:"source_card_id"`
	}

	ActionItemStatusRequest struct {
		BoardCommandRequest

		Status model.ActionItemStatus `json:"status"`
	}

	ReviewActionItemRequest struct {
		BoardCommandRequest

		Outcome model.ReviewOutcome `json:"outcome"`
	}

	PersonView struct {
		UserID      uuid.UUID `json:"user_id"`
		DisplayName string    `json:"display_name"`
	}

	ActionItemView struct {
		ID           uuid.UUID              `json:"id"`
		Title        string                 `json:"title"`
		Assignee     *PersonView            `json:"assignee"`
		Mentions     []PersonView           `json:"mentions"`
		DueOn        *string                `json:"due_on"`
		Status       model.ActionItemStatus `json:"status"`
		SourceCardID *uuid.UUID             `json:"source_card_id"`
		IsMine       bool                   `json:"is_mine,omitzero"`
	}

	PreviousActionView struct {
		ActionItemView

		Review *model.ReviewOutcome `json:"review"`
	}

	ActionItemUpsertedEvent struct {
		BoardEventHead

		ActionItem ActionItemView `json:"action_item"`
	}

	ActionItemRemovedEvent struct {
		BoardEventHead

		ActionItemID uuid.UUID `json:"action_item_id"`
	}

	PreviousActionUpsertedEvent struct {
		BoardEventHead

		PreviousAction PreviousActionView `json:"previous_action"`
	}

	BoardPeopleResponse struct {
		People []PersonView `json:"people"`
	}

	ActionItemBoardView struct {
		ID    uuid.UUID `json:"id"`
		Title string    `json:"title"`
	}

	ActionItemListItem struct {
		ActionItemView

		Board    *ActionItemBoardView `json:"board"`
		TeamID   *uuid.UUID           `json:"team_id,omitempty"`
		TeamName string               `json:"team_name,omitempty"`
	}

	ActionItemCountsView struct {
		Open    int `json:"open"`
		Done    int `json:"done"`
		Dropped int `json:"dropped"`
	}

	ActionItemListResponse struct {
		ActionItems    []ActionItemListItem `json:"action_items"`
		Total          int                  `json:"total"`
		Limit          int                  `json:"limit"`
		Offset         int                  `json:"offset"`
		Counts         ActionItemCountsView `json:"counts"`
		CompletionRate *float64             `json:"completion_rate"`
	}

	ActionItemMentionView struct {
		ActionItemID uuid.UUID              `json:"action_item_id"`
		Title        string                 `json:"title"`
		Status       model.ActionItemStatus `json:"status"`
		DueOn        *string                `json:"due_on"`
	}

	TextMentionView struct {
		CardID    uuid.UUID  `json:"card_id"`
		CommentID *uuid.UUID `json:"comment_id,omitempty"`
		Body      string     `json:"body"`
	}

	MentionView struct {
		Kind model.MentionKind `json:"kind"`

		*ActionItemMentionView
		*TextMentionView

		OrgID      uuid.UUID  `json:"org_id"`
		OrgSlug    string     `json:"org_slug"`
		OrgName    string     `json:"org_name"`
		TeamID     *uuid.UUID `json:"team_id,omitempty"`
		BoardID    *uuid.UUID `json:"board_id,omitempty"`
		BoardTitle string     `json:"board_title,omitempty"`
	}

	MentionListResponse struct {
		Mentions []MentionView `json:"mentions"`
	}
)

func (r CreateActionItemRequest) ClientIDs() []uuid.UUID {
	return []uuid.UUID{r.ID}
}
