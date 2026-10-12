import type {
    CreatedInviteResponse,
    CreateInviteRequest,
    InviteDetailsResponse,
    InviteRegistrationRequest,
    InviteRegistrationResponse,
    InviteTokenRequest,
    JoinedOrganisationResponse,
    OrganisationInviteListResponse,
    OrganisationInviteResponse,
} from "../../types/api";
import { apiDelete, apiFetch, apiPost } from "../client";

function invitesPath(orgId: string, action = ""): string {
    return `/orgs/${encodeURIComponent(orgId)}/invites${action}`;
}

function invitePath(orgId: string, inviteId: string, action = ""): string {
    return invitesPath(orgId, `/${encodeURIComponent(inviteId)}${action}`);
}

export async function listInvites(orgId: string): Promise<OrganisationInviteListResponse> {
    return apiFetch<OrganisationInviteListResponse>(invitesPath(orgId));
}

export async function createInvite(orgId: string, body: CreateInviteRequest): Promise<CreatedInviteResponse> {
    return apiPost<CreatedInviteResponse, CreateInviteRequest>(invitesPath(orgId), body);
}

export async function resendInvite(orgId: string, inviteId: string): Promise<OrganisationInviteResponse> {
    return apiPost<OrganisationInviteResponse>(invitePath(orgId, inviteId, "/resend"));
}

export async function revokeInvite(orgId: string, inviteId: string): Promise<void> {
    return apiDelete<void>(invitePath(orgId, inviteId));
}

export async function lookUpInvite(body: InviteTokenRequest): Promise<InviteDetailsResponse> {
    return apiPost<InviteDetailsResponse, InviteTokenRequest>("/invites/lookup", body);
}

export async function acceptInvite(body: InviteTokenRequest): Promise<JoinedOrganisationResponse> {
    return apiPost<JoinedOrganisationResponse, InviteTokenRequest>("/invites/accept", body);
}

export async function registerThroughInvite(body: InviteRegistrationRequest): Promise<InviteRegistrationResponse> {
    return apiPost<InviteRegistrationResponse, InviteRegistrationRequest>("/invites/register", body);
}
