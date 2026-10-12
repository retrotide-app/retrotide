package policy

import (
	"slices"

	"retrotide/internal/model"
)

var (
	CanDeleteBoard = CanArchiveBoard
	CanMoveBoard   = CanArchiveBoard
)

func CanArchiveBoard(access model.BoardAccess) error {
	if !slices.Contains(access.Capabilities, model.BoardCapabilityManage) {
		return ErrNotAllowed
	}

	return nil
}

func CanImportCards(access model.BoardAccess) error {
	if err := participating(access); err != nil {
		return err
	}

	if !slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return ErrNotAllowed
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	return inStage(access, model.StageWrite, ErrCardsOutOfStage)
}
