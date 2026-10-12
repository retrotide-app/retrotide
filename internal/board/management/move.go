package management

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
)

func (s *service) Move(ctx context.Context, command MoveCommand) (*dto.BoardCommandResponse, error) {
	move := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanMoveBoard(access); err != nil {
			return nil, err
		}

		if err := s.checkDestination(ctx, access, command.TeamID); err != nil {
			return nil, err
		}

		if sameTeam(access.Header.TeamID, command.TeamID) {
			return nil, fmt.Errorf("%w: board %s", ErrSameTeam, access.Header.ID)
		}

		written, err := s.boards.MoveBoard(ctx, spec.BoardTeamWrite{BoardWrite: target, TeamID: command.TeamID})
		if err != nil {
			return nil, fmt.Errorf("move board %s: %w", target.BoardID, err)
		}

		if err := s.settle(ctx, access, written); err != nil {
			return nil, err
		}

		for _, team := range []*uuid.UUID{access.Header.TeamID, command.TeamID} {
			if team != nil {
				s.events.AnnounceTeamActionItems(*team, []uuid.UUID{access.Header.ID})
			}
		}

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, board.VersionReplay, move)
}

func (s *service) checkDestination(ctx context.Context, access model.BoardAccess, teamID *uuid.UUID) error {
	userID := access.Principal.ID

	if teamID == nil {
		decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: access.Header.OrgID, UserID: userID, Permission: orgauthz.PermOrgCreateBoard})
		if err != nil {
			return fmt.Errorf("decide whether user %s may hold a board in organisation %s: %w", userID, access.Header.OrgID, err)
		}

		if !decision.Allowed {
			return fmt.Errorf("%w: user %s may not create boards in organisation %s", ErrMoveNotAllowed, userID, access.Header.OrgID)
		}

		return nil
	}

	team, err := s.teams.GetHeader(ctx, *teamID)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrTeamNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("read the header of team %s: %w", *teamID, err)
	}

	if team.OrgID != access.Header.OrgID {
		return fmt.Errorf("%w: team %s belongs to organisation %s, not %s", ErrTeamNotFound, team.ID, team.OrgID, access.Header.OrgID)
	}

	if team.Archived() {
		return fmt.Errorf("%w: team %s", ErrTeamArchived, team.ID)
	}

	decision, err := s.orgAuthz.CanInTeam(ctx, spec.TeamPermissionCheck{TeamID: team.ID, UserID: userID, Permission: orgauthz.PermTeamCreateBoard})
	if err != nil {
		return fmt.Errorf("decide whether user %s may hold a board on team %s: %w", userID, team.ID, err)
	}

	if !decision.Allowed {
		return fmt.Errorf("%w: user %s may not create boards on team %s", ErrMoveNotAllowed, userID, team.ID)
	}

	return nil
}

func sameTeam(current *uuid.UUID, wanted *uuid.UUID) bool {
	if current == nil || wanted == nil {
		return current == nil && wanted == nil
	}

	return *current == *wanted
}
