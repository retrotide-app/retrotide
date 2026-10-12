package live

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"retrotide/internal/board/projection"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type (
	passwordBudget struct {
		mu      sync.Mutex
		refill  time.Duration
		burst   float64
		byBoard map[uuid.UUID]*passwordBucket
		swept   time.Time
	}

	passwordBucket struct {
		tokens float64
		at     time.Time
	}
)

const (
	PasswordFailureBurst  = 10
	PasswordFailureRefill = time.Minute

	budgetSweepInterval = time.Minute
)

var (
	linkRefusals = []error{ErrNoAccess, ErrLinkDisabled, ErrPasswordRequired, ErrAccountRequired, ErrGuestsNotAllowed}
)

func (s *service) PreviewLink(ctx context.Context, query LinkQuery) (*dto.JoinPreview, error) {
	header, err := s.linkHeader(ctx, query.Slug)
	if err != nil {
		return nil, err
	}

	if query.Principal != nil {
		_, err := s.resolve(ctx, *query.Principal, *header)
		if err == nil {
			preview := projection.JoinPreview(projection.LinkVerdict{CanJoin: true})

			return &preview, nil
		}

		if !refusedByLink(err) {
			return nil, err
		}
	}

	verdict, err := linkVerdict(*header, joinerKind(query.Principal))
	if err != nil {
		return nil, err
	}

	preview := projection.JoinPreview(verdict)

	return &preview, nil
}

func (s *service) JoinByLink(ctx context.Context, command LinkJoinCommand) (*LinkJoined, error) {
	found, err := s.linkHeader(ctx, command.Slug)
	if err != nil {
		return nil, err
	}

	admitted, err := s.alreadyIn(ctx, command.Principal, *found)
	if err != nil {
		return nil, err
	}

	if !admitted {
		if err := admitsThroughLink(*found, model.Principal{Kind: joinerKind(command.Principal)}); err != nil {
			return nil, err
		}

		if err := s.checkPassword(*found, command.Password); err != nil {
			return nil, err
		}
	}

	release, err := s.Acquire(ctx, found.ID)
	if err != nil {
		return nil, err
	}
	defer release()

	header, err := s.currentLinkHeader(ctx, *found)
	if err != nil {
		return nil, err
	}

	var existing *model.BoardParticipant
	if command.Principal == nil {
		if err := admitsThroughLink(*header, model.Principal{Kind: model.PrincipalGuest}); err != nil {
			return nil, err
		}
	} else {
		access, err := s.resolve(ctx, *command.Principal, *header)
		if err == nil && access.ParticipantID != nil {
			return &LinkJoined{Response: projection.LinkJoined(*access)}, nil
		}

		if err == nil {
			if _, err := s.seatMember(ctx, access); err != nil {
				return nil, err
			}

			return &LinkJoined{Response: projection.LinkJoined(*access)}, nil
		}

		if !refusedByLink(err) {
			return nil, err
		}

		if err := admitsThroughLink(*header, *command.Principal); err != nil {
			return nil, err
		}

		if admitted {
			if err := s.checkPassword(*header, command.Password); err != nil {
				return nil, err
			}
		}

		existing, err = s.participant(ctx, *command.Principal, *header)
		if err != nil {
			return nil, err
		}
	}

	if existing == nil {
		if err := s.checkCapacity(ctx, *header); err != nil {
			return nil, err
		}
	}

	principal, token, err := s.joiner(ctx, command.Principal)
	if err != nil {
		return nil, err
	}

	join := spec.LinkParticipantJoin{
		OrgID: header.OrgID, BoardID: header.ID, Version: header.Version,
		AccessEpoch: header.AccessEpoch,
	}
	if principal.Kind == model.PrincipalUser {
		join.UserID = new(principal.ID)
	} else {
		join.GuestDeviceID = new(principal.ID)
		join.DisplayName = cleanGuestName(command.DisplayName)
	}

	joined, err := s.boards.JoinLinkParticipant(ctx, join)
	if err != nil {
		return nil, fmt.Errorf("join %s %s to board %s through its link: %w", principal.Kind, principal.ID, header.ID, err)
	}

	header.Version = joined.Version

	access, err := s.resolve(ctx, principal, *header)
	if err != nil {
		return nil, fmt.Errorf("resolve %s %s on board %s after joining it: %w", principal.Kind, principal.ID, header.ID, err)
	}

	if err := s.announceParticipant(ctx, *access, joined.Version); err != nil {
		return nil, err
	}

	return &LinkJoined{Response: projection.LinkJoined(*access), GuestToken: token}, nil
}

