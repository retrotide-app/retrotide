package model

import "github.com/google/uuid"

type (
	CardVote struct {
		CardID                  uuid.UUID
		ParticipantID           uuid.UUID
		Count                   int
		CardAuthorParticipantID *uuid.UUID
		CardColumnID            uuid.UUID
	}

	CardVoteResult struct {
		Card  Card
		Mine  []CardVote
		Count int
	}

	CardMerge struct {
		Card      Card
		Restacked []uuid.UUID
	}
)

const (
	BoardMaxVotesLimit = 99
)

func VotesUsed(votes []CardVote) int {
	used := 0
	for _, vote := range votes {
		used += vote.Count
	}

	return used
}
