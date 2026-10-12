import { keepPreviousData, useQuery } from "@tanstack/react-query";
import {
    getAdminInvitations,
    getAdminPermissions,
    getAdminSettings,
    getAdminUser,
    getAdminUserAuditLog,
    getAdminUsers,
    getAuditLog,
} from "../../api/endpoints/admin";
import { queryKeys } from "../../api/queryKeys";
import type {
    AdminInvitationListResponse,
    AdminPermissionsResponse,
    AdminSettingsResponse,
    AdminUserDetailResponse,
    AdminUserListQuery,
    AdminUserListResponse,
    AuditLogListResponse,
    AuditLogQuery,
    PageQuery,
} from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useAdminUsers(query: AdminUserListQuery): QueryState<AdminUserListResponse> {
    const users = useQuery({
        queryKey: queryKeys.admin.users.list(query),
        queryFn: () => getAdminUsers(query),
        placeholderData: keepPreviousData,
    });

    return queryState(users);
}

export function useAdminUser(userId: string): QueryState<AdminUserDetailResponse> {
    const user = useQuery({
        queryKey: queryKeys.admin.users.detail(userId),
        queryFn: () => getAdminUser(userId),
        enabled: userId !== "",
    });

    return queryState(user);
}

export function useAdminUserAuditLog(
    userId: string,
    page: PageQuery,
    enabled = true,
): QueryState<AuditLogListResponse> {
    const log = useQuery({
        queryKey: queryKeys.admin.users.auditLog(userId, page),
        queryFn: () => getAdminUserAuditLog(userId, page),
        enabled: enabled && userId !== "",
        placeholderData: keepPreviousData,
    });

    return queryState(log);
}

export function useAdminInvitations(page: PageQuery): QueryState<AdminInvitationListResponse> {
    const invitations = useQuery({
        queryKey: queryKeys.admin.invitations.list(page),
        queryFn: () => getAdminInvitations(page),
        placeholderData: keepPreviousData,
    });

    return queryState(invitations);
}

export function useAdminSettings(): QueryState<AdminSettingsResponse> {
    const settings = useQuery({ queryKey: queryKeys.admin.settings(), queryFn: getAdminSettings });

    return queryState(settings);
}

export function useAdminPermissions(): QueryState<AdminPermissionsResponse> {
    const permissions = useQuery({ queryKey: queryKeys.admin.permissions(), queryFn: getAdminPermissions });

    return queryState(permissions);
}

export function useAuditLog(query: AuditLogQuery): QueryState<AuditLogListResponse> {
    const log = useQuery({
        queryKey: queryKeys.admin.auditLog.list(query),
        queryFn: () => getAuditLog(query),
        placeholderData: keepPreviousData,
    });

    return queryState(log);
}
