package surveys

import (
	"context"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/repository"
)

type (
	Service interface {
		Templates() *dto.SurveyTemplateListResponse
		Create(ctx context.Context, command CreateCommand) (*dto.BoardCommandResponse, error)
		Close(ctx context.Context, command SurveyCommand) (*dto.BoardCommandResponse, error)
		Delete(ctx context.Context, command SurveyCommand) (*dto.BoardCommandResponse, error)
		Respond(ctx context.Context, command RespondCommand) (*dto.BoardCommandResponse, error)
	}

	Deps struct {
		Boards  repository.BoardRepository
		Cards   repository.CardRepository
		Surveys repository.SurveyRepository
		Live    live.Service
		Events  live.Publisher
	}

	service struct {
		board.Runner

		surveys repository.SurveyRepository
		events  live.Publisher
	}
)

func NewService(deps Deps) Service {
	return &service{
		Runner:  board.NewRunner(deps.Live, deps.Boards, deps.Cards),
		surveys: deps.Surveys,
		events:  deps.Events,
	}
}
