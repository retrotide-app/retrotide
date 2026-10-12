package policy

import (
	"retrotide/internal/board/projection"
	"retrotide/internal/model"
)

var (
	CanSetBackground = CanManageBoard
)

func CanReact(access model.BoardAccess, card model.Card) error {
	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	return publicCard(access, card)
}

func CanPin(access model.BoardAccess, card model.Card) error {
	if err := CanFacilitate(access); err != nil {
		return err
	}

	return publicCard(access, card)
}

func CanSetCardImage(access model.BoardAccess, card model.Card) error {
	if err := CanEditCard(access, card); err != nil {
		return err
	}

	if !access.OrgMember {
		return ErrMembersOnly
	}

	return nil
}

func CanUploadImage(access model.BoardAccess) error {
	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	if !access.OrgMember {
		return ErrMembersOnly
	}

	return nil
}

func publicCard(access model.BoardAccess, card model.Card) error {
	if visible, _ := standing(access, card); !visible {
		return ErrCardNotFound
	}

	if !projection.VisibleToAll(access.Header.Board, card) {
		return ErrCardNotPublic
	}

	return nil
}
