import type { BoardExportFormat } from "../../types/api";
import { apiUrl } from "../origin";

export const SELF_EXPORT_URL = apiUrl("/auth/export");

export function boardExportUrl(boardId: string, format: BoardExportFormat): string {
    const query = new URLSearchParams({ format });

    return apiUrl(`/boards/${encodeURIComponent(boardId)}/export?${query.toString()}`);
}

export function organisationExportUrl(orgId: string): string {
    return apiUrl(`/orgs/${encodeURIComponent(orgId)}/export`);
}
