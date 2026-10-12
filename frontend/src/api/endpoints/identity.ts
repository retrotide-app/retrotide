import type { IdentityListResponse, IdentityStartRequest, IdentityStartResponse } from "../../types/api";
import { apiDelete, apiFetch, apiPost } from "../client";

export async function startProviderSignIn(
    provider: string,
    body: IdentityStartRequest,
): Promise<IdentityStartResponse> {
    return apiPost<IdentityStartResponse, IdentityStartRequest>(`/auth/${encodeURIComponent(provider)}/start`, body);
}

export async function listIdentities(): Promise<IdentityListResponse> {
    return apiFetch<IdentityListResponse>("/auth/identities");
}

export async function unlinkIdentity(provider: string): Promise<void> {
    return apiDelete<void>(`/auth/identities/${encodeURIComponent(provider)}`);
}
