package projection

import (
	"encoding/json"
	"maps"
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	Class int

	HiddenTally map[uuid.UUID]map[uuid.UUID]int

	Event struct {
		version      int64
		author       *uuid.UUID
		forAuthor    any
		forOthers    any
		forStaff     any
		forViewer    func(Viewer) any
		concealed    bool
		cleared      []uuid.UUID
		cardSteps    []tallyStep
		hidden       HiddenTally
		deferredCard *uuid.UUID
	}

	tallyStep struct {
		column uuid.UUID
		author uuid.UUID
		delta  int
	}

	PresenceViewer struct {
		ParticipantID uuid.UUID
		DisplayName   string
		State         string
	}
)

const (
	ClassNone   Class = -1
	ClassAuthor Class = 0
	ClassOthers Class = 1
	ClassStaff  Class = 2
)

const (
	EventResync              = "resync"
	EventBoardUpdated        = "board_updated"
	EventParticipantUpserted = "participant_upserted"
	EventColumnUpserted      = "column_upserted"
	EventColumnMoved         = "column_moved"
	EventColumnDeleted       = "column_deleted"
	EventColumnCleared       = "column_cleared"
	EventCardUpserted        = "card_upserted"
	EventCardMoved           = "card_moved"
	EventCardDeleted         = "card_deleted"
	EventCardsMerged         = "cards_merged"
	EventCardUnmerged        = "card_unmerged"
	EventHiddenCounts        = "hidden_counts"
)

func (e Event) Version() int64 {
	return e.version
}

func (e Event) Author() *uuid.UUID {
	if e.author == nil {
		return nil
	}

	return new(*e.author)
}

func (e Event) ClassOf(viewer Viewer) Class {
	if e.concealed && !slices.Contains(viewer.Capabilities, model.BoardCapabilityFacilitate) {
		return ClassNone
	}

	if e.author != nil && viewer.ParticipantID != nil && *viewer.ParticipantID == *e.author {
		if e.forAuthor == nil {
			return ClassNone
		}

		return ClassAuthor
	}

	if e.forStaff != nil && slices.Contains(viewer.Capabilities, model.BoardCapabilityFacilitate) {
		return ClassStaff
	}

	if e.forOthers == nil {
		return ClassNone
	}

	return ClassOthers
}

func (e Event) Payload(class Class) any {
	switch class {
	case ClassAuthor:
		return e.forAuthor
	case ClassOthers:
		return e.forOthers
	case ClassStaff:
		return e.forStaff
	default:
		return nil
	}
}

func (e Event) PayloadFor(viewer Viewer) (any, bool) {
	if e.concealed && !slices.Contains(viewer.Capabilities, model.BoardCapabilityFacilitate) {
		return nil, false
	}

	if e.forViewer == nil {
		return nil, false
	}

	payload := e.forViewer(viewer)
	if payload == nil {
		return nil, false
	}

	return payload, true
}

func (e Event) Hides() (HiddenTally, bool) {
	if e.hidden == nil {
		return nil, false
	}

	return e.hidden.Clone(), true
}

func (e Event) DeferredCard() (uuid.UUID, bool) {
	if e.deferredCard == nil {
		return uuid.Nil, false
	}

	return *e.deferredCard, true
}

func (e Event) MovesHiddenCards() bool {
	return len(e.cardSteps) > 0
}

func (e Event) ClearsColumns() bool {
	return len(e.cleared) > 0
}

func (e Event) ClearColumns(tally HiddenTally) HiddenTally {
	next := tally.Clone()
	for _, column := range e.cleared {
		delete(next, column)
	}

	return next
}

func (e Event) MoveHiddenCards(tally HiddenTally) HiddenTally {
	next := tally.Clone()
	for _, step := range e.cardSteps {
		next.add(step.column, step.author, step.delta)
	}

	return next
}

func Resync(version int64) Event {
	return Event{version: version, forOthers: head(EventResync, version)}
}

func BoardUpdated(board model.Board, version int64) Event {
	payload := dto.BoardUpdatedEvent{
		BoardEventHead: head(EventBoardUpdated, version),
		Title:          board.Title,
		Context:        board.Context,
	}

	return everyone(payload, version)
}

