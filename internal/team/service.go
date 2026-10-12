package team

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/hierarchy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		Create(ctx context.Context, request CreateRequest) (*dto.TeamResponse, error)
		List(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.TeamListResponse, error)
		Get(ctx context.Context, viewer Viewer) (*dto.TeamResponse, error)
		Rename(ctx context.Context, request RenameRequest) (*dto.TeamResponse, error)
		Archive(ctx context.Context, command OrgTeamCommand) error
		Delete(ctx context.Context, command OrgTeamCommand) error

		ListMembers(ctx context.Context, teamID uuid.UUID) (*dto.TeamMemberListResponse, error)
		AddMember(ctx context.Context, change MemberRole) error
		SetMemberRole(ctx context.Context, change MemberRole) error
		RemoveMember(ctx context.Context, command MemberCommand) error
	}

	Actor struct {
		ActorID uuid.UUID
		IP      string
	}

	CreateRequest struct {
		Actor

		OrgID uuid.UUID
		Name  string
	}

	OrgTeamCommand struct {
		Actor

		OrgID  uuid.UUID
		TeamID uuid.UUID
	}

	Command struct {
		Actor

		TeamID uuid.UUID
	}

	RenameRequest struct {
		Command

		Name string
	}

	MemberCommand struct {
		Command

		UserID uuid.UUID
	}

	MemberRole struct {
		MemberCommand

		Role model.TeamRole
	}

	Viewer struct {
		TeamID uuid.UUID
		UserID uuid.UUID
	}

	Deps struct {
		Teams         repository.TeamRepository
		Organisations repository.OrganisationRepository
		AuditLog      repository.AuditLogRepository
		Hierarchy     hierarchy.Service
		Entitlements  entitlements.Service
	}

	service struct {
		teams         repository.TeamRepository
		organisations repository.OrganisationRepository
		auditLog      repository.AuditLogRepository
		hierarchy     hierarchy.Service
		entitlements  entitlements.Service
	}
)

func NewService(deps Deps) Service {
	return &service{
		teams:         deps.Teams,
		organisations: deps.Organisations,
		auditLog:      deps.AuditLog,
		hierarchy:     deps.Hierarchy,
		entitlements:  deps.Entitlements,
	}
}

func CleanName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrNameRequired
	}

	if utf8.RuneCountInString(name) > model.TeamNameMaxRunes {
		return "", ErrNameTooLong
	}

	return name, nil
}

func validRole(r model.TeamRole) bool {
	return r == model.TeamRoleLead || r == model.TeamRoleMember
}

func (s *service) requireLiveOrganisation(ctx context.Context, orgID uuid.UUID) error {
	found, err := s.organisations.GetByID(ctx, orgID)
	if errors.Is(err, dao.ErrNotFound) {
		return ErrOrganisationNotFound
	}
	if err != nil {
		return fmt.Errorf("read organisation %s: %w", orgID, err)
	}

	if found.Deleted() {
		return ErrOrganisationNotFound
	}

	return nil
}

func (s *service) header(ctx context.Context, teamID uuid.UUID) (*model.Team, error) {
	found, err := s.teams.GetHeader(ctx, teamID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read the header of team %s: %w", teamID, err)
	}

	if err := s.requireLiveOrganisation(ctx, found.OrgID); err != nil {
		return nil, err
	}

	return found, nil
}

func (s *service) activeHeader(ctx context.Context, teamID uuid.UUID) (*model.Team, error) {
	found, err := s.header(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if found.Archived() {
		return nil, ErrTeamArchived
	}

	return found, nil
}

func (s *service) viewerRole(ctx context.Context, team model.Team, userID uuid.UUID) (model.TeamRole, error) {
	member, err := s.organisations.GetMember(ctx, spec.OrganisationMemberKey{OrgID: team.OrgID, UserID: userID})
	if errors.Is(err, dao.ErrNotFound) {
		return "", ErrTeamNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read the membership of user %s in organisation %s: %w", userID, team.OrgID, err)
	}

	if orgauthz.EffectiveTeamRole(member.Role, "") == model.TeamRoleLead {
		return model.TeamRoleLead, nil
	}

	teamMember, err := s.teams.GetMember(ctx, spec.TeamMemberKey{OrgID: team.OrgID, TeamID: team.ID, UserID: userID})
	if errors.Is(err, dao.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read the membership of user %s in team %s: %w", userID, team.ID, err)
	}

	return teamMember.Role, nil
}

func (s *service) record(ctx context.Context, entry audit.NewEntry) error {
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return nil
}

func settle(writeErr error, auditErr error) error {
	if writeErr != nil {
		return fmt.Errorf("%w: %w", ErrCacheStale, errors.Join(writeErr, auditErr))
	}

	return auditErr
}

func teamEntry(actor Actor, team model.Team, action audit.Action, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      team.OrgID,
		ActorID:    actor.ActorID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetTeam,
		TargetID:   team.ID.String(),
		Details:    details,
		IP:         actor.IP,
	}
}

func memberEntry(actor Actor, team model.Team, action audit.Action, userID uuid.UUID, details map[string]string) audit.NewEntry {
	entry := teamEntry(actor, team, action, details)
	entry.SubjectID = userID

	return entry
}