func (s *service) alreadyIn(ctx context.Context, principal *model.Principal, header model.BoardHeader) (bool, error) {
	if principal == nil {
		return false, nil
	}

	_, err := s.resolve(ctx, *principal, header)
	if err == nil {
		return true, nil
	}

	if refusedByLink(err) {
		return false, nil
	}

	return false, err
}

func (s *service) linkHeader(ctx context.Context, slug string) (*model.BoardHeader, error) {
	if slug == "" {
		return nil, fmt.Errorf("%w: an empty share link", ErrNoAccess)
	}

	header, err := s.boards.GetHeaderBySlug(ctx, slug)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrNoAccess, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the board behind a share link: %w", err)
	}

	if header.Gone() {
		return nil, fmt.Errorf("%w: board %s or its organisation is deleted", ErrNoAccess, header.ID)
	}

	if header.Archived() {
		return nil, fmt.Errorf("%w: board %s", ErrArchived, header.ID)
	}

	return header, nil
}

func (s *service) currentLinkHeader(ctx context.Context, found model.BoardHeader) (*model.BoardHeader, error) {
	header, err := s.boards.GetHeader(ctx, found.ID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrNoAccess, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the header of board %s: %w", found.ID, err)
	}

	if header.Gone() || header.ShareSlug != found.ShareSlug {
		return nil, fmt.Errorf("%w: board %s was deleted or its link rotated", ErrNoAccess, header.ID)
	}

	if header.Archived() {
		return nil, fmt.Errorf("%w: board %s", ErrArchived, header.ID)
	}

	if !samePassword(found.PasswordHash, header.PasswordHash) {
		return nil, fmt.Errorf("%w: board %s changed its password during the join", ErrPasswordRequired, header.ID)
	}

	return header, nil
}

func (s *service) seatMember(ctx context.Context, access *model.BoardAccess) (*model.BoardAccess, error) {
	if access.Header.Archived() {
		return nil, fmt.Errorf("%w: board %s", ErrArchived, access.Header.ID)
	}

	join := spec.BoardParticipantJoin{
		OrgID: access.Header.OrgID, BoardID: access.Header.ID, Version: access.Header.Version,
		UserID: access.Principal.ID,
		Role:   model.BoardRoleParticipant,
	}

	joined, err := s.boards.JoinParticipant(ctx, join)
	if err != nil {
		return nil, fmt.Errorf("join user %s to board %s: %w", access.Principal.ID, access.Header.ID, err)
	}

	access.ParticipantID = new(joined.Value.ID)
	access.Header.Version = joined.Version

	if err := s.announceParticipant(ctx, *access, joined.Version); err != nil {
		return nil, err
	}

	return access, nil
}

func (s *service) checkPassword(header model.BoardHeader, presented string) error {
	if header.PasswordHash == nil {
		return nil
	}

	if presented == "" {
		return fmt.Errorf("%w: board %s", ErrPasswordRequired, header.ID)
	}

	now := time.Now()
	if wait := s.passwords.take(header.ID, now); wait > 0 {
		return NewPasswordAttemptsError(header.ID, wait)
	}

	err := bcrypt.CompareHashAndPassword([]byte(*header.PasswordHash), []byte(presented))
	if err == nil {
		s.passwords.refund(header.ID, now)

		return nil
	}

	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		s.passwords.refund(header.ID, now)

		return fmt.Errorf("compare the password of board %s: %w", header.ID, err)
	}

	return fmt.Errorf("%w: board %s", ErrWrongPassword, header.ID)
}

