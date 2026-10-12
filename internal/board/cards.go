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
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) AddCard(ctx context.Context, command AddCardCommand) (*dto.BoardCommandResponse, error) {
	if err := ClientID(command.CardID); err != nil {
		return nil, err
	}

	body, err := CleanCardBody(command.Body)
	if err != nil {
		return nil, err
	}

	if err := checkColour(command.Colour); err != nil {
		return nil, err
	}

	mentions, err := CleanMentions(command.Mentions)
	if err != nil {
		return nil, err
	}

	add := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanCreateCard(access); err != nil {
			return nil, err
		}

		if err := s.requireColumn(ctx, access, command.ColumnID); err != nil {
			return nil, err
		}

		if err := policy.CanUseColumn(access, command.ColumnID); err != nil {
			return nil, err
		}

		if err := s.members.CheckNamed(ctx, access, mentions); err != nil {
			return nil, err
		}

		siblings, err := s.cardOrder(ctx, access, command.ColumnID)
		if err != nil {
			return nil, err
		}

		position, err := appendKey(siblings)
		if err != nil {
			return nil, fmt.Errorf("place card %s in column %s of board %s: %w", command.CardID, command.ColumnID, target.BoardID, err)
		}

		written, err := s.cards.AddCard(ctx, spec.CardCreate{
			BoardWrite:          target,
			CardID:              command.CardID,
			ColumnID:            command.ColumnID,
			AuthorParticipantID: access.ParticipantID,
			Body:                body,
			Colour:              command.Colour,
			Position:            position,
			Mentions:            mentions,
		})
		if errors.Is(err, dao.ErrConflict) {
			return nil, fmt.Errorf("%w: card %s: %w", ErrIDConflict, command.CardID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("add card %s to board %s: %w", command.CardID, target.BoardID, err)
		}

		return s.announceCard(ctx, access, written.Value, written.Version, true)
	}

	return s.Run(ctx, command.Command, s.cardReplay(ctx, command.CardID), add)
}

func (s *service) EditCard(ctx context.Context, command EditCardCommand) (*dto.BoardCommandResponse, error) {
	body, err := CleanCardBody(command.Body)
	if err != nil {
		return nil, err
	}

	mentions, err := CleanMentions(command.Mentions)
	if err != nil {
		return nil, err
	}

	edit := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanEditCard(access, *card); err != nil {
			return nil, err
		}

		if err := s.members.CheckNamed(ctx, access, NewMentions(mentions, card.Mentions)); err != nil {
			return nil, err
		}

		written, err := s.cards.EditCard(ctx, spec.CardBodyWrite{BoardWrite: target, CardID: card.ID, Body: body, Mentions: mentions})
		if err != nil {
			return nil, fmt.Errorf("edit card %s of board %s: %w", card.ID, target.BoardID, err)
		}

		return s.announceCard(ctx, access, written.Value, written.Version, false)
	}

	return s.Run(ctx, command.Command, VersionReplay, edit)
}

func (s *service) RecolourCard(ctx context.Context, command RecolourCardCommand) (*dto.BoardCommandResponse, error) {
	if err := checkColour(command.Colour); err != nil {
		return nil, err
	}

	recolour := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanEditCard(access, *card); err != nil {
			return nil, err
		}

		written, err := s.cards.RecolourCard(ctx, spec.CardColourWrite{BoardWrite: target, CardID: card.ID, Colour: command.Colour})
		if err != nil {
			return nil, fmt.Errorf("recolour card %s of board %s: %w", card.ID, target.BoardID, err)
		}

		return s.announceCard(ctx, access, written.Value, written.Version, false)
	}

	return s.Run(ctx, command.Command, VersionReplay, recolour)
}

func (s *service) MoveCard(ctx context.Context, command MoveCardCommand) (*dto.BoardCommandResponse, error) {
	move := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanMoveCard(access, *card); err != nil {
			return nil, err
		}

		if err := s.requireColumn(ctx, access, command.ColumnID); err != nil {
			return nil, err
		}

		if err := policy.CanMoveTo(access, *card, command.ColumnID); err != nil {
			return nil, err
		}

		order, err := s.cardOrder(ctx, access, command.ColumnID)
		if err != nil {
			return nil, err
		}

		siblings := slices.DeleteFunc(order, func(sibling placed) bool {
			return sibling.id == card.ID
		})
		placing := place(siblings, command.AfterCardID)

		stacked, err := s.hiddenStack(ctx, access, *card, command.ColumnID)
		if err != nil {
			return nil, err
		}

		written, err := s.cards.MoveCard(ctx, spec.CardPlacement{
			BoardWrite: target,
			CardID:     card.ID,
			ColumnID:   command.ColumnID,
			Position:   placing.position,
			Rewrite:    placing.rewrite,
		})
		if err != nil {
			return nil, fmt.Errorf("move card %s of board %s: %w", card.ID, target.BoardID, err)
		}

		events := []projection.Event{projection.CardMoved(access.Header.Board, written.Value, card.ColumnID, placing.rewrite, written.Version)}
		for _, child := range stacked {
			child.ColumnID = command.ColumnID

			author, err := s.profile(ctx, access, child.AuthorParticipantID)
			if err != nil {
				s.events.Publish(access, projection.Resync(written.Version))

				return nil, err
			}

			events = append(events, projection.StackedCardMoved(access.Header.Board, child, author, card.ColumnID, written.Version))
		}

		s.events.Publish(access, events...)

		moved := written.Value
		moved.Mentions = card.Mentions

		return s.cardResponse(ctx, access, moved, written.Version)
	}

	return s.Run(ctx, command.Command, VersionReplay, move)
}

