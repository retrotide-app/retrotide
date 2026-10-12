package spec

import "github.com/google/uuid"

type (
	VoteBudgetSpend struct {
		BoardID       uuid.UUID
		ParticipantID uuid.UUID
		MaxVotes      int
	}

	CardVoteKey struct {
		BoardID       uuid.UUID
		CardID        uuid.UUID
		ParticipantID uuid.UUID
	}

	CardVoteIncrement struct {
		BoardID         uuid.UUID
		CardID          uuid.UUID
		ParticipantID   uuid.UUID
		MaxVotesPerCard int
	}

	CardVoteTotalChange struct {
		BoardID uuid.UUID
		CardID  uuid.UUID
		Delta   int
	}

	VoteHandover struct {
		BoardID           uuid.UUID
		FromParticipantID uuid.UUID
		ToParticipantID   uuid.UUID
	}

	CardVote struct {
		BoardWrite

		CardID          uuid.UUID
		ParticipantID   uuid.UUID
		MaxVotes        int
		MaxVotesPerCard int
		ColumnID        uuid.UUID
		ColumnMaxVotes  int
	}

	ColumnVoteBudgetCheck struct {
		BoardID       uuid.UUID
		ColumnID      uuid.UUID
		ParticipantID uuid.UUID
		MaxVotes      int
	}

	ColumnVotesResetWrite struct {
		BoardWrite

		ColumnID uuid.UUID
	}
)
