package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/logger"
	"retrotide/internal/model"
	"retrotide/internal/notification"
)

type (
	site struct {
		name    string
		baseURL string
	}
)

const (
	methodAutomatic       = "automatic"
	methodAccountSettings = "account_settings"

	detailProvider     = "provider"
	detailProviderName = "provider_name"
	detailMethod       = "method"

	signInMethodsPage  = "/account#sign-in-methods"
	forgotPasswordPage = "/forgot-password"

	createdOnLayout = "2 January 2006"
)

func (s *service) announceLink(ctx context.Context, linked model.UserIdentity, method string, ip string) error {
	var issues []error

	entry := s.identityEntry(linked, audit.ActionIdentityLinked, ip)
	entry.Details[detailMethod] = method

	if err := s.auditLog.Create(ctx, entry); err != nil {
		issues = append(issues, fmt.Errorf("%w: %w", ErrNotAudited, err))
	}

	if err := s.sendLinkNotice(ctx, linked, method == methodAutomatic); err != nil {
		issues = append(issues, fmt.Errorf("%w: %w", ErrNoticeNotSent, err))
	}

	logger.Ctx(ctx).Info().Str("user_id", linked.UserID.String()).Str("provider", string(linked.Provider)).Str("method", method).Msg("identity linked")

	return errors.Join(issues...)
}

func (s *service) announceUnlink(ctx context.Context, removed model.UserIdentity, ip string) error {
	var issues []error

	if err := s.auditLog.Create(ctx, s.identityEntry(removed, audit.ActionIdentityUnlinked, ip)); err != nil {
		issues = append(issues, fmt.Errorf("%w: %w", ErrNotAudited, err))
	}

	if err := s.sendUnlinkNotice(ctx, removed); err != nil {
		issues = append(issues, fmt.Errorf("%w: %w", ErrNoticeNotSent, err))
	}

	return errors.Join(issues...)
}

func (s *service) sendLinkNotice(ctx context.Context, linked model.UserIdentity, automatic bool) error {
	account, err := s.userRepo.GetByID(ctx, linked.UserID)
	if err != nil {
		return fmt.Errorf("look up user %s for the link notice: %w", linked.UserID, err)
	}

	current, err := s.readSite(ctx)
	if err != nil {
		return err
	}

	message, err := notification.IdentityLinkedEmail(notification.IdentityLinkedEmailData{
		To:               account.Email,
		SiteName:         current.name,
		Provider:         s.providerName(linked.Provider),
		IdentityEmail:    linked.Email,
		Automatic:        automatic,
		HasPassword:      account.HasPassword(),
		AccountCreatedOn: account.CreatedAt.UTC().Format(createdOnLayout),
		SignInMethodsURL: current.baseURL + signInMethodsPage,
		ResetPasswordURL: current.baseURL + forgotPasswordPage,
	})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, message); err != nil {
		return fmt.Errorf("send the link notice to user %s: %w", linked.UserID, err)
	}

	return nil
}

func (s *service) sendUnlinkNotice(ctx context.Context, removed model.UserIdentity) error {
	account, err := s.userRepo.GetByID(ctx, removed.UserID)
	if err != nil {
		return fmt.Errorf("look up user %s for the unlink notice: %w", removed.UserID, err)
	}

	current, err := s.readSite(ctx)
	if err != nil {
		return err
	}

	message, err := notification.IdentityUnlinkedEmail(notification.IdentityUnlinkedEmailData{
		To:               account.Email,
		SiteName:         current.name,
		Provider:         s.providerName(removed.Provider),
		IdentityEmail:    removed.Email,
		ResetPasswordURL: current.baseURL + forgotPasswordPage,
	})
	if err != nil {
		return err
	}

	if err := s.email.Send(ctx, message); err != nil {
		return fmt.Errorf("send the unlink notice to user %s: %w", removed.UserID, err)
	}

	return nil
}

func (s *service) readSite(ctx context.Context) (site, error) {
	baseURL, err := s.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return site{}, fmt.Errorf("read the base url: %w", err)
	}

	name, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return site{}, fmt.Errorf("read the site name: %w", err)
	}

	return site{name: name, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s *service) identityEntry(identity model.UserIdentity, action audit.Action, ip string) audit.NewEntry {
	return audit.NewEntry{
		ActorID:    identity.UserID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetIdentity,
		TargetID:   string(identity.Provider),
		SubjectID:  identity.UserID,
		Details:    map[string]string{detailProvider: string(identity.Provider), detailProviderName: s.providerName(identity.Provider)},
		IP:         ip,
	}
}
