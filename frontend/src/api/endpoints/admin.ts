import type {
    AdminInvitationListResponse,
    AdminInvitationResponse,
    AdminInviteRequest,
    AdminPasswordResetResponse,
    AdminPermissionsResponse,
    AdminSetEmailRequest,
    AdminSettingsResponse,
    AdminUserDetailResponse,
    AdminUserListQuery,
    AdminUserListResponse,
    AuditLogListResponse,
    AuditLogQuery,
    BanUserRequest,
    LockUserRequest,
    PageQuery,
    SetRoleRequest,
    StaffRole,
    UpdateDisplayNameRequest,
    UpdatePermissionsRequest,
    UpdateSettingsRequest,
} from "../../types/api";
import { apiDelete, apiDeleteWithBody, apiFetch, apiPost, apiPut, buildQueryString } from "../client";

function userPath(userId: string, action = ""): string {
    return `/admin/users/${encodeURIComponent(userId)}${action}`;
}

function invitationPath(invitationId: string, action = ""): string {
    return `/admin/invitations/${encodeURIComponent(invitationId)}${action}`;
}

function pageQueryString(page: PageQuery): string {
    return buildQueryString({ limit: page.limit, offset: page.offset });
}

export async function getAdminUsers(query: AdminUserListQuery): Promise<AdminUserListResponse> {
    const qs = buildQueryString({ search: query.search, limit: query.limit, offset: query.offset });

    return apiFetch<AdminUserListResponse>(`/admin/users${qs}`);
}

export async function getAdminUser(userId: string): Promise<AdminUserDetailResponse> {
    return apiFetch<AdminUserDetailResponse>(userPath(userId));
}

export async function getAdminUserAuditLog(userId: string, page: PageQuery): Promise<AuditLogListResponse> {
    return apiFetch<AuditLogListResponse>(`${userPath(userId, "/audit-log")}${pageQueryString(page)}`);
}

export async function setUserRole(userId: string, body: SetRoleRequest): Promise<void> {
    return apiPost<void, SetRoleRequest>(userPath(userId, "/role"), body);
}

export async function removeUserRole(userId: string, body: SetRoleRequest): Promise<void> {
    return apiDeleteWithBody<void, SetRoleRequest>(userPath(userId, "/role"), body);
}

export async function banUser(userId: string, body: BanUserRequest): Promise<void> {
    return apiPost<void, BanUserRequest>(userPath(userId, "/ban"), body);
}

export async function unbanUser(userId: string): Promise<void> {
    return apiPost<void>(userPath(userId, "/unban"));
}

export async function lockUser(userId: string, body: LockUserRequest): Promise<void> {
    return apiPost<void, LockUserRequest>(userPath(userId, "/lock"), body);
}

export async function unlockUser(userId: string): Promise<void> {
    return apiPost<void>(userPath(userId, "/unlock"));
}

export async function forceLogoutUser(userId: string): Promise<void> {
    return apiPost<void>(userPath(userId, "/force-logout"));
}

export async function deleteUser(userId: string): Promise<void> {
    return apiDelete<void>(userPath(userId));
}

export async function resetUserPassword(userId: string): Promise<AdminPasswordResetResponse> {
    return apiPost<AdminPasswordResetResponse>(userPath(userId, "/reset-password"));
}

export async function setUserEmail(userId: string, body: AdminSetEmailRequest): Promise<void> {
    return apiPut<void, AdminSetEmailRequest>(userPath(userId, "/email"), body);
}

export async function verifyUserEmail(userId: string): Promise<void> {
    return apiPost<void>(userPath(userId, "/verify-email"));
}

export async function unverifyUserEmail(userId: string): Promise<void> {
    return apiPost<void>(userPath(userId, "/unverify-email"));
}

export async function setUserDisplayName(userId: string, body: UpdateDisplayNameRequest): Promise<void> {
    return apiPut<void, UpdateDisplayNameRequest>(userPath(userId, "/display-name"), body);
}

export async function getAdminInvitations(page: PageQuery): Promise<AdminInvitationListResponse> {
    return apiFetch<AdminInvitationListResponse>(`/admin/invitations${pageQueryString(page)}`);
}

export async function inviteUser(body: AdminInviteRequest): Promise<AdminInvitationResponse> {
    return apiPost<AdminInvitationResponse, AdminInviteRequest>("/admin/invitations", body);
}

export async function resendInvitation(invitationId: string): Promise<void> {
    return apiPost<void>(invitationPath(invitationId, "/resend"));
}

export async function cancelInvitation(invitationId: string): Promise<void> {
    return apiDelete<void>(invitationPath(invitationId));
}

export async function getAdminSettings(): Promise<AdminSettingsResponse> {
    return apiFetch<AdminSettingsResponse>("/admin/settings");
}

export async function updateAdminSettings(body: UpdateSettingsRequest): Promise<void> {
    return apiPut<void, UpdateSettingsRequest>("/admin/settings", body);
}

export async function sendTestEmail(): Promise<void> {
    return apiPost<void>("/admin/settings/test-email");
}

export async function getAdminPermissions(): Promise<AdminPermissionsResponse> {
    return apiFetch<AdminPermissionsResponse>("/admin/permissions");
}

export async function updateRolePermissions(role: StaffRole, body: UpdatePermissionsRequest): Promise<void> {
    return apiPut<void, UpdatePermissionsRequest>(`/admin/permissions/roles/${encodeURIComponent(role)}`, body);
}

export async function getAuditLog(query: AuditLogQuery): Promise<AuditLogListResponse> {
    const qs = buildQueryString({ action: query.action, limit: query.limit, offset: query.offset });

    return apiFetch<AuditLogListResponse>(`/admin/audit-log${qs}`);
}
