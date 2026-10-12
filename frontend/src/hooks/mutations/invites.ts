import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
    acceptInvite,
    createInvite,
    registerThroughInvite,
    resendInvite,
    revokeInvite,
} from "../../api/endpoints/invites";
import { queryKeys } from "../../api/queryKeys";
import type { CreateInviteRequest } from "../../types/api";
import type { OrganisationTarget } from "./organisations";
import { refreshWhenChanged } from "./refresh";

export type InviteCreation = OrganisationTarget & CreateInviteRequest;

export interface InviteTarget extends OrganisationTarget {
    inviteId: string;
}

const GONE_STATUS = 404;

const ALREADY_MEMBER_STATUS = 409;

export function useCreateInvite() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, ...body }: InviteCreation) => createInvite(orgId, body),
        onSettled: (_data, error, { orgId }) => refreshWhenChanged(qc, error, [queryKeys.organisations.invites(orgId)]),
    });
}

export function useResendInvite() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, inviteId }: InviteTarget) => resendInvite(orgId, inviteId),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(qc, error, [queryKeys.organisations.invites(orgId)], [GONE_STATUS]),
    });
}

export function useRevokeInvite() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, inviteId }: InviteTarget) => revokeInvite(orgId, inviteId),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(qc, error, [queryKeys.organisations.invites(orgId)], [GONE_STATUS]),
    });
}

export function useAcceptInvite() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: acceptInvite,
        onSettled: (_data, error) =>
            refreshWhenChanged(qc, error, [queryKeys.organisations.list()], [ALREADY_MEMBER_STATUS]),
    });
}

export function useRegisterThroughInvite() {
    return useMutation({ mutationFn: registerThroughInvite });
}
