import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
    addTeamMember,
    archiveTeam,
    changeTeamMemberRole,
    createTeam,
    deleteTeam,
    removeTeamMember,
    renameTeam,
} from "../../api/endpoints/teams";
import { queryKeys } from "../../api/queryKeys";
import type {
    AddTeamMemberRequest,
    ChangeTeamMemberRoleRequest,
    CreateTeamRequest,
    RenameTeamRequest,
} from "../../types/api";
import type { OrganisationTarget } from "./organisations";
import { refreshWhenChanged } from "./refresh";

export type TeamCreation = OrganisationTarget & CreateTeamRequest;

export interface OrganisationTeamTarget extends OrganisationTarget {
    teamId: string;
}

export type TeamRename = OrganisationTeamTarget & RenameTeamRequest;

export interface TeamTarget {
    teamId: string;
}

export type TeamMemberAddition = TeamTarget & AddTeamMemberRequest;

export interface TeamMemberTarget extends TeamTarget {
    userId: string;
}

export type TeamMemberRoleChange = TeamMemberTarget & ChangeTeamMemberRoleRequest;

const GONE_STATUS = 404;

const CONFLICT_STATUS = 409;

export function useCreateTeam() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, ...body }: TeamCreation) => createTeam(orgId, body),
        onSettled: (_data, error, { orgId }) => refreshWhenChanged(qc, error, [queryKeys.organisations.teams(orgId)]),
    });
}

export function useArchiveTeam() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, teamId }: OrganisationTeamTarget) => archiveTeam(orgId, teamId),
        onSettled: (_data, error, { orgId, teamId }) =>
            refreshWhenChanged(
                qc,
                error,
                [
                    queryKeys.organisations.teams(orgId),
                    queryKeys.teams.scope(teamId),
                    queryKeys.boards.organisation(orgId),
                ],
                [GONE_STATUS, CONFLICT_STATUS],
            ),
    });
}

export function useDeleteTeam() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, teamId }: OrganisationTeamTarget) => deleteTeam(orgId, teamId),
        onSuccess: (_data, { teamId }) => {
            qc.removeQueries({ queryKey: queryKeys.teams.scope(teamId) });
        },
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.organisations.teams(orgId), queryKeys.boards.organisation(orgId)],
                [GONE_STATUS],
            ),
    });
}

export function useRenameTeam() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ teamId, name }: TeamRename) => renameTeam(teamId, { name }),
        onSettled: (_data, error, { orgId, teamId }) =>
            refreshWhenChanged(
                qc,
                error,
                [
                    queryKeys.organisations.teams(orgId),
                    queryKeys.teams.detail(teamId),
                    queryKeys.boards.organisation(orgId),
                ],
                [GONE_STATUS, CONFLICT_STATUS],
            ),
    });
}

export function useAddTeamMember() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ teamId, ...body }: TeamMemberAddition) => addTeamMember(teamId, body),
        onSettled: (_data, error, { teamId }) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.teams.members(teamId), queryKeys.boards.organisations],
                [CONFLICT_STATUS],
            ),
    });
}

export function useChangeTeamMemberRole() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ teamId, userId, ...body }: TeamMemberRoleChange) => changeTeamMemberRole(teamId, userId, body),
        onSettled: (_data, error, { teamId }) =>
            refreshWhenChanged(qc, error, [queryKeys.teams.members(teamId)], [GONE_STATUS]),
    });
}

export function useRemoveTeamMember() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ teamId, userId }: TeamMemberTarget) => removeTeamMember(teamId, userId),
        onSettled: (_data, error, { teamId }) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.teams.members(teamId), queryKeys.boards.organisations],
                [GONE_STATUS],
            ),
    });
}
