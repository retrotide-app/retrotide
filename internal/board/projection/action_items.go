package projection

import (
	"slices"
	"time"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

const (
	EventActionItemUpserted     = "action_item_upserted"
	EventActionItemRemoved      = "action_item_removed"
	EventPreviousActionUpserted = "previous_action_upserted"
)

func ActionItemResponse(board model.Board, item model.ActionItem, viewer Viewer, version int64) dto.BoardCommandResponse {
	view := actionItemView(item, sourceShown(board))
	view.IsMine = OwnsActionItem(board, item, viewer)

	return dto.BoardCommandResponse{Version: version, ActionItem: &view}
}

func ReplayActionItemResponse(board model.Board, item model.ActionItem, viewer Viewer, version int64) dto.BoardCommandResponse {
	response := ReplayResponse(version, new(item.ID))
	view := actionItemView(item, sourceShown(board))
	view.IsMine = OwnsActionItem(board, item, viewer)
	response.ActionItem = &view

	return response
}

func OwnsActionItem(board model.Board, item model.ActionItem, viewer Viewer) bool {
	if board.Anonymous() || item.CreatedBy == nil || viewer.UserID == nil {
		return false
	}

	return *item.CreatedBy == *viewer.UserID
}

func MentionShown(mention model.Mention) bool {
	return !mention.CardsHidden || mention.OwnCard
}

func PreviousActionResponse(action model.PreviousAction, version int64) dto.BoardCommandResponse {
	view := previousActionView(action)

	return dto.BoardCommandResponse{Version: version, PreviousAction: &view}
}

func ActionItemUpserted(board model.Board, item model.ActionItem, version int64) Event {
	view := actionItemView(item, sourceShown(board))

	event := everyone(dto.ActionItemUpsertedEvent{BoardEventHead: head(EventActionItemUpserted, version), ActionItem: view}, version)

	event.forViewer = func(viewer Viewer) any {
		if !OwnsActionItem(board, item, viewer) {
			return nil
		}

		mine := view
		mine.IsMine = true
		mine.Mentions = slices.Clone(view.Mentions)

		return dto.ActionItemUpsertedEvent{BoardEventHead: head(EventActionItemUpserted, version), ActionItem: mine}
	}

	return event
}

func ActionItemRemoved(itemID uuid.UUID, version int64) Event {
	payload := dto.ActionItemRemovedEvent{
		BoardEventHead: head(EventActionItemRemoved, version),
		ActionItemID:   itemID,
	}

	return everyone(payload, version)
}

func PreviousActionUpserted(action model.PreviousAction, version int64) Event {
	payload := dto.PreviousActionUpsertedEvent{
		BoardEventHead: head(EventPreviousActionUpserted, version),
		PreviousAction: previousActionView(action),
	}

	return everyone(payload, version)
}

func People(people []model.ActionItemPerson) dto.BoardPeopleResponse {
	return dto.BoardPeopleResponse{People: personViews(people)}
}

func ActionItemList(page model.ActionItemPage, limit, offset, total int) dto.ActionItemListResponse {
	items := make([]dto.ActionItemListItem, 0, len(page.Items))
	for _, listing := range page.Items {
		item := dto.ActionItemListItem{
			ActionItemView: actionItemView(listing.ActionItem, noSource),
			TeamName:       listing.TeamName,
		}

		if listing.TeamID != nil {
			item.TeamID = new(*listing.TeamID)
		}

		if listing.BoardID != nil {
			item.Board = &dto.ActionItemBoardView{ID: *listing.BoardID, Title: listing.BoardTitle}
		}

		items = append(items, item)
	}

	return dto.ActionItemListResponse{
		ActionItems: items,
		Total:       total,
		Limit:       limit,
		Offset:      offset,
		Counts: dto.ActionItemCountsView{
			Open:    page.Counts.Open,
			Done:    page.Counts.Done,
			Dropped: page.Counts.Dropped,
		},
		CompletionRate: page.Counts.CompletionRate(),
	}
}

func Mentions(mentions []model.Mention) dto.MentionListResponse {
	views := make([]dto.MentionView, 0, len(mentions))
	for _, mention := range mentions {
		if !MentionShown(mention) {
			continue
		}

		view := dto.MentionView{
			Kind:    model.MentionActionItem,
			OrgID:   mention.OrgID,
			OrgSlug: mention.OrgSlug,
			OrgName: mention.OrgName,
		}

		switch mention.Kind {
		case model.MentionCard:
			view.Kind = model.MentionCard
			view.TextMentionView = &dto.TextMentionView{CardID: mention.CardID, Body: mention.Body}
		case model.MentionComment:
			view.Kind = model.MentionComment
			view.TextMentionView = &dto.TextMentionView{CardID: mention.CardID, CommentID: new(mention.CommentID), Body: mention.Body}
		default:
			view.ActionItemMentionView = &dto.ActionItemMentionView{
				ActionItemID: mention.ActionItemID,
				Title:        mention.Title,
				Status:       mention.Status,
				DueOn:        dueOn(mention.DueOn),
			}
		}

		if mention.TeamID != nil {
			view.TeamID = new(*mention.TeamID)
		}

		if mention.BoardID != nil {
			view.BoardID = new(*mention.BoardID)
			view.BoardTitle = mention.BoardTitle
		}

		views = append(views, view)
	}

	return dto.MentionListResponse{Mentions: views}
}

func actionItemViews(board model.Board, items []model.ActionItem, shown func(uuid.UUID) bool, viewer Viewer) []dto.ActionItemView {
	views := make([]dto.ActionItemView, 0, len(items))
	for _, item := range items {
		view := actionItemView(item, shown)
		view.IsMine = OwnsActionItem(board, item, viewer)

		views = append(views, view)
	}

	return views
}

func previousActionViews(actions []model.PreviousAction) []dto.PreviousActionView {
	views := make([]dto.PreviousActionView, 0, len(actions))
	for _, action := range actions {
		views = append(views, previousActionView(action))
	}

	return views
}

func actionItemView(item model.ActionItem, shown func(uuid.UUID) bool) dto.ActionItemView {
	view := dto.ActionItemView{
		ID:       item.ID,
		Title:    item.Title,
		Mentions: personViews(item.Mentions),
		DueOn:    dueOn(item.DueOn),
		Status:   item.Status,
	}

	if item.AssigneeUserID != nil {
		view.Assignee = &dto.PersonView{UserID: *item.AssigneeUserID, DisplayName: item.AssigneeName}
	}

	if item.SourceCardID != nil && shown(*item.SourceCardID) {
		view.SourceCardID = new(*item.SourceCardID)
	}

	return view
}

func previousActionView(action model.PreviousAction) dto.PreviousActionView {
	view := dto.PreviousActionView{ActionItemView: actionItemView(action.ActionItem, noSource)}

	if action.Review != nil {
		view.Review = new(*action.Review)
	}

	return view
}

func personViews(people []model.ActionItemPerson) []dto.PersonView {
	views := make([]dto.PersonView, 0, len(people))
	for _, person := range people {
		views = append(views, dto.PersonView{UserID: person.UserID, DisplayName: person.DisplayName})
	}

	return views
}

func mentionedPeople(people []model.ActionItemPerson) []dto.PersonView {
	if len(people) == 0 {
		return nil
	}

	return personViews(people)
}

func sourceShown(board model.Board) func(uuid.UUID) bool {
	return func(uuid.UUID) bool {
		return !board.CardsHidden && !board.HasHiddenColumns()
	}
}

func noSource(uuid.UUID) bool {
	return false
}

func dueOn(date *time.Time) *string {
	if date == nil {
		return nil
	}

	return new(date.Format(model.DueDateLayout))
}
