package images

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

func (s *service) SetCardImage(ctx context.Context, command CardImageCommand) (*dto.BoardCommandResponse, error) {
	illustrate := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanSetCardImage(access, *card); err != nil {
			return nil, err
		}

		if err := s.attachable(ctx, access, command.MediaID, model.MediaKindCard); err != nil {
			return nil, err
		}

		written, err := s.cards.SetCardImage(ctx, spec.CardImageWrite{BoardWrite: target, CardID: card.ID, MediaID: command.MediaID})
		if err != nil {
			return nil, fmt.Errorf("set the image of card %s on board %s: %w", card.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.CardImageChanged(access.Header.Board, written.Value, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, illustrate)
}
