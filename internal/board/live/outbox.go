package live

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/ws"

	"github.com/google/uuid"
)

type (
	liveBoard struct {
		id          uuid.UUID
		board       model.Board
		viewers     map[uuid.UUID]*viewer
		pending     []pendingEvent
		tally       projection.HiddenTally
		deferred    map[uuid.UUID]struct{}
		countsDue   bool
		flushDue    bool
		presenceDue bool
		tickDue     bool
		sending     chan struct{}
	}

	viewer struct {
		access      model.BoardAccess
		displayName string
		tabs        map[uuid.UUID]ws.Conn
		state       ws.ViewerState
		lastTo      int64
		counts      map[uuid.UUID]int
	}

	pendingEvent struct {
		version  int64
		event    projection.Event
		payloads [3]json.RawMessage
		counts   projection.HiddenTally
		exclude  *uuid.UUID
		hides    bool
	}

	delivery struct {
		byPrincipal map[uuid.UUID]int
		payloads    [][]byte
	}

	boardTimer struct {
		s       *service
		boardID uuid.UUID
	}
)

var (
	eventClasses = []projection.Class{projection.ClassAuthor, projection.ClassOthers, projection.ClassStaff}
)

func (s *service) Publish(access model.BoardAccess, events ...projection.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[access.Header.ID]
	if !ok {
		return
	}

	wasHidden := live.board.CardsHidden
	live.board = access.Header.Board

	if !live.board.CardsHidden {
		live.tally = nil
		live.deferred = nil
		live.countsDue = false
		for _, member := range live.viewers {
			member.counts = nil
		}
	}

	if wasHidden != live.board.CardsHidden {
		s.schedulePresence(live)
	}

	s.append(live, events)
}

func (s *service) PublishMeToEveryone(access model.BoardAccess, votes []model.CardVote, version int64) {
	s.mu.Lock()
	live, ok := s.live[access.Header.ID]
	if !ok {
		s.mu.Unlock()
		return
	}

	seated := make(map[uuid.UUID]model.BoardAccess, len(live.viewers))
	for principalID, member := range live.viewers {
		if member.access.ParticipantID == nil {
			continue
		}

		seated[principalID] = member.access
	}
	s.mu.Unlock()

	board := access.Header.Board
	for principalID, held := range seated {
		me := projection.Me(board, votes, projection.ViewerFor(held))
		s.hub.SendToPrincipal(principalID, ws.Message{Type: ws.TypeBoardMe, Data: projection.MeFrame(board.ID, version, me)})
	}
}

func (s *service) PublishMe(access model.BoardAccess, me dto.BoardMeFrame) {
	s.mu.Lock()
	seated := false
	if live, ok := s.live[access.Header.ID]; ok {
		_, seated = live.viewers[access.Principal.ID]
	}
	s.mu.Unlock()

	if !seated {
		return
	}

	s.hub.SendToPrincipal(access.Principal.ID, ws.Message{Type: ws.TypeBoardMe, Data: me})
}

func (s *service) AnnounceTeamActionItems(teamID uuid.UUID, exclude []uuid.UUID) {
	s.mu.Lock()
	recipients := make(map[uuid.UUID]struct{})
	for boardID, live := range s.live {
		if slices.Contains(exclude, boardID) || live.board.TeamID == nil || *live.board.TeamID != teamID {
			continue
		}

		for principalID := range live.viewers {
			recipients[principalID] = struct{}{}
		}
	}
	s.mu.Unlock()

	message := ws.Message{Type: ws.TypeTeamActionItemsChanged, Data: ws.TeamActionItemsChanged{TeamID: teamID}}
	for principalID := range recipients {
		s.hub.SendToPrincipal(principalID, message)
	}
}

func (s *service) PublishedTally(boardID uuid.UUID) (projection.HiddenTally, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[boardID]
	if !ok || live.tally == nil {
		return nil, false
	}

	return live.tally.Clone(), true
}

func (s *service) Drain(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	boardIDs := slices.Collect(maps.Keys(s.live))
	s.mu.Unlock()

	s.stopBackground(errDraining)

	ticked := make(chan struct{})
	go func() {
		s.ticks.Wait()
		close(ticked)
	}()

	select {
	case <-ticked:
	case <-ctx.Done():
		return fmt.Errorf("wait for the board ticks to stop: %w", context.Cause(ctx))
	}

	for _, boardID := range boardIDs {
		boardTimer{s: s, boardID: boardID}.flush()
	}

	return nil
}

