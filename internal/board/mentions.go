package board

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
)

type (
	Members struct {
		Organisations repository.OrganisationRepository
		Teams         repository.TeamRepository
	}
)

func (s *service) People(ctx context.Context, query spec.BoardAccessQuery) (*dto.BoardPeopleResponse, error) {
	access, err := s.live.Resolve(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := policy.CanNameMembers(*access); err != nil {
		return nil, err
	}

	people, err := s.members.people(ctx, access.Header)
	if err != nil {
		return nil, err
	}

	response := projection.People(people)

	return &response, nil
}

func (m Members) CheckNamed(ctx context.Context, access model.BoardAccess, added []uuid.UUID) error {
	if len(added) == 0 {
		return nil
	}

	if err := policy.CanNameMembers(access); err != nil {
		return err
	}

	people, err := m.people(ctx, access.Header)
	if err != nil {
		return err
	}

	for _, userID := range added {
		named := slices.ContainsFunc(people, func(person model.ActionItemPerson) bool {
			return person.UserID == userID
		})
		if !named {
			return fmt.Errorf("%w: user %s cannot open board %s", ErrNotBoardMember, userID, access.Header.ID)
		}
	}

	return nil
}

func (m Members) people(ctx context.Context, header model.BoardHeader) ([]model.ActionItemPerson, error) {
	members, err := m.Organisations.ListMembers(ctx, header.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the members of organisation %s: %w", header.OrgID, err)
	}

	teamRoles := map[uuid.UUID]model.TeamRole{}
	if header.TeamID != nil {
		teamMembers, err := m.Teams.ListMembers(ctx, spec.TeamKey{OrgID: header.OrgID, TeamID: *header.TeamID})
		if err != nil {
			return nil, fmt.Errorf("list the members of team %s: %w", *header.TeamID, err)
		}

		for _, member := range teamMembers {
			teamRoles[member.UserID] = member.Role
		}
	}

	people := make([]model.ActionItemPerson, 0, len(members))
	for _, member := range members {
		if member.OrgID != header.OrgID {
			continue
		}

		role := orgauthz.EffectiveTeamRole(member.Role, teamRoles[member.UserID])
		if header.TeamID != nil && !orgauthz.TeamRoleGrants(role, orgauthz.PermTeamView) {
			continue
		}

		people = append(people, model.ActionItemPerson{UserID: member.UserID, DisplayName: member.DisplayName})
	}

	slices.SortFunc(people, func(a, b model.ActionItemPerson) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)), strings.Compare(a.UserID.String(), b.UserID.String()))
	})

	return people, nil
}

func NewMentions(mentions []uuid.UUID, current []model.ActionItemPerson) []uuid.UUID {
	var added []uuid.UUID
	for _, userID := range mentions {
		kept := slices.ContainsFunc(current, func(person model.ActionItemPerson) bool {
			return person.UserID == userID
		})
		if !kept {
			added = append(added, userID)
		}
	}

	return added
}

func CleanMentions(raw []uuid.UUID) ([]uuid.UUID, error) {
	var mentions []uuid.UUID
	for _, userID := range raw {
		if userID == uuid.Nil {
			return nil, fmt.Errorf("%w: an empty user ID", ErrNotBoardMember)
		}

		if !slices.Contains(mentions, userID) {
			mentions = append(mentions, userID)
		}
	}

	if len(mentions) > model.MaxMentions {
		return nil, ErrTooManyMentions
	}

	return mentions, nil
}
