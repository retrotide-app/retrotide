import type {
    BoardCommandRequest,
    BoardSharingResponse,
    JoinBoardRequest,
    JoinBoardResponse,
    JoinPreviewResponse,
    SetBoardPasswordRequest,
    SetLinkAccessRequest,
    SharingCommandResponse,
} from "../../types/api";
import { apiFetch, apiPost, apiPut } from "../client";

function shareLinkPath(slug: string, action = ""): string {
    return `/b/${encodeURIComponent(slug)}${action}`;
}

function sharingPath(boardId: string, action = ""): string {
    return `/boards/${encodeURIComponent(boardId)}/sharing${action}`;
}

export async function getJoinPreview(slug: string): Promise<JoinPreviewResponse> {
    return apiFetch<JoinPreviewResponse>(shareLinkPath(slug));
}

export async function joinBoardByLink(slug: string, body: JoinBoardRequest): Promise<JoinBoardResponse> {
    return apiPost<JoinBoardResponse, JoinBoardRequest>(shareLinkPath(slug, "/join"), body);
}

export async function getBoardSharing(boardId: string): Promise<BoardSharingResponse> {
    return apiFetch<BoardSharingResponse>(sharingPath(boardId));
}

export async function setLinkAccess(boardId: string, body: SetLinkAccessRequest): Promise<SharingCommandResponse> {
    return apiPut<SharingCommandResponse, SetLinkAccessRequest>(sharingPath(boardId, "/link-access"), body);
}

export async function setBoardPassword(
    boardId: string,
    body: SetBoardPasswordRequest,
): Promise<SharingCommandResponse> {
    return apiPut<SharingCommandResponse, SetBoardPasswordRequest>(sharingPath(boardId, "/password"), body);
}

export async function rotateShareLink(boardId: string, body: BoardCommandRequest): Promise<SharingCommandResponse> {
    return apiPost<SharingCommandResponse, BoardCommandRequest>(sharingPath(boardId, "/rotate"), body);
}
