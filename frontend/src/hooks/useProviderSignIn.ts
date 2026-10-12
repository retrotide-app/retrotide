import { useCallback, useState } from "react";
import { leaveForExternalPage } from "../platform/location";
import type { IdentityProviderResponse, IdentityStartRequest } from "../types/api";
import { useStartProviderSignIn } from "./mutations/identity";

export interface ProviderSignIn {
    start: (provider: IdentityProviderResponse, request: IdentityStartRequest) => void;
    provider: IdentityProviderResponse | null;
    pending: boolean;
    error: Error | null;
    reset: () => void;
}

export function useProviderSignIn(): ProviderSignIn {
    const { mutate, variables, isPending, isSuccess, error, reset: resetStart } = useStartProviderSignIn();
    const [departureError, setDepartureError] = useState<Error | null>(null);

    const start = useCallback(
        (provider: IdentityProviderResponse, request: IdentityStartRequest) => {
            setDepartureError(null);
            mutate(
                { provider, request },
                {
                    onSuccess: started => {
                        try {
                            leaveForExternalPage(started.url);
                        } catch (refused) {
                            setDepartureError(
                                refused instanceof Error ? refused : new Error(String(refused), { cause: refused }),
                            );
                        }
                    },
                },
            );
        },
        [mutate],
    );

    const reset = useCallback(() => {
        setDepartureError(null);
        resetStart();
    }, [resetStart]);

    const provider = variables?.provider ?? null;

    if (departureError !== null) {
        return { start, provider, pending: false, error: departureError, reset };
    }

    return { start, provider, pending: isPending || isSuccess, error, reset };
}
