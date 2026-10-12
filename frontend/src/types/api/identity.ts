export type IdentityPurpose = "sign_in" | "link";

export interface IdentityProviderResponse {
    id: string;
    name: string;
}

export interface IdentityStartRequest {
    purpose: IdentityPurpose;
    current_password?: string;
    return_to?: string;
    reauthenticate_with?: string;
}

export interface IdentityStartResponse {
    url: string;
}

export interface IdentityResponse {
    provider: string;
    provider_name: string;
    email: string;
    created_at: string;
}

export interface IdentityListResponse {
    identities: IdentityResponse[];
}
