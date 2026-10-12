package cardpolish

import (
	"context"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/repository"
)

type (
	Service interface {
		React(ctx context.Context, command ReactCommand) (*dto.BoardCommandResponse, error)
		Unreact(ctx context.Context, command ReactCommand) (*dto.BoardCommandResponse, error)
		Pin(ctx context.Context, command PinCommand) (*dto.BoardCommandResponse, error)
	}

	Deps struct {
		Boards    repository.BoardRepository
		Cards     repository.CardRepository
		Reactions repository.CardReactionRepository
		Live      live.Service
		Events    live.Publisher
	}

	service struct {
		board.Runner

		cards     repository.CardRepository
		reactions repository.CardReactionRepository
		events    live.Publisher
	}
)

func NewService(deps Deps) Service {
	return &service{
		Runner:    board.NewRunner(deps.Live, deps.Boards, deps.Cards),
		cards:     deps.Cards,
		reactions: deps.Reactions,
		events:    deps.Events,
	}
}
