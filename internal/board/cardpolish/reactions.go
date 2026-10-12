package cardpolish

import (
	"context"
	"fmt"
	"slices"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) React(ctx context.Context, command ReactCommand) (*dto.BoardCommandResponse, error) {
	if err := knownReaction(command.Reaction); err != nil {
		return nil, err
	}

	return s.Run(ctx, command.Command, board.VersionReplay, s.reaction(ctx, command, true))
}

func (s *service) Unreact(ctx context.Context, command ReactCommand) (*dto.BoardCommandResponse, error) {
	if err := knownReaction(command.Reaction); err != nil {
		return nil, err
	}

	return s.Run(ctx, command.Command, board.VersionReplay, s.reaction(ctx, command, false))
}

func (s *service) reaction(ctx context.Context, command ReactCommand, add bool) board.Write {
	return func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanReact(access, *card); err != nil {
			return nil, err
		}

		written, err := s.reactions.React(ctx, spec.CardReactionWrite{
			BoardWrite:    target,
			CardID:        card.ID,
			ParticipantID: *access.ParticipantID,
			Reaction:      command.Reaction,
			Add:           add,
		})
		if err != nil {
			return nil, fmt.Errorf("change the %s reaction on card %s of board %s: %w", command.Reaction, card.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.CardReactionsChanged(access.Header.Board, *card, written.Value, written.Version))

		response := projection.ReactionsResponse(access.Header.Board, *card, written.Value, projection.ViewerFor(access), written.Version)

		return &response, nil
	}
}

func knownReaction(reaction model.Reaction) error {
	if !slices.Contains(model.Reactions, reaction) {
		return fmt.Errorf("%w: %q", ErrUnknownReaction, reaction)
	}

	return nil
}