func ParticipantUpserted(board model.Board, profile model.BoardParticipantProfile, version int64) Event {
	payload := dto.ParticipantUpsertedEvent{
		BoardEventHead: head(EventParticipantUpserted, version),
		Participant: dto.ParticipantView{
			ParticipantID: profile.ID,
			DisplayName:   ParticipantName(board, profile),
			Role:          profile.Role,
		},
	}

	return everyone(payload, version)
}

func CardAdded(board model.Board, card model.Card, author *model.BoardParticipantProfile, version int64) Event {
	event := CardChanged(board, card, author, version)

	if board.CardsHiddenIn(card.ColumnID) {
		event.cardSteps = []tallyStep{{column: card.ColumnID, author: authorKey(card), delta: 1}}
	}

	return event
}

func CardChanged(board model.Board, card model.Card, author *model.BoardParticipantProfile, version int64) Event {
	event := Event{version: version, concealed: slices.Contains(board.HiddenColumnIDs, card.ColumnID)}

	if card.AuthorParticipantID != nil {
		event.author = new(*card.AuthorParticipantID)
		event.forAuthor = dto.CardUpsertedEvent{
			BoardEventHead: head(EventCardUpserted, version),
			Card:           cardView(board, card, author, Viewer{ParticipantID: event.author}),
		}
	}

	if !board.CardsHiddenIn(card.ColumnID) {
		event.forOthers = dto.CardUpsertedEvent{
			BoardEventHead: head(EventCardUpserted, version),
			Card:           cardView(board, card, author, Viewer{}),
		}
	}

	return event
}

func CardMoved(board model.Board, card model.Card, fromColumnID uuid.UUID, rewritten []spec.PositionChange, version int64) Event {
	payload := dto.CardMovedEvent{
		BoardEventHead: head(EventCardMoved, version),
		CardID:         card.ID,
		ColumnID:       card.ColumnID,
		Position:       card.Position,
		Rewritten:      positions(rewritten),
	}

	event := seenWithCard(board, card, payload, version)
	if slices.Contains(board.HiddenColumnIDs, fromColumnID) {
		event.concealed = true
	}

	if fromColumnID != card.ColumnID {
		event.cardSteps = crossSteps(board, card, fromColumnID)
	}

	return event
}

func CardDeleted(board model.Board, card model.Card, version int64) Event {
	payload := dto.CardDeletedEvent{BoardEventHead: head(EventCardDeleted, version), CardID: card.ID}

	event := seenWithCard(board, card, payload, version)

	if board.CardsHiddenIn(card.ColumnID) {
		event.cardSteps = []tallyStep{{column: card.ColumnID, author: authorKey(card), delta: -1}}
	}

	return event
}

func CardsMerged(board model.Board, merged model.CardMerge, version int64) Event {
	card := merged.Card

	payload := dto.CardsMergedEvent{
		BoardEventHead: head(EventCardsMerged, version),
		CardID:         card.ID,
		ParentCardID:   parentOf(card),
		ColumnID:       card.ColumnID,
		Position:       card.Position,
		Restacked:      append([]uuid.UUID{}, merged.Restacked...),
	}

	return seenWithCard(board, card, payload, version)
}

func CardUnmerged(board model.Board, card model.Card, rewritten []spec.PositionChange, version int64) Event {
	payload := dto.CardUnmergedEvent{
		BoardEventHead: head(EventCardUnmerged, version),
		CardID:         card.ID,
		ColumnID:       card.ColumnID,
		Position:       card.Position,
		Rewritten:      positions(rewritten),
	}

	return seenWithCard(board, card, payload, version)
}

func HiddenCounts(counts map[uuid.UUID]int, version int64) dto.HiddenCountsEvent {
	return dto.HiddenCountsEvent{BoardEventHead: head(EventHiddenCounts, version), HiddenCounts: maps.Clone(counts)}
}

func EventsFrame(boardID uuid.UUID, from, to int64, events []json.RawMessage) dto.BoardEventsFrame {
	return dto.BoardEventsFrame{BoardID: boardID, From: from, To: to, Events: events}
}

