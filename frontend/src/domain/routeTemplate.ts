import { JOIN_BOARD_ROUTE, SHARED_BOARD_ROUTE, SHARED_BOARD_SUMMARY_ROUTE } from "./board/paths";
import { BOARD_PRINT_ROUTE } from "./board/print";
import {
    COMPARE_EASYRETRO_PATH,
    LEGAL_ROUTE,
    PRICING_PATH,
    PRICING_VISIBLE,
    SELF_HOST_PATH,
    TEMPLATE_ROUTE,
    TEMPLATES_PATH,
} from "./marketing";
import { INVITE_PATH, ONBOARDING_PATH } from "./organisations";
import { TWO_FACTOR_PATH } from "./twoFactor";

export const UNKNOWN_ROUTE_TEMPLATE = "unknown";

const PARAM_PREFIX = ":";

const ORGANISATION_ROUTE = "/o/:orgSlug";

const ADMIN_ROUTE = "/admin";

const ORGANISATION_CHILD_ROUTES: readonly string[] = [
    "teams",
    "teams/:teamId",
    "action-items",
    "templates",
    "boards/new",
    "boards/:boardId",
    "boards/:boardId/settings",
    "boards/:boardId/summary",
    "members",
    "invites",
    "settings",
];

const ADMIN_CHILD_ROUTES: readonly string[] = [
    "users",
    "users/:userId",
    "organisations",
    "organisations/:orgId",
    "invitations",
    "permissions",
    "settings",
    "audit-log",
];

const ROUTE_TEMPLATES: readonly string[] = [
    "/",
    ...(PRICING_VISIBLE ? [PRICING_PATH] : []),
    COMPARE_EASYRETRO_PATH,
    SELF_HOST_PATH,
    TEMPLATES_PATH,
    TEMPLATE_ROUTE,
    LEGAL_ROUTE,
    "/login",
    "/register",
    "/verify-email",
    "/forgot-password",
    "/reset-password",
    TWO_FACTOR_PATH,
    "/accept-invitation",
    INVITE_PATH,
    JOIN_BOARD_ROUTE,
    BOARD_PRINT_ROUTE,
    SHARED_BOARD_ROUTE,
    SHARED_BOARD_SUMMARY_ROUTE,
    ONBOARDING_PATH,
    "/account",
    ORGANISATION_ROUTE,
    ...ORGANISATION_CHILD_ROUTES.map(child => `${ORGANISATION_ROUTE}/${child}`),
    ADMIN_ROUTE,
    ...ADMIN_CHILD_ROUTES.map(child => `${ADMIN_ROUTE}/${child}`),
];

function segmentsOf(path: string): string[] {
    return path.split("/").filter(segment => segment !== "");
}

function matches(templateSegments: readonly string[], pathSegments: readonly string[]): boolean {
    if (templateSegments.length !== pathSegments.length) {
        return false;
    }

    return templateSegments.every(
        (segment, index) =>
            segment.startsWith(PARAM_PREFIX) || segment.toLowerCase() === pathSegments[index].toLowerCase(),
    );
}

export function routeTemplate(pathname: string): string {
    const pathSegments = segmentsOf(pathname);

    let best: string | null = null;
    let bestScore = -1;

    for (const template of ROUTE_TEMPLATES) {
        const templateSegments = segmentsOf(template);

        if (!matches(templateSegments, pathSegments)) {
            continue;
        }

        const score = templateSegments.filter(segment => !segment.startsWith(PARAM_PREFIX)).length;

        if (score > bestScore) {
            best = template;
            bestScore = score;
        }
    }

    return best ?? UNKNOWN_ROUTE_TEMPLATE;
}