func (s *service) DeleteCard(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error) {
	remove := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		card, err := s.Card(ctx, access, command.CardID)
		if err != nil {
			return nil, err
		}

		if err := policy.CanDeleteCard(access, *card); err != nil {
			return nil, err
		}

		written, err := s.cards.DeleteCard(ctx, spec.CardRemoval{BoardWrite: target, CardID: card.ID})
		if err != nil {
			return nil, fmt.Errorf("delete card %s of board %s: %w", card.ID, target.BoardID, err)
		}

		s.events.Publish(access, projection.CardDeleted(access.Header.Board, *card, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, remove)
}

func (s *service) hiddenStack(ctx context.Context, access model.BoardAccess, card model.Card, columnID uuid.UUID) ([]model.Card, error) {
	if !access.Header.CardsHiddenIn(card.ColumnID) || card.ParentCardID != nil || card.ColumnID == columnID {
		return nil, nil
	}

	cards, err := s.cards.ListInColumn(ctx, spec.BoardColumnKey{BoardID: access.Header.ID, ColumnID: card.ColumnID})
	if err != nil {
		return nil, fmt.Errorf("list the cards stacked under card %s of board %s: %w", card.ID, access.Header.ID, err)
	}

	var stacked []model.Card
	for _, candidate := range cards {
		if candidate.ParentCardID != nil && *candidate.ParentCardID == card.ID {
			stacked = append(stacked, candidate)
		}
	}

	return stacked, nil
}

func (s *service) cardReplay(ctx context.Context, cardID uuid.UUID) Replay {
	return func(access model.BoardAccess, version int64) (*dto.BoardCommandResponse, error) {
		created := projection.ReplayResponse(version, &cardID)

		card, err := s.cards.GetByID(ctx, spec.CardRef{BoardID: access.Header.ID, CardID: cardID})
		if errors.Is(err, dao.ErrNotFound) {
			return &created, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read the replayed card %s of board %s: %w", cardID, access.Header.ID, err)
		}

		viewer := projection.ViewerFor(access)
		if !projection.Owns(*card, viewer) {
			response := projection.ReplayCardResponse(access.Header.Board, *card, nil, viewer, version)

			return &response, nil
		}

		author, err := s.profile(ctx, access, card.AuthorParticipantID)
		if err != nil {
			return nil, err
		}

		response := projection.ReplayCardResponse(access.Header.Board, *card, author, viewer, version)

		return &response, nil
	}
}

func (s *service) cardOrder(ctx context.Context, access model.BoardAccess, columnID uuid.UUID) ([]placed, error) {
	cards, err := s.cards.ListInColumn(ctx, spec.BoardColumnKey{BoardID: access.Header.ID, ColumnID: columnID})
	if err != nil {
		return nil, fmt.Errorf("list the cards in column %s of board %s: %w", columnID, access.Header.ID, err)
	}

	viewer := projection.ViewerFor(access)

	order := make([]placed, 0, len(cards))
	for _, card := range cards {
		if card.ParentCardID != nil || !projection.Visible(access.Header.Board, card, viewer) {
			continue
		}

		order = append(order, placed{id: card.ID, position: card.Position})
	}

	return order, nil
}

func (s *service) announceCard(ctx context.Context, access model.BoardAccess, card model.Card, version int64, added bool) (*dto.BoardCommandResponse, error) {
	author, err := s.profile(ctx, access, card.AuthorParticipantID)
	if err != nil {
		s.events.Publish(access, projection.Resync(version))

		return nil, err
	}

	event := projection.CardChanged(access.Header.Board, card, author, version)
	if added {
		event = projection.CardAdded(access.Header.Board, card, author, version)
	}

	s.events.Publish(access, event)

	response := projection.CardResponse(access.Header.Board, card, author, projection.ViewerFor(access), version)

	return &response, nil
}

func (s *service) cardResponse(ctx context.Context, access model.BoardAccess, card model.Card, version int64) (*dto.BoardCommandResponse, error) {
	author, err := s.profile(ctx, access, card.AuthorParticipantID)
	if err != nil {
		return nil, err
	}

	response := projection.CardResponse(access.Header.Board, card, author, projection.ViewerFor(access), version)

	return &response, nil
}

func (s *service) profile(ctx context.Context, access model.BoardAccess, participantID *uuid.UUID) (*model.BoardParticipantProfile, error) {
	if access.Header.Board.Anonymous() || participantID == nil {
		return nil, nil
	}

	participants, err := s.boards.ListParticipants(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the participants of board %s: %w", access.Header.ID, err)
	}

	for _, participant := range participants {
		if participant.ID == *participantID {
			return &participant, nil
		}
	}

	return nil, nil
}
