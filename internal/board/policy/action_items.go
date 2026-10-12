package policy

import (
	"slices"

	"retrotide/internal/model"
)

func CanWriteActionItems(access model.BoardAccess) error {
	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	return inStage(access, model.StageActions, ErrActionItemsOutOfStage)
}

func CanNameMembers(access model.BoardAccess) error {
	if !organisationMember(access) {
		return ErrMembersOnly
	}

	return nil
}

func CanMarkActionItem(access model.BoardAccess) error {
	if access.Header.Archived() {
		return ErrReadOnly
	}

	if access.Header.Closed() && !organisationMember(access) {
		return ErrReadOnly
	}

	if access.Header.Closed() {
		return nil
	}

	if err := participating(access); err != nil {
		return err
	}

	return inStage(access, model.StageActions, ErrActionItemsOutOfStage)
}

func CanDeleteActionItem(access model.BoardAccess, item model.ActionItem) error {
	if err := writable(access); err != nil {
		return err
	}

	if slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return nil
	}

	if err := participating(access); err != nil {
		return err
	}

	if err := inStage(access, model.StageActions, ErrActionItemsOutOfStage); err != nil {
		return err
	}

	if access.Header.Anonymous() || access.Principal.Kind != model.PrincipalUser || item.CreatedBy == nil || *item.CreatedBy != access.Principal.ID {
		return ErrNotAllowed
	}

	return nil
}

func CanReviewActionItems(access model.BoardAccess) error {
	if err := participating(access); err != nil {
		return err
	}

	return CanNameMembers(access)
}

func organisationMember(access model.BoardAccess) bool {
	return access.Principal.Kind == model.PrincipalUser && access.OrgMember
}
