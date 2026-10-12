package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		Board(ctx context.Context, query BoardQuery) (*Download, error)
		Organisation(ctx context.Context, viewer spec.OrganisationMemberKey) (*Download, error)
		Boards(ctx context.Context, query BoardsQuery) (*Download, error)
		Self(ctx context.Context, userID uuid.UUID) (*Download, error)
	}

	BoardQuery struct {
		Access spec.BoardAccessQuery
		Format engine.Format
	}

	BoardsQuery struct {
		Viewer   spec.OrganisationMemberKey
		BoardIDs []uuid.UUID
	}

	Download struct {
		Title  string
		Engine engine.Engine
		Write  func(w io.Writer) error
	}

	Deps struct {
		Engines       *EngineManager
		Boards        repository.BoardRepository
		Teams         repository.TeamRepository
		Organisations repository.OrganisationRepository
		Users         repository.UserRepository
		Identities    repository.UserIdentityRepository
		Cards         repository.CardRepository
		Comments      repository.CardCommentRepository
		ActionItems   repository.ActionItemRepository
		Live          live.Service
		OrgAuthz      orgauthz.Service
	}

	service struct {
		engines       *EngineManager
		boards        repository.BoardRepository
		teams         repository.TeamRepository
		organisations repository.OrganisationRepository
		users         repository.UserRepository
		identities    repository.UserIdentityRepository
		cards         repository.CardRepository
		comments      repository.CardCommentRepository
		actionItems   repository.ActionItemRepository
		live          live.Service
		orgAuthz      orgauthz.Service
	}
)

const (
	boardsTitleSuffix = " - boards"
)

func NewService(deps Deps) Service {
	return &service{
		engines:       deps.Engines,
		boards:        deps.Boards,
		teams:         deps.Teams,
		organisations: deps.Organisations,
		users:         deps.Users,
		identities:    deps.Identities,
		cards:         deps.Cards,
		comments:      deps.Comments,
		actionItems:   deps.ActionItems,
		live:          deps.Live,
		orgAuthz:      deps.OrgAuthz,
	}
}

func (s *service) Board(ctx context.Context, query BoardQuery) (*Download, error) {
	format, err := s.engines.Engine(query.Format)
	if err != nil {
		return nil, err
	}

	access, err := s.live.Resolve(ctx, query.Access)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(access.Capabilities, model.BoardCapabilityExport) {
		return nil, fmt.Errorf("%w: %s %s may not export board %s", policy.ErrNotAllowed, access.Principal.Kind, access.Principal.ID, access.Header.ID)
	}

	board, err := s.project(ctx, *access)
	if err != nil {
		return nil, err
	}

	write := func(w io.Writer) error {
		return format.WriteBoard(w, *board)
	}

	return &Download{Title: board.Title, Engine: format, Write: write}, nil
}

func (s *service) Organisation(ctx context.Context, viewer spec.OrganisationMemberKey) (*Download, error) {
	decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: viewer.OrgID, UserID: viewer.UserID, Permission: orgauthz.PermOrgExport})
	if err != nil {
		return nil, fmt.Errorf("decide whether user %s may export organisation %s: %w", viewer.UserID, viewer.OrgID, err)
	}

	if !decision.Member {
		return nil, fmt.Errorf("%w: user %s is not a member of organisation %s", ErrOrganisationNotFound, viewer.UserID, viewer.OrgID)
	}

	if !decision.Allowed {
		return nil, fmt.Errorf("%w: user %s in organisation %s", ErrOrganisationNotAllowed, viewer.UserID, viewer.OrgID)
	}

	organisation, err := s.liveOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, err
	}

	headers, err := s.boards.ListForOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the boards of organisation %s: %w", viewer.OrgID, err)
	}

	return s.organisationDownload(ctx, viewer, *organisation, headers, organisation.Name)
}