func (s *service) watched(boardID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.live[boardID]

	return ok
}

func (s *service) appendTo(boardID uuid.UUID, events ...projection.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[boardID]
	if !ok {
		return
	}

	s.append(live, events)
}

func (s *service) seat(access model.BoardAccess, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[access.Header.ID]
	if !ok {
		return
	}

	seated, ok := live.viewers[access.Principal.ID]
	if !ok {
		return
	}

	seated.access.ParticipantID = access.ParticipantID
	seated.displayName = name
	s.schedulePresence(live)
}

func (s *service) append(live *liveBoard, events []projection.Event) {
	for _, event := range events {
		if hidden, hides := event.Hides(); hides {
			live.tally = hidden
			live.countsDue = false
			live.pending = append(live.pending, pendingEvent{version: event.Version(), counts: hidden.Clone(), hides: true})

			continue
		}

		if cardID, deferred := event.DeferredCard(); deferred {
			s.deferCard(live, cardID)

			continue
		}

		live.pending = append(live.pending, encodeEvent(event))

		if event.ClearsColumns() && live.tally != nil {
			live.tally = event.ClearColumns(live.tally)
		}

		if !event.MovesHiddenCards() || !live.board.CardsHidden {
			continue
		}

		if live.board.Anonymous() {
			live.countsDue = true
			s.scheduleTick(live)

			continue
		}

		if live.tally == nil {
			continue
		}

		live.tally = event.MoveHiddenCards(live.tally)
		live.pending = append(live.pending, pendingEvent{version: event.Version(), counts: live.tally.Clone(), exclude: event.Author()})
	}

	s.scheduleFlush(live)
}

func (s *service) deferCard(live *liveBoard, cardID uuid.UUID) {
	if live.deferred == nil {
		live.deferred = map[uuid.UUID]struct{}{}
	}

	live.deferred[cardID] = struct{}{}
	live.countsDue = true
	s.scheduleTick(live)
}

func (s *service) scheduleFlush(live *liveBoard) {
	if live.flushDue || s.closed || len(live.pending) == 0 {
		return
	}

	live.flushDue = true
	time.AfterFunc(s.timings.flush, boardTimer{s: s, boardID: live.id}.flush)
}

func (s *service) schedulePresence(live *liveBoard) {
	if live.presenceDue || s.closed {
		return
	}

	live.presenceDue = true
	time.AfterFunc(s.timings.presence, boardTimer{s: s, boardID: live.id}.presence)
}

func (s *service) scheduleTick(live *liveBoard) {
	if live.tickDue || s.closed {
		return
	}

	live.tickDue = true
	time.AfterFunc(s.timings.tick, boardTimer{s: s, boardID: live.id}.tick)
}

func (t boardTimer) flush() {
	s := t.s

	s.mu.Lock()
	live, ok := s.live[t.boardID]
	if !ok {
		s.mu.Unlock()
		return
	}

	live.flushDue = false
	live.sending <- struct{}{}
	planned := plan(live)
	s.mu.Unlock()

	defer func() {
		<-live.sending
	}()

	if planned == nil {
		return
	}

	s.hub.SendToBoardClassified(t.boardID, planned.classify, planned.payloads)
}

func (t boardTimer) presence() {
	s := t.s

	s.mu.Lock()
	live, ok := s.live[t.boardID]
	if !ok {
		s.mu.Unlock()
		return
	}

	live.presenceDue = false
	recipients := make(map[uuid.UUID]int, len(live.viewers))
	entries := make([]projection.PresenceViewer, 0, len(live.viewers))
	for principalID, present := range live.viewers {
		recipients[principalID] = 0

		if present.access.ParticipantID == nil {
			continue
		}

		entries = append(entries, projection.PresenceViewer{ParticipantID: *present.access.ParticipantID, DisplayName: present.displayName, State: string(present.state)})
	}
	board := live.board
	s.mu.Unlock()

	slices.SortFunc(entries, func(a, b projection.PresenceViewer) int {
		if order := strings.Compare(a.DisplayName, b.DisplayName); order != 0 {
			return order
		}

		return strings.Compare(a.ParticipantID.String(), b.ParticipantID.String())
	})

	data, err := ws.Encode(ws.Message{Type: ws.TypeBoardPresence, Data: projection.Presence(t.boardID, board, entries)})
	if err != nil {
		logger.Log.Error().Err(err).Msg("board presence could not be encoded, so this window sends none")
		return
	}

	planned := delivery{byPrincipal: recipients, payloads: [][]byte{data}}
	s.hub.SendToBoardLossy(t.boardID, planned.classify, planned.payloads)
}

