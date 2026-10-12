package management

import (
	"context"
	"errors"
	"fmt"

	"retrotide/internal/audit"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
)

func (s *service) Restore(ctx context.Context, command RestoreCommand) error {
	if command.Principal.Kind != model.PrincipalUser {
		return fmt.Errorf("%w: only a signed-in member restores a board", ErrRestoreNotAllowed)
	}

	header, err := s.boards.GetByID(ctx, spec.BoardKey{OrgID: command.OrgID, BoardID: command.BoardID})
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: %w", ErrBoardNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("read board %s of organisation %s: %w", command.BoardID, command.OrgID, err)
	}

	if !header.Deleted() || header.OrgDeletedAt != nil {
		return fmt.Errorf("%w: board %s is not waiting to be restored", ErrBoardNotFound, header.ID)
	}

	allowed, err := s.restorable(ctx, command.Principal.ID, *header)
	if err != nil {
		return err
	}

	if !allowed {
		return fmt.Errorf("%w: user %s on board %s", ErrRestoreNotAllowed, command.Principal.ID, header.ID)
	}

	restore := spec.BoardRestore{OrgID: header.OrgID, BoardID: header.ID, DeletedSince: s.now().Add(-model.BoardRestoreWindow)}

	_, err = s.boards.Restore(ctx, restore)
	if errors.Is(err, dao.ErrNotFound) {
		return fmt.Errorf("%w: board %s was deleted more than %s ago: %w", ErrBoardNotFound, header.ID, model.BoardRestoreWindow, err)
	}
	if err != nil {
		return fmt.Errorf("restore board %s: %w", header.ID, err)
	}

	entry := audit.NewEntry{
		OrgID:      header.OrgID,
		ActorID:    command.Principal.ID,
		ActorKind:  audit.ActorUser,
		Action:     audit.ActionBoardRestored,
		TargetType: audit.TargetBoard,
		TargetID:   header.ID.String(),
		SubjectID:  command.Principal.ID,
	}
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return nil
}

func (s *service) ListDeleted(ctx context.Context, viewer spec.OrganisationMemberKey) (*dto.DeletedBoardListResponse, error) {
	query := spec.DeletedBoardsQuery{OrgID: viewer.OrgID, DeletedSince: s.now().Add(-model.BoardRestoreWindow)}

	deleted, err := s.boards.ListDeletedForOrganisation(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list the deleted boards of organisation %s: %w", viewer.OrgID, err)
	}

	restorable := make([]model.BoardHeader, 0, len(deleted))
	for _, header := range deleted {
		if header.OrgID != viewer.OrgID {
			continue
		}

		allowed, err := s.restorable(ctx, viewer.UserID, header)
		if err != nil {
			return nil, err
		}

		if allowed {
			restorable = append(restorable, header)
		}
	}

	teams, err := s.teams.ListForOrganisation(ctx, viewer.OrgID)
	if err != nil {
		return nil, fmt.Errorf("list the teams of organisation %s: %w", viewer.OrgID, err)
	}

	names := make(map[uuid.UUID]string, len(teams))
	for _, team := range teams {
		names[team.ID] = team.Name
	}

	response := projection.DeletedBoardList(restorable, names)

	return &response, nil
}

func (s *service) restorable(ctx context.Context, userID uuid.UUID, header model.BoardHeader) (bool, error) {
	manages, err := s.managesBoards(ctx, userID, header)
	if err != nil {
		return false, err
	}

	if manages {
		return true, nil
	}

	if header.CreatedBy == nil || *header.CreatedBy != userID {
		return false, nil
	}

	decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: header.OrgID, UserID: userID, Permission: orgauthz.PermOrgView})
	if err != nil {
		return false, fmt.Errorf("decide whether user %s is still in organisation %s: %w", userID, header.OrgID, err)
	}

	return decision.Member, nil
}

func (s *service) managesBoards(ctx context.Context, userID uuid.UUID, header model.BoardHeader) (bool, error) {
	if header.TeamID == nil {
		decision, err := s.orgAuthz.CanInOrg(ctx, spec.OrgPermissionCheck{OrgID: header.OrgID, UserID: userID, Permission: orgauthz.PermOrgManageBoards})
		if err != nil {
			return false, fmt.Errorf("decide whether user %s manages the boards of organisation %s: %w", userID, header.OrgID, err)
		}

		return decision.Allowed, nil
	}

	decision, err := s.orgAuthz.CanInTeam(ctx, spec.TeamPermissionCheck{TeamID: *header.TeamID, UserID: userID, Permission: orgauthz.PermTeamManageBoards})
	if err != nil {
		return false, fmt.Errorf("decide whether user %s manages the boards of team %s: %w", userID, *header.TeamID, err)
	}

	return decision.Allowed, nil
}
