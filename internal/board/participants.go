package board

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

func (s *service) ChangeParticipantRole(ctx context.Context, command ParticipantRoleCommand) (*dto.BoardCommandResponse, error) {
	if !slices.Contains(model.BoardRoles, command.Role) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidRole, command.Role)
	}

	assign := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		participant, err := s.seatedParticipant(ctx, access, command.ParticipantID)
		if err != nil {
			return nil, err
		}

		if err := s.checkRoleChange(ctx, access, *participant, command.Role); err != nil {
			return nil, err
		}

		written, err := s.boards.ChangeParticipantRole(ctx, spec.ParticipantRoleWrite{BoardWrite: target, ParticipantID: participant.ID, Role: command.Role})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: participant %s of board %s: %w", ErrParticipantNotFound, participant.ID, target.BoardID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("make participant %s of board %s a %s: %w", participant.ID, target.BoardID, command.Role, err)
		}

		if err := s.live.ReevaluateHeld(ctx, target.BoardID); err != nil {
			s.events.Publish(access, projection.Resync(written.Version))

			return nil, fmt.Errorf("re-evaluate the viewers of board %s after a role change: %w", target.BoardID, err)
		}

		profile, err := s.participantProfile(ctx, access, participant.ID)
		if err != nil {
			s.events.Publish(access, projection.Resync(written.Version))

			return nil, err
		}

		s.events.Publish(access, projection.ParticipantRoleChanged(access.Header.Board, *profile, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, assign)
}

func (s *service) RemoveParticipant(ctx context.Context, command ParticipantCommand) (*dto.BoardCommandResponse, error) {
	remove := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		participant, err := s.seatedParticipant(ctx, access, command.ParticipantID)
		if err != nil {
			return nil, err
		}

		if access.ParticipantID != nil && *access.ParticipantID == participant.ID {
			return nil, ErrRemoveSelf
		}

		lead, err := s.leadsByMembership(ctx, access, *participant)
		if err != nil {
			return nil, err
		}

		if lead {
			return nil, fmt.Errorf("%w: participant %s of board %s", ErrLeadFacilitates, participant.ID, target.BoardID)
		}

		written, err := s.boards.RemoveBoardParticipant(ctx, spec.ParticipantRemovalWrite{BoardWrite: target, ParticipantID: participant.ID})
		if errors.Is(err, dao.ErrNotFound) {
			return nil, fmt.Errorf("%w: participant %s of board %s: %w", ErrParticipantNotFound, participant.ID, target.BoardID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("remove participant %s from board %s: %w", participant.ID, target.BoardID, err)
		}

		if err := s.live.ReevaluateHeld(ctx, target.BoardID); err != nil {
			s.events.Publish(access, projection.Resync(written.Version))

			return nil, fmt.Errorf("re-evaluate the viewers of board %s after a removal: %w", target.BoardID, err)
		}

		s.events.Publish(access, projection.ParticipantRemoved(participant.ID, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, remove)
}

func (s *service) seatedParticipant(ctx context.Context, access model.BoardAccess, participantID uuid.UUID) (*model.BoardParticipant, error) {
	participant, err := s.boards.GetParticipant(ctx, spec.BoardParticipantKey{BoardID: access.Header.ID, ParticipantID: participantID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrParticipantNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read participant %s of board %s: %w", participantID, access.Header.ID, err)
	}

	if participant.Removed() {
		return nil, fmt.Errorf("%w: participant %s of board %s was removed", ErrParticipantNotFound, participantID, access.Header.ID)
	}

	return participant, nil
}

func (s *service) checkRoleChange(ctx context.Context, access model.BoardAccess, participant model.BoardParticipant, role model.BoardRole) error {
	if participant.UserID == nil {
		if role == model.BoardRoleFacilitator {
			return fmt.Errorf("%w: participant %s of board %s is a guest", ErrCannotPromote, participant.ID, access.Header.ID)
		}

		return nil
	}

	member, err := s.live.MembershipRole(ctx, access.Header, model.Principal{Kind: model.PrincipalUser, ID: *participant.UserID})
	if err != nil {
		return fmt.Errorf("read the membership of participant %s of board %s: %w", participant.ID, access.Header.ID, err)
	}

	if role == model.BoardRoleFacilitator && member == "" {
		return fmt.Errorf("%w: participant %s of board %s came in through the link alone", ErrCannotPromote, participant.ID, access.Header.ID)
	}

	if role == model.BoardRoleParticipant && member == model.BoardRoleFacilitator {
		return fmt.Errorf("%w: participant %s of board %s", ErrLeadFacilitates, participant.ID, access.Header.ID)
	}

	return nil
}

func (s *service) leadsByMembership(ctx context.Context, access model.BoardAccess, participant model.BoardParticipant) (bool, error) {
	if participant.UserID == nil {
		return false, nil
	}

	member, err := s.live.MembershipRole(ctx, access.Header, model.Principal{Kind: model.PrincipalUser, ID: *participant.UserID})
	if err != nil {
		return false, fmt.Errorf("read the membership of participant %s of board %s: %w", participant.ID, access.Header.ID, err)
	}

	return member == model.BoardRoleFacilitator, nil
}

func (s *service) participantProfile(ctx context.Context, access model.BoardAccess, participantID uuid.UUID) (*model.BoardParticipantProfile, error) {
	profiles, err := s.boards.ListParticipants(ctx, access.Header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the participants of board %s: %w", access.Header.ID, err)
	}

	for _, profile := range profiles {
		if profile.ID == participantID {
			return &profile, nil
		}
	}

	return nil, fmt.Errorf("%w: participant %s of board %s", live.ErrParticipantUnlisted, participantID, access.Header.ID)
}