func (t boardTimer) tick() {
	s := t.s

	s.mu.Lock()
	live, ok := s.live[t.boardID]
	if !ok || s.closed {
		s.mu.Unlock()
		return
	}

	live.tickDue = false
	s.ticks.Add(1)
	s.mu.Unlock()

	defer s.ticks.Done()

	ctx, cancel := context.WithTimeout(s.background, s.timings.tickTimeout)
	defer cancel()

	if err := t.publishCounts(ctx); err != nil {
		logger.Ctx(ctx).Warn().Err(err).Msg("board hidden counts tick failed, so it runs again on the next tick")
		t.retryTick()
	}
}

func (t boardTimer) publishCounts(ctx context.Context) error {
	s := t.s

	release, err := s.Acquire(ctx, t.boardID)
	if err != nil {
		return fmt.Errorf("take the gate of board %s for its hidden counts: %w", t.boardID, err)
	}
	defer release()

	s.mu.Lock()
	live, ok := s.live[t.boardID]
	if !ok || !live.countsDue {
		s.mu.Unlock()
		return nil
	}

	live.countsDue = false
	published := live.tally
	deferred := maps.Clone(live.deferred)
	s.mu.Unlock()

	header, err := s.boards.GetHeader(ctx, t.boardID)
	if err != nil {
		return fmt.Errorf("read the header of board %s for its hidden counts: %w", t.boardID, err)
	}

	if !header.Anonymous() || !header.CardsHidden {
		t.forgetDeferred(deferred)

		return nil
	}

	cards, err := s.cards.List(ctx, t.boardID)
	if err != nil {
		return fmt.Errorf("tally the hidden cards of board %s: %w", t.boardID, err)
	}

	tally := projection.Tally(header.Board, cards)
	if len(deferred) == 0 && published != nil && maps.EqualFunc(tally, published, maps.Equal[map[uuid.UUID]int, map[uuid.UUID]int, uuid.UUID, int]) {
		return nil
	}

	version, err := s.boards.BumpVersion(ctx, spec.BoardWrite{OrgID: header.OrgID, BoardID: t.boardID, Version: header.Version})
	if err != nil {
		return fmt.Errorf("bump the version of board %s for its hidden counts: %w", t.boardID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok = s.live[t.boardID]
	if !ok {
		return nil
	}

	live.board = header.Board
	live.tally = tally
	live.pending = append(live.pending, pendingEvent{version: version, counts: tally.Clone()})

	for _, card := range cards {
		if _, due := deferred[card.ID]; !due {
			continue
		}

		live.pending = append(live.pending, encodeEvent(projection.CardChanged(header.Board, card, nil, version)))
	}

	for cardID := range deferred {
		delete(live.deferred, cardID)
	}

	s.scheduleFlush(live)

	return nil
}

func (t boardTimer) forgetDeferred(deferred map[uuid.UUID]struct{}) {
	if len(deferred) == 0 {
		return
	}

	t.s.mu.Lock()
	defer t.s.mu.Unlock()

	live, ok := t.s.live[t.boardID]
	if !ok {
		return
	}

	for cardID := range deferred {
		delete(live.deferred, cardID)
	}
}

func (t boardTimer) retryTick() {
	s := t.s

	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[t.boardID]
	if !ok {
		return
	}

	live.countsDue = true
	s.scheduleTick(live)
}

func (d delivery) classify(principalID uuid.UUID) int {
	index, ok := d.byPrincipal[principalID]
	if !ok {
		return -1
	}

	return index
}

func plan(live *liveBoard) *delivery {
	pending := live.pending
	live.pending = nil

	if len(pending) == 0 {
		return nil
	}

	planned := &delivery{byPrincipal: make(map[uuid.UUID]int, len(live.viewers))}
	shared := map[string]int{}

	for principalID, member := range live.viewers {
		events, key, to := eventsFor(principalID, member, live.board, pending)
		if len(events) == 0 {
			continue
		}

		from := member.lastTo + 1
		member.lastTo = to

		frameKey := fmt.Sprintf("%d|%s", from, key)
		index, ok := shared[frameKey]
		if !ok {
			data, err := ws.Encode(ws.Message{Type: ws.TypeBoardEvents, Data: projection.EventsFrame(live.id, from, to, events)})
			if err != nil {
				logger.Log.Error().Err(err).Int64("from", from).Int64("to", to).Msg("board events frame could not be encoded, so its viewers resync on the next frame")
				continue
			}

			index = len(planned.payloads)
			planned.payloads = append(planned.payloads, data)
			shared[frameKey] = index
		}

		planned.byPrincipal[principalID] = index
	}

	if len(planned.payloads) == 0 {
		return nil
	}

	return planned
}

func eventsFor(principalID uuid.UUID, member *viewer, board model.Board, pending []pendingEvent) ([]json.RawMessage, string, int64) {
	seen := projection.ViewerFor(member.access)

	var events []json.RawMessage
	var key strings.Builder
	to := member.lastTo

	for i, queued := range pending {
		if queued.version <= member.lastTo {
			continue
		}

		if queued.counts != nil {
			raw := countsFor(member, seen, board, queued)
			if raw == nil {
				continue
			}

			events = append(events, raw)
			fmt.Fprintf(&key, "%d#%s;", i, principalID)
			to = queued.version

			continue
		}

		class := queued.event.ClassOf(seen)
		if class == projection.ClassNone || queued.payloads[class] == nil {
			continue
		}

		raw, personal := personalFor(seen, queued)
		if personal {
			if raw == nil {
				continue
			}

			events = append(events, raw)
			fmt.Fprintf(&key, "%d@%s;", i, principalID)
			to = queued.version

			continue
		}

		events = append(events, queued.payloads[class])
		fmt.Fprintf(&key, "%d.%d;", i, class)
		to = queued.version
	}

	return events, key.String(), to
}

func personalFor(seen projection.Viewer, queued pendingEvent) (json.RawMessage, bool) {
	payload, personal := queued.event.PayloadFor(seen)
	if !personal {
		return nil, false
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		logger.Log.Error().Err(err).Int64("version", queued.version).Msg("board event could not be encoded for this viewer, so they resync on the next frame")
		return nil, true
	}

	return raw, true
}

func countsFor(member *viewer, seen projection.Viewer, board model.Board, queued pendingEvent) json.RawMessage {
	if queued.exclude != nil && seen.ParticipantID != nil && *queued.exclude == *seen.ParticipantID {
		return nil
	}

	counts := queued.counts.For(board, seen)
	if !queued.hides && member.counts != nil && maps.Equal(counts, member.counts) {
		return nil
	}

	var payload any = projection.HiddenCounts(counts, queued.version)
	if queued.hides {
		payload = projection.CardsHiddenCounts(counts, queued.version)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		logger.Log.Error().Err(err).Int64("version", queued.version).Msg("hidden counts could not be encoded, so this viewer resyncs on the next frame")
		return nil
	}

	member.counts = counts

	return raw
}

func encodeEvent(event projection.Event) pendingEvent {
	queued := pendingEvent{version: event.Version(), event: event}

	for _, class := range eventClasses {
		payload := event.Payload(class)
		if payload == nil {
			continue
		}

		raw, err := json.Marshal(payload)
		if err != nil {
			logger.Log.Error().Err(err).Int64("version", event.Version()).Msg("board event could not be encoded, so its viewers are told to resync")
			raw, err = json.Marshal(projection.Resync(event.Version()).Payload(projection.ClassOthers))
		}
		if err != nil {
			logger.Log.Error().Err(err).Int64("version", event.Version()).Msg("resync event could not be encoded either, so these viewers resync on the next frame")
			continue
		}

		queued.payloads[class] = raw
	}

	return queued
}
