package projection

import (
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

const (
	EventCommentUpserted = "comment_upserted"
	EventCommentDeleted  = "comment_deleted"
)

func OwnsComment(comment model.CardComment, viewer Viewer) bool {
	if viewer.ParticipantID == nil || comment.AuthorParticipantID == nil {
		return false
	}

	return *viewer.ParticipantID == *comment.AuthorParticipantID
}

func CommentResponse(board model.Board, card model.Card, comment model.CardComment, author *model.BoardParticipantProfile, viewer Viewer, version int64) dto.BoardCommandResponse {
	response := dto.BoardCommandResponse{Version: version}
	if !Visible(board, card, viewer) {
		return response
	}

	view := commentView(board, comment, author, viewer)
	response.Comment = &view

	return response
}

func ReplayCommentResponse(board model.Board, card model.Card, comment model.CardComment, author *model.BoardParticipantProfile, viewer Viewer, version int64) dto.BoardCommandResponse {
	response := ReplayResponse(version, new(comment.ID))
	if !OwnsComment(comment, viewer) || !Visible(board, card, viewer) {
		return response
	}

	view := commentView(board, comment, author, viewer)
	response.Comment = &view

	return response
}

func CommentUpserted(board model.Board, card model.Card, comment model.CardComment, author *model.BoardParticipantProfile, version int64) Event {
	event := Event{version: version, concealed: slices.Contains(board.HiddenColumnIDs, card.ColumnID)}

	if comment.AuthorParticipantID != nil {
		writer := Viewer{ParticipantID: new(*comment.AuthorParticipantID)}
		event.author = new(*comment.AuthorParticipantID)

		if Visible(board, card, writer) {
			event.forAuthor = dto.CommentUpsertedEvent{
				BoardEventHead: head(EventCommentUpserted, version),
				Comment:        commentView(board, comment, author, writer),
			}
		}
	}

	if !board.CardsHiddenIn(card.ColumnID) {
		event.forOthers = dto.CommentUpsertedEvent{
			BoardEventHead: head(EventCommentUpserted, version),
			Comment:        commentView(board, comment, author, Viewer{}),
		}
	}

	return event
}

func CommentDeleted(board model.Board, card model.Card, comment model.CardComment, version int64) Event {
	payload := dto.CommentDeletedEvent{
		BoardEventHead: head(EventCommentDeleted, version),
		CommentID:      comment.ID,
		CardID:         comment.CardID,
	}

	return seenWithCard(board, card, payload, version)
}

func commentView(board model.Board, comment model.CardComment, author *model.BoardParticipantProfile, viewer Viewer) dto.CommentView {
	view := dto.CommentView{
		ID:       comment.ID,
		CardID:   comment.CardID,
		Body:     comment.Body,
		IsMine:   OwnsComment(comment, viewer),
		Mentions: mentionedPeople(comment.Mentions),
	}

	if board.Anonymous() {
		return view
	}

	view.Author = commentAuthorView(board, comment, author)
	view.CreatedAt = comment.CreatedAt.UTC().Format(timeLayout)
	if comment.UpdatedAt != nil {
		view.UpdatedAt = comment.UpdatedAt.UTC().Format(timeLayout)
	}

	return view
}

func commentAuthorProfile(comment model.CardComment, profiles map[uuid.UUID]model.BoardParticipantProfile) *model.BoardParticipantProfile {
	if comment.AuthorParticipantID == nil {
		return nil
	}

	profile, ok := profiles[*comment.AuthorParticipantID]
	if !ok {
		return nil
	}

	return &profile
}

func commentAuthorView(board model.Board, comment model.CardComment, author *model.BoardParticipantProfile) *dto.AuthorView {
	if comment.AuthorParticipantID == nil {
		return nil
	}

	view := &dto.AuthorView{ParticipantID: *comment.AuthorParticipantID}
	if author != nil && author.ID == *comment.AuthorParticipantID {
		view.DisplayName = ParticipantName(board, *author)
	}

	return view
}
