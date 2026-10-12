import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
    changeMemberRole,
    createWorkspace,
    deleteOrganisation,
    leaveOrganisation,
    removeMember,
    restoreOrganisation,
    transferOwnership,
    updateOrganisation,
} from "../../api/endpoints/organisations";
import { queryKeys } from "../../api/queryKeys";
import { withOrganisation } from "../../domain/organisations";
import type {
    ChangeMemberRoleRequest,
    OrganisationListResponse,
    TransferOwnershipRequest,
    UpdateOrganisationRequest,
} from "../../types/api";
import { refreshWhenChanged } from "./refresh";

export interface OrganisationTarget {
    orgId: string;
}

export type OrganisationUpdate = OrganisationTarget & UpdateOrganisationRequest;

export type OwnershipTransfer = OrganisationTarget & TransferOwnershipRequest;

export interface OrganisationMemberTarget extends OrganisationTarget {
    userId: string;
}

export type OrganisationMemberRoleChange = OrganisationMemberTarget & ChangeMemberRoleRequest;

const GONE_STATUS = 404;

const CHANGED_MEANWHILE_STATUS = 409;

export function useCreateWorkspace() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: createWorkspace,
        onSuccess: workspace => {
            qc.setQueryData<OrganisationListResponse>(queryKeys.organisations.list(), listing =>
                withOrganisation(listing, workspace.organisation),
            );
        },
        onSettled: (_data, error) => refreshWhenChanged(qc, error, [queryKeys.organisations.list()]),
    });
}

export function useUpdateOrganisation() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, ...body }: OrganisationUpdate) => updateOrganisation(orgId, body),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(qc, error, [queryKeys.organisations.list(), queryKeys.organisations.detail(orgId)]),
    });
}

export function useDeleteOrganisation() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId }: OrganisationTarget) => deleteOrganisation(orgId),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.organisations.list(), queryKeys.organisations.scope(orgId)],
                [GONE_STATUS],
            ),
    });
}

export function useRestoreOrganisation() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId }: OrganisationTarget) => restoreOrganisation(orgId),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.organisations.list(), queryKeys.organisations.scope(orgId)],
                [CHANGED_MEANWHILE_STATUS],
            ),
    });
}

export function useTransferOwnership() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, ...body }: OwnershipTransfer) => transferOwnership(orgId, body),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.organisations.list(), queryKeys.organisations.scope(orgId)],
                [GONE_STATUS, CHANGED_MEANWHILE_STATUS],
            ),
    });
}

export function useLeaveOrganisation() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId }: OrganisationTarget) => leaveOrganisation(orgId),
        onSuccess: (_data, { orgId }) => {
            qc.removeQueries({ queryKey: queryKeys.organisations.scope(orgId) });
        },
        onSettled: (_data, error) => refreshWhenChanged(qc, error, [queryKeys.organisations.list()], [GONE_STATUS]),
    });
}

export function useChangeMemberRole() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, userId, ...body }: OrganisationMemberRoleChange) => changeMemberRole(orgId, userId, body),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(qc, error, [queryKeys.organisations.members(orgId)], [GONE_STATUS]),
    });
}

export function useRemoveMember() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, userId }: OrganisationMemberTarget) => removeMember(orgId, userId),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(qc, error, [queryKeys.organisations.members(orgId), queryKeys.teams.all], [GONE_STATUS]),
    });
}
