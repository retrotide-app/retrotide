package projection

import (
	"maps"
	"slices"
	"time"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

const (
	EventCardsRevealed      = "cards_revealed"
	EventCardsHidden        = "cards_hidden"
	EventParticipantRemoved = "participant_removed"
	EventTimerChanged       = "timer_changed"
	EventStageChanged       = "stage_changed"
)

func Timer(board model.Board) *dto.TimerView {
	timer := board.Timer()
	if !timer.Set() {
		return nil
	}

	view := &dto.TimerView{DurationMS: *timer.DurationMS}

	if timer.EndsAt != nil {
		view.EndsAt = new(timer.EndsAt.UTC().Format(timeLayout))
	}

	if timer.PausedRemainingMS != nil {
		view.PausedRemainingMS = new(*timer.PausedRemainingMS)
	}

	return view
}

func WithServerNow(snapshot dto.BoardSnapshot, now time.Time) dto.BoardSnapshot {
	snapshot.ServerNow = now.UTC().Format(timeLayout)

	return snapshot
}

func CardsRevealed(state model.BoardState, version int64) Event {
	event := everyone(revealedCards(state, Viewer{}, version), version)

	if state.Header.Board.HasHiddenColumns() {
		staff := Viewer{Capabilities: []model.BoardCapability{model.BoardCapabilityFacilitate}}
		event.forStaff = revealedCards(state, staff, version)
	}

	event.forViewer = func(viewer Viewer) any {
		if !ownsAnyRevealed(state, viewer) {
			return nil
		}

		return revealedCards(state, viewer, version)
	}

	return event
}

func revealedCards(state model.BoardState, viewer Viewer, version int64) dto.CardsRevealedEvent {
	board := state.Header.Board

	profiles := make(map[uuid.UUID]model.BoardParticipantProfile, len(state.Participants))
	for _, profile := range state.Participants {
		profiles[profile.ID] = profile
	}

	reactions := reactionsByCard(state.Reactions)

	shown := make(map[uuid.UUID]struct{}, len(state.Cards))
	cards := make([]dto.CardView, 0, len(state.Cards))
	for _, card := range state.Cards {
		if !ColumnShown(board, card.ColumnID, viewer) {
			continue
		}

		shown[card.ID] = struct{}{}
		cards = append(cards, cardViewWithReactions(board, card, authorProfile(card, profiles), viewer, reactions[card.ID]))
	}

	comments := make([]dto.CommentView, 0, len(state.Comments))
	for _, comment := range state.Comments {
		if _, ok := shown[comment.CardID]; !ok {
			continue
		}

		comments = append(comments, commentView(board, comment, commentAuthorProfile(comment, profiles), viewer))
	}

	return dto.CardsRevealedEvent{
		BoardEventHead: head(EventCardsRevealed, version),
		Stage:          board.Stage,
		Cards:          cards,
		Comments:       comments,
	}
}

func StageChanged(board model.Board, version int64) Event {
	payload := dto.StageChangedEvent{
		BoardEventHead: head(EventStageChanged, version),
		Stage:          board.Stage,
	}

	return everyone(payload, version)
}

func ownsAnyRevealed(state model.BoardState, viewer Viewer) bool {
	for _, card := range state.Cards {
		if Owns(card, viewer) {
			return true
		}
	}

	for _, comment := range state.Comments {
		if OwnsComment(comment, viewer) {
			return true
		}
	}

	return false
}

func CardsHidden(board model.Board, cards []model.Card, version int64) Event {
	return Event{version: version, hidden: Tally(board, cards)}
}

func CardsHiddenCounts(counts map[uuid.UUID]int, version int64) dto.CardsHiddenEvent {
	return dto.CardsHiddenEvent{BoardEventHead: head(EventCardsHidden, version), HiddenCounts: maps.Clone(counts)}
}

func StackedCardMoved(board model.Board, card model.Card, author *model.BoardParticipantProfile, fromColumnID uuid.UUID, version int64) Event {
	event := Event{version: version, deferredCard: new(card.ID)}
	if !board.Anonymous() || !board.CardsHiddenIn(card.ColumnID) {
		event = CardChanged(board, card, author, version)
	}

	if slices.Contains(board.HiddenColumnIDs, card.ColumnID) || slices.Contains(board.HiddenColumnIDs, fromColumnID) {
		event.concealed = true
	}

	if fromColumnID != card.ColumnID {
		event.cardSteps = crossSteps(board, card, fromColumnID)
	}

	return event
}

func ParticipantRoleChanged(board model.Board, profile model.BoardParticipantProfile, version int64) Event {
	event := ParticipantUpserted(board, profile, version)
	event.author = new(profile.ID)
	event.forAuthor = head(EventResync, version)

	return event
}

func ParticipantRemoved(participantID uuid.UUID, version int64) Event {
	payload := dto.ParticipantRemovedEvent{
		BoardEventHead: head(EventParticipantRemoved, version),
		ParticipantID:  participantID,
	}

	return everyone(payload, version)
}

func TimerChanged(board model.Board, now time.Time, version int64) Event {
	payload := dto.TimerChangedEvent{
		BoardEventHead: head(EventTimerChanged, version),
		Timer:          Timer(board),
		ServerNow:      now.UTC().Format(timeLayout),
	}

	return everyone(payload, version)
}
