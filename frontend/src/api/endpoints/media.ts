import type {
    BoardBackgroundRequest,
    BoardCommandResponse,
    CardImageRequest,
    MediaKind,
    MediaView,
} from "../../types/api";
import { apiPut, apiUpload } from "../client";

function boardPath(boardId: string, rest: string): string {
    return `/boards/${encodeURIComponent(boardId)}${rest}`;
}

export async function uploadMedia(boardId: string, kind: MediaKind, file: File): Promise<MediaView> {
    return apiUpload<MediaView>(boardPath(boardId, `/media?kind=${encodeURIComponent(kind)}`), file, file.type);
}

export async function setCardImage(
    boardId: string,
    cardId: string,
    body: CardImageRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, CardImageRequest>(
        boardPath(boardId, `/cards/${encodeURIComponent(cardId)}/image`),
        body,
    );
}

export async function setBoardBackground(boardId: string, body: BoardBackgroundRequest): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, BoardBackgroundRequest>(boardPath(boardId, "/background"), body);
}
