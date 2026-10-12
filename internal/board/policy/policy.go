package policy

import (
	"slices"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

var (
	CanControlTimer = CanShapeColumns
)

func Capabilities(role model.BoardRole, orgMember bool) []model.BoardCapability {
	if role == model.BoardRoleFacilitator {
		return []model.BoardCapability{
			model.BoardCapabilityView,
			model.BoardCapabilityParticipate,
			model.BoardCapabilityFacilitate,
			model.BoardCapabilityManage,
			model.BoardCapabilityExport,
		}
	}

	if role != model.BoardRoleParticipant {
		return []model.BoardCapability{}
	}

	if orgMember {
		return []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityParticipate, model.BoardCapabilityExport}
	}

	return []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityParticipate}
}

func CanCreateCard(access model.BoardAccess) error {
	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	if err := inStage(access, model.StageWrite, ErrCardsOutOfStage); err != nil {
		return err
	}

	if access.Header.CardsLocked && !slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return ErrCardsLocked
	}

	return nil
}

func CanEditCard(access model.BoardAccess, card model.Card) error {
	if err := participating(access); err != nil {
		return err
	}

	visible, own := standing(access, card)
	if !visible {
		return ErrCardNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if err := inStage(access, model.StageWrite, ErrCardsOutOfStage); err != nil {
		return err
	}

	if !own {
		return ErrNotAllowed
	}

	if access.Header.CardsLocked && !slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return ErrCardsLocked
	}

	return nil
}

func CanDeleteCard(access model.BoardAccess, card model.Card) error {
	if err := writable(access); err != nil {
		return err
	}

	visible, own := standing(access, card)
	if !visible {
		return ErrCardNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return nil
	}

	if err := participating(access); err != nil {
		return err
	}

	if err := inStage(access, model.StageWrite, ErrCardsOutOfStage); err != nil {
		return err
	}

	if !own {
		return ErrNotAllowed
	}

	if access.Header.CardsLocked {
		return ErrCardsLocked
	}

	return nil
}

func CanMoveCard(access model.BoardAccess, card model.Card) error {
	if err := writable(access); err != nil {
		return err
	}

	visible, own := standing(access, card)
	if !visible {
		return ErrCardNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if err := inStage(access, model.StageWrite, ErrCardsOutOfStage); err != nil {
		return err
	}

	if slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return nil
	}

	if err := participating(access); err != nil {
		return err
	}

	if !own {
		return ErrNotAllowed
	}

	if access.Header.MovesLocked {
		return ErrMovesLocked
	}

	return nil
}

func CanManageColumns(access model.BoardAccess) error {
	if err := writable(access); err != nil {
		return err
	}

	if !slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return ErrNotAllowed
	}

	return nil
}

func CanShapeColumns(access model.BoardAccess) error {
	if err := writable(access); err != nil {
		return err
	}

	if slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return nil
	}

	if !access.Header.AnyoneCanEdit {
		return ErrNotAllowed
	}

	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotAllowed
	}

	return nil
}

func CanUseColumn(access model.BoardAccess, columnID uuid.UUID) error {
	if slices.Contains(access.Header.HiddenColumnIDs, columnID) {
		return ErrColumnHidden
	}

	return nil
}

func CanMoveTo(access model.BoardAccess, card model.Card, columnID uuid.UUID) error {
	if err := CanUseColumn(access, columnID); err != nil {
		return err
	}

	if access.Header.CardsHiddenIn(card.ColumnID) != access.Header.CardsHiddenIn(columnID) {
		return ErrMoveAcrossReveal
	}

	return nil
}

func CanManageBoard(access model.BoardAccess) error {
	if err := writable(access); err != nil {
		return err
	}

	if !slices.Contains(access.Capabilities, model.BoardCapabilityManage) {
		return ErrNotAllowed
	}

	return nil
}

func CanFacilitate(access model.BoardAccess) error {
	if err := writable(access); err != nil {
		return err
	}

	if !slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return ErrNotAllowed
	}

	return nil
}

func CanHideCards(access model.BoardAccess) error {
	if err := CanFacilitate(access); err != nil {
		return err
	}

	return inStage(access, model.StageWrite, ErrHideOutOfStage)
}

