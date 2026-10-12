package projection

import (
	"maps"
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

const (
	EventSettingsChanged = "settings_changed"
	EventVotesChanged    = "votes_changed"
	EventVotesReset      = "votes_reset"
)

func Settings(board model.Board) dto.SettingsView {
	return dto.SettingsView{
		VotingOpen:       board.VotingOpen,
		VoteCountsHidden: board.VoteCountsHidden,
		MaxVotes:         board.MaxVotes,
		MaxVotesPerCard:  board.MaxVotesPerCard,
		CommentsEnabled:  board.CommentsEnabled,
		MergingEnabled:   board.MergingEnabled,
		SortMode:         board.SortMode,
		CardsLocked:      board.CardsLocked,
		MovesLocked:      board.MovesLocked,
		BoardLocked:      board.Locked(),
		TimerPlaySound:   board.TimerPlaySound,
		TimerAutoLock:    board.TimerAutoLock,
		SortEnforced:     board.SortEnforced,
		AnyoneCanEdit:    board.AnyoneCanEdit,
		ShowCardDates:    board.ShowCardDates,
	}
}

func Me(board model.Board, votes []model.CardVote, viewer Viewer) dto.MeView {
	mine := map[uuid.UUID]int{}
	used := 0

	for _, vote := range votes {
		if viewer.ParticipantID == nil || vote.ParticipantID != *viewer.ParticipantID {
			continue
		}

		used += vote.Count

		if !voteVisible(board, vote, viewer) {
			continue
		}

		mine[vote.CardID] = vote.Count
	}

	return dto.MeView{VotesRemaining: max(0, board.MaxVotes-used), MyVotes: mine}
}

func MeFrame(boardID uuid.UUID, version int64, me dto.MeView) dto.BoardMeFrame {
	return dto.BoardMeFrame{BoardID: boardID, Version: version, VotesRemaining: me.VotesRemaining, MyVotes: maps.Clone(me.MyVotes)}
}

func VoteResponse(board model.Board, result model.CardVoteResult, viewer Viewer, version int64) dto.BoardCommandResponse {
	me := Me(board, result.Mine, viewer)

	return dto.BoardCommandResponse{Version: version, Me: &me}
}

func SettingsChanged(board model.Board, version int64) Event {
	payload := dto.SettingsChangedEvent{
		BoardEventHead: head(EventSettingsChanged, version),
		Settings:       Settings(board),
	}

	return everyone(payload, version)
}

func VotesChanged(board model.Board, card model.Card, voterID uuid.UUID, version int64) Event {
	payload := dto.VotesChangedEvent{
		BoardEventHead: head(EventVotesChanged, version),
		CardID:         card.ID,
		Total:          voteTotal(board, card),
	}

	event := Event{version: version, author: new(voterID), concealed: slices.Contains(board.HiddenColumnIDs, card.ColumnID)}

	if Visible(board, card, Viewer{ParticipantID: new(voterID)}) {
		event.forAuthor = payload
	}

	if !board.CardsHiddenIn(card.ColumnID) && !board.TotalsHidden() {
		event.forOthers = payload
	}

	return event
}

func VotesReset(version int64) Event {
	return Event{version: version, forOthers: head(EventVotesReset, version)}
}

func votesView(board model.Board, card model.Card) dto.VotesView {
	return dto.VotesView{Total: voteTotal(board, card)}
}

func voteTotal(board model.Board, card model.Card) *int {
	if board.TotalsHidden() {
		return nil
	}

	return new(card.VoteTotal)
}

func voteVisible(board model.Board, vote model.CardVote, viewer Viewer) bool {
	if slices.Contains(board.HiddenColumnIDs, vote.CardColumnID) && !slices.Contains(viewer.Capabilities, model.BoardCapabilityFacilitate) {
		return false
	}

	if !board.CardsHiddenIn(vote.CardColumnID) {
		return true
	}

	return viewer.ParticipantID != nil && vote.CardAuthorParticipantID != nil && *vote.CardAuthorParticipantID == *viewer.ParticipantID
}
