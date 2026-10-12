import type { StaffRole } from "./admin";
import type { TeamResponse } from "./team";

export type OrgRole = "owner" | "admin" | "member";

export type AssignableOrgRole = Exclude<OrgRole, "owner">;

export type PlanCode = "free";

export type PlanLimit = "teams" | "members" | "saved_templates" | "card_image_bytes";

export interface CreateWorkspaceRequest {
    name: string;
    team_name: string;
}

export interface UpdateOrganisationRequest {
    name: string;
    slug: string;
    allow_guests: boolean;
}

export interface OrganisationResponse {
    id: string;
    slug: string;
    name: string;
    allow_guests: boolean;
    role?: OrgRole;
    created_at: string;
    deleted_at?: string;
    restorable_until?: string;
}

export interface OrganisationListResponse {
    organisations: OrganisationResponse[];
}

export interface WorkspaceResponse {
    organisation: OrganisationResponse;
    team: TeamResponse;
}

export interface OrganisationMemberResponse {
    user_id: string;
    email: string;
    display_name: string;
    role: OrgRole;
    site_role?: StaffRole;
    joined_at: string;
}

export interface OrganisationMemberListResponse {
    members: OrganisationMemberResponse[];
}

export interface ChangeMemberRoleRequest {
    role: AssignableOrgRole;
}

export interface TransferOwnershipRequest {
    user_id: string;
}

export interface CreateInviteRequest {
    email?: string;
    role?: AssignableOrgRole;
    team_id?: string;
    max_uses?: number;
}

export interface OrganisationInviteResponse {
    id: string;
    email?: string;
    role: OrgRole;
    team_id?: string;
    max_uses?: number;
    use_count: number;
    expired: boolean;
    expires_at: string;
    created_at: string;
}

export interface OrganisationInviteListResponse {
    invites: OrganisationInviteResponse[];
}

export interface InviteTokenRequest {
    token: string;
}

export interface InviteDetailsResponse {
    org_name: string;
    org_slug: string;
    team_name?: string;
    inviter_name?: string;
    role: OrgRole;
    email?: string;
    expires_at: string;
}

export interface EntitlementsResponse {
    plan: PlanCode;
    limits: Partial<Record<PlanLimit, number>>;
}
