package management

import (
	"context"
	"fmt"

	"retrotide/internal/board/policy"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) Pin(ctx context.Context, command PinCommand) error {
	if command.Principal.Kind != model.PrincipalUser {
		return fmt.Errorf("%w: only a signed-in member pins a board", policy.ErrMembersOnly)
	}

	access, err := s.live.Resolve(ctx, spec.BoardAccessQuery{Principal: command.Principal, BoardID: command.BoardID})
	if err != nil {
		return err
	}

	if !access.OrgMember {
		return fmt.Errorf("%w: user %s is not in organisation %s", policy.ErrMembersOnly, command.Principal.ID, access.Header.OrgID)
	}

	key := spec.BoardPinKey{OrgID: access.Header.OrgID, BoardID: access.Header.ID, UserID: command.Principal.ID}

	if command.Pinned {
		if err := s.boards.Pin(ctx, key); err != nil {
			return fmt.Errorf("pin board %s for user %s: %w", key.BoardID, key.UserID, err)
		}

		return nil
	}

	if err := s.boards.Unpin(ctx, key); err != nil {
		return fmt.Errorf("unpin board %s for user %s: %w", key.BoardID, key.UserID, err)
	}

	return nil
}