func Presence(boardID uuid.UUID, board model.Board, viewers []PresenceViewer) dto.BoardPresenceFrame {
	quiet := board.Anonymous() && board.CardsHidden

	views := make([]dto.PresenceView, 0, len(viewers))
	for _, viewer := range viewers {
		view := dto.PresenceView{ParticipantID: viewer.ParticipantID, DisplayName: viewer.DisplayName}
		if !quiet {
			view.State = viewer.State
		}

		views = append(views, view)
	}

	return dto.BoardPresenceFrame{BoardID: boardID, Viewers: views}
}

func SnapshotFromTally(state model.BoardState, viewer Viewer, tally HiddenTally) dto.BoardSnapshot {
	snapshot := Snapshot(state, viewer)

	if state.Header.Board.CardsHidden {
		snapshot.HiddenCounts = tally.For(state.Header.Board, viewer)
	}

	return snapshot
}

func Tally(board model.Board, cards []model.Card) HiddenTally {
	tally := HiddenTally{}
	for _, card := range cards {
		if !board.CardsHiddenIn(card.ColumnID) {
			continue
		}

		tally.add(card.ColumnID, authorKey(card), 1)
	}

	return tally
}

func (t HiddenTally) For(board model.Board, viewer Viewer) map[uuid.UUID]int {
	counts := map[uuid.UUID]int{}
	for column, byAuthor := range t {
		if !board.CardsHiddenIn(column) || !ColumnShown(board, column, viewer) {
			continue
		}

		hidden := 0
		for author, count := range byAuthor {
			if viewer.ParticipantID != nil && *viewer.ParticipantID == author {
				continue
			}

			hidden += count
		}

		if hidden > 0 {
			counts[column] = hidden
		}
	}

	return counts
}

func (t HiddenTally) Clone() HiddenTally {
	next := make(HiddenTally, len(t))
	for column, byAuthor := range t {
		next[column] = maps.Clone(byAuthor)
	}

	return next
}

func (t HiddenTally) add(column, author uuid.UUID, delta int) {
	byAuthor, ok := t[column]
	if !ok {
		byAuthor = map[uuid.UUID]int{}
		t[column] = byAuthor
	}

	byAuthor[author] += delta
	if byAuthor[author] <= 0 {
		delete(byAuthor, author)
	}

	if len(byAuthor) == 0 {
		delete(t, column)
	}
}

func seenWithCard(board model.Board, card model.Card, payload any, version int64) Event {
	event := Event{version: version, concealed: slices.Contains(board.HiddenColumnIDs, card.ColumnID)}

	if card.AuthorParticipantID != nil {
		event.author = new(*card.AuthorParticipantID)
		event.forAuthor = payload
	}

	if !board.CardsHiddenIn(card.ColumnID) {
		event.forOthers = payload
	}

	return event
}

func crossSteps(board model.Board, card model.Card, fromColumnID uuid.UUID) []tallyStep {
	var steps []tallyStep

	if board.CardsHiddenIn(fromColumnID) {
		steps = append(steps, tallyStep{column: fromColumnID, author: authorKey(card), delta: -1})
	}

	if board.CardsHiddenIn(card.ColumnID) {
		steps = append(steps, tallyStep{column: card.ColumnID, author: authorKey(card), delta: 1})
	}

	return steps
}

func everyone(payload any, version int64) Event {
	return Event{version: version, forOthers: payload}
}

func head(eventType string, version int64) dto.BoardEventHead {
	return dto.BoardEventHead{Type: eventType, Version: version}
}

func positions(changes []spec.PositionChange) []dto.PositionView {
	views := make([]dto.PositionView, 0, len(changes))
	for _, change := range changes {
		views = append(views, dto.PositionView{ID: change.ID, Position: change.Position})
	}

	return views
}

func parentOf(card model.Card) uuid.UUID {
	if card.ParentCardID == nil {
		return uuid.Nil
	}

	return *card.ParentCardID
}

func authorKey(card model.Card) uuid.UUID {
	if card.AuthorParticipantID == nil {
		return uuid.Nil
	}

	return *card.AuthorParticipantID
}
