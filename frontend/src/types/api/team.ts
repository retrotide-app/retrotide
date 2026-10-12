import type { StaffRole } from "./admin";
import type { OrgRole } from "./organisation";

export type TeamRole = "lead" | "member";

export interface CreateTeamRequest {
    name: string;
}

export interface RenameTeamRequest {
    name: string;
}

export interface TeamResponse {
    id: string;
    org_id: string;
    name: string;
    role?: TeamRole;
    created_at: string;
    archived_at?: string;
}

export interface TeamListResponse {
    teams: TeamResponse[];
}

export interface AddTeamMemberRequest {
    user_id: string;
    role: TeamRole;
}

export interface ChangeTeamMemberRoleRequest {
    role: TeamRole;
}

export interface TeamMemberResponse {
    user_id: string;
    email: string;
    display_name: string;
    role: TeamRole;
    org_role: OrgRole;
    site_role?: StaffRole;
    joined_at: string;
}

export interface TeamMemberListResponse {
    members: TeamMemberResponse[];
}
