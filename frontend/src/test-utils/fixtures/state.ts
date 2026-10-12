import { type Mock, vi } from "vitest";
import type { QueryState } from "../../hooks/queries/queryState";

export interface ApiErrorFixture {
    status: number;
    message: string;
    code?: string;
}

export type MutationStatus = "idle" | "pending" | "success" | "error";

export interface MutationStateFixture {
    mutate: Mock;
    mutateAsync: Mock;
    reset: Mock;
    status: MutationStatus;
    isIdle: boolean;
    isPending: boolean;
    isSuccess: boolean;
    isError: boolean;
    error: Error | null;
    data: unknown;
    variables: unknown;
}

export function makeApiError({ status, message, code }: ApiErrorFixture): Error {
    const body = code === undefined ? { error: message } : { error: message, code };

    return Object.assign(new Error(message), { name: "ApiError", status, code: code ?? null, body });
}

export function makeQueryState<T>(overrides: Partial<QueryState<T>> = {}): QueryState<T> {
    return {
        data: undefined,
        loading: false,
        error: null,
        refetch: vi.fn().mockResolvedValue(undefined),
        ...overrides,
    };
}

function statusOf(flags: Partial<MutationStateFixture>): MutationStatus {
    if (flags.isPending) {
        return "pending";
    }

    if (flags.isError) {
        return "error";
    }

    if (flags.isSuccess) {
        return "success";
    }

    return "idle";
}

export function makeMutationState(overrides: Partial<MutationStateFixture> = {}): MutationStateFixture {
    const status = overrides.status ?? statusOf(overrides);

    return {
        mutate: vi.fn(),
        mutateAsync: vi.fn().mockResolvedValue(undefined),
        reset: vi.fn(),
        status,
        isIdle: status === "idle",
        isPending: status === "pending",
        isSuccess: status === "success",
        isError: status === "error",
        error: null,
        data: undefined,
        variables: undefined,
        ...overrides,
    };
}
