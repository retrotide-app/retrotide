import type { BoardCommandRequest, BoardCommandResponse, CardPinRequest, ReactionKey } from "../../types/api";
import { apiDeleteWithBody, apiPut } from "../client";

function cardPath(boardId: string, cardId: string, action: string): string {
    return `/boards/${encodeURIComponent(boardId)}/cards/${encodeURIComponent(cardId)}${action}`;
}

function reactionPath(boardId: string, cardId: string, reaction: ReactionKey): string {
    return cardPath(boardId, cardId, `/reactions/${encodeURIComponent(reaction)}`);
}

export async function addReaction(
    boardId: string,
    cardId: string,
    reaction: ReactionKey,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, BoardCommandRequest>(reactionPath(boardId, cardId, reaction), body);
}

export async function removeReaction(
    boardId: string,
    cardId: string,
    reaction: ReactionKey,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(reactionPath(boardId, cardId, reaction), body);
}

export async function pinCard(boardId: string, cardId: string, body: CardPinRequest): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, CardPinRequest>(cardPath(boardId, cardId, "/pin"), body);
}
