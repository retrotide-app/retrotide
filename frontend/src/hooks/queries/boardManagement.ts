import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { bulkExportUrl, listDeletedBoards } from "../../api/endpoints/boardManagement";
import { queryKeys } from "../../api/queryKeys";
import type { DeletedBoardListResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useDeletedBoards(orgId: string, enabled: boolean): QueryState<DeletedBoardListResponse> {
    const deleted = useQuery({
        queryKey: queryKeys.organisations.deletedBoards(orgId),
        queryFn: () => listDeletedBoards(orgId),
        enabled: enabled && orgId !== "",
    });

    return queryState(deleted);
}

export function useBulkExportUrl(orgId: string, boardIds: readonly string[]): string {
    return useMemo(() => bulkExportUrl(orgId, boardIds), [orgId, boardIds]);
}
