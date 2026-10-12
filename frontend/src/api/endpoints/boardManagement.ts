import type {
    BoardCommandRequest,
    BoardCommandResponse,
    BulkBoardRequest,
    BulkBoardResponse,
    CsvImportRequest,
    CsvImportResponse,
    DeletedBoardListResponse,
    MoveBoardRequest,
} from "../../types/api";
import { apiDelete, apiDeleteWithBody, apiFetch, apiPost, apiPut } from "../client";
import { apiUrl } from "../origin";

const BOARD_ID_QUERY = "board_id";

function boardPath(boardId: string, action = ""): string {
    return `/boards/${encodeURIComponent(boardId)}${action}`;
}

function organisationBoardsPath(orgId: string, action: string): string {
    return `/orgs/${encodeURIComponent(orgId)}/boards${action}`;
}

export async function archiveBoard(boardId: string, body: BoardCommandRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(boardPath(boardId, "/archive"), body);
}

export async function unarchiveBoard(boardId: string, body: BoardCommandRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(boardPath(boardId, "/unarchive"), body);
}

export async function deleteBoard(boardId: string, body: BoardCommandRequest): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(boardPath(boardId), body);
}

export async function moveBoard(boardId: string, body: MoveBoardRequest): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, MoveBoardRequest>(boardPath(boardId, "/team"), body);
}

export async function importCsv(boardId: string, body: CsvImportRequest): Promise<CsvImportResponse> {
    return apiPost<CsvImportResponse, CsvImportRequest>(boardPath(boardId, "/import"), body);
}

export async function pinBoard(boardId: string): Promise<void> {
    return apiPut<void, undefined>(boardPath(boardId, "/pin"), undefined);
}

export async function unpinBoard(boardId: string): Promise<void> {
    return apiDelete<void>(boardPath(boardId, "/pin"));
}

export async function restoreBoard(orgId: string, boardId: string): Promise<void> {
    return apiPost<void>(organisationBoardsPath(orgId, `/${encodeURIComponent(boardId)}/restore`));
}

export async function listDeletedBoards(orgId: string): Promise<DeletedBoardListResponse> {
    return apiFetch<DeletedBoardListResponse>(organisationBoardsPath(orgId, "/deleted"));
}

export async function bulkBoards(orgId: string, body: BulkBoardRequest): Promise<BulkBoardResponse> {
    return apiPost<BulkBoardResponse, BulkBoardRequest>(organisationBoardsPath(orgId, "/bulk"), body);
}

export function bulkExportUrl(orgId: string, boardIds: readonly string[]): string {
    const query = new URLSearchParams();

    for (const boardId of boardIds) {
        query.append(BOARD_ID_QUERY, boardId);
    }

    return apiUrl(`${organisationBoardsPath(orgId, "/export")}?${query.toString()}`);
}
