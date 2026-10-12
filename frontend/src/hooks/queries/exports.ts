import { useMemo } from "react";
import { boardExportUrl, organisationExportUrl, SELF_EXPORT_URL } from "../../api/endpoints/exports";
import { BOARD_EXPORT_FORMATS } from "../../domain/board/exports";

export interface ExportLink {
    label: string;
    href: string;
}

export function useBoardExportLinks(boardId: string): ExportLink[] {
    return useMemo(
        () => BOARD_EXPORT_FORMATS.map(({ format, label }) => ({ label, href: boardExportUrl(boardId, format) })),
        [boardId],
    );
}

export function useOrganisationExportUrl(orgId: string): string {
    return useMemo(() => organisationExportUrl(orgId), [orgId]);
}

export function useSelfExportUrl(): string {
    return SELF_EXPORT_URL;
}
