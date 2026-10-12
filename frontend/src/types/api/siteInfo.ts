import type { IdentityProviderResponse } from "./identity";

export type DeploymentMode = "hosted" | "self_hosted";

export type SiteMode = "public" | "private";

export interface SiteInfoResponse {
    site_name: string;
    deployment_mode: DeploymentMode;
    site_mode: SiteMode;
    registration_open: boolean;
    password_sign_in: boolean;
    email_enabled: boolean;
    maintenance_mode: boolean;
    turnstile_enabled: boolean;
    turnstile_site_key: string;
    identity_providers: IdentityProviderResponse[];
}
