package board

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/boardtemplate"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
)

func (s *service) GetSnapshot(ctx context.Context, query spec.BoardSnapshotQuery) (*dto.BoardSnapshot, error) {
	access, err := s.live.Resolve(ctx, spec.BoardAccessQuery{Principal: query.Principal, BoardID: query.BoardID})
	if err != nil {
		return nil, err
	}

	if query.KnownVersion != nil && *query.KnownVersion == access.Header.Version {
		return nil, ErrNotModified
	}

	state, err := s.boards.GetSnapshot(ctx, spec.BoardKey{OrgID: access.Header.OrgID, BoardID: access.Header.ID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: board %s went away after access was resolved: %w", live.ErrNoAccess, access.Header.ID, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the snapshot of board %s: %w", access.Header.ID, err)
	}

	viewer := projection.ViewerFor(*access)

	if state.Header.Anonymous() && state.Header.CardsHidden {
		if tally, published := s.events.PublishedTally(access.Header.ID); published {
			snapshot := projection.WithServerNow(projection.SnapshotFromTally(*state, viewer, tally), s.now())

			return &snapshot, nil
		}
	}

	snapshot := projection.WithServerNow(projection.Snapshot(*state, viewer), s.now())

	return &snapshot, nil
}

func (s *service) List(ctx context.Context, teamID uuid.UUID) (*dto.BoardListResponse, error) {
	team, err := s.liveTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	boards, err := s.boards.ListForTeam(ctx, spec.TeamKey{OrgID: team.OrgID, TeamID: team.ID})
	if err != nil {
		return nil, fmt.Errorf("list the boards of team %s: %w", team.ID, err)
	}

	response := projection.BoardList(boards)

	return &response, nil
}

func (s *service) ListForOrganisation(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.OrgBoardListResponse, error) {
	if err := s.liveOrganisation(ctx, viewer.OrgID); err != nil {
		return nil, err
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

	roles := make(map[uuid.UUID]model.TeamRole, len(joined))
	for _, membership := range joined {
		roles[membership.ID] = membership.Role
	}

	viewable := make(map[uuid.UUID]string, len(teams))
	for _, team := range teams {
		role := orgauthz.EffectiveTeamRole(member.Role, roles[team.ID])
		if orgauthz.TeamRoleGrants(role, orgauthz.PermTeamView) {
			viewable[team.ID] = team.Name
		}
	}

	boards, err := s.boards.ListForOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the boards of organisation %s: %w", viewer.OrgID, err)
	}

	visible := make([]model.BoardHeader, 0, len(boards))
	for _, header := range boards {
		if header.OrgID != viewer.OrgID {
			continue
		}

		if header.TeamID == nil {
			visible = append(visible, header)
			continue
		}

		if _, ok := viewable[*header.TeamID]; ok {
			visible = append(visible, header)
		}
	}

	pins, err := s.boards.ListPinned(ctx, viewer)
	if err != nil {
		return nil, fmt.Errorf("list the boards user %s pinned in organisation %s: %w", viewer.UserID, viewer.OrgID, err)
	}

	pinned := make(map[uuid.UUID]struct{}, len(pins))
	for _, boardID := range pins {
		pinned[boardID] = struct{}{}
	}

	response := projection.OrgBoardList(visible, viewable, pinned)

	return &response, nil
}

