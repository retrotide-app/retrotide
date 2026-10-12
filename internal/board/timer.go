package board

import (
	"context"
	"fmt"
	"slices"
	"time"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

func (s *service) ControlTimer(ctx context.Context, command TimerCommand) (*dto.BoardCommandResponse, error) {
	if !slices.Contains(model.TimerActions, command.Action) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTimerAction, command.Action)
	}

	if command.Action == model.TimerActionStart && !timerDurationAllowed(command.DurationMS) {
		return nil, fmt.Errorf("%w: %d ms", ErrInvalidTimerDuration, command.DurationMS)
	}

	control := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanControlTimer(access); err != nil {
			return nil, err
		}

		now := s.now()

		timer, err := nextTimer(access.Header.Timer(), command, now)
		if err != nil {
			return nil, err
		}

		written, err := s.boards.SetBoardTimer(ctx, spec.BoardTimerWrite{BoardWrite: target, Timer: timer})
		if err != nil {
			return nil, fmt.Errorf("%s the timer of board %s: %w", command.Action, target.BoardID, err)
		}

		s.scheduleAutoLock(written.Value)

		changed := access
		changed.Header.Board = written.Value

		s.events.Publish(changed, projection.TimerChanged(written.Value, now, written.Version))

		response := projection.VersionResponse(written.Version)

		return &response, nil
	}

	return s.Run(ctx, command.Command, VersionReplay, control)
}

func nextTimer(current model.BoardTimer, command TimerCommand, now time.Time) (model.BoardTimer, error) {
	extension := int(model.TimerExtension.Milliseconds())

	switch command.Action {
	case model.TimerActionStart:
		return model.BoardTimer{DurationMS: new(command.DurationMS), EndsAt: new(now.Add(milliseconds(command.DurationMS)))}, nil

	case model.TimerActionPause:
		if current.EndsAt == nil {
			return model.BoardTimer{}, ErrTimerNotRunning
		}

		remaining := current.EndsAt.Sub(now).Milliseconds()
		if remaining <= 0 {
			return model.BoardTimer{}, fmt.Errorf("%w: it ended at %s", ErrTimerNotRunning, current.EndsAt.UTC().Format(time.RFC3339))
		}

		return model.BoardTimer{DurationMS: current.DurationMS, PausedRemainingMS: new(int(remaining))}, nil

	case model.TimerActionResume:
		if current.PausedRemainingMS == nil {
			return model.BoardTimer{}, ErrTimerNotPaused
		}

		return model.BoardTimer{DurationMS: current.DurationMS, EndsAt: new(now.Add(milliseconds(*current.PausedRemainingMS)))}, nil

	case model.TimerActionAddMinute:
		if !current.Set() {
			return model.BoardTimer{}, ErrTimerNotRunning
		}

		if !current.Paused() && !current.EndsAt.After(now) {
			return model.BoardTimer{DurationMS: new(extension), EndsAt: new(now.Add(model.TimerExtension))}, nil
		}

		duration := *current.DurationMS + extension
		if !timerDurationAllowed(duration) {
			return model.BoardTimer{}, fmt.Errorf("%w: %d ms", ErrTimerTooLong, duration)
		}

		if current.Paused() {
			return model.BoardTimer{DurationMS: new(duration), PausedRemainingMS: new(*current.PausedRemainingMS + extension)}, nil
		}

		return model.BoardTimer{DurationMS: new(duration), EndsAt: new(current.EndsAt.Add(model.TimerExtension))}, nil

	default:
		return model.BoardTimer{}, nil
	}
}

func timerDurationAllowed(durationMS int) bool {
	return durationMS >= int(model.TimerMinDuration.Milliseconds()) && durationMS <= int(model.TimerMaxDuration.Milliseconds())
}

func milliseconds(count int) time.Duration {
	return time.Duration(count) * time.Millisecond
}
