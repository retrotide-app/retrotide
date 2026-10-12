package actionitem

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Service interface {
		List(ctx context.Context, query ListQuery) (*dto.ActionItemListResponse, error)
		Mentions(ctx context.Context, userID uuid.UUID) (*dto.MentionListResponse, error)
	}

	ListQuery struct {
		Viewer             spec.OrganisationMemberKey
		Status             *model.ActionItemStatus
		AssigneeUserID     *uuid.UUID
		Unassigned         bool
		TeamID             *uuid.UUID
		OrganisationBoards bool
		Limit              int
		Offset             int
	}

	Deps struct {
		ActionItems   repository.ActionItemRepository
		Cards         repository.CardRepository
		Comments      repository.CardCommentRepository
		Organisations repository.OrganisationRepository
		OrgAuthz      orgauthz.Service
	}

	service struct {
		actionItems   repository.ActionItemRepository
		cards         repository.CardRepository
		comments      repository.CardCommentRepository
		organisations repository.OrganisationRepository
		orgAuthz      orgauthz.Service
	}

	accessKey struct {
		orgID  uuid.UUID
		teamID uuid.UUID
	}
)

const (
	DefaultLimit = 50
	MaxLimit     = 200

	mentionsRead  = 200
	mentionsShown = 100
)

func NewService(deps Deps) Service {
	return &service{
		actionItems:   deps.ActionItems,
		cards:         deps.Cards,
		comments:      deps.Comments,
		organisations: deps.Organisations,
		orgAuthz:      deps.OrgAuthz,
	}
}

func (s *service) List(ctx context.Context, query ListQuery) (*dto.ActionItemListResponse, error) {
	if query.Status != nil && !slices.Contains(model.ActionItemStatuses, *query.Status) {
		return nil, ErrInvalidStatus
	}

	member, err := s.liveMember(ctx, query.Viewer)
	if err != nil {
		return nil, err
	}

	limit := query.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	offset := max(query.Offset, 0)

	everyTeam := orgauthz.TeamRoleGrants(orgauthz.EffectiveTeamRole(member.Role, ""), orgauthz.PermTeamView)

	page, err := s.actionItems.ListPage(ctx, spec.ActionItemFilter{
		OrgID:              query.Viewer.OrgID,
		ViewerID:           query.Viewer.UserID,
		EveryTeam:          everyTeam,
		Status:             query.Status,
		AssigneeUserID:     query.AssigneeUserID,
		Unassigned:         query.Unassigned,
		TeamID:             query.TeamID,
		OrganisationBoards: query.OrganisationBoards,
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list the action items of organisation %s for user %s: %w", query.Viewer.OrgID, query.Viewer.UserID, err)
	}

	total := page.Counts.Total()
	if query.Status != nil {
		total = page.Counts.Of(*query.Status)
	}

	response := projection.ActionItemList(*page, limit, offset, total)

	return &response, nil
}

func (s *service) Mentions(ctx context.Context, userID uuid.UUID) (*dto.MentionListResponse, error) {
	query := spec.UserMentionsQuery{UserID: userID, Limit: mentionsRead}

	items, err := s.actionItems.ListMentionsForUser(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list the action item mentions of user %s: %w", userID, err)
	}

	cards, err := s.cards.ListMentionsForUser(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list the card mentions of user %s: %w", userID, err)
	}

	comments, err := s.comments.ListMentionsForUser(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list the comment mentions of user %s: %w", userID, err)
	}

	mentions := slices.Concat(items, cards, comments)
	slices.SortStableFunc(mentions, func(a, b model.Mention) int {
		return b.MentionedAt.Compare(a.MentionedAt)
	})

	decided := map[accessKey]bool{}
	visible := make([]model.Mention, 0, min(len(mentions), mentionsShown))
	for _, mention := range mentions {
		if len(visible) == mentionsShown {
			break
		}

		if !projection.MentionShown(mention) {
			continue
		}

		key := accessKey{orgID: mention.OrgID}
		if mention.TeamID != nil {
			key.teamID = *mention.TeamID
		}

		allowed, known := decided[key]
		if !known {
			allowed, err = s.mayOpen(ctx, userID, key)
			if err != nil {
				return nil, err
			}

			decided[key] = allowed
		}

		if allowed {
			visible = append(visible, mention)
		}
	}

	response := projection.Mentions(visible)

	return &response, nil
}

func (s *service) mayOpen(ctx context.Context, userID uuid.UUID, key accessKey) (bool, error) {
	if key.teamID == uuid.Nil {
		decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: key.orgID, UserID: userID, Permission: orgauthz.PermOrgView})
		if err != nil {
			return false, fmt.Errorf("decide whether user %s may open the boards of organisation %s: %w", userID, key.orgID, err)
		}

		return decision.Allowed, nil
	}

	decision, err := s.orgAuthz.CanInTeam(ctx, spec.TeamPermissionCheck{TeamID: key.teamID, UserID: userID, Permission: orgauthz.PermTeamView})
	if err != nil {
		return false, fmt.Errorf("decide whether user %s may open the boards of team %s: %w", userID, key.teamID, err)
	}

	return decision.Allowed, nil
}

func (s *service) liveMember(ctx context.Context, viewer spec.OrganisationMemberKey) (*model.OrganisationMember, error) {
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

	return member, nil
}