func CanVote(access model.BoardAccess, card model.Card) error {
	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	if visible, _ := standing(access, card); !visible {
		return ErrCardNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if err := inStage(access, model.StageVote, ErrVotesOutOfStage); err != nil {
		return err
	}

	if !access.Header.VotingOpen {
		return ErrVotingClosed
	}

	return nil
}

func CanComment(access model.BoardAccess, card model.Card) error {
	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	if visible, _ := standing(access, card); !visible {
		return ErrCardNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if err := inStage(access, model.StageDiscuss, ErrCommentsOutOfStage); err != nil {
		return err
	}

	if !access.Header.CommentsEnabled {
		return ErrCommentsDisabled
	}

	return nil
}

func CanEditComment(access model.BoardAccess, card model.Card, comment model.CardComment) error {
	if err := participating(access); err != nil {
		return err
	}

	if visible, _ := standing(access, card); !visible {
		return ErrCommentNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if err := inStage(access, model.StageDiscuss, ErrCommentsOutOfStage); err != nil {
		return err
	}

	if !projection.OwnsComment(comment, projection.ViewerFor(access)) {
		return ErrNotAllowed
	}

	if !access.Header.CommentsEnabled {
		return ErrCommentsDisabled
	}

	return nil
}

func CanDeleteComment(access model.BoardAccess, card model.Card, comment model.CardComment) error {
	if err := writable(access); err != nil {
		return err
	}

	if visible, _ := standing(access, card); !visible {
		return ErrCommentNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return nil
	}

	if err := participating(access); err != nil {
		return err
	}

	if err := inStage(access, model.StageDiscuss, ErrCommentsOutOfStage); err != nil {
		return err
	}

	if !projection.OwnsComment(comment, projection.ViewerFor(access)) {
		return ErrNotAllowed
	}

	return nil
}

func CanMerge(access model.BoardAccess, card, target model.Card) error {
	if err := writable(access); err != nil {
		return err
	}

	cardVisible, _ := standing(access, card)
	targetVisible, _ := standing(access, target)
	if !cardVisible || !targetVisible {
		return ErrCardNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if err := CanUseColumn(access, target.ColumnID); err != nil {
		return err
	}

	if err := inStage(access, model.StageDiscuss, ErrMergesOutOfStage); err != nil {
		return err
	}

	if access.Header.CardsHiddenIn(card.ColumnID) || access.Header.CardsHiddenIn(target.ColumnID) {
		return ErrCardsHidden
	}

	if !access.Header.MergingEnabled {
		return ErrMergingDisabled
	}

	if card.ID == target.ID {
		return ErrMergeIntoSelf
	}

	return mover(access)
}

func CanUnmerge(access model.BoardAccess, card model.Card) error {
	if err := writable(access); err != nil {
		return err
	}

	if visible, _ := standing(access, card); !visible {
		return ErrCardNotFound
	}

	if err := CanUseColumn(access, card.ColumnID); err != nil {
		return err
	}

	if err := inStage(access, model.StageDiscuss, ErrMergesOutOfStage); err != nil {
		return err
	}

	if access.Header.CardsHiddenIn(card.ColumnID) {
		return ErrCardsHidden
	}

	if card.ParentCardID == nil {
		return ErrNotMerged
	}

	return mover(access)
}

func CanViewSharing(access model.BoardAccess) error {
	if !slices.Contains(access.Capabilities, model.BoardCapabilityManage) {
		return ErrNotAllowed
	}

	return nil
}

func writable(access model.BoardAccess) error {
	if access.Header.Archived() || access.Header.Closed() {
		return ErrReadOnly
	}

	return nil
}

func participating(access model.BoardAccess) error {
	if err := writable(access); err != nil {
		return err
	}

	if access.Header.Locked() {
		return ErrReadOnly
	}

	if !slices.Contains(access.Capabilities, model.BoardCapabilityParticipate) {
		return ErrNotAllowed
	}

	return nil
}

func inStage(access model.BoardAccess, stage model.Stage, refusal error) error {
	if access.Header.Stage != stage {
		return refusal
	}

	return nil
}

func mover(access model.BoardAccess) error {
	if slices.Contains(access.Capabilities, model.BoardCapabilityFacilitate) {
		return nil
	}

	if err := participating(access); err != nil {
		return err
	}

	if access.Header.MovesLocked {
		return ErrMovesLocked
	}

	return nil
}

func standing(access model.BoardAccess, card model.Card) (bool, bool) {
	viewer := projection.ViewerFor(access)

	return projection.Visible(access.Header.Board, card, viewer), projection.Owns(card, viewer)
}
