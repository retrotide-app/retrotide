import { useQuery } from "@tanstack/react-query";
import { getSiteInfo } from "../../api/endpoints/siteInfo";
import { queryKeys } from "../../api/queryKeys";
import type { SiteInfoResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useSiteInfoQuery(): QueryState<SiteInfoResponse> {
    const query = useQuery({ queryKey: queryKeys.siteInfo(), queryFn: getSiteInfo });

    return queryState(query);
}
