package guest

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/audit"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	ClaimService interface {
		Offer(ctx context.Context, deviceID uuid.UUID) (*dto.GuestClaimOffer, error)
		Claim(ctx context.Context, claim spec.GuestClaim) (*dto.GuestClaimResult, error)
	}

	BoardClaimer interface {
		ClaimGuest(ctx context.Context, claim spec.GuestClaim) ([]model.Board, error)
	}

	DeviceRetirer interface {
		Retire(ctx context.Context, deviceID uuid.UUID) error
	}

	ClaimDeps struct {
		Boards   repository.BoardRepository
		Claimer  BoardClaimer
		Devices  DeviceRetirer
		AuditLog repository.AuditLogRepository
	}

	claimService struct {
		boards   repository.BoardRepository
		claimer  BoardClaimer
		devices  DeviceRetirer
		auditLog repository.AuditLogRepository
	}
)

func NewClaimService(deps ClaimDeps) ClaimService {
	return &claimService{boards: deps.Boards, claimer: deps.Claimer, devices: deps.Devices, auditLog: deps.AuditLog}
}

func (s *claimService) Offer(ctx context.Context, deviceID uuid.UUID) (*dto.GuestClaimOffer, error) {
	if deviceID == uuid.Nil {
		offer := projection.GuestClaimOffer(nil)

		return &offer, nil
	}

	joined, err := s.boards.ListForGuestDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	offer := projection.GuestClaimOffer(joined)

	return &offer, nil
}

func (s *claimService) Claim(ctx context.Context, claim spec.GuestClaim) (*dto.GuestClaimResult, error) {
	if len(claim.BoardIDs) == 0 {
		return nil, ErrNothingClaimed
	}

	claimed, err := s.claimer.ClaimGuest(ctx, claim)
	if err != nil {
		return nil, fmt.Errorf("claim the boards of guest device %s for user %s: %w", claim.DeviceID, claim.UserID, err)
	}

	if err := s.devices.Retire(ctx, claim.DeviceID); err != nil {
		return nil, fmt.Errorf("retire guest device %s after its claim: %w", claim.DeviceID, err)
	}

	result := projection.GuestClaimResult(claimed)

	var unaudited []error
	for _, board := range claimed {
		entry := audit.NewEntry{
			OrgID:      board.OrgID,
			ActorID:    claim.UserID,
			ActorKind:  audit.ActorUser,
			Action:     audit.ActionGuestClaimed,
			TargetType: audit.TargetBoard,
			TargetID:   board.ID.String(),
			SubjectID:  claim.UserID,
		}
		if err := s.auditLog.Create(ctx, entry); err != nil {
			unaudited = append(unaudited, err)
		}
	}

	if len(unaudited) > 0 {
		return &result, fmt.Errorf("%w: %w", ErrNotAudited, errors.Join(unaudited...))
	}

	return &result, nil
}
