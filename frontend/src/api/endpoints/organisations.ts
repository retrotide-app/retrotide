import type {
    ChangeMemberRoleRequest,
    CreateWorkspaceRequest,
    EntitlementsResponse,
    OrganisationListResponse,
    OrganisationMemberListResponse,
    OrganisationResponse,
    TransferOwnershipRequest,
    UpdateOrganisationRequest,
    WorkspaceResponse,
} from "../../types/api";
import { apiDelete, apiFetch, apiPost, apiPut } from "../client";

function organisationPath(orgId: string, action = ""): string {
    return `/orgs/${encodeURIComponent(orgId)}${action}`;
}

function memberPath(orgId: string, userId: string, action = ""): string {
    return organisationPath(orgId, `/members/${encodeURIComponent(userId)}${action}`);
}

export async function createWorkspace(body: CreateWorkspaceRequest): Promise<WorkspaceResponse> {
    return apiPost<WorkspaceResponse, CreateWorkspaceRequest>("/orgs", body);
}

export async function listOrganisations(): Promise<OrganisationListResponse> {
    return apiFetch<OrganisationListResponse>("/orgs");
}

export async function getOrganisation(orgId: string): Promise<OrganisationResponse> {
    return apiFetch<OrganisationResponse>(organisationPath(orgId));
}

export async function updateOrganisation(
    orgId: string,
    body: UpdateOrganisationRequest,
): Promise<OrganisationResponse> {
    return apiPut<OrganisationResponse, UpdateOrganisationRequest>(organisationPath(orgId), body);
}

export async function deleteOrganisation(orgId: string): Promise<void> {
    return apiDelete<void>(organisationPath(orgId));
}

export async function restoreOrganisation(orgId: string): Promise<OrganisationResponse> {
    return apiPost<OrganisationResponse>(organisationPath(orgId, "/restore"));
}

export async function transferOwnership(orgId: string, body: TransferOwnershipRequest): Promise<void> {
    return apiPost<void, TransferOwnershipRequest>(organisationPath(orgId, "/transfer"), body);
}

export async function leaveOrganisation(orgId: string): Promise<void> {
    return apiPost<void>(organisationPath(orgId, "/leave"));
}

export async function listOrganisationMembers(orgId: string): Promise<OrganisationMemberListResponse> {
    return apiFetch<OrganisationMemberListResponse>(organisationPath(orgId, "/members"));
}

export async function changeMemberRole(orgId: string, userId: string, body: ChangeMemberRoleRequest): Promise<void> {
    return apiPut<void, ChangeMemberRoleRequest>(memberPath(orgId, userId, "/role"), body);
}

export async function removeMember(orgId: string, userId: string): Promise<void> {
    return apiDelete<void>(memberPath(orgId, userId));
}

export async function getEntitlements(orgId: string): Promise<EntitlementsResponse> {
    return apiFetch<EntitlementsResponse>(organisationPath(orgId, "/entitlements"));
}
