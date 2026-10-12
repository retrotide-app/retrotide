package board

import (
	"context"
	"fmt"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) RevealCards(ctx context.Context, command Command) (*dto.BoardCommandResponse, error) {
	reveal := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		if !access.Header.CardsHidden {
			response := projection.VersionResponse(access.Header.Version)

			return &response, nil
		}

		positions, err := s.shuffledPositions(ctx, access)
		if err != nil {
			return nil, err
		}

		written, err := s.boards.RevealCards(ctx, spec.CardsRevealWrite{BoardWrite: target, Positions: positions})
		if err != nil {
			return nil, fmt.Errorf("reveal the cards of board %s: %w", target.BoardID, err)
		}

		revealed := access
		revealed.Header.Board = written.Value.Header.Board

		s.events.Publish(revealed, projection.CardsRevealed(written.Value, written.Version))
		s.events.PublishMeToEveryone(revealed, written.Value.Votes, written.Version)

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command, VersionReplay, reveal)
}

func (s *service) HideCards(ctx context.Context, command Command) (*dto.BoardCommandResponse, error) {
	hide := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanHideCards(access); err != nil {
			return nil, err
		}

		if access.Header.CardsHidden {
			response := projection.VersionResponse(access.Header.Version)

			return &response, nil
		}

		written, err := s.boards.HideCards(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("hide the cards of board %s: %w", target.BoardID, err)
		}

		hidden := access
		hidden.Header.Board = written.Value.Header.Board

		s.events.Publish(hidden, projection.CardsHidden(written.Value.Header.Board, written.Value.Cards, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command, VersionReplay, hide)
}

func (s *service) shuffledPositions(ctx context.Context, access model.BoardAccess) ([]spec.PositionChange, error) {
	if !access.Header.Anonymous() || !access.Header.CardsHidden {
		return nil, nil
	}

	cards, err := s.cards.List(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the cards of board %s to shuffle them: %w", access.Header.ID, err)
	}

	var columns []uuid.UUID
	byColumn := map[uuid.UUID][]uuid.UUID{}
	for _, card := range cards {
		if _, seen := byColumn[card.ColumnID]; !seen {
			columns = append(columns, card.ColumnID)
		}

		byColumn[card.ColumnID] = append(byColumn[card.ColumnID], card.ID)
	}

	positions := make([]spec.PositionChange, 0, len(cards))
	for _, column := range columns {
		ids := byColumn[column]
		s.shuffle(len(ids), func(i, j int) {
			ids[i], ids[j] = ids[j], ids[i]
		})

		keys := fracindex.Spread(len(ids))
		for i, id := range ids {
			positions = append(positions, spec.PositionChange{ID: id, Position: keys[i]})
		}
	}

	return positions, nil
}
