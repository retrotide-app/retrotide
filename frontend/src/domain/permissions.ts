export type StaffPermission =
    | "view_admin_panel"
    | "view_stats"
    | "view_audit_log"
    | "manage_settings"
    | "manage_roles"
    | "view_users"
    | "ban_user"
    | "manage_user_account"
    | "manage_user_email"
    | "set_email_verified"
    | "reset_password"
    | "delete_any_user"
    | "view_organisations"
    | "grant_entitlements";

export const STAFF_PERMISSIONS: readonly StaffPermission[] = [
    "view_admin_panel",
    "view_stats",
    "view_audit_log",
    "manage_settings",
    "manage_roles",
    "view_users",
    "ban_user",
    "manage_user_account",
    "manage_user_email",
    "set_email_verified",
    "reset_password",
    "delete_any_user",
    "view_organisations",
    "grant_entitlements",
];

export function staffPermissions(granted: readonly string[]): StaffPermission[] {
    return STAFF_PERMISSIONS.filter(permission => granted.includes(permission));
}
