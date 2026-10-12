package savedtemplate

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"retrotide/internal/board/live"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		List(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.SavedTemplateListResponse, error)
		Save(ctx context.Context, command SaveCommand) (*dto.SavedTemplateResponse, error)
		Delete(ctx context.Context, removal spec.SavedTemplateRemoval) error
	}

	SaveCommand struct {
		Principal   model.Principal
		BoardID     uuid.UUID
		Name        string
		Description string
		Scope       model.SavedTemplateScope
	}

	Deps struct {
		SavedTemplates repository.SavedTemplateRepository
		Columns        repository.BoardColumnRepository
		Teams          repository.TeamRepository
		Organisations  repository.OrganisationRepository
		Live           live.Service
		OrgAuthz       orgauthz.Service
		Entitlements   entitlements.Service
	}

	service struct {
		savedTemplates repository.SavedTemplateRepository
		columns        repository.BoardColumnRepository
		teams          repository.TeamRepository
		organisations  repository.OrganisationRepository
		live           live.Service
		orgAuthz       orgauthz.Service
		entitlements   entitlements.Service
	}

	allowance struct {
		plan model.PlanCode
		max  int
	}

	permit struct {
		visible   bool
		canDelete bool
	}
)

func NewService(deps Deps) Service {
	return &service{
		savedTemplates: deps.SavedTemplates,
		columns:        deps.Columns,
		teams:          deps.Teams,
		organisations:  deps.Organisations,
		live:           deps.Live,
		orgAuthz:       deps.OrgAuthz,
		entitlements:   deps.Entitlements,
	}
}

func CleanName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrNameRequired
	}

	if utf8.RuneCountInString(name) > model.SavedTemplateNameMaxRunes {
		return "", ErrNameTooLong
	}

	return name, nil
}

func CleanDescription(raw string) (string, error) {
	description := strings.TrimSpace(raw)
	if utf8.RuneCountInString(description) > model.SavedTemplateDescriptionMaxRunes {
		return "", ErrDescriptionTooLong
	}

	return description, nil
}

func CleanScope(raw model.SavedTemplateScope) (model.SavedTemplateScope, error) {
	if raw != model.SavedTemplatePersonal && raw != model.SavedTemplateTeam {
		return "", fmt.Errorf("%w: %q", ErrInvalidScope, raw)
	}

	return raw, nil
}

func (s *service) List(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.SavedTemplateListResponse, error) {
	roles, err := s.viewableTeams(ctx, viewer)
	if err != nil {
		return nil, err
	}

	templates, err := s.savedTemplates.List(ctx, spec.SavedTemplateViewer(viewer))
	if err != nil {
		return nil, fmt.Errorf("list the saved templates of organisation %s for user %s: %w", viewer.OrgID, viewer.UserID, err)
	}

	response := dto.SavedTemplateListResponse{Templates: make([]dto.SavedTemplateResponse, 0, len(templates))}
	for _, template := range templates {
		if template.OrgID != viewer.OrgID {
			continue
		}

		allowed := permitFor(template, viewer.UserID, roles)
		if !allowed.visible {
			continue
		}

		response.Templates = append(response.Templates, dto.NewSavedTemplateResponse(template, allowed.canDelete))
	}

	return &response, nil
}

func (s *service) Save(ctx context.Context, command SaveCommand) (*dto.SavedTemplateResponse, error) {
	if command.Principal.Kind != model.PrincipalUser {
		return nil, fmt.Errorf("%w: only a signed-in member saves a template", ErrNotAllowed)
	}

	name, err := CleanName(command.Name)
	if err != nil {
		return nil, err
	}

	description, err := CleanDescription(command.Description)
	if err != nil {
		return nil, err
	}

	scope, err := CleanScope(command.Scope)
	if err != nil {
		return nil, err
	}

	access, err := s.live.Resolve(ctx, spec.BoardAccessQuery{Principal: command.Principal, BoardID: command.BoardID})
	if err != nil {
		return nil, err
	}

	if !access.OrgMember || !slices.Contains(access.Capabilities, model.BoardCapabilityManage) {
		return nil, fmt.Errorf("%w: user %s may not manage board %s", ErrNotAllowed, command.Principal.ID, command.BoardID)
	}

	header := access.Header
	place := spec.NewSavedTemplate{
		OrgID:       header.OrgID,
		CreatedBy:   command.Principal.ID,
		Name:        name,
		Description: description,
	}

	if scope == model.SavedTemplateTeam {
		teamID, err := s.teamScope(ctx, header, command.Principal.ID)
		if err != nil {
			return nil, err
		}

		place.TeamID = new(teamID)
	} else {
		place.OwnerID = new(command.Principal.ID)
	}

	columns, err := s.columns.List(ctx, header.ID)
	if err != nil {
		return nil, fmt.Errorf("list the columns of board %s: %w", header.ID, err)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("%w: board %s has none", ErrNoColumns, header.ID)
	}

	place.Columns = make([]model.SavedTemplateColumn, 0, len(columns))
	for _, column := range columns {
		place.Columns = append(place.Columns, model.SavedTemplateColumn{Title: column.Title, Description: column.Description, Colour: column.Colour})
	}

	allowed, err := s.templateAllowance(ctx, header.OrgID)
	if err != nil {
		return nil, err
	}

	current, err := s.savedTemplates.Count(ctx, header.OrgID)
	if err != nil {
		return nil, fmt.Errorf("count the saved templates of organisation %s: %w", header.OrgID, err)
	}

	if err := s.entitlements.CheckWithin(ctx, spec.LimitCheck{OrgID: header.OrgID, Limit: model.LimitSavedTemplates, Current: current}); err != nil {
		return nil, fmt.Errorf("check the saved template allowance of organisation %s: %w", header.OrgID, err)
	}

	place.MaxTemplates = allowed.max

	created, err := s.savedTemplates.Create(ctx, place)
	if errors.Is(err, dao.ErrQuotaExceeded) {
		return nil, &entitlements.DeniedError{Plan: allowed.plan, Limit: model.LimitSavedTemplates, Allowed: allowed.max, Current: allowed.max}
	}
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: the board's organisation, team or membership went away: %w", live.ErrNoAccess, err)
	}
	if err != nil {
		return nil, fmt.Errorf("save board %s as template %q: %w", header.ID, name, err)
	}

	if created.TeamID != nil {
		team, err := s.teams.GetByID(ctx, spec.TeamKey{OrgID: created.OrgID, TeamID: *created.TeamID})
		if err != nil {
			return nil, fmt.Errorf("read team %s of saved template %s: %w", *created.TeamID, created.ID, err)
		}

		created.TeamName = team.Name
	}

	response := dto.NewSavedTemplateResponse(*created, true)

	return &response, nil
}

