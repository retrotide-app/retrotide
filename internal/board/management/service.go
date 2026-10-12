package management

import (
	"context"
	"time"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
)

type (
	Service interface {
		Archive(ctx context.Context, command board.Command) (*dto.BoardCommandResponse, error)
		Unarchive(ctx context.Context, command board.Command) (*dto.BoardCommandResponse, error)
		Delete(ctx context.Context, command board.Command) (*dto.BoardCommandResponse, error)
		Move(ctx context.Context, command MoveCommand) (*dto.BoardCommandResponse, error)
		ImportCSV(ctx context.Context, command ImportCommand) (*dto.CSVImportResponse, error)
		Restore(ctx context.Context, command RestoreCommand) error
		ListDeleted(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.DeletedBoardListResponse, error)
		Pin(ctx context.Context, command PinCommand) error
		Bulk(ctx context.Context, command BulkCommand) ([]BulkOutcome, error)
		PurgeDeleted(ctx context.Context) (int64, error)
	}

	Deps struct {
		Boards        repository.BoardRepository
		Columns       repository.BoardColumnRepository
		Cards         repository.CardRepository
		Teams         repository.TeamRepository
		Organisations repository.OrganisationRepository
		Live          live.Service
		Events        live.Publisher
		OrgAuthz      orgauthz.Service
		AuditLog      repository.AuditLogRepository
		Now           func() time.Time
	}

	service struct {
		board.Runner

		boards   repository.BoardRepository
		columns  repository.BoardColumnRepository
		cards    repository.CardRepository
		teams    repository.TeamRepository
		live     live.Service
		events   live.Publisher
		orgAuthz orgauthz.Service
		auditLog repository.AuditLogRepository
		now      func() time.Time
	}
)

func NewService(deps Deps) Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}

	return &service{
		Runner:   board.NewRunner(deps.Live, deps.Boards, deps.Cards),
		boards:   deps.Boards,
		columns:  deps.Columns,
		cards:    deps.Cards,
		teams:    deps.Teams,
		live:     deps.Live,
		events:   deps.Events,
		orgAuthz: deps.OrgAuthz,
		auditLog: deps.AuditLog,
		now:      now,
	}
}
