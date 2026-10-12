import type { UseQueryResult } from "@tanstack/react-query";

export interface QueryState<T> {
    data: T | undefined;
    loading: boolean;
    error: Error | null;
    refetch: () => Promise<unknown>;
}

export function queryState<T>(query: UseQueryResult<T, Error>): QueryState<T> {
    return { data: query.data, loading: query.isLoading, error: query.error, refetch: query.refetch };
}
