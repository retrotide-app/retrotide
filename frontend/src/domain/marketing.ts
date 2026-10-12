import type { DeploymentMode, SiteMode } from "../types/api";

export type LegalPageKey = "privacy" | "terms" | "acceptable-use" | "cookies" | "sub-processors" | "security";

export interface LegalLink {
    page: LegalPageKey;
    label: string;
}

export const PRICING_PATH = "/pricing";

export const COMPARE_EASYRETRO_PATH = "/compare/easyretro";

export const SELF_HOST_PATH = "/self-host";

export const SELF_HOST_COMPOSE_PATH = `${SELF_HOST_PATH}/docker-compose.yml`;

export const SELF_HOST_PORT = 8080;

export const TEMPLATES_PATH = "/templates";

export const TEMPLATE_ROUTE = `${TEMPLATES_PATH}/:key`;

export const LEGAL_ROUTE = "/legal/:page";

export const LEGAL_LINKS: readonly LegalLink[] = [
    { page: "privacy", label: "Privacy" },
    { page: "terms", label: "Terms" },
    { page: "acceptable-use", label: "Acceptable use" },
    { page: "cookies", label: "Cookies" },
    { page: "sub-processors", label: "Sub-processors" },
    { page: "security", label: "Security" },
];

export function legalPath(page: LegalPageKey): string {
    return `/legal/${page}`;
}

export function offersMarketing(deploymentMode: DeploymentMode, siteMode: SiteMode): boolean {
    return deploymentMode === "hosted" && siteMode === "public";
}
