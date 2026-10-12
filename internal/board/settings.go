package board

import (
	"context"
	"fmt"
	"slices"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) ChangeSettings(ctx context.Context, command SettingsCommand) (*dto.BoardCommandResponse, error) {
	if err := checkSettings(command.Settings); err != nil {
		return nil, err
	}

	configure := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanFacilitate(access); err != nil {
			return nil, err
		}

		written, err := s.boards.SetBoardSettings(ctx, spec.BoardSettingsWrite{BoardWrite: target, Patch: command.Settings})
		if err != nil {
			return nil, fmt.Errorf("change the settings of board %s: %w", target.BoardID, err)
		}

		s.scheduleAutoLock(written.Value)

		totalsShownAgain := access.Header.TotalsHidden() && !written.Value.TotalsHidden()
		lockChanged := access.Header.Locked() != written.Value.Locked()

		changed := access
		changed.Header.Board = written.Value

		event := projection.SettingsChanged(written.Value, written.Version)
		if totalsShownAgain || lockChanged {
			event = projection.Resync(written.Version)
		}

		s.events.Publish(changed, event)

		if lockChanged {
			if err := s.live.ReevaluateHeld(ctx, target.BoardID); err != nil {
				return nil, fmt.Errorf("re-evaluate the viewers of board %s after its lock changed: %w", target.BoardID, err)
			}
		}

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, configure)
}

func checkSettings(patch spec.BoardSettingsPatch) error {
	for _, limit := range []*int{patch.MaxVotes, patch.MaxVotesPerCard} {
		if limit != nil && (*limit < 0 || *limit > model.BoardMaxVotesLimit) {
			return ErrInvalidMaxVotes
		}
	}

	if patch.SortMode != nil && !slices.Contains(model.SortModes, *patch.SortMode) {
		return ErrInvalidSortMode
	}

	return nil
}
