package spec

import "github.com/google/uuid"

type (
	NewCardComment struct {
		ID                  uuid.UUID
		BoardID             uuid.UUID
		CardID              uuid.UUID
		AuthorParticipantID *uuid.UUID
		Body                string
	}

	CardCommentRef struct {
		BoardID   uuid.UUID
		CommentID uuid.UUID
	}

	CardCommentEdit struct {
		BoardID   uuid.UUID
		CommentID uuid.UUID
		Body      string
	}

	CommentMentionKey struct {
		OrgID     uuid.UUID
		BoardID   uuid.UUID
		CommentID uuid.UUID
		UserID    uuid.UUID
	}

	CommentCreate struct {
		BoardWrite

		CommentID           uuid.UUID
		CardID              uuid.UUID
		AuthorParticipantID *uuid.UUID
		Body                string
		Mentions            []uuid.UUID
	}

	CommentBodyWrite struct {
		BoardWrite

		CommentID uuid.UUID
		Body      string
		Mentions  []uuid.UUID
	}

	CommentRemoval struct {
		BoardWrite

		CommentID uuid.UUID
	}
)
