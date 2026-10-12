import { useQuery } from "@tanstack/react-query";
import { getAdminStats } from "../../api/endpoints/adminStats";
import { queryKeys } from "../../api/queryKeys";
import type { AdminStatsResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useAdminStats(enabled = true): QueryState<AdminStatsResponse> {
    const stats = useQuery({ queryKey: queryKeys.admin.stats(), queryFn: getAdminStats, enabled });

    return queryState(stats);
}
