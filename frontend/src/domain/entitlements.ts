import type { PlanCode, PlanLimit, UpgradeRequiredResponse } from "../types/api";
import { errorCode, errorStatus } from "../utils/errorMessage";

export const UPGRADE_REQUIRED_STATUS = 402;

export const UPGRADE_REQUIRED_CODE = "upgrade_required";

export const PLAN_LABELS: Readonly<Record<PlanCode, string>> = {
    free: "Free",
};

const PLAN_NAMES: Readonly<Record<PlanCode, string>> = {
    free: "free plan",
};

const LIMIT_NOUNS: Readonly<Record<PlanLimit, { one: string; many: string }>> = {
    teams: { one: "team", many: "teams" },
    members: { one: "member", many: "members" },
    saved_templates: { one: "saved template", many: "saved templates" },
    card_image_bytes: { one: "byte per image", many: "bytes per image" },
    media_bytes: { one: "byte of image storage", many: "bytes of image storage" },
};

function isRecord(value: unknown): value is Record<string, unknown> {
    return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isPlanCode(value: unknown): value is PlanCode {
    return typeof value === "string" && Object.hasOwn(PLAN_NAMES, value);
}

function isPlanLimit(value: unknown): value is PlanLimit {
    return typeof value === "string" && Object.hasOwn(LIMIT_NOUNS, value);
}

export function readUpgradeRequired(error: unknown): UpgradeRequiredResponse | null {
    if (errorStatus(error) !== UPGRADE_REQUIRED_STATUS || errorCode(error) !== UPGRADE_REQUIRED_CODE) {
        return null;
    }

    const body: unknown = isRecord(error) ? error.body : undefined;

    if (!isRecord(body)) {
        return null;
    }

    const { limit, allowed, current, plan } = body;

    if (!isPlanLimit(limit) || typeof allowed !== "number" || typeof current !== "number" || !isPlanCode(plan)) {
        return null;
    }

    const message = typeof body.error === "string" ? body.error : "";

    return { error: message, code: UPGRADE_REQUIRED_CODE, limit, allowed, current, plan };
}

export function planAllowance(upgrade: Pick<UpgradeRequiredResponse, "plan" | "limit" | "allowed">): string {
    const noun = LIMIT_NOUNS[upgrade.limit];
    const counted = upgrade.allowed === 1 ? noun.one : noun.many;
    const plan = PLAN_NAMES[upgrade.plan];

    return `The ${plan} allows ${upgrade.allowed} ${counted}.`;
}
