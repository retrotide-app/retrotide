import { useQuery } from "@tanstack/react-query";
import { listIdentities } from "../../api/endpoints/identity";
import { queryKeys } from "../../api/queryKeys";
import type { IdentityListResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useIdentities(enabled = true): QueryState<IdentityListResponse> {
    const query = useQuery({ queryKey: queryKeys.auth.identities(), queryFn: listIdentities, enabled });

    return queryState(query);
}
