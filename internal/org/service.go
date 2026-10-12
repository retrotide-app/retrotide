package org

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"
	"retrotide/internal/settings"

	"github.com/google/uuid"
)

type (
	Service interface {
		CreateWorkspace(ctx context.Context, request WorkspaceRequest) (*dto.WorkspaceResponse, error)
		ListForUser(ctx context.Context, userID uuid.UUID) (*dto.OrganisationListResponse, error)
		Get(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.OrganisationResponse, error)
		Update(ctx context.Context, request UpdateRequest) (*dto.OrganisationResponse, error)
		Delete(ctx context.Context, command Command) error
		Restore(ctx context.Context, command Command) (*dto.OrganisationResponse, error)
		TransferOwnership(ctx context.Context, command MemberCommand) error
		Leave(ctx context.Context, command Command) error

		ListMembers(ctx context.Context, orgID uuid.UUID) (*dto.OrganisationMemberListResponse, error)
		ChangeMemberRole(ctx context.Context, change RoleChange) error
		RemoveMember(ctx context.Context, command MemberCommand) error

		DeleteAccount(ctx context.Context, userID uuid.UUID) error
	}

	Actor struct {
		ActorID uuid.UUID
		IP      string
	}

	Command struct {
		Actor

		OrgID uuid.UUID
	}

	WorkspaceRequest struct {
		Actor

		Name     string
		TeamName string
	}

	UpdateRequest struct {
		Command

		Name        string
		Slug        string
		AllowGuests bool
	}

	MemberCommand struct {
		Command

		UserID uuid.UUID
	}

	RoleChange struct {
		MemberCommand

		Role model.OrgRole
	}

	Deps struct {
		Organisations repository.OrganisationRepository
		Users         repository.UserRepository
		AuditLog      repository.AuditLogRepository
		Hierarchy     hierarchy.Service
		Settings      settings.Service
		MaxOwned      *config.SiteSettingDef
	}

	service struct {
		organisations repository.OrganisationRepository
		users         repository.UserRepository
		auditLog      repository.AuditLogRepository
		hierarchy     hierarchy.Service
		settings      settings.Service
		maxOwned      *config.SiteSettingDef
		now           func() time.Time
		slugSuffix    func() string
	}
)

func NewService(deps Deps) Service {
	return &service{
		organisations: deps.Organisations,
		users:         deps.Users,
		auditLog:      deps.AuditLog,
		hierarchy:     deps.Hierarchy,
		settings:      deps.Settings,
		maxOwned:      deps.MaxOwned,
		now:           time.Now,
		slugSuffix: func() string {
			return strings.ToLower(rand.Text()[:slugSuffixLength])
		},
	}
}

func cleanName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrNameRequired
	}

	if utf8.RuneCountInString(name) > model.OrganisationNameMaxRunes {
		return "", ErrNameTooLong
	}

	return name, nil
}

func (s *service) maxOwnedPerOwner(ctx context.Context) (int, error) {
	allowed, err := s.settings.GetInt(ctx, s.maxOwned)
	if err != nil {
		return 0, fmt.Errorf("read how many organisations an owner may hold: %w", err)
	}

	return allowed, nil
}

func (s *service) ownedAllowance(ctx context.Context, ownerID uuid.UUID) (int, error) {
	allowed, err := s.maxOwnedPerOwner(ctx)
	if err != nil {
		return 0, err
	}

	owned, err := s.organisations.CountOwnedByUser(ctx, ownerID)
	if err != nil {
		return 0, fmt.Errorf("count the organisations user %s owns: %w", ownerID, err)
	}

	if owned >= allowed {
		return 0, ErrOwnedLimitReached
	}

	return allowed, nil
}

func (s *service) record(ctx context.Context, entries ...audit.NewEntry) error {
	var failures []error
	for _, entry := range entries {
		if err := s.auditLog.Create(ctx, entry); err != nil {
			failures = append(failures, err)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("%w: %w", ErrNotAudited, errors.Join(failures...))
	}

	return nil
}

func settle(writeErr error, auditErr error) error {
	if writeErr != nil {
		return fmt.Errorf("%w: %w", ErrCacheStale, errors.Join(writeErr, auditErr))
	}

	return auditErr
}

func orgEntry(actor Actor, orgID uuid.UUID, action audit.Action, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      orgID,
		ActorID:    actor.ActorID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetOrganisation,
		TargetID:   orgID.String(),
		Details:    details,
		IP:         actor.IP,
	}
}

func memberEntry(actor Actor, orgID uuid.UUID, action audit.Action, userID uuid.UUID, details map[string]string) audit.NewEntry {
	entry := orgEntry(actor, orgID, action, details)
	entry.SubjectID = userID

	return entry
}