func (s *service) Boards(ctx context.Context, query BoardsQuery) (*Download, error) {
	viewer := query.Viewer

	decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: viewer.OrgID, UserID: viewer.UserID, Permission: orgauthz.PermOrgView})
	if err != nil {
		return nil, fmt.Errorf("decide whether user %s may export boards of organisation %s: %w", viewer.UserID, viewer.OrgID, err)
	}

	if !decision.Member {
		return nil, fmt.Errorf("%w: user %s is not a member of organisation %s", ErrOrganisationNotFound, viewer.UserID, viewer.OrgID)
	}

	organisation, err := s.liveOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, err
	}

	headers, err := s.boards.ListForOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the boards of organisation %s: %w", viewer.OrgID, err)
	}

	chosen := make([]model.BoardHeader, 0, len(query.BoardIDs))
	for _, header := range headers {
		if slices.Contains(query.BoardIDs, header.ID) {
			chosen = append(chosen, header)
		}
	}

	return s.organisationDownload(ctx, viewer, *organisation, chosen, organisation.Name+boardsTitleSuffix)
}

func (s *service) liveOrganisation(ctx context.Context, orgID uuid.UUID) (*model.Organisation, error) {
	organisation, err := s.organisations.GetByID(ctx, orgID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrOrganisationNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read organisation %s: %w", orgID, err)
	}

	if organisation.Deleted() {
		return nil, fmt.Errorf("%w: organisation %s is deleted", ErrOrganisationNotFound, orgID)
	}

	return organisation, nil
}

func (s *service) organisationDownload(ctx context.Context, viewer spec.OrganisationMemberKey, organisation model.Organisation, headers []model.BoardHeader, title string) (*Download, error) {
	teams, err := s.teams.ListForOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the teams of organisation %s: %w", viewer.OrgID, err)
	}

	boards := newOrganisationBoards(s, viewer, headers, teams)
	if err := boards.load(ctx); err != nil {
		return nil, err
	}

	header := dto.OrganisationExportHeader{Name: organisation.Name, Slug: organisation.Slug}

	write := func(w io.Writer) error {
		return boards.write(ctx, w, header)
	}

	return &Download{Title: title, Engine: engines.JSON{}, Write: write}, nil
}

func (s *service) Self(ctx context.Context, userID uuid.UUID) (*Download, error) {
	user, err := s.users.GetByID(ctx, userID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrAccountNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read user %s: %w", userID, err)
	}

	identities, err := s.identities.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the sign-in identities of user %s: %w", userID, err)
	}

	organisations, err := s.organisations.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the organisations of user %s: %w", userID, err)
	}

	memberships := make([]dto.SelfExportMembership, 0, len(organisations))
	for _, organisation := range organisations {
		teams, err := s.teams.ListForMember(ctx, spec.OrganisationMemberKey{OrgID: organisation.ID, UserID: userID})
		if err != nil {
			return nil, fmt.Errorf("list the teams of user %s in organisation %s: %w", userID, organisation.ID, err)
		}

		memberships = append(memberships, dto.NewSelfExportMembership(organisation, teams))
	}

	cards, err := s.cards.ListWrittenByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the cards of user %s: %w", userID, err)
	}

	comments, err := s.comments.ListWrittenByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the comments of user %s: %w", userID, err)
	}

	items, err := s.actionItems.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list the action items of user %s: %w", userID, err)
	}

	document := dto.SelfExport{
		Account:       dto.NewSelfExportAccount(*user),
		Identities:    dto.NewSelfExportIdentities(identities),
		Organisations: memberships,
		Cards:         dto.NewSelfExportCards(cards),
		Comments:      dto.NewSelfExportComments(comments),
		ActionItems:   dto.NewSelfExportActionItems(items),
	}

	write := func(w io.Writer) error {
		return engines.JSON{}.WriteDocument(w, document)
	}

	return &Download{Title: user.DisplayName, Engine: engines.JSON{}, Write: write}, nil
}

func (s *service) project(ctx context.Context, access model.BoardAccess) (*dto.BoardExport, error) {
	state, err := s.boards.GetSnapshot(ctx, spec.BoardKey{OrgID: access.Header.OrgID, BoardID: access.Header.ID})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: board %s went away after access was resolved: %w", live.ErrNoAccess, access.Header.ID, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read the snapshot of board %s: %w", access.Header.ID, err)
	}

	tally := projection.Tally(state.Header.Board, state.Cards)
	if state.Header.Anonymous() && state.Header.CardsHidden {
		if published, ok := s.live.PublishedTally(access.Header.ID); ok {
			tally = published
		}
	}

	board := projection.Export(*state, projection.ViewerFor(access), tally)

	return &board, nil
}
