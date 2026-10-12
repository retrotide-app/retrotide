import type { IdentityProviderResponse, IdentityResponse, SessionResponse } from "../../types/api";
import type { SessionUser } from "../../types/app";

export function makeSessionUser(overrides: Partial<SessionUser> = {}): SessionUser {
    return {
        id: "00000000-0000-4000-8000-000000000001",
        email: "sam@example.test",
        display_name: "Sam",
        email_verified: true,
        has_password: true,
        email_notifications: true,
        created_at: "2026-01-05T09:30:00Z",
        ...overrides,
    };
}

export function makeSession(overrides: Partial<SessionResponse> = {}): SessionResponse {
    return {
        authenticated: true,
        user: makeSessionUser(),
        ...overrides,
    };
}

export function makeIdentityProvider(overrides: Partial<IdentityProviderResponse> = {}): IdentityProviderResponse {
    return {
        id: "google",
        name: "Google",
        ...overrides,
    };
}

export function makeIdentity(overrides: Partial<IdentityResponse> = {}): IdentityResponse {
    return {
        provider: "google",
        provider_name: "Google",
        email: "sam@gmail.com",
        created_at: "2026-02-10T14:00:00Z",
        ...overrides,
    };
}
