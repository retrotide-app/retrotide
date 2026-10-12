package projection

import (
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	LinkVerdict struct {
		RequiresPassword bool
		RequiresAccount  bool
		CanJoin          bool
	}
)

func JoinPreview(verdict LinkVerdict) dto.JoinPreview {
	return dto.JoinPreview{
		RequiresPassword: verdict.RequiresPassword,
		RequiresAccount:  verdict.RequiresAccount,
		CanJoin:          verdict.CanJoin,
	}
}

func LinkJoined(access model.BoardAccess) dto.LinkJoinResponse {
	response := dto.LinkJoinResponse{BoardID: access.Header.ID}

	if access.OrgMember {
		response.OrgID = &access.Header.OrgID
	}

	return response
}

func Sharing(board model.Board, viewer Viewer) *dto.SharingView {
	if !slices.Contains(viewer.Capabilities, model.BoardCapabilityManage) {
		return nil
	}

	return &dto.SharingView{
		ShareSlug:   board.ShareSlug,
		LinkAccess:  board.LinkAccess,
		HasPassword: board.HasPassword(),
	}
}

func SharingResponse(board model.Board, viewer Viewer, version int64) dto.BoardCommandResponse {
	return dto.BoardCommandResponse{Version: version, Sharing: Sharing(board, viewer)}
}

func GuestClaimOffer(joined []model.GuestBoard) dto.GuestClaimOffer {
	boards := make([]dto.GuestClaimBoard, 0, len(joined))
	for _, entry := range joined {
		if entry.Board.Anonymous() || entry.Participant.Removed() {
			continue
		}

		name := entry.Participant.DisplayName
		if name == "" {
			name = AnonymousName
		}

		boards = append(boards, dto.GuestClaimBoard{BoardID: entry.Board.ID, Title: entry.Board.Title, DisplayName: name})
	}

	return dto.GuestClaimOffer{Boards: boards}
}

func GuestClaimResult(claimed []model.Board) dto.GuestClaimResult {
	ids := make([]uuid.UUID, 0, len(claimed))
	for _, board := range claimed {
		ids = append(ids, board.ID)
	}

	return dto.GuestClaimResult{Claimed: ids}
}
