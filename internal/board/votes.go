package board

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) AddVote(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error) {
	vote := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.stackTop(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanVote(access, *card); err != nil {
			return nil, err
		}

		column, err := s.columns.GetByID(ctx, spec.BoardColumnKey{BoardID: access.Header.ID, ColumnID: card.ColumnID})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrColumnNotFound, err)
		}
		if err != nil {
			return nil, fmt.Errorf("read column %s of board %s: %w", card.ColumnID, target.BoardID, err)
		}

		cast := spec.CardVote{
			BoardWrite:      target,
			CardID:          card.ID,
			ParticipantID:   *access.ParticipantID,
			MaxVotes:        access.Header.MaxVotes,
			MaxVotesPerCard: access.Header.MaxVotesPerCard,
		}
		if column.MaxVotes != nil {
			cast.ColumnID = column.ID
			cast.ColumnMaxVotes = *column.MaxVotes
		}

		written, err := s.votes.AddVote(ctx, cast)
		if errors.Is(err, dao.ErrColumnVoteBudgetSpent) {
			return nil, fmt.Errorf("%w: %w", ErrColumnVoteLimitReached, err)
		}
		if errors.Is(err, dao.ErrVoteBudgetSpent) {
			return nil, fmt.Errorf("%w: %w", ErrVoteLimitReached, err)
		}
		if errors.Is(err, dao.ErrCardVoteCapReached) {
			return nil, fmt.Errorf("%w: %w", ErrCardVoteLimitReached, err)
		}
		if err != nil {
			return nil, fmt.Errorf("vote for card %s of board %s: %w", card.ID, target.BoardID, err)
		}

		return s.announceVote(access, written), nil
	}

	return s.Run(ctx, command.Command, VersionReplay, vote)
}

func (s *service) RemoveVote(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error) {
	unvote := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanVote(access, *card); err != nil {
			return nil, err
		}

		written, err := s.votes.RemoveVote(ctx, spec.CardVote{
			BoardWrite:      target,
			CardID:          card.ID,
			ParticipantID:   *access.ParticipantID,
			MaxVotes:        access.Header.MaxVotes,
			MaxVotesPerCard: access.Header.MaxVotesPerCard,
		})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: card %s: %w", ErrNoVote, card.ID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("take back a vote on card %s of board %s: %w", card.ID, target.BoardID, err)
		}

		return s.announceVote(access, written), nil
	}

	return s.Run(ctx, command.Command, VersionReplay, unvote)
}

func (s *service) ResetVotes(ctx context.Context, command Command) (*dto.BoardCommandResponse, error) {
	reset := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		written, err := s.votes.ResetVotes(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("reset the votes of board %s: %w", target.BoardID, err)
		}

		s.events.Publish(access, projection.VotesReset(written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command, VersionReplay, reset)
}

func (s *service) announceVote(access model.BoardAccess, written *model.Versioned[model.CardVoteResult]) *dto.BoardCommandResponse {
	board := access.Header.Board

	s.events.Publish(access, projection.VotesChanged(board, written.Value.Card, *access.ParticipantID, written.Version))

	response := projection.VoteResponse(board, written.Value, projection.ViewerFor(access), written.Version)
	s.events.PublishMe(access, projection.MeFrame(board.ID, written.Version, *response.Me))

	return &response
}

func (s *service) stackTop(ctx context.Context, access model.BoardAccess, cardID uuid.UUID) (*model.Card, error) {
	card, err := s.Card(ctx, access, cardID)
	if err != nil {
		return nil, err
	}

	if card.ParentCardID == nil {
		return card, nil
	}

	return s.Card(ctx, access, *card.ParentCardID)
}
