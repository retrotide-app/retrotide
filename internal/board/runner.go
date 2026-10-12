package board

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Runner struct {
		live   live.Service
		boards repository.BoardRepository
		cards  repository.CardRepository
	}

	Write func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error)

	Replay func(access model.BoardAccess, version int64) (*dto.BoardCommandResponse, error)
)

func NewRunner(boardLive live.Service, boards repository.BoardRepository, cards repository.CardRepository) Runner {
	return Runner{live: boardLive, boards: boards, cards: cards}
}

func (r Runner) Run(ctx context.Context, command Command, replayed Replay, apply Write) (*dto.BoardCommandResponse, error) {
	if command.OpID == uuid.Nil {
		return nil, ErrOpRequired
	}

	release, err := r.live.Acquire(ctx, command.BoardID)
	if err != nil {
		return nil, err
	}
	defer release()

	access, err := r.live.Resolve(ctx, spec.BoardAccessQuery{Principal: command.Principal, BoardID: command.BoardID})
	if err != nil {
		return nil, err
	}

	applied, err := r.appliedOp(ctx, *access, command.OpID)
	if err != nil {
		return nil, err
	}

	if applied != nil {
		return replayed(*access, applied.Version)
	}

	target := spec.BoardWrite{OrgID: access.Header.OrgID, BoardID: access.Header.ID, Version: access.Header.Version, OpID: command.OpID}

	response, err := apply(*access, target)
	if !errors.Is(err, dao.ErrDuplicateOp) {
		return response, err
	}

	applied, err = r.appliedOp(ctx, *access, command.OpID)
	if err != nil {
		return nil, err
	}

	if applied == nil {
		return nil, fmt.Errorf("op %s on board %s was refused as a duplicate but is not recorded: %w", command.OpID, access.Header.ID, dao.ErrDuplicateOp)
	}

	return replayed(*access, applied.Version)
}

func (r Runner) Card(ctx context.Context, access model.BoardAccess, cardID uuid.UUID) (*model.Card, error) {
	card, err := r.cards.GetByID(ctx, spec.CardRef{BoardID: access.Header.ID, CardID: cardID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", policy.ErrCardNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read card %s of board %s: %w", cardID, access.Header.ID, err)
	}

	return card, nil
}

func (r Runner) appliedOp(ctx context.Context, access model.BoardAccess, opID uuid.UUID) (*model.BoardOp, error) {
	op, err := r.boards.GetOp(ctx, spec.BoardOpKey{BoardID: access.Header.ID, OpID: opID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("look up op %s on board %s: %w", opID, access.Header.ID, err)
	}

	return op, nil
}

func VersionReplay(_ model.BoardAccess, version int64) (*dto.BoardCommandResponse, error) {
	response := projection.ReplayResponse(version, nil)

	return &response, nil
}
