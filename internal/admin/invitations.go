package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/notification"
	"retrotide/internal/settings"
	"retrotide/internal/token"
)

const (
	invitationTTL = 7 * 24 * time.Hour

	defaultInvitationPageSize = 20
	acceptInvitationPage      = "accept-invitation"
)

func (s *service) ListInvitations(ctx context.Context, listing spec.InvitationListing) (*dto.AdminInvitationListResponse, error) {
	limit, offset := clampPage(listing.Limit, listing.Offset, defaultInvitationPageSize)

	rows, err := s.invitations.List(ctx, spec.InvitationListing{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}

	total, err := s.invitations.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count invitations: %w", err)
	}

	now := time.Now()
	listed := make([]dto.AdminInvitationResponse, 0, len(rows))
	for _, row := range rows {
		listed = append(listed, dto.NewAdminInvitationResponse(row, now))
	}

	return &dto.AdminInvitationListResponse{Invitations: listed, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *service) InviteUser(ctx context.Context, invitation spec.StaffInvitation) (*dto.AdminInvitationResponse, error) {
	address, err := normaliseEmail(invitation.Email)
	if err != nil {
		return nil, err
	}

	if err := s.requireInvitable(ctx, address); err != nil {
		return nil, err
	}

	drawn, err := token.New()
	if err != nil {
		return nil, err
	}

	created, err := s.invitations.Create(ctx, spec.NewInvitation{Email: address, TokenHash: token.Hash(drawn), ExpiresAt: time.Now().Add(invitationTTL)})
	if errors.Is(err, dao.ErrConflict) {
		return nil, ErrInvitationPending
	}
	if err != nil {
		return nil, fmt.Errorf("create the invitation: %w", err)
	}

	response := new(dto.NewAdminInvitationResponse(*created, time.Now()))

	auditErr := s.record(ctx, staffEntry(invitation.StaffAction, audit.ActionInviteUser, audit.TargetInvitation, created.ID.String(), map[string]string{"email": created.Email}))

	if err := s.sendInvitation(ctx, *created, drawn); err != nil {
		return response, fmt.Errorf("%w: %w", ErrInvitationNotSent, errors.Join(err, auditErr))
	}

	return response, auditErr
}

func (s *service) ResendInvitation(ctx context.Context, action spec.StaffInvitationAction) error {
	pending, err := s.invitations.GetByID(ctx, action.InvitationID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrInvitationNotFound
	}
	if err != nil {
		return fmt.Errorf("look up invitation %s: %w", action.InvitationID, err)
	}

	if err := s.requireInvitable(ctx, pending.Email); err != nil {
		return err
	}

	drawn, err := token.New()
	if err != nil {
		return err
	}

	renewed, err := s.invitations.Renew(ctx, spec.InvitationRenewal{ID: pending.ID, TokenHash: token.Hash(drawn), ExpiresAt: time.Now().Add(invitationTTL)})
	if errors.Is(err, dao.ErrNotFound) {
		return ErrInvitationNotFound
	}
	if err != nil {
		return fmt.Errorf("renew invitation %s: %w", pending.ID, err)
	}

	auditErr := s.record(ctx, staffEntry(action.StaffAction, audit.ActionResendInvitation, audit.TargetInvitation, renewed.ID.String(), map[string]string{"email": renewed.Email}))

	if err := s.sendInvitation(ctx, *renewed, drawn); err != nil {
		return fmt.Errorf("%w: %w", ErrInvitationNotSent, errors.Join(err, auditErr))
	}

	return auditErr
}

func (s *service) CancelInvitation(ctx context.Context, action spec.StaffInvitationAction) error {
	removed, err := s.invitations.Delete(ctx, action.InvitationID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrInvitationNotFound
	}
	if err != nil {
		return fmt.Errorf("cancel invitation %s: %w", action.InvitationID, err)
	}

	return s.record(ctx, staffEntry(action.StaffAction, audit.ActionCancelInvitation, audit.TargetInvitation, removed.ID.String(), map[string]string{"email": removed.Email}))
}

func (s *service) requireInvitable(ctx context.Context, address string) error {
	mode, err := settings.ReadSiteMode(ctx, s.settings)
	if err != nil {
		return err
	}

	if mode != config.SiteModePrivate {
		return ErrSitePublic
	}

	enabled, err := s.email.Enabled(ctx)
	if err != nil {
		return fmt.Errorf("check the email provider: %w", err)
	}

	if !enabled {
		return ErrEmailUnavailable
	}

	inUse, err := s.users.EmailInUse(ctx, spec.UserEmailFilter{Email: address})
	if err != nil {
		return fmt.Errorf("check whether the email address has an account: %w", err)
	}

	if inUse {
		return ErrAccountExists
	}

	return nil
}

func (s *service) sendInvitation(ctx context.Context, invitation model.Invitation, drawn string) error {
	current, err := s.readSite(ctx)
	if err != nil {
		return err
	}

	message, err := notification.InvitationEmail(notification.InvitationEmailData{To: invitation.Email, SiteName: current.name, LinkURL: current.tokenLink(acceptInvitationPage, drawn)})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, message); err != nil {
		return fmt.Errorf("send the invitation email: %w", err)
	}

	return nil
}
