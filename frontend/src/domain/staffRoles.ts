import type { StaffRole } from "../types/api";
import type { StaffPermission } from "./permissions";

export type StaffActionBlock = "self" | "super_admin" | "outranked";

export interface StaffActionTarget {
    id: string;
    role?: StaffRole;
}

export const STAFF_ROLE_LABELS: Readonly<Record<StaffRole, string>> = {
    super_admin: "Super admin",
    admin: "Admin",
    support: "Support",
};

export const GRANTABLE_STAFF_ROLES: readonly StaffRole[] = ["support", "admin"];

export function staffRoleLabel(role: StaffRole | undefined): string {
    if (role === undefined) {
        return "No role";
    }

    return STAFF_ROLE_LABELS[role];
}

export function isGrantableStaffRole(value: string | undefined): value is StaffRole {
    return GRANTABLE_STAFF_ROLES.some(role => role === value);
}

export function withPermission(granted: readonly string[], permission: string, on: boolean): string[] {
    const without = granted.filter(held => held !== permission);

    if (!on) {
        return without;
    }

    return [...without, permission];
}

export function samePermissions(a: readonly string[], b: readonly string[]): boolean {
    const left = new Set(a);
    const right = new Set(b);

    if (left.size !== right.size) {
        return false;
    }

    return [...left].every(permission => right.has(permission));
}

export function staffActionBlock(
    viewerId: string | null,
    viewerPermissions: readonly StaffPermission[],
    target: StaffActionTarget,
): StaffActionBlock | null {
    if (viewerId === target.id) {
        return "self";
    }

    if (target.role === "super_admin") {
        return "super_admin";
    }

    if (target.role !== undefined && !viewerPermissions.includes("manage_roles")) {
        return "outranked";
    }

    return null;
}
