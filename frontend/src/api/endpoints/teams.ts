import type {
    AddTeamMemberRequest,
    ChangeTeamMemberRoleRequest,
    CreateTeamRequest,
    RenameTeamRequest,
    TeamListResponse,
    TeamMemberListResponse,
    TeamResponse,
} from "../../types/api";
import { apiDelete, apiFetch, apiPost, apiPut } from "../client";

function organisationTeamsPath(orgId: string, action = ""): string {
    return `/orgs/${encodeURIComponent(orgId)}/teams${action}`;
}

function teamPath(teamId: string, action = ""): string {
    return `/teams/${encodeURIComponent(teamId)}${action}`;
}

function teamMemberPath(teamId: string, userId: string, action = ""): string {
    return teamPath(teamId, `/members/${encodeURIComponent(userId)}${action}`);
}

export async function listTeams(orgId: string): Promise<TeamListResponse> {
    return apiFetch<TeamListResponse>(organisationTeamsPath(orgId));
}

export async function createTeam(orgId: string, body: CreateTeamRequest): Promise<TeamResponse> {
    return apiPost<TeamResponse, CreateTeamRequest>(organisationTeamsPath(orgId), body);
}

export async function archiveTeam(orgId: string, teamId: string): Promise<void> {
    return apiPost<void>(organisationTeamsPath(orgId, `/${encodeURIComponent(teamId)}/archive`));
}

export async function deleteTeam(orgId: string, teamId: string): Promise<void> {
    return apiDelete<void>(organisationTeamsPath(orgId, `/${encodeURIComponent(teamId)}`));
}

export async function getTeam(teamId: string): Promise<TeamResponse> {
    return apiFetch<TeamResponse>(teamPath(teamId));
}

export async function renameTeam(teamId: string, body: RenameTeamRequest): Promise<TeamResponse> {
    return apiPut<TeamResponse, RenameTeamRequest>(teamPath(teamId), body);
}

export async function listTeamMembers(teamId: string): Promise<TeamMemberListResponse> {
    return apiFetch<TeamMemberListResponse>(teamPath(teamId, "/members"));
}

export async function addTeamMember(teamId: string, body: AddTeamMemberRequest): Promise<void> {
    return apiPost<void, AddTeamMemberRequest>(teamPath(teamId, "/members"), body);
}

export async function changeTeamMemberRole(
    teamId: string,
    userId: string,
    body: ChangeTeamMemberRoleRequest,
): Promise<void> {
    return apiPut<void, ChangeTeamMemberRoleRequest>(teamMemberPath(teamId, userId, "/role"), body);
}

export async function removeTeamMember(teamId: string, userId: string): Promise<void> {
    return apiDelete<void>(teamMemberPath(teamId, userId));
}