func (s *service) Create(ctx context.Context, command CreateCommand) (*dto.BoardSnapshot, error) {
	if command.Principal.Kind != model.PrincipalUser {
		return nil, fmt.Errorf("%w: only a signed-in member creates a board", policy.ErrNotAllowed)
	}

	title, err := cleanTitle(command.Title)
	if err != nil {
		return nil, err
	}

	line, err := cleanContext(command.Context)
	if err != nil {
		return nil, err
	}

	anonymity, err := cleanAnonymity(command.Anonymity)
	if err != nil {
		return nil, err
	}

	var template boardtemplate.Template
	if command.SavedTemplateID == nil {
		found, ok := boardtemplate.Lookup(command.TemplateKey)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownTemplate, command.TemplateKey)
		}

		template = found
	}

	target, err := s.creationPlace(ctx, command)
	if err != nil {
		return nil, err
	}

	if command.SavedTemplateID != nil {
		template, err = s.savedTemplate(ctx, *command.SavedTemplateID, target.orgID, command.Principal.ID)
		if err != nil {
			return nil, err
		}
	}

	positions := fracindex.Spread(len(template.Columns))
	columns := make([]spec.BoardColumnSeed, 0, len(template.Columns))
	for i, column := range template.Columns {
		columns = append(columns, spec.BoardColumnSeed{
			ID:          uuid.New(),
			Title:       column.Title,
			Description: column.Description,
			Colour:      column.Colour,
			Position:    positions[i],
		})
	}

	state, err := s.boards.CreateBoard(ctx, spec.NewBoard{
		OrgID:       target.orgID,
		TeamID:      target.teamID,
		Title:       title,
		ShareSlug:   rand.Text(),
		CreatedBy:   command.Principal.ID,
		Context:     line,
		TemplateKey: template.Key,
		Anonymity:   anonymity,
		CardsHidden: !command.CardsShown,
		Columns:     columns,
	})
	if err != nil {
		return nil, fmt.Errorf("create a board in organisation %s: %w", target.orgID, err)
	}

	creator := model.BoardAccess{
		Principal:    command.Principal,
		Header:       state.Header,
		Role:         model.BoardRoleFacilitator,
		OrgMember:    true,
		Capabilities: policy.Capabilities(model.BoardRoleFacilitator, true),
	}
	for _, participant := range state.Participants {
		if participant.UserID != nil && *participant.UserID == command.Principal.ID {
			creator.ParticipantID = new(participant.ID)
		}
	}

	snapshot := projection.Snapshot(*state, projection.ViewerFor(creator))

	return &snapshot, nil
}

