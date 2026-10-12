package board

import (
	"context"
	"fmt"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) Present(ctx context.Context, command CardSpotlightCommand) (*dto.BoardCommandResponse, error) {
	present := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		cardID, err := s.spotlitCard(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		spotlight := access.Header.Spotlight
		spotlight.PresentingCardID = cardID

		return s.writeSpotlight(ctx, access, target, spotlight)
	}

	return s.Run(ctx, command.Command, VersionReplay, present)
}

func (s *service) Highlight(ctx context.Context, command CardSpotlightCommand) (*dto.BoardCommandResponse, error) {
	highlight := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		cardID, err := s.spotlitCard(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		spotlight := access.Header.Spotlight
		spotlight.HighlightedCardID = cardID

		return s.writeSpotlight(ctx, access, target, spotlight)
	}

	return s.Run(ctx, command.Command, VersionReplay, highlight)
}

func (s *service) Focus(ctx context.Context, command ColumnFocusCommand) (*dto.BoardCommandResponse, error) {
	focus := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		columnID, err := s.spotlitColumn(ctx, access, command.ColumnID)
		if err != nil {
			return nil, err
		}

		spotlight := access.Header.Spotlight
		spotlight.FocusedColumnID = columnID

		return s.writeSpotlight(ctx, access, target, spotlight)
	}

	return s.Run(ctx, command.Command, VersionReplay, focus)
}

func (s *service) spotlitCard(ctx context.Context, access model.BoardAccess, cardID *uuid.UUID) (*uuid.UUID, error) {
	if cardID == nil {
		return nil, nil
	}

	card, err := s.stackTop(ctx, access, *cardID)
	if err != nil {
		return nil, err
	}

	board := access.Header.Board
	if !projection.Visible(board, *card, projection.ViewerFor(access)) {
		return nil, fmt.Errorf("%w: card %s of board %s is hidden from the viewer", policy.ErrCardNotFound, card.ID, board.ID)
	}

	if !projection.VisibleToAll(board, *card) {
		return nil, fmt.Errorf("%w: card %s of board %s is hidden from someone", policy.ErrCardNotShown, card.ID, board.ID)
	}

	return new(card.ID), nil
}

func (s *service) spotlitColumn(ctx context.Context, access model.BoardAccess, columnID *uuid.UUID) (*uuid.UUID, error) {
	if columnID == nil {
		return nil, nil
	}

	if err := s.requireColumn(ctx, access, *columnID); err != nil {
		return nil, err
	}

	if err := policy.CanUseColumn(access, *columnID); err != nil {
		return nil, fmt.Errorf("focus column %s of board %s: %w", *columnID, access.Header.ID, err)
	}

	return new(*columnID), nil
}

func (s *service) writeSpotlight(ctx context.Context, access model.BoardAccess, target spec.BoardWrite, spotlight model.BoardSpotlight) (*dto.BoardCommandResponse, error) {
	board := access.Header.Board
	if spotlightUnchanged(board.Spotlight, spotlight) {
		response := projection.VersionResponse(board.Version)

		return &response, nil
	}

	written, err := s.boards.SetFacilitationState(ctx, spec.BoardFacilitationWrite{
		BoardWrite:        target,
		RevealedColumnIDs: board.RevealedColumnIDs,
		HiddenColumnIDs:   board.HiddenColumnIDs,
		Spotlight:         spotlight,
	})
	if err != nil {
		return nil, fmt.Errorf("set the spotlight of board %s: %w", target.BoardID, err)
	}

	lit := access
	lit.Header.Board = written.Value.Header.Board

	s.events.Publish(lit, projection.SpotlightChanged(lit.Header.Board, written.Version))

	response := projection.VersionResponse(written.Version)

	return &response, nil
}

func spotlightUnchanged(current, next model.BoardSpotlight) bool {
	return sameSpotlitID(current.PresentingCardID, next.PresentingCardID) &&
		sameSpotlitID(current.HighlightedCardID, next.HighlightedCardID) &&
		sameSpotlitID(current.FocusedColumnID, next.FocusedColumnID)
}

func sameSpotlitID(current, next *uuid.UUID) bool {
	if current == nil || next == nil {
		return current == next
	}

	return *current == *next
}