func (s *service) checkCapacity(ctx context.Context, header model.BoardHeader) error {
	limit, err := s.settings.GetInt(ctx, config.SettingMaxBoardParticipants)
	if err != nil {
		return fmt.Errorf("read the participant cap: %w", err)
	}

	count, err := s.boards.CountLiveParticipants(ctx, spec.BoardSeatCount{BoardID: header.ID, AccessEpoch: header.AccessEpoch})
	if err != nil {
		return err
	}

	if count >= limit {
		return fmt.Errorf("%w: board %s holds %d of %d", ErrBoardFull, header.ID, count, limit)
	}

	return nil
}

func (s *service) joiner(ctx context.Context, principal *model.Principal) (model.Principal, string, error) {
	if principal != nil {
		return *principal, "", nil
	}

	if s.guests == nil {
		return model.Principal{}, "", errNoGuestIssuer
	}

	issued, err := s.guests.Create(ctx)
	if err != nil {
		return model.Principal{}, "", fmt.Errorf("issue a guest device: %w", err)
	}

	return model.Principal{Kind: model.PrincipalGuest, ID: issued.DeviceID}, issued.Token, nil
}

func linkVerdict(header model.BoardHeader, kind model.PrincipalKind) (projection.LinkVerdict, error) {
	err := admitsThroughLink(header, model.Principal{Kind: kind})
	if errors.Is(err, ErrAccountRequired) || errors.Is(err, ErrGuestsNotAllowed) {
		return projection.LinkVerdict{RequiresAccount: true}, nil
	}
	if err != nil {
		return projection.LinkVerdict{}, err
	}

	return projection.LinkVerdict{RequiresPassword: header.HasPassword(), CanJoin: true}, nil
}

func joinerKind(principal *model.Principal) model.PrincipalKind {
	if principal == nil {
		return model.PrincipalGuest
	}

	return principal.Kind
}

func refusedByLink(err error) bool {
	for _, refusal := range linkRefusals {
		if errors.Is(err, refusal) {
			return true
		}
	}

	return false
}

func samePassword(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func cleanGuestName(raw string) string {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) <= model.BoardParticipantNameMaxRunes {
		return name
	}

	return strings.TrimSpace(string([]rune(name)[:model.BoardParticipantNameMaxRunes]))
}

func newPasswordBudget() *passwordBudget {
	return &passwordBudget{
		refill:  PasswordFailureRefill,
		burst:   PasswordFailureBurst,
		byBoard: make(map[uuid.UUID]*passwordBucket),
	}
}

func (b *passwordBudget) take(boardID uuid.UUID, now time.Time) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	if now.Sub(b.swept) >= budgetSweepInterval {
		maps.DeleteFunc(b.byBoard, func(_ uuid.UUID, bucket *passwordBucket) bool {
			return b.level(*bucket, now) >= b.burst
		})
		b.swept = now
	}

	bucket := b.bucket(boardID, now)
	if bucket.tokens < 1 {
		return time.Duration((1 - bucket.tokens) * float64(b.refill))
	}

	bucket.tokens--

	return 0
}

func (b *passwordBudget) refund(boardID uuid.UUID, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	bucket := b.bucket(boardID, now)
	bucket.tokens = min(b.burst, bucket.tokens+1)
}

func (b *passwordBudget) bucket(boardID uuid.UUID, now time.Time) *passwordBucket {
	bucket, ok := b.byBoard[boardID]
	if !ok {
		bucket = &passwordBucket{tokens: b.burst, at: now}
		b.byBoard[boardID] = bucket

		return bucket
	}

	bucket.tokens = b.level(*bucket, now)
	if now.After(bucket.at) {
		bucket.at = now
	}

	return bucket
}

func (b *passwordBudget) level(bucket passwordBucket, now time.Time) float64 {
	if !now.After(bucket.at) {
		return bucket.tokens
	}

	return min(b.burst, bucket.tokens+float64(now.Sub(bucket.at))/float64(b.refill))
}
