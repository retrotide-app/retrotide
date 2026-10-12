import type {
    AdminInvitationListResponse,
    AdminInvitationResponse,
    AdminPermissionsResponse,
    AdminUserDetailResponse,
    AdminUserListResponse,
    AdminUserResponse,
    AuditLogEntryResponse,
    AuditLogListResponse,
} from "../../types/api";

export function makeAdminInvitation(overrides: Partial<AdminInvitationResponse> = {}): AdminInvitationResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000bb",
        email: "kit@example.test",
        expired: false,
        expires_at: "2026-10-03T12:00:00Z",
        sent_at: "2026-09-26T12:00:00Z",
        created_at: "2026-09-26T12:00:00Z",
        ...overrides,
    };
}

export function makeAdminInvitationList(
    invitations: AdminInvitationResponse[],
    overrides: Partial<AdminInvitationListResponse> = {},
): AdminInvitationListResponse {
    return { invitations, total: invitations.length, limit: 20, offset: 0, ...overrides };
}

export function makeAdminUser(overrides: Partial<AdminUserResponse> = {}): AdminUserResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000aa",
        email: "robin@example.test",
        display_name: "Robin",
        email_verified: true,
        banned: false,
        locked: false,
        created_at: "2026-03-01T08:00:00Z",
        ...overrides,
    };
}

export function makeAdminUserDetail(overrides: Partial<AdminUserDetailResponse> = {}): AdminUserDetailResponse {
    return {
        ...makeAdminUser(),
        has_password: true,
        ...overrides,
    };
}

export function makeAdminUserList(
    users: AdminUserResponse[],
    overrides: Partial<AdminUserListResponse> = {},
): AdminUserListResponse {
    return { users, total: users.length, limit: 20, offset: 0, ...overrides };
}

export function makeAuditLogEntry(overrides: Partial<AuditLogEntryResponse> = {}): AuditLogEntryResponse {
    return {
        id: 1,
        actor_id: "00000000-0000-4000-8000-000000000001",
        actor_kind: "staff",
        actor_name: "Sam",
        action: "ban_user",
        target_type: "user",
        target_id: "00000000-0000-4000-8000-0000000000aa",
        subject_id: "00000000-0000-4000-8000-0000000000aa",
        subject_name: "Robin",
        details: { reason: "spam" },
        created_at: "2026-09-20T12:00:00Z",
        ...overrides,
    };
}

export function makeAuditLogList(
    entries: AuditLogEntryResponse[],
    overrides: Partial<AuditLogListResponse> = {},
): AuditLogListResponse {
    return { entries, total: entries.length, limit: 50, offset: 0, ...overrides };
}

export function makeAdminPermissions(overrides: Partial<AdminPermissionsResponse> = {}): AdminPermissionsResponse {
    return {
        permissions: [
            { permission: "view_admin_panel", label: "View admin panel" },
            { permission: "view_stats", label: "View site stats" },
            { permission: "view_audit_log", label: "View audit log" },
            { permission: "view_users", label: "View user records" },
            { permission: "ban_user", label: "Ban and lock users" },
            { permission: "manage_user_account", label: "Manage user accounts" },
            { permission: "manage_user_email", label: "Manage user email addresses" },
            { permission: "set_email_verified", label: "Set email verified" },
            { permission: "reset_password", label: "Reset user passwords" },
            { permission: "delete_any_user", label: "Delete any user" },
            { permission: "view_organisations", label: "View organisations" },
            { permission: "grant_entitlements", label: "Grant entitlements" },
        ],
        roles: [
            {
                role: "support",
                label: "Support",
                permissions: [
                    "ban_user",
                    "manage_user_account",
                    "view_admin_panel",
                    "view_organisations",
                    "view_stats",
                    "view_users",
                ],
            },
        ],
        ...overrides,
    };
}