func (s *service) Delete(ctx context.Context, removal spec.SavedTemplateRemoval) error {
	roles, err := s.viewableTeams(ctx, spec.OrganisationMemberKey{OrgID: removal.OrgID, UserID: removal.UserID})
	if err != nil {
		return err
	}

	key := spec.SavedTemplateKey{OrgID: removal.OrgID, ID: removal.ID}

	template, err := s.savedTemplates.Get(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrTemplateNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("read saved template %s of organisation %s: %w", removal.ID, removal.OrgID, err)
	}

	allowed := permitFor(*template, removal.UserID, roles)
	if !allowed.visible {
		return fmt.Errorf("%w: user %s cannot see saved template %s", ErrTemplateNotFound, removal.UserID, removal.ID)
	}

	if !allowed.canDelete {
		return fmt.Errorf("%w: user %s may not delete saved template %s", ErrDeleteNotAllowed, removal.UserID, removal.ID)
	}

	_, err = s.savedTemplates.Delete(ctx, key)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrTemplateNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("delete saved template %s of organisation %s: %w", removal.ID, removal.OrgID, err)
	}

	return nil
}

func (s *service) teamScope(ctx context.Context, header model.BoardHeader, userID uuid.UUID) (uuid.UUID, error) {
	if header.TeamID == nil {
		return uuid.Nil, fmt.Errorf("%w: board %s belongs to no team", ErrNotTeamBoard, header.ID)
	}

	teamID := *header.TeamID

	decision, err := s.orgAuthz.CanInTeam(ctx, spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: orgauthz.PermTeamCreateBoard})
	if err != nil {
		return uuid.Nil, fmt.Errorf("decide whether user %s may save a template for team %s: %w", userID, teamID, err)
	}

	if !decision.Allowed {
		return uuid.Nil, fmt.Errorf("%w: user %s may not create boards on team %s", ErrNotAllowed, userID, teamID)
	}

	return teamID, nil
}

func (s *service) viewableTeams(ctx context.Context, viewer spec.OrganisationMemberKey) (map[uuid.UUID]model.TeamRole, error) {
	organisation, err := s.organisations.GetByID(ctx, viewer.OrgID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrOrganisationNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read organisation %s: %w", viewer.OrgID, err)
	}

	if organisation.Deleted() {
		return nil, fmt.Errorf("%w: organisation %s is deleted", ErrOrganisationNotFound, viewer.OrgID)
	}

	member, err := s.organisations.GetMember(ctx, viewer)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: user %s is not a member: %w", ErrOrganisationNotFound, viewer.UserID, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the membership of user %s in organisation %s: %w", viewer.UserID, viewer.OrgID, err)
	}

	teams, err := s.teams.ListForOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the teams of organisation %s: %w", viewer.OrgID, err)
	}

	joined, err := s.teams.ListForMember(ctx, viewer)
	if err != nil {
		return nil, fmt.Errorf("list the teams of user %s in organisation %s: %w", viewer.UserID, viewer.OrgID, err)
	}

	memberships := make(map[uuid.UUID]model.TeamRole, len(joined))
	for _, membership := range joined {
		memberships[membership.ID] = membership.Role
	}

	viewable := make(map[uuid.UUID]model.TeamRole, len(teams))
	for _, team := range teams {
		role := orgauthz.EffectiveTeamRole(member.Role, memberships[team.ID])
		if orgauthz.TeamRoleGrants(role, orgauthz.PermTeamView) {
			viewable[team.ID] = role
		}
	}

	return viewable, nil
}

func (s *service) templateAllowance(ctx context.Context, orgID uuid.UUID) (allowance, error) {
	plan, err := s.entitlements.For(ctx, orgID)
	if err != nil {
		return allowance{}, fmt.Errorf("read the plan of organisation %s: %w", orgID, err)
	}

	allowed, known := plan.Allowance(model.LimitSavedTemplates)
	if !known {
		return allowance{}, fmt.Errorf("%w: %q on the %s plan", entitlements.ErrUnknownLimit, model.LimitSavedTemplates, plan.Code)
	}

	return allowance{plan: plan.Code, max: allowed}, nil
}

func permitFor(template model.SavedTemplate, userID uuid.UUID, roles map[uuid.UUID]model.TeamRole) permit {
	if template.TeamID == nil {
		owned := template.OwnerID != nil && *template.OwnerID == userID

		return permit{visible: owned, canDelete: owned}
	}

	role, ok := roles[*template.TeamID]
	if !ok {
		return permit{}
	}

	created := template.CreatedBy != nil && *template.CreatedBy == userID

	return permit{visible: true, canDelete: created || orgauthz.TeamRoleGrants(role, orgauthz.PermTeamManageBoards)}
}
