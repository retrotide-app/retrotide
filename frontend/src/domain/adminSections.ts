import type { OrganisationResponse, SiteMode } from "../types/api";
import { isDeleted, organisationHomePath } from "./organisations";
import type { StaffPermission } from "./permissions";

export type AdminSectionGroup = "overview" | "accounts" | "site";

export interface AdminSection {
    path: string;
    label: string;
    permission: StaffPermission;
    group: AdminSectionGroup;
    privateSiteOnly?: boolean;
}

export interface AdminSectionGroupView {
    group: AdminSectionGroup;
    label: string;
    sections: AdminSection[];
}

export const ADMIN_HOME_PATH = "/admin";

export const ADMIN_SECTION_GROUP_LABELS: Readonly<Record<AdminSectionGroup, string>> = {
    overview: "Overview",
    accounts: "Accounts",
    site: "Site",
};

const ADMIN_SECTION_GROUPS: readonly AdminSectionGroup[] = ["overview", "accounts", "site"];

export const ADMIN_SECTIONS: readonly AdminSection[] = [
    { path: ADMIN_HOME_PATH, label: "Dashboard", permission: "view_admin_panel", group: "overview" },
    { path: "/admin/users", label: "Users", permission: "view_users", group: "accounts" },
    { path: "/admin/organisations", label: "Organisations", permission: "view_organisations", group: "accounts" },
    {
        path: "/admin/invitations",
        label: "Invitations",
        permission: "manage_user_account",
        group: "accounts",
        privateSiteOnly: true,
    },
    { path: "/admin/permissions", label: "Permissions", permission: "manage_roles", group: "site" },
    { path: "/admin/settings", label: "Settings", permission: "manage_settings", group: "site" },
    { path: "/admin/audit-log", label: "Audit log", permission: "view_audit_log", group: "site" },
];

export function visibleAdminSections(granted: readonly StaffPermission[], siteMode: SiteMode): AdminSection[] {
    return ADMIN_SECTIONS.filter(
        section => granted.includes(section.permission) && (siteMode === "private" || section.privateSiteOnly !== true),
    );
}

export function appReturnPath(current: OrganisationResponse | null): string {
    if (current === null || isDeleted(current)) {
        return "/account";
    }

    return organisationHomePath(current.slug);
}

export function groupedAdminSections(granted: readonly StaffPermission[], siteMode: SiteMode): AdminSectionGroupView[] {
    const visible = visibleAdminSections(granted, siteMode);

    return ADMIN_SECTION_GROUPS.map(group => ({
        group,
        label: ADMIN_SECTION_GROUP_LABELS[group],
        sections: visible.filter(section => section.group === group),
    })).filter(view => view.sections.length > 0);
}
