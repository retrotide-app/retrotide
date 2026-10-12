import type { BoardCommandRequest } from "./board";

export interface MoveBoardRequest extends BoardCommandRequest {
    team_id: string | null;
}

export interface CsvImportRequest extends BoardCommandRequest {
    csv: string;
}

export interface CsvImportResponse {
    version: number;
    columns_created: number;
    cards_created: number;
}

export type BulkBoardAction = "archive" | "unarchive" | "delete";

export interface BulkBoardRequest {
    action: BulkBoardAction;
    board_ids: string[];
}

export interface BulkBoardResult {
    board_id: string;
    ok: boolean;
    code?: string;
    error?: string;
}

export interface BulkBoardResponse {
    results: BulkBoardResult[];
}

export interface DeletedBoardItem {
    id: string;
    team_id?: string;
    team_name?: string;
    title: string;
    deleted_at: string;
    restore_until: string;
}

export interface DeletedBoardListResponse {
    boards: DeletedBoardItem[];
}