func (s *service) Describe(ctx context.Context, command DescribeCommand) (*dto.BoardCommandResponse, error) {
	line, err := cleanContext(command.Context)
	if err != nil {
		return nil, err
	}

	describe := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageBoard(access); err != nil {
			return nil, err
		}

		written, err := s.boards.SetBoardContext(ctx, spec.BoardContextWrite{BoardWrite: target, Context: line})
		if err != nil {
			return nil, fmt.Errorf("describe board %s: %w", target.BoardID, err)
		}

		s.events.Publish(access, projection.BoardUpdated(written.Value, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, describe)
}

func (s *service) savedTemplate(ctx context.Context, templateID uuid.UUID, orgID uuid.UUID, userID uuid.UUID) (boardtemplate.Template, error) {
	saved, err := s.savedTemplates.Get(ctx, spec.SavedTemplateKey{OrgID: orgID, ID: templateID})
	if errors.Is(err, dao.ErrNotFound) {
		return boardtemplate.Template{}, fmt.Errorf("%w: saved template %s: %w", ErrUnknownTemplate, templateID, err)
	}
	if err != nil {
		return boardtemplate.Template{}, fmt.Errorf("read saved template %s of organisation %s: %w", templateID, orgID, err)
	}

	if saved.TeamID == nil && (saved.OwnerID == nil || *saved.OwnerID != userID) {
		return boardtemplate.Template{}, fmt.Errorf("%w: saved template %s belongs to another member", ErrUnknownTemplate, templateID)
	}

	if saved.TeamID != nil {
		decision, err := s.orgAuthz.CanInTeam(ctx, spec.TeamPermissionCheck{TeamID: *saved.TeamID, UserID: userID, Permission: orgauthz.PermTeamView})
		if err != nil {
			return boardtemplate.Template{}, fmt.Errorf("decide whether user %s may use saved template %s: %w", userID, templateID, err)
		}

		if !decision.Allowed {
			return boardtemplate.Template{}, fmt.Errorf("%w: user %s cannot see team %s", ErrUnknownTemplate, userID, *saved.TeamID)
		}
	}

	columns := make([]boardtemplate.Column, 0, len(saved.Columns))
	for _, column := range saved.Columns {
		columns = append(columns, boardtemplate.Column{Title: column.Title, Description: column.Description, Colour: column.Colour})
	}

	return boardtemplate.Template{Title: saved.Name, Description: saved.Description, Columns: columns}, nil
}

func (s *service) creationPlace(ctx context.Context, command CreateCommand) (boardHome, error) {
	if command.TeamID == nil {
		return s.organisationPlace(ctx, command)
	}

	team, err := s.liveTeam(ctx, *command.TeamID)
	if err != nil {
		return boardHome{}, err
	}

	if command.OrgID != uuid.Nil && team.OrgID != command.OrgID {
		return boardHome{}, fmt.Errorf("%w: team %s belongs to organisation %s, not %s", ErrTeamNotFound, team.ID, team.OrgID, command.OrgID)
	}

	decision, err := s.orgAuthz.CanInTeam(ctx, spec.TeamPermissionCheck{TeamID: team.ID, UserID: command.Principal.ID, Permission: orgauthz.PermTeamCreateBoard})
	if err != nil {
		return boardHome{}, fmt.Errorf("decide whether user %s may create a board on team %s: %w", command.Principal.ID, team.ID, err)
	}

	if !decision.Member {
		return boardHome{}, fmt.Errorf("%w: user %s is not in organisation %s", ErrTeamNotFound, command.Principal.ID, team.OrgID)
	}

	if !decision.Allowed {
		return boardHome{}, fmt.Errorf("%w: user %s may not create a board on team %s", ErrCreateNotAllowed, command.Principal.ID, team.ID)
	}

	if team.Archived() {
		return boardHome{}, ErrTeamArchived
	}

	return boardHome{orgID: team.OrgID, teamID: new(team.ID)}, nil
}

func (s *service) organisationPlace(ctx context.Context, command CreateCommand) (boardHome, error) {
	if err := s.liveOrganisation(ctx, command.OrgID); err != nil {
		return boardHome{}, err
	}

	decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: command.OrgID, UserID: command.Principal.ID, Permission: orgauthz.PermOrgCreateBoard})
	if err != nil {
		return boardHome{}, fmt.Errorf("decide whether user %s may create a board in organisation %s: %w", command.Principal.ID, command.OrgID, err)
	}

	if !decision.Member {
		return boardHome{}, fmt.Errorf("%w: user %s is not a member", ErrOrganisationNotFound, command.Principal.ID)
	}

	if !decision.Allowed {
		return boardHome{}, fmt.Errorf("%w: user %s may not create a board in organisation %s", ErrCreateNotAllowed, command.Principal.ID, command.OrgID)
	}

	return boardHome{orgID: command.OrgID}, nil
}

func (s *service) liveOrganisation(ctx context.Context, orgID uuid.UUID) error {
	organisation, err := s.organisations.GetByID(ctx, orgID)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrOrganisationNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("read organisation %s: %w", orgID, err)
	}

	if organisation.Deleted() {
		return fmt.Errorf("%w: organisation %s is deleted", ErrOrganisationNotFound, orgID)
	}

	return nil
}

func (s *service) liveTeam(ctx context.Context, teamID uuid.UUID) (*model.Team, error) {
	team, err := s.teams.GetHeader(ctx, teamID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrTeamNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the header of team %s: %w", teamID, err)
	}

	organisation, err := s.organisations.GetByID(ctx, team.OrgID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: organisation %s is gone: %w", ErrTeamNotFound, team.OrgID, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read organisation %s: %w", team.OrgID, err)
	}

	if organisation.Deleted() {
		return nil, fmt.Errorf("%w: organisation %s is deleted", ErrTeamNotFound, team.OrgID)
	}

	return team, nil
}

func createdReplay(id uuid.UUID) Replay {
	return func(_ model.BoardAccess, version int64) (*dto.BoardCommandResponse, error) {
		response := projection.ReplayResponse(version, &id)

		return &response, nil
	}
}
