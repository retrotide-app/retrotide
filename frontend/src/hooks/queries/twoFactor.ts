import { useQuery } from "@tanstack/react-query";
import { getTwoFactorStatus } from "../../api/endpoints/twoFactor";
import { queryKeys } from "../../api/queryKeys";
import type { TwoFactorStatusResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useTwoFactorStatus(): QueryState<TwoFactorStatusResponse> {
    const query = useQuery({ queryKey: queryKeys.auth.twoFactor(), queryFn: getTwoFactorStatus });

    return queryState(query);
}
