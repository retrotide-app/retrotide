package auth

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/token"
	"retrotide/internal/user"
)

func (s *service) GetInvitation(ctx context.Context, presented string) (*dto.InvitationResponse, error) {
	pending, err := s.liveInvitation(ctx, presented)
	if err != nil {
		return nil, err
	}

	return new(dto.NewInvitationResponse(*pending)), nil
}

func (s *service) AcceptInvitation(ctx context.Context, acceptance spec.InvitationAcceptance) (*dto.InvitationAcceptedResponse, error) {
	pending, err := s.liveInvitation(ctx, acceptance.Token)
	if err != nil {
		return nil, err
	}

	if user.ClampDisplayName(acceptance.DisplayName) == "" {
		return nil, user.ErrDisplayNameRequired
	}

	account, err := s.users.NewAccountSpec(ctx, spec.NewAccount{Email: pending.Email, Password: acceptance.Password, DisplayName: acceptance.DisplayName})
	if err != nil {
		return nil, err
	}

	created, err := s.invitations.Redeem(ctx, spec.InvitationRedemption{TokenHash: pending.TokenHash, User: account})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrInvalidInvitation
	}
	if errors.Is(err, dao.ErrConflict) {
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, err
	}

	logger.Ctx(ctx).Info().Str("user_id", created.ID.String()).Str("invitation_id", pending.ID.String()).Msg("invitation accepted")

	response := &dto.InvitationAcceptedResponse{Email: created.Email}

	entry := audit.NewEntry{
		ActorID:    created.ID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionInvitationAccepted,
		TargetType: audit.TargetInvitation,
		TargetID:   pending.ID.String(),
		SubjectID:  created.ID,
		Details:    map[string]string{"email": created.Email},
		IP:         acceptance.IP,
	}
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return response, fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return response, nil
}

func (s *service) liveInvitation(ctx context.Context, presented string) (*model.Invitation, error) {
	if presented == "" {
		return nil, ErrInvalidInvitation
	}

	pending, err := s.invitations.GetLiveByTokenHash(ctx, token.Hash(presented))
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrInvalidInvitation
	}
	if err != nil {
		return nil, fmt.Errorf("look up the invitation: %w", err)
	}

	return pending, nil
}
