package cardpolish

import (
	"context"
	"fmt"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) Pin(ctx context.Context, command PinCommand) (*dto.BoardCommandResponse, error) {
	pin := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanPin(access, *card); err != nil {
			return nil, err
		}

		written, err := s.cards.PinCard(ctx, spec.CardPinWrite{BoardWrite: target, CardID: card.ID, Pinned: command.Pinned})
		if err != nil {
			return nil, fmt.Errorf("set pinned %t on card %s of board %s: %w", command.Pinned, card.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.CardPinned(access.Header.Board, written.Value, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, pin)
}
