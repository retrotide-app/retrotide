package live

import (
	"context"
	"fmt"
	"sync"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/ws"

	"github.com/google/uuid"
)

type (
	Service interface {
		Publisher
		ws.BoardPresence

		Acquire(ctx context.Context, boardID uuid.UUID) (func(), error)
		Resolve(ctx context.Context, query spec.BoardAccessQuery) (*model.BoardAccess, error)
		JoinAsMember(ctx context.Context, query spec.BoardAccessQuery) (*model.BoardAccess, error)
		PreviewLink(ctx context.Context, query LinkQuery) (*dto.JoinPreview, error)
		JoinByLink(ctx context.Context, command LinkJoinCommand) (*LinkJoined, error)
		ReevaluateHeld(ctx context.Context, boardID uuid.UUID) error
		MembershipRole(ctx context.Context, header model.BoardHeader, principal model.Principal) (model.BoardRole, error)
		ClaimGuest(ctx context.Context, claim spec.GuestClaim) ([]model.Board, error)
		Drain(ctx context.Context) error
	}

	GuestIssuer interface {
		Create(ctx context.Context) (model.GuestCredential, error)
	}

	Publisher interface {
		Publish(access model.BoardAccess, events ...projection.Event)
		PublishMe(access model.BoardAccess, me dto.BoardMeFrame)
		PublishMeToEveryone(access model.BoardAccess, votes []model.CardVote, version int64)
		PublishedTally(boardID uuid.UUID) (projection.HiddenTally, bool)
		AnnounceTeamActionItems(teamID uuid.UUID, exclude []uuid.UUID)
	}

	Hub interface {
		JoinBoard(boardID, principalID uuid.UUID)
		LeaveBoard(boardID, principalID uuid.UUID)
		SendToPrincipal(principalID uuid.UUID, msg ws.Message)
		SendToBoardClassified(boardID uuid.UUID, classify func(principalID uuid.UUID) int, payloads [][]byte) int
		SendToBoardLossy(boardID uuid.UUID, classify func(principalID uuid.UUID) int, payloads [][]byte) int
	}

	Deps struct {
		Boards   repository.BoardRepository
		Cards    repository.CardRepository
		OrgAuthz orgauthz.Service
		Hub      Hub
		Settings settings.Service
		Guests   GuestIssuer
	}

	LinkQuery struct {
		Principal *model.Principal
		Slug      string
	}

	LinkJoinCommand struct {
		Principal   *model.Principal
		Slug        string
		DisplayName string
		Password    string
	}

	LinkJoined struct {
		Response   dto.LinkJoinResponse
		GuestToken string
	}

	service struct {
		gates     *gates
		boards    repository.BoardRepository
		cards     repository.CardRepository
		orgAuthz  orgauthz.Service
		hub       Hub
		settings  settings.Service
		guests    GuestIssuer
		passwords *passwordBudget
		timings   timings

		mu     sync.Mutex
		live   map[uuid.UUID]*liveBoard
		closed bool
		ticks  sync.WaitGroup

		background     context.Context
		stopBackground context.CancelCauseFunc
	}

	timings struct {
		flush       time.Duration
		presence    time.Duration
		tick        time.Duration
		tickTimeout time.Duration
	}
)

const (
	GateWait = 5 * time.Second
)

var (
	defaultTimings = timings{
		flush:       50 * time.Millisecond,
		presence:    500 * time.Millisecond,
		tick:        10 * time.Second,
		tickTimeout: 8 * time.Second,
	}
)

func New(deps Deps) Service {
	background, stop := context.WithCancelCause(context.Background())

	return &service{
		gates:          newGates(),
		boards:         deps.Boards,
		cards:          deps.Cards,
		orgAuthz:       deps.OrgAuthz,
		hub:            deps.Hub,
		settings:       deps.Settings,
		guests:         deps.Guests,
		passwords:      newPasswordBudget(),
		timings:        defaultTimings,
		live:           make(map[uuid.UUID]*liveBoard),
		background:     background,
		stopBackground: stop,
	}
}

func (s *service) Acquire(ctx context.Context, boardID uuid.UUID) (func(), error) {
	waitCtx, cancel := context.WithTimeoutCause(ctx, GateWait, ErrBusy)
	defer cancel()

	return s.gates.Acquire(waitCtx, boardID)
}

func (s *service) JoinAsMember(ctx context.Context, query spec.BoardAccessQuery) (*model.BoardAccess, error) {
	release, err := s.Acquire(ctx, query.BoardID)
	if err != nil {
		return nil, err
	}
	defer release()

	access, err := s.Resolve(ctx, query)
	if err != nil {
		return nil, err
	}

	if access.ParticipantID != nil {
		return access, nil
	}

	return s.seatMember(ctx, access)
}

func (s *service) announceParticipant(ctx context.Context, access model.BoardAccess, version int64) error {
	if !s.watched(access.Header.ID) {
		return nil
	}

	profiles, err := s.boards.ListParticipants(ctx, access.Header.ID)
	if err != nil {
		s.appendTo(access.Header.ID, projection.Resync(version))

		return fmt.Errorf("list the participants of board %s to announce participant %s: %w", access.Header.ID, *access.ParticipantID, err)
	}

	for _, profile := range profiles {
		if profile.ID != *access.ParticipantID {
			continue
		}

		s.seat(access, projection.ParticipantName(access.Header.Board, profile))
		s.appendTo(access.Header.ID, projection.ParticipantUpserted(access.Header.Board, profile, version))

		return nil
	}

	s.appendTo(access.Header.ID, projection.Resync(version))

	return fmt.Errorf("%w: participant %s of board %s", ErrParticipantUnlisted, *access.ParticipantID, access.Header.ID)
}
