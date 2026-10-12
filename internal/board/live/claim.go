package live

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) ClaimGuest(ctx context.Context, claim spec.GuestClaim) ([]model.Board, error) {
	joined, err := s.boards.ListForGuestDevice(ctx, claim.DeviceID)
	if err != nil {
		return nil, err
	}

	targets := make([]uuid.UUID, 0, len(joined))
	for _, entry := range joined {
		if !entry.Board.Anonymous() && slices.Contains(claim.BoardIDs, entry.Board.ID) {
			targets = append(targets, entry.Board.ID)
		}
	}

	slices.SortFunc(targets, func(a, b uuid.UUID) int {
		return strings.Compare(a.String(), b.String())
	})

	claimed := make([]model.Board, 0, len(targets))
	for _, boardID := range targets {
		board, err := s.claimOn(ctx, claim, boardID)
		if err != nil {
			return claimed, err
		}

		if board != nil {
			claimed = append(claimed, *board)
		}
	}

	return claimed, nil
}

func (s *service) claimOn(ctx context.Context, claim spec.GuestClaim, boardID uuid.UUID) (*model.Board, error) {
	release, err := s.Acquire(ctx, boardID)
	if err != nil {
		return nil, err
	}
	defer release()

	header, err := s.boards.GetHeader(ctx, boardID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the header of board %s to claim it: %w", boardID, err)
	}

	if header.Gone() || header.Anonymous() {
		return nil, nil
	}

	guestRow, err := s.participant(ctx, model.Principal{Kind: model.PrincipalGuest, ID: claim.DeviceID}, *header)
	if err != nil {
		return nil, err
	}

	if guestRow == nil || guestRow.Removed() {
		return nil, nil
	}

	userRow, err := s.participant(ctx, model.Principal{Kind: model.PrincipalUser, ID: claim.UserID}, *header)
	if err != nil {
		return nil, err
	}

	if userRow != nil && userRow.Removed() {
		return nil, nil
	}

	write := spec.GuestClaimWrite{
		OrgID: header.OrgID, BoardID: header.ID, Version: header.Version,
		GuestParticipantID: guestRow.ID,
		UserID:             claim.UserID,
	}
	if userRow != nil {
		write.UserParticipantID = new(userRow.ID)
	}

	written, err := s.boards.ClaimGuestParticipant(ctx, write)
	if err != nil {
		return nil, fmt.Errorf("claim the guest participant %s of board %s for user %s: %w", guestRow.ID, header.ID, claim.UserID, err)
	}

	s.rebind(header.ID, claim.UserID, written.Value.ID)
	s.appendTo(header.ID, projection.Resync(written.Version))

	return &header.Board, nil
}

func (s *service) rebind(boardID, principalID, participantID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	live, ok := s.live[boardID]
	if !ok {
		return
	}

	member, ok := live.viewers[principalID]
	if !ok {
		return
	}

	member.access.ParticipantID = new(participantID)
}
