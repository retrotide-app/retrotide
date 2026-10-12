package management

import (
	"context"
	"fmt"
	"strconv"

	"retrotide/internal/audit"
	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

const (
	archivedDetail = "archived"
)

func (s *service) Archive(ctx context.Context, command board.Command) (*dto.BoardCommandResponse, error) {
	archive := s.archiver(ctx, true, uuid.Nil)

	return s.Run(ctx, command, board.VersionReplay, archive)
}

func (s *service) Unarchive(ctx context.Context, command board.Command) (*dto.BoardCommandResponse, error) {
	unarchive := s.archiver(ctx, false, uuid.Nil)

	return s.Run(ctx, command, board.VersionReplay, unarchive)
}

func (s *service) archiver(ctx context.Context, archived bool, orgID uuid.UUID) board.Write {
	return func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := inOrganisation(access, orgID); err != nil {
			return nil, err
		}

		if err := policy.CanArchiveBoard(access); err != nil {
			return nil, err
		}

		written, err := s.boards.ArchiveBoard(ctx, spec.BoardArchiveWrite{BoardWrite: target, Archived: archived})
		if err != nil {
			return nil, fmt.Errorf("set board %s archived to %t: %w", target.BoardID, archived, err)
		}

		if err := s.settle(ctx, access, written); err != nil {
			return nil, err
		}

		response := projection.VersionResponse(written.Version)

		entry := boardEntry(access, audit.ActionBoardArchived)
		entry.Details = map[string]string{archivedDetail: strconv.FormatBool(archived)}

		return s.audited(ctx, &response, entry)
	}
}

func inOrganisation(access model.BoardAccess, orgID uuid.UUID) error {
	if orgID == uuid.Nil || access.Header.OrgID == orgID {
		return nil
	}

	return fmt.Errorf("%w: board %s belongs to organisation %s, not %s", live.ErrNoAccess, access.Header.ID, access.Header.OrgID, orgID)
}

func (s *service) settle(ctx context.Context, access model.BoardAccess, written *model.Versioned[model.Board]) error {
	changed := access
	changed.Header.Board = written.Value

	s.events.Publish(changed, projection.Resync(written.Version))

	if err := s.live.ReevaluateHeld(ctx, access.Header.ID); err != nil {
		return fmt.Errorf("re-evaluate the viewers of board %s after it changed: %w", access.Header.ID, err)
	}

	return nil
}

func boardEntry(access model.BoardAccess, action audit.Action) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      access.Header.OrgID,
		ActorID:    access.Principal.ID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetBoard,
		TargetID:   access.Header.ID.String(),
		SubjectID:  access.Principal.ID,
	}
}

func (s *service) audited(ctx context.Context, response *dto.BoardCommandResponse, entry audit.NewEntry) (*dto.BoardCommandResponse, error) {
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return response, fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return response, nil
}
