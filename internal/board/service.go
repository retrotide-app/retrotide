package board

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		GetSnapshot(ctx context.Context, query spec.BoardSnapshotQuery) (*dto.BoardSnapshot, error)
		List(ctx context.Context, teamID uuid.UUID) (*dto.BoardListResponse, error)
		ListForOrganisation(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.OrgBoardListResponse, error)
		Create(ctx context.Context, command CreateCommand) (*dto.BoardSnapshot, error)
		Describe(ctx context.Context, command DescribeCommand) (*dto.BoardCommandResponse, error)
		Templates() *dto.TemplateListResponse
		Template(key string) (*dto.TemplateResponse, error)

		Sharing(ctx context.Context, query spec.BoardAccessQuery) (*dto.SharingView, error)
		SetLinkAccess(ctx context.Context, command SetLinkAccessCommand) (*dto.BoardCommandResponse, error)
		SetPassword(ctx context.Context, command SetPasswordCommand) (*dto.BoardCommandResponse, error)
		RotateLink(ctx context.Context, command Command) (*dto.BoardCommandResponse, error)

		AddColumn(ctx context.Context, command AddColumnCommand) (*dto.BoardCommandResponse, error)
		EditColumn(ctx context.Context, command EditColumnCommand) (*dto.BoardCommandResponse, error)
		MoveColumn(ctx context.Context, command MoveColumnCommand) (*dto.BoardCommandResponse, error)
		DeleteColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error)
		ClearColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error)

		AddCard(ctx context.Context, command AddCardCommand) (*dto.BoardCommandResponse, error)
		EditCard(ctx context.Context, command EditCardCommand) (*dto.BoardCommandResponse, error)
		RecolourCard(ctx context.Context, command RecolourCardCommand) (*dto.BoardCommandResponse, error)
		MoveCard(ctx context.Context, command MoveCardCommand) (*dto.BoardCommandResponse, error)
		DeleteCard(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error)
		MergeCard(ctx context.Context, command MergeCardCommand) (*dto.BoardCommandResponse, error)
		UnmergeCard(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error)

		AddVote(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error)
		RemoveVote(ctx context.Context, command CardCommand) (*dto.BoardCommandResponse, error)
		ResetVotes(ctx context.Context, command Command) (*dto.BoardCommandResponse, error)

		AddComment(ctx context.Context, command AddCommentCommand) (*dto.BoardCommandResponse, error)
		EditComment(ctx context.Context, command EditCommentCommand) (*dto.BoardCommandResponse, error)
		DeleteComment(ctx context.Context, command CommentCommand) (*dto.BoardCommandResponse, error)

		ChangeSettings(ctx context.Context, command SettingsCommand) (*dto.BoardCommandResponse, error)

		RevealCards(ctx context.Context, command Command) (*dto.BoardCommandResponse, error)
		HideCards(ctx context.Context, command Command) (*dto.BoardCommandResponse, error)
		ControlTimer(ctx context.Context, command TimerCommand) (*dto.BoardCommandResponse, error)
		ChangeParticipantRole(ctx context.Context, command ParticipantRoleCommand) (*dto.BoardCommandResponse, error)
		RemoveParticipant(ctx context.Context, command ParticipantCommand) (*dto.BoardCommandResponse, error)
		ChangeStage(ctx context.Context, command StageCommand) (*dto.BoardCommandResponse, error)

		People(ctx context.Context, query spec.BoardAccessQuery) (*dto.BoardPeopleResponse, error)

		Close(ctx context.Context, command Command) (*dto.BoardCommandResponse, error)
		EndIdle(ctx context.Context) (int64, error)

		RevealColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error)
		HideColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error)
		ShowColumn(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error)
		SetColumnVoteLimit(ctx context.Context, command ColumnVoteLimitCommand) (*dto.BoardCommandResponse, error)
		ResetColumnVotes(ctx context.Context, command ColumnCommand) (*dto.BoardCommandResponse, error)

		Present(ctx context.Context, command CardSpotlightCommand) (*dto.BoardCommandResponse, error)
		Highlight(ctx context.Context, command CardSpotlightCommand) (*dto.BoardCommandResponse, error)
		Focus(ctx context.Context, command ColumnFocusCommand) (*dto.BoardCommandResponse, error)

		RestoreTimers(ctx context.Context) error
		StopTimers(ctx context.Context) error
	}

	Deps struct {
		Boards         repository.BoardRepository
		Columns        repository.BoardColumnRepository
		Cards          repository.CardRepository
		Votes          repository.CardVoteRepository
		Comments       repository.CardCommentRepository
		Teams          repository.TeamRepository
		Organisations  repository.OrganisationRepository
		Live           live.Service
		Events         live.Publisher
		OrgAuthz       orgauthz.Service
		AuditLog       repository.AuditLogRepository
		SavedTemplates repository.SavedTemplateRepository
	}

	service struct {
		Runner

		boards         repository.BoardRepository
		columns        repository.BoardColumnRepository
		cards          repository.CardRepository
		votes          repository.CardVoteRepository
		comments       repository.CardCommentRepository
		teams          repository.TeamRepository
		organisations  repository.OrganisationRepository
		members        Members
		live           live.Service
		events         live.Publisher
		orgAuthz       orgauthz.Service
		auditLog       repository.AuditLogRepository
		savedTemplates repository.SavedTemplateRepository
		now            func() time.Time
		shuffle        func(n int, swap func(i, j int))
		timers         *timerRegistry
	}

	boardHome struct {
		orgID  uuid.UUID
		teamID *uuid.UUID
	}
)

func NewService(deps Deps) Service {
	s := &service{
		Runner:         NewRunner(deps.Live, deps.Boards, deps.Cards),
		boards:         deps.Boards,
		columns:        deps.Columns,
		cards:          deps.Cards,
		votes:          deps.Votes,
		comments:       deps.Comments,
		teams:          deps.Teams,
		organisations:  deps.Organisations,
		members:        Members{Organisations: deps.Organisations, Teams: deps.Teams},
		live:           deps.Live,
		events:         deps.Events,
		orgAuthz:       deps.OrgAuthz,
		auditLog:       deps.AuditLog,
		savedTemplates: deps.SavedTemplates,
		now:            time.Now,
		shuffle:        rand.Shuffle,
	}
	s.timers = newTimerRegistry(s.expireTimer)

	return s
}

func ClientID(id uuid.UUID) error {
	if id.Version() != 4 || id.Variant() != uuid.RFC4122 {
		return fmt.Errorf("%w: %s is version %d", ErrInvalidID, id, id.Version())
	}

	return nil
}
