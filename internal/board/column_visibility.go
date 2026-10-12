package board

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) RevealColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error) {
	reveal := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		if err := s.requireColumn(ctx, access, command.ColumnID); err != nil {
			return nil, err
		}

		if err := policy.CanUseColumn(access, command.ColumnID); err != nil {
			return nil, err
		}

		if !access.Header.CardsHiddenIn(command.ColumnID) {
			response := projection.VersionResponse(access.Header.Version)

			return &response, nil
		}

		positions, err := s.shuffledColumn(ctx, access, command.ColumnID)
		if err != nil {
			return nil, err
		}

		written, err := s.boards.SetFacilitationState(ctx, spec.BoardFacilitationWrite{
			BoardWrite:        target,
			RevealedColumnIDs: append(slices.Clone(access.Header.RevealedColumnIDs), command.ColumnID),
			HiddenColumnIDs:   access.Header.HiddenColumnIDs,
			Spotlight:         access.Header.Spotlight,
			RevealedColumnID:  new(command.ColumnID),
			Positions:         positions,
		})
		if err != nil {
			return nil, fmt.Errorf("reveal column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		revealed := access
		revealed.Header.Board = written.Value.Header.Board

		s.events.Publish(revealed, projection.ColumnRevealed(written.Value, command.ColumnID, written.Version))
		s.events.PublishMeToEveryone(revealed, written.Value.Votes, written.Version)

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, reveal)
}

func (s *service) HideColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error) {
	hide := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		if err := s.requireColumn(ctx, access, command.ColumnID); err != nil {
			return nil, err
		}

		if slices.Contains(access.Header.HiddenColumnIDs, command.ColumnID) {
			response := projection.VersionResponse(access.Header.Version)

			return &response, nil
		}

		spotlight, err := s.spotlightWithout(ctx, access, command.ColumnID)
		if err != nil {
			return nil, err
		}

		written, err := s.boards.SetFacilitationState(ctx, spec.BoardFacilitationWrite{
			BoardWrite:        target,
			RevealedColumnIDs: access.Header.RevealedColumnIDs,
			HiddenColumnIDs:   append(slices.Clone(access.Header.HiddenColumnIDs), command.ColumnID),
			Spotlight:         spotlight,
		})
		if err != nil {
			return nil, fmt.Errorf("hide column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		hidden := access
		hidden.Header.Board = written.Value.Header.Board

		events := []projection.Event{projection.ColumnHidden(command.ColumnID, written.Version)}
		if !spotlightUnchanged(access.Header.Spotlight, spotlight) {
			events = append(events, projection.SpotlightChanged(hidden.Header.Board, written.Version))
		}

		s.events.Publish(hidden, events...)

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, hide)
}

func (s *service) ShowColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error) {
	show := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		if err := s.requireColumn(ctx, access, command.ColumnID); err != nil {
			return nil, err
		}

		if !slices.Contains(access.Header.HiddenColumnIDs, command.ColumnID) {
			response := projection.VersionResponse(access.Header.Version)

			return &response, nil
		}

		remaining := slices.DeleteFunc(slices.Clone(access.Header.HiddenColumnIDs), func(id uuid.UUID) bool {
			return id == command.ColumnID
		})

		written, err := s.boards.SetFacilitationState(ctx, spec.BoardFacilitationWrite{
			BoardWrite:        target,
			RevealedColumnIDs: access.Header.RevealedColumnIDs,
			HiddenColumnIDs:   remaining,
			Spotlight:         access.Header.Spotlight,
		})
		if err != nil {
			return nil, fmt.Errorf("show column %s of board %s: %w", command.ColumnID, target.BoardID, err)
		}

		shown := access
		shown.Header.Board = written.Value.Header.Board

		s.events.Publish(shown, projection.Resync(written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, show)
}

func (s *service) shuffledColumn(ctx context.Context, access model.BoardAccess, columnID uuid.UUID) ([]spec.PositionChange, error) {
	if !access.Header.Anonymous() {
		return nil, nil
	}

	cards, err := s.cards.ListInColumn(ctx, spec.BoardColumnKey{BoardID: access.Header.ID, ColumnID: columnID})
	if err != nil {
		return nil, fmt.Errorf("list the cards in column %s of board %s to shuffle them: %w", columnID, access.Header.ID, err)
	}

	ids := make([]uuid.UUID, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.ID)
	}

	s.shuffle(len(ids), func(i, j int) {
		ids[i], ids[j] = ids[j], ids[i]
	})

	keys := fracindex.Spread(len(ids))
	positions := make([]spec.PositionChange, 0, len(ids))
	for i, id := range ids {
		positions = append(positions, spec.PositionChange{ID: id, Position: keys[i]})
	}

	return positions, nil
}

func (s *service) spotlightWithout(ctx context.Context, access model.BoardAccess, columnID uuid.UUID) (model.BoardSpotlight, error) {
	spotlight := access.Header.Spotlight

	presenting, err := s.cardOutside(ctx, access, spotlight.PresentingCardID, columnID)
	if err != nil {
		return model.BoardSpotlight{}, err
	}

	highlighted, err := s.cardOutside(ctx, access, spotlight.HighlightedCardID, columnID)
	if err != nil {
		return model.BoardSpotlight{}, err
	}

	focused := spotlight.FocusedColumnID
	if focused != nil && *focused == columnID {
		focused = nil
	}

	return model.BoardSpotlight{PresentingCardID: presenting, HighlightedCardID: highlighted, FocusedColumnID: focused}, nil
}

func (s *service) cardOutside(ctx context.Context, access model.BoardAccess, cardID *uuid.UUID, columnID uuid.UUID) (*uuid.UUID, error) {
	if cardID == nil {
		return nil, nil
	}

	card, err := s.cards.GetByID(ctx, spec.CardRef{BoardID: access.Header.ID, CardID: *cardID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read spotlit card %s of board %s: %w", *cardID, access.Header.ID, err)
	}

	if card.ColumnID == columnID {
		return nil, nil
	}

	return cardID, nil
}
