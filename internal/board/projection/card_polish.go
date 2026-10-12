package projection

import (
	"fmt"
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

const (
	EventCardReactionsChanged   = "card_reactions_changed"
	EventCardPinned             = "card_pinned"
	EventCardImageChanged       = "card_image_changed"
	EventBoardBackgroundChanged = "board_background_changed"
)

func CardReactionsChanged(board model.Board, card model.Card, reactions []model.CardReaction, version int64) Event {
	event := Event{version: version, concealed: slices.Contains(board.HiddenColumnIDs, card.ColumnID)}
	if board.CardsHiddenIn(card.ColumnID) {
		return event
	}

	event.forOthers = cardReactionsEvent(card, reactions, Viewer{}, version)

	event.forViewer = func(viewer Viewer) any {
		if len(myReactions(reactions, viewer)) == 0 {
			return nil
		}

		return cardReactionsEvent(card, reactions, viewer, version)
	}

	return event
}

func ReactionsResponse(board model.Board, card model.Card, reactions []model.CardReaction, viewer Viewer, version int64) dto.BoardCommandResponse {
	response := dto.BoardCommandResponse{Version: version}
	if !VisibleToAll(board, card) {
		return response
	}

	view := cardReactionsView(card, reactions, viewer)
	response.Reactions = &view

	return response
}

func CardPinned(board model.Board, card model.Card, version int64) Event {
	payload := dto.CardPinnedEvent{
		BoardEventHead: head(EventCardPinned, version),
		CardID:         card.ID,
		Pinned:         card.PinnedAt != nil,
	}

	return seenWithCard(board, card, payload, version)
}

func CardImageChanged(board model.Board, card model.Card, version int64) Event {
	payload := dto.CardImageChangedEvent{
		BoardEventHead: head(EventCardImageChanged, version),
		CardID:         card.ID,
	}

	if card.ImageMediaID != nil {
		payload.ImagePath = new(fmt.Sprintf(model.MediaPathFormat, board.ID, *card.ImageMediaID))
	}

	return seenWithCard(board, card, payload, version)
}

func BoardBackgroundChanged(board model.Board, version int64) Event {
	payload := dto.BoardBackgroundChangedEvent{BoardEventHead: head(EventBoardBackgroundChanged, version)}

	if board.BackgroundMediaID != nil {
		payload.BackgroundPath = new(fmt.Sprintf(model.MediaPathFormat, board.ID, *board.BackgroundMediaID))
	}

	return everyone(payload, version)
}

func cardViewWithReactions(board model.Board, card model.Card, author *model.BoardParticipantProfile, viewer Viewer, reactions []model.CardReaction) dto.CardView {
	view := cardView(board, card, author, viewer)
	if board.CardsHiddenIn(card.ColumnID) {
		return view
	}

	counts := reactionCounts(reactions)
	if len(counts) > 0 {
		view.Reactions = counts
	}

	mine := myReactions(reactions, viewer)
	if len(mine) > 0 {
		view.MyReactions = mine
	}

	return view
}

func cardReactionsEvent(card model.Card, reactions []model.CardReaction, viewer Viewer, version int64) dto.CardReactionsChangedEvent {
	return dto.CardReactionsChangedEvent{
		BoardEventHead:    head(EventCardReactionsChanged, version),
		CardReactionsView: cardReactionsView(card, reactions, viewer),
	}
}

func cardReactionsView(card model.Card, reactions []model.CardReaction, viewer Viewer) dto.CardReactionsView {
	return dto.CardReactionsView{
		CardID:      card.ID,
		Reactions:   reactionCounts(reactions),
		MyReactions: myReactions(reactions, viewer),
	}
}

func reactionsByCard(reactions []model.CardReaction) map[uuid.UUID][]model.CardReaction {
	byCard := map[uuid.UUID][]model.CardReaction{}
	for _, reaction := range reactions {
		byCard[reaction.CardID] = append(byCard[reaction.CardID], reaction)
	}

	return byCard
}

func reactionCounts(reactions []model.CardReaction) []dto.ReactionCountView {
	tally := make(map[model.Reaction]int, len(model.Reactions))
	for _, reaction := range reactions {
		tally[reaction.Reaction]++
	}

	counts := []dto.ReactionCountView{}
	for _, key := range model.Reactions {
		if tally[key] == 0 {
			continue
		}

		counts = append(counts, dto.ReactionCountView{Reaction: key, Count: tally[key]})
	}

	return counts
}

func myReactions(reactions []model.CardReaction, viewer Viewer) []model.Reaction {
	mine := []model.Reaction{}
	if viewer.ParticipantID == nil {
		return mine
	}

	held := map[model.Reaction]struct{}{}
	for _, reaction := range reactions {
		if reaction.ParticipantID == nil || *reaction.ParticipantID != *viewer.ParticipantID {
			continue
		}

		held[reaction.Reaction] = struct{}{}
	}

	for _, key := range model.Reactions {
		if _, ok := held[key]; ok {
			mine = append(mine, key)
		}
	}

	return mine
}
