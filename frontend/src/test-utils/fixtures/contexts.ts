import { vi } from "vitest";
import type { AuthContextValue } from "../../context/authContextValue";
import type { OrgContextValue } from "../../context/orgContextValue";
import type { SiteInfoContextValue } from "../../context/siteInfoContextValue";
import type { ThemeContextValue } from "../../context/themeContextValue";
import type { SiteInfo } from "../../types/app";

export function makeSiteInfo(overrides: Partial<SiteInfo> = {}): SiteInfo {
    return {
        site_name: "RetroTide",
        deployment_mode: "hosted",
        site_mode: "public",
        registration_open: true,
        password_sign_in: true,
        email_enabled: true,
        maintenance_mode: false,
        turnstile_enabled: false,
        turnstile_site_key: "",
        identity_providers: [],
        ...overrides,
    };
}

export function makeSiteInfoContext(overrides: Partial<SiteInfoContextValue> = {}): SiteInfoContextValue {
    return {
        siteInfo: makeSiteInfo(),
        loading: false,
        error: null,
        retry: vi.fn().mockResolvedValue(undefined),
        ...overrides,
    };
}

export function makeAuthContext(overrides: Partial<AuthContextValue> = {}): AuthContextValue {
    return {
        user: null,
        guestId: null,
        permissions: [],
        siteRole: null,
        loading: false,
        error: null,
        retry: vi.fn().mockResolvedValue(undefined),
        ...overrides,
    };
}

export function makeOrgContext(overrides: Partial<OrgContextValue> = {}): OrgContextValue {
    const organisations = overrides.organisations ?? [];
    const current = overrides.current === undefined ? (organisations[0] ?? null) : overrides.current;

    return {
        loading: false,
        error: null,
        retry: vi.fn().mockResolvedValue(undefined),
        selectOrganisation: vi.fn(),
        ...overrides,
        organisations,
        current,
        role: overrides.role === undefined ? (current?.role ?? null) : overrides.role,
    };
}

export function makeThemeContext(overrides: Partial<ThemeContextValue> = {}): ThemeContextValue {
    return {
        preference: "system",
        setPreference: vi.fn(),
        ...overrides,
    };
}
