import { type QueryClient, useMutation, useQueryClient } from "@tanstack/react-query";
import {
    banUser,
    cancelInvitation,
    deleteUser,
    forceLogoutUser,
    inviteUser,
    lockUser,
    removeUserRole,
    resendInvitation,
    resetUserPassword,
    sendTestEmail,
    setUserDisplayName,
    setUserEmail,
    setUserRole,
    unbanUser,
    unlockUser,
    unverifyUserEmail,
    updateAdminSettings,
    updateRolePermissions,
    verifyUserEmail,
} from "../../api/endpoints/admin";
import { queryKeys } from "../../api/queryKeys";
import type {
    AdminSetEmailRequest,
    BanUserRequest,
    LockUserRequest,
    SetRoleRequest,
    StaffRole,
    UpdateDisplayNameRequest,
    UpdatePermissionsRequest,
} from "../../types/api";
import { errorStatus } from "../../utils/errorMessage";

export interface AdminUserTarget {
    userId: string;
}

export type AdminRoleChange = AdminUserTarget & SetRoleRequest;

export type AdminBan = AdminUserTarget & BanUserRequest;

export type AdminLock = AdminUserTarget & LockUserRequest;

export type AdminEmailChange = AdminUserTarget & AdminSetEmailRequest;

export type AdminDisplayNameChange = AdminUserTarget & UpdateDisplayNameRequest;

export type AdminRolePermissionsChange = { role: StaffRole } & UpdatePermissionsRequest;

export interface AdminInvitationTarget {
    invitationId: string;
}

const SAVED_BUT_FAILED_STATUS = 500;

const GONE_STATUS = 404;

function mayHaveChanged(error: Error | null): boolean {
    return error === null || errorStatus(error) === SAVED_BUT_FAILED_STATUS;
}

async function refreshInvitations(qc: QueryClient, error: Error | null): Promise<void> {
    if (!mayHaveChanged(error) && errorStatus(error) !== GONE_STATUS) {
        return;
    }

    await Promise.all([
        qc.invalidateQueries({ queryKey: queryKeys.admin.invitations.all }),
        qc.invalidateQueries({ queryKey: queryKeys.admin.auditLog.all }),
    ]);
}

async function refreshUserRecords(qc: QueryClient, error: Error | null): Promise<void> {
    if (!mayHaveChanged(error)) {
        return;
    }

    await Promise.all([
        qc.invalidateQueries({ queryKey: queryKeys.admin.users.all }),
        qc.invalidateQueries({ queryKey: queryKeys.admin.organisations.all }),
        qc.invalidateQueries({ queryKey: queryKeys.admin.stats() }),
        qc.invalidateQueries({ queryKey: queryKeys.admin.auditLog.all }),
    ]);
}

export function useSetUserRole() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId, ...body }: AdminRoleChange) => setUserRole(userId, body),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useRemoveUserRole() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId, ...body }: AdminRoleChange) => removeUserRole(userId, body),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useBanUser() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId, ...body }: AdminBan) => banUser(userId, body),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useUnbanUser() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId }: AdminUserTarget) => unbanUser(userId),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useLockUser() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId, ...body }: AdminLock) => lockUser(userId, body),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useUnlockUser() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId }: AdminUserTarget) => unlockUser(userId),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useForceLogoutUser() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId }: AdminUserTarget) => forceLogoutUser(userId),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useDeleteUser() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId }: AdminUserTarget) => deleteUser(userId),
        onSuccess: async (_data, { userId }) => {
            qc.removeQueries({ queryKey: queryKeys.admin.users.detail(userId) });
            qc.removeQueries({ queryKey: queryKeys.admin.users.auditLogPages(userId) });

            await refreshUserRecords(qc, null);
        },
        onError: error => refreshUserRecords(qc, error),
    });
}

export function useResetUserPassword() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId }: AdminUserTarget) => resetUserPassword(userId),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useSetUserEmail() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId, ...body }: AdminEmailChange) => setUserEmail(userId, body),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useVerifyUserEmail() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId }: AdminUserTarget) => verifyUserEmail(userId),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useUnverifyUserEmail() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId }: AdminUserTarget) => unverifyUserEmail(userId),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useSetUserDisplayName() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ userId, ...body }: AdminDisplayNameChange) => setUserDisplayName(userId, body),
        onSettled: (_data, error) => refreshUserRecords(qc, error),
    });
}

export function useInviteUser() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: inviteUser,
        onSettled: (_data, error) => refreshInvitations(qc, error),
    });
}

export function useResendInvitation() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ invitationId }: AdminInvitationTarget) => resendInvitation(invitationId),
        onSettled: (_data, error) => refreshInvitations(qc, error),
    });
}

export function useCancelInvitation() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ invitationId }: AdminInvitationTarget) => cancelInvitation(invitationId),
        onSettled: (_data, error) => refreshInvitations(qc, error),
    });
}

export function useUpdateAdminSettings() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: updateAdminSettings,
        onSettled: async (_data, error) => {
            if (!mayHaveChanged(error)) {
                return;
            }

            await Promise.all([
                qc.invalidateQueries({ queryKey: queryKeys.admin.settings() }),
                qc.invalidateQueries({ queryKey: queryKeys.siteInfo() }),
                qc.invalidateQueries({ queryKey: queryKeys.admin.stats() }),
                qc.invalidateQueries({ queryKey: queryKeys.admin.auditLog.all }),
            ]);
        },
    });
}

export function useSendTestEmail() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: sendTestEmail,
        onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.admin.auditLog.all }),
    });
}

export function useUpdateRolePermissions() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ role, ...body }: AdminRolePermissionsChange) => updateRolePermissions(role, body),
        onSettled: async (_data, error) => {
            if (!mayHaveChanged(error)) {
                return;
            }

            await Promise.all([
                qc.invalidateQueries({ queryKey: queryKeys.admin.permissions() }),
                qc.invalidateQueries({ queryKey: queryKeys.admin.auditLog.all }),
            ]);
        },
    });
}
