package projection

import (
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

const (
	EventColumnRevealed   = "column_revealed"
	EventColumnHidden     = "column_hidden"
	EventColumnVotesReset = "column_votes_reset"
)

func ColumnUpserted(board model.Board, column model.BoardColumn, version int64) Event {
	payload := dto.ColumnUpsertedEvent{
		BoardEventHead: head(EventColumnUpserted, version),
		Column:         columnView(board, column),
	}

	event := everyone(payload, version)
	event.concealed = slices.Contains(board.HiddenColumnIDs, column.ID)

	return event
}

func ColumnMoved(board model.Board, column model.BoardColumn, rewritten []spec.PositionChange, version int64) Event {
	payload := dto.ColumnMovedEvent{
		BoardEventHead: head(EventColumnMoved, version),
		ColumnID:       column.ID,
		Position:       column.Position,
		Rewritten:      positions(rewritten),
	}

	event := everyone(payload, version)
	if slices.Contains(board.HiddenColumnIDs, column.ID) {
		event.concealed = true

		return event
	}

	if !board.HasHiddenColumns() {
		return event
	}

	shown := make([]spec.PositionChange, 0, len(rewritten))
	for _, change := range rewritten {
		if slices.Contains(board.HiddenColumnIDs, change.ID) {
			continue
		}

		shown = append(shown, change)
	}

	others := payload
	others.Rewritten = positions(shown)
	event.forOthers = others
	event.forStaff = payload

	return event
}

func ColumnDeleted(board model.Board, columnID uuid.UUID, version int64) Event {
	event := everyone(dto.ColumnRemovedEvent{BoardEventHead: head(EventColumnDeleted, version), ColumnID: columnID}, version)
	event.cleared = []uuid.UUID{columnID}
	event.concealed = slices.Contains(board.HiddenColumnIDs, columnID)

	return event
}

func ColumnCleared(board model.Board, columnID uuid.UUID, version int64) Event {
	event := everyone(dto.ColumnRemovedEvent{BoardEventHead: head(EventColumnCleared, version), ColumnID: columnID}, version)
	event.cleared = []uuid.UUID{columnID}
	event.concealed = slices.Contains(board.HiddenColumnIDs, columnID)

	return event
}

func ColumnRevealed(state model.BoardState, columnID uuid.UUID, version int64) Event {
	event := everyone(revealedColumn(state, columnID, Viewer{}, version), version)
	event.cleared = []uuid.UUID{columnID}

	event.forViewer = func(viewer Viewer) any {
		if !ownsAnyIn(state, columnID, viewer) {
			return nil
		}

		return revealedColumn(state, columnID, viewer, version)
	}

	return event
}

func ColumnHidden(columnID uuid.UUID, version int64) Event {
	payload := dto.ColumnHiddenEvent{BoardEventHead: head(EventColumnHidden, version), ColumnID: columnID}

	return everyone(payload, version)
}

func ColumnVotesReset(columnID uuid.UUID, version int64) Event {
	payload := dto.ColumnVotesResetEvent{BoardEventHead: head(EventColumnVotesReset, version), ColumnID: columnID}

	return everyone(payload, version)
}

func revealedColumn(state model.BoardState, columnID uuid.UUID, viewer Viewer, version int64) dto.ColumnRevealedEvent {
	board := state.Header.Board

	profiles := make(map[uuid.UUID]model.BoardParticipantProfile, len(state.Participants))
	for _, profile := range state.Participants {
		profiles[profile.ID] = profile
	}

	reactions := reactionsByCard(state.Reactions)

	inColumn := map[uuid.UUID]struct{}{}
	cards := []dto.CardView{}
	for _, card := range state.Cards {
		if card.ColumnID != columnID {
			continue
		}

		inColumn[card.ID] = struct{}{}
		cards = append(cards, cardViewWithReactions(board, card, authorProfile(card, profiles), viewer, reactions[card.ID]))
	}

	comments := []dto.CommentView{}
	for _, comment := range state.Comments {
		if _, ok := inColumn[comment.CardID]; !ok {
			continue
		}

		comments = append(comments, commentView(board, comment, commentAuthorProfile(comment, profiles), viewer))
	}

	return dto.ColumnRevealedEvent{
		BoardEventHead: head(EventColumnRevealed, version),
		ColumnID:       columnID,
		Cards:          cards,
		Comments:       comments,
	}
}

func ownsAnyIn(state model.BoardState, columnID uuid.UUID, viewer Viewer) bool {
	inColumn := map[uuid.UUID]struct{}{}
	for _, card := range state.Cards {
		if card.ColumnID != columnID {
			continue
		}

		if Owns(card, viewer) {
			return true
		}

		inColumn[card.ID] = struct{}{}
	}

	for _, comment := range state.Comments {
		if _, ok := inColumn[comment.CardID]; ok && OwnsComment(comment, viewer) {
			return true
		}
	}

	return false
}
