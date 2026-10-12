package actionitems

import (
	"context"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/repository"
)

type (
	Service interface {
		Add(ctx context.Context, command AddCommand) (*dto.BoardCommandResponse, error)
		Edit(ctx context.Context, command EditCommand) (*dto.BoardCommandResponse, error)
		SetStatus(ctx context.Context, command StatusCommand) (*dto.BoardCommandResponse, error)
		Delete(ctx context.Context, command ItemCommand) (*dto.BoardCommandResponse, error)
		Review(ctx context.Context, command ReviewCommand) (*dto.BoardCommandResponse, error)
	}

	Deps struct {
		Boards        repository.BoardRepository
		Cards         repository.CardRepository
		ActionItems   repository.ActionItemRepository
		Teams         repository.TeamRepository
		Organisations repository.OrganisationRepository
		Live          live.Service
		Events        live.Publisher
	}

	service struct {
		board.Runner

		boards      repository.BoardRepository
		actionItems repository.ActionItemRepository
		members     board.Members
		live        live.Service
		events      live.Publisher
	}
)

func NewService(deps Deps) Service {
	return &service{
		Runner:      board.NewRunner(deps.Live, deps.Boards, deps.Cards),
		boards:      deps.Boards,
		actionItems: deps.ActionItems,
		members:     board.Members{Organisations: deps.Organisations, Teams: deps.Teams},
		live:        deps.Live,
		events:      deps.Events,
	}
}
