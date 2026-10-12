package notification

import (
	"context"
	"fmt"
	"strings"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
)

type (
	Service interface {
		Notify(ctx context.Context, params dto.NotifyParams) error
		List(ctx context.Context, listing spec.NotificationListing) (*dto.NotificationListResponse, error)
		MarkRead(ctx context.Context, lookup spec.NotificationLookup) error
		MarkAllRead(ctx context.Context, userID uuid.UUID) error
		UnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
		PruneOld(ctx context.Context) (int64, error)
	}

	service struct {
		repo     repository.NotificationRepository
		users    repository.UserRepository
		email    email.Service
		settings settings.Service
	}
)

const (
	retention = 90 * 24 * time.Hour

	defaultPageSize = 20
	maxPageSize     = 100
	maxOffset       = 1000
)

func NewService(repo repository.NotificationRepository, users repository.UserRepository, emailSvc email.Service, settingsSvc settings.Service) Service {
	return &service{repo: repo, users: users, email: emailSvc, settings: settingsSvc}
}

func (s *service) Notify(ctx context.Context, params dto.NotifyParams) error {
	if params.RecipientID == params.ActorID {
		return nil
	}

	_, err := s.repo.Create(ctx, spec.NewNotification{
		UserID:        params.RecipientID,
		OrgID:         params.OrgID,
		Type:          params.Type,
		ReferenceType: params.ReferenceType,
		ReferenceID:   params.ReferenceID,
		ActorID:       params.ActorID,
		Message:       params.Message,
	})
	if err != nil {
		return fmt.Errorf("save the %s notification: %w", params.Type, err)
	}

	if err := s.sendEmail(ctx, params); err != nil {
		return fmt.Errorf("email the %s notification to user %s (the notification itself is saved): %w", params.Type, params.RecipientID, err)
	}

	return nil
}

func (s *service) List(ctx context.Context, listing spec.NotificationListing) (*dto.NotificationListResponse, error) {
	page := clampPage(listing)

	rows, err := s.repo.ListByUser(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	total, err := s.repo.CountByUser(ctx, page.UserID)
	if err != nil {
		return nil, fmt.Errorf("count notifications: %w", err)
	}

	notifications := make([]dto.NotificationResponse, 0, len(rows))
	for _, row := range rows {
		notifications = append(notifications, dto.NewNotificationResponse(row))
	}

	return &dto.NotificationListResponse{
		Notifications: notifications,
		Total:         total,
		Limit:         page.Limit,
		Offset:        page.Offset,
	}, nil
}

func (s *service) MarkRead(ctx context.Context, lookup spec.NotificationLookup) error {
	if err := s.repo.MarkRead(ctx, lookup); err != nil {
		return fmt.Errorf("mark the notification read: %w", err)
	}

	return nil
}

func (s *service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.repo.MarkAllRead(ctx, userID); err != nil {
		return fmt.Errorf("mark every notification read: %w", err)
	}

	return nil
}

func (s *service) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}

	return count, nil
}

func (s *service) PruneOld(ctx context.Context) (int64, error) {
	deleted, err := s.repo.DeleteCreatedBefore(ctx, time.Now().Add(-retention))
	if err != nil {
		return 0, fmt.Errorf("prune old notifications: %w", err)
	}

	return deleted, nil
}

func (s *service) sendEmail(ctx context.Context, params dto.NotifyParams) error {
	enabled, err := s.email.Enabled(ctx)
	if err != nil {
		return err
	}

	if !enabled {
		return nil
	}

	recipient, err := s.users.GetByID(ctx, params.RecipientID)
	if err != nil {
		return fmt.Errorf("look up the recipient: %w", err)
	}

	if followsEmailPreference(params.Type) && !recipient.EmailNotifications {
		return nil
	}

	siteName, err := s.settings.Get(ctx, config.SettingSiteName)
	if err != nil {
		return fmt.Errorf("read the site name: %w", err)
	}

	link, err := s.absoluteLink(ctx, params.Link)
	if err != nil {
		return err
	}

	message, err := notificationEmail(notificationEmailData{To: recipient.Email, SiteName: siteName, Message: params.Message, LinkURL: link})
	if err != nil {
		return err
	}

	return s.email.Send(ctx, message)
}

func (s *service) absoluteLink(ctx context.Context, link string) (string, error) {
	if link == "" || strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "http://") {
		return link, nil
	}

	baseURL, err := s.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return "", fmt.Errorf("read the base url: %w", err)
	}

	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(link, "/"), nil
}

func followsEmailPreference(notificationType model.NotificationType) bool {
	return notificationType == model.NotificationActionItemAssigned
}

func clampPage(listing spec.NotificationListing) spec.NotificationListing {
	limit := listing.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}

	return spec.NotificationListing{
		UserID: listing.UserID,
		Limit:  min(limit, maxPageSize),
		Offset: min(max(listing.Offset, 0), maxOffset),
	}
}
