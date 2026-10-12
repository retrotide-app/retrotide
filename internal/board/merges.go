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

func (s *service) MergeCard(ctx context.Context, command MergeCardCommand) (*dto.BoardCommandResponse, error) {
	merge := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		stack, err := s.stackTop(ctx, access, command.TargetCardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanMerge(access, *card, *stack); err != nil {
			return nil, err
		}

		position, err := s.stackTail(ctx, access, stack.ColumnID)
		if err != nil {
			return nil, err
		}

		written, err := s.cards.MergeCard(ctx, spec.CardMergeWrite{
			BoardWrite:   target,
			CardID:       card.ID,
			ParentCardID: stack.ID,
			ColumnID:     stack.ColumnID,
			Position:     position,
		})
		if err != nil {
			return nil, fmt.Errorf("merge card %s into card %s of board %s: %w", card.ID, stack.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.CardsMerged(access.Header.Board, written.Value, written.Version))

		merged := written.Value.Card
		merged.Mentions = card.Mentions

		return s.cardResponse(ctx, access, merged, written.Version)
	}

	return s.Run(ctx, command.Command, VersionReplay, merge)
}

func (s *service) UnmergeCard(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error) {
	unmerge := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanUnmerge(access, *card); err != nil {
			return nil, err
		}

		parent, err := s.Card(ctx, access, *card.ParentCardID)
		if err != nil {
			return nil, err
		}

		order, err := s.cardOrder(ctx, access, parent.ColumnID)
		if err != nil {
			return nil, err
		}

		placing := place(order, new(parent.ID))

		written, err := s.cards.MoveCard(ctx, spec.CardPlacement{
			BoardWrite: target,
			CardID:     card.ID,
			ColumnID:   parent.ColumnID,
			Position:   placing.position,
			Rewrite:    placing.rewrite,
		})
		if err != nil {
			return nil, fmt.Errorf("unmerge card %s from card %s of board %s: %w", card.ID, parent.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.CardUnmerged(access.Header.Board, written.Value, placing.rewrite, written.Version))

		unmerged := written.Value
		unmerged.Mentions = card.Mentions

		return s.cardResponse(ctx, access, unmerged, written.Version)
	}

	return s.Run(ctx, command.Command, VersionReplay, unmerge)
}

func (s *service) stackTail(ctx context.Context, access model.BoardAccess, columnID uuid.UUID) (string, error) {
	cards, err := s.cards.ListInColumn(ctx, spec.BoardColumnKey{BoardID: access.Header.ID, ColumnID: columnID})
	if err != nil {
		return "", fmt.Errorf("list the cards in column %s of board %s: %w", columnID, access.Header.ID, err)
	}

	siblings := make([]placed, 0, len(cards))
	for _, card := range cards {
		siblings = append(siblings, placed{id: card.ID, position: card.Position})
	}

	key, err := appendKey(siblings)
	if err != nil {
		return "", fmt.Errorf("place a merged card at the end of column %s of board %s: %w", columnID, access.Header.ID, err)
	}

	return key, nil
}
