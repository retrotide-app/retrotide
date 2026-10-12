export type StaffRole = "super_admin" | "admin" | "support";

export type AuditActorKind = "user" | "staff" | "system";

export type AuditTargetType = "user" | "settings" | "role" | "identity" | "invitation";

export type AuditAction =
    | "org_created"
    | "org_updated"
    | "org_deleted"
    | "org_restored"
    | "org_ownership_transferred"
    | "member_invited"
    | "invite_revoked"
    | "member_joined"
    | "member_role_changed"
    | "member_removed"
    | "team_created"
    | "team_archived"
    | "team_deleted"
    | "team_member_added"
    | "team_member_role_changed"
    | "team_member_removed"
    | "board_created"
    | "board_archived"
    | "board_deleted"
    | "board_restored"
    | "board_sharing_changed"
    | "board_password_changed"
    | "board_link_rotated"
    | "board_closed"
    | "subscription_changed"
    | "entitlement_override_set"
    | "guest_claimed"
    | "identity_linked"
    | "identity_unlinked"
    | "invitation_accepted"
    | "two_factor_enabled"
    | "two_factor_disabled"
    | "set_role"
    | "remove_role"
    | "ban_user"
    | "unban_user"
    | "lock_user"
    | "unlock_user"
    | "delete_user"
    | "reset_password"
    | "set_user_email"
    | "verify_user_email"
    | "unverify_user_email"
    | "set_display_name"
    | "force_logout"
    | "update_settings"
    | "send_test_email"
    | "update_role_permissions"
    | "invite_user"
    | "resend_invitation"
    | "cancel_invitation";

export interface PageQuery {
    limit?: number;
    offset?: number;
}

export interface AdminUserListQuery extends PageQuery {
    search?: string;
}

export interface AuditLogQuery extends PageQuery {
    action?: AuditAction;
}

export interface AdminUserResponse {
    id: string;
    email: string;
    display_name: string;
    role?: StaffRole;
    email_verified: boolean;
    banned: boolean;
    locked: boolean;
    created_at: string;
}

export interface AdminUserListResponse {
    users: AdminUserResponse[];
    total: number;
    limit: number;
    offset: number;
}

export interface AdminUserDetailResponse extends AdminUserResponse {
    has_password: boolean;
    ban_reason?: string;
    banned_at?: string;
    banned_by?: string;
    lock_reason?: string;
    locked_at?: string;
    locked_by?: string;
    last_seen_ip?: string;
    last_seen_ip_at?: string;
}

export interface AdminPasswordResetResponse {
    password: string;
}

export interface SetRoleRequest {
    role: StaffRole;
}

export interface BanUserRequest {
    reason: string;
}

export interface LockUserRequest {
    reason: string;
}

export interface AdminSetEmailRequest {
    email: string;
}

export interface AuditLogEntryResponse {
    id: number;
    actor_id?: string;
    actor_kind: AuditActorKind;
    actor_name?: string;
    action: AuditAction;
    target_type: AuditTargetType;
    target_id?: string;
    subject_id?: string;
    subject_name?: string;
    details: Record<string, string>;
    created_at: string;
}

export interface AuditLogListResponse {
    entries: AuditLogEntryResponse[];
    total: number;
    limit: number;
    offset: number;
}

export interface AdminInviteRequest {
    email: string;
}

export interface AdminInvitationResponse {
    id: string;
    email: string;
    expired: boolean;
    expires_at: string;
    sent_at: string;
    created_at: string;
}

export interface AdminInvitationListResponse {
    invitations: AdminInvitationResponse[];
    total: number;
    limit: number;
    offset: number;
}

export interface AdminSettingsResponse {
    settings: Record<string, string>;
}

export interface UpdateSettingsRequest {
    settings: Record<string, string>;
}

export interface PermissionCatalogueItem {
    permission: string;
    label: string;
}

export interface RolePermissionsItem {
    role: StaffRole;
    label: string;
    permissions: string[];
}

export interface AdminPermissionsResponse {
    permissions: PermissionCatalogueItem[];
    roles: RolePermissionsItem[];
}

export interface UpdatePermissionsRequest {
    permissions: string[];
}
