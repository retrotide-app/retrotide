package board

import (
	"context"
	"crypto/rand"
	"fmt"
	"slices"
	"strings"

	"retrotide/internal/audit"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"golang.org/x/crypto/bcrypt"
)

type (
	shareChange struct {
		action  audit.Action
		details map[string]string
	}
)

const (
	passwordSet     = "set"
	passwordCleared = "cleared"
)

func (s *service) Sharing(ctx context.Context, query spec.BoardAccessQuery) (*dto.SharingView, error) {
	access, err := s.live.Resolve(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := policy.CanViewSharing(*access); err != nil {
		return nil, err
	}

	return projection.Sharing(access.Header.Board, projection.ViewerFor(*access)), nil
}

func (s *service) SetLinkAccess(ctx context.Context, command SetLinkAccessCommand) (*dto.BoardCommandResponse, error) {
	if !slices.Contains(model.LinkAccessLevels, command.LinkAccess) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidLinkAccess, command.LinkAccess)
	}

	share := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageBoard(access); err != nil {
			return nil, err
		}

		written, err := s.boards.SetBoardLinkAccess(ctx, spec.BoardLinkAccessWrite{BoardWrite: target, LinkAccess: command.LinkAccess})
		if err != nil {
			return nil, fmt.Errorf("set the link access of board %s: %w", target.BoardID, err)
		}

		change := shareChange{action: audit.ActionBoardSharingChanged, details: map[string]string{"link_access": string(command.LinkAccess)}}

		return s.shared(ctx, access, *written, change)
	}

	return s.Run(ctx, command.Command, VersionReplay, share)
}

func (s *service) SetPassword(ctx context.Context, command SetPasswordCommand) (*dto.BoardCommandResponse, error) {
	if err := validateBoardPassword(command.Password); err != nil {
		return nil, err
	}

	outcome := passwordCleared
	if command.Password != nil {
		outcome = passwordSet
	}

	protect := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageBoard(access); err != nil {
			return nil, err
		}

		hash, err := hashBoardPassword(command.Password)
		if err != nil {
			return nil, err
		}

		written, err := s.boards.SetBoardPassword(ctx, spec.BoardPasswordWrite{BoardWrite: target, PasswordHash: hash})
		if err != nil {
			return nil, fmt.Errorf("set the password of board %s: %w", target.BoardID, err)
		}

		change := shareChange{action: audit.ActionBoardPasswordChanged, details: map[string]string{"password": outcome}}

		return s.shared(ctx, access, *written, change)
	}

	return s.Run(ctx, command.Command, VersionReplay, protect)
}

func (s *service) RotateLink(ctx context.Context, command Command) (*dto.BoardCommandResponse, error) {
	rotate := func(access model.BoardAccess, target spec.BoardWrite) (*dto.BoardCommandResponse, error) {
		if err := policy.CanManageBoard(access); err != nil {
			return nil, err
		}

		written, err := s.boards.RotateBoardLink(ctx, spec.BoardLinkRotationWrite{BoardWrite: target, ShareSlug: rand.Text()})
		if err != nil {
			return nil, fmt.Errorf("rotate the share link of board %s: %w", target.BoardID, err)
		}

		return s.shared(ctx, access, *written, shareChange{action: audit.ActionBoardLinkRotated})
	}

	return s.Run(ctx, command, VersionReplay, rotate)
}

func (s *service) shared(ctx context.Context, access model.BoardAccess, written model.Versioned[model.Board], change shareChange) (*dto.BoardCommandResponse, error) {
	access.Header.Board = written.Value
	s.events.Publish(access)

	if err := s.live.ReevaluateHeld(ctx, access.Header.ID); err != nil {
		return nil, fmt.Errorf("re-evaluate the viewers of board %s after its sharing changed: %w", access.Header.ID, err)
	}

	response := projection.SharingResponse(written.Value, projection.ViewerFor(access), written.Version)

	entry := audit.NewEntry{
		OrgID:      access.Header.OrgID,
		ActorID:    access.Principal.ID,
		ActorKind:  audit.ActorUser,
		Action:     change.action,
		TargetType: audit.TargetBoard,
		TargetID:   access.Header.ID.String(),
		SubjectID:  access.Principal.ID,
		Details:    change.details,
	}
	if err := s.auditLog.Create(ctx, entry); err != nil {
		return &response, fmt.Errorf("%w: %w", ErrNotAudited, err)
	}

	return &response, nil
}

func validateBoardPassword(password *string) error {
	if password == nil {
		return nil
	}

	if strings.TrimSpace(*password) == "" {
		return ErrPasswordBlank
	}

	if len(*password) > model.BoardPasswordMaxBytes {
		return ErrPasswordTooLong
	}

	return nil
}

func hashBoardPassword(password *string) (*string, error) {
	if password == nil {
		return nil, nil
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash the board password: %w", err)
	}

	return new(string(hashed)), nil
}
