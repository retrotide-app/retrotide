import { useMutation, useQueryClient } from "@tanstack/react-query";
import { startProviderSignIn, unlinkIdentity } from "../../api/endpoints/identity";
import { queryKeys } from "../../api/queryKeys";
import type { IdentityProviderResponse, IdentityResponse, IdentityStartRequest } from "../../types/api";

export interface ProviderStart {
    provider: IdentityProviderResponse;
    request: IdentityStartRequest;
}

export function useStartProviderSignIn() {
    return useMutation({
        mutationFn: ({ provider, request }: ProviderStart) => startProviderSignIn(provider.id, request),
    });
}

export function useUnlinkIdentity() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: (identity: IdentityResponse) => unlinkIdentity(identity.provider),
        onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.auth.identities() }),
    });
}
