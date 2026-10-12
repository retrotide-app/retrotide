import type {
    ActionItemFieldsRequest,
    ActionItemListQuery,
    ActionItemListResponse,
    ActionItemStatusRequest,
    BoardCommandRequest,
    BoardCommandResponse,
    BoardPeopleResponse,
    CreateActionItemRequest,
    MentionListResponse,
    ReviewActionItemRequest,
} from "../../types/api";
import { apiDeleteWithBody, apiFetch, apiPost, apiPut, buildQueryString } from "../client";

function boardPath(boardId: string, action: string): string {
    return `/boards/${encodeURIComponent(boardId)}${action}`;
}

function actionItemPath(boardId: string, itemId: string, action = ""): string {
    return boardPath(boardId, `/action-items/${encodeURIComponent(itemId)}${action}`);
}

export async function listBoardPeople(boardId: string): Promise<BoardPeopleResponse> {
    return apiFetch<BoardPeopleResponse>(boardPath(boardId, "/people"));
}

export async function addActionItem(boardId: string, body: CreateActionItemRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, CreateActionItemRequest>(boardPath(boardId, "/action-items"), body);
}

export async function editActionItem(
    boardId: string,
    itemId: string,
    body: ActionItemFieldsRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, ActionItemFieldsRequest>(actionItemPath(boardId, itemId), body);
}

export async function markActionItem(
    boardId: string,
    itemId: string,
    body: ActionItemStatusRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, ActionItemStatusRequest>(actionItemPath(boardId, itemId, "/status"), body);
}

export async function deleteActionItem(
    boardId: string,
    itemId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(actionItemPath(boardId, itemId), body);
}

export async function reviewPreviousAction(
    boardId: string,
    itemId: string,
    body: ReviewActionItemRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, ReviewActionItemRequest>(
        boardPath(boardId, `/previous-actions/${encodeURIComponent(itemId)}/review`),
        body,
    );
}

export async function listOrganisationActionItems(
    orgId: string,
    query: ActionItemListQuery,
): Promise<ActionItemListResponse> {
    const { status, assignee, team, limit, offset } = query;
    const search = buildQueryString({ status, assignee, team, limit, offset });

    return apiFetch<ActionItemListResponse>(`/orgs/${encodeURIComponent(orgId)}/action-items${search}`);
}

export async function listMentions(): Promise<MentionListResponse> {
    return apiFetch<MentionListResponse>("/auth/mentions");
}
