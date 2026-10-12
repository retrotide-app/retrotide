import type { PageQuery } from "./admin";
import type { OrganisationMemberResponse, PlanCode } from "./organisation";

export interface AdminOrganisationListQuery extends PageQuery {
    search?: string;
}

export interface AdminOrganisationResponse {
    id: string;
    name: string;
    slug: string;
    plan: PlanCode;
    owner_id?: string;
    owner_display_name?: string;
    owner_email?: string;
    member_count: number;
    team_count: number;
    board_count: number;
    created_at: string;
    deleted_at?: string;
}

export interface AdminOrganisationListResponse {
    organisations: AdminOrganisationResponse[];
    total: number;
    limit: number;
    offset: number;
}

export interface AdminOrganisationTeamResponse {
    id: string;
    name: string;
    member_count: number;
    board_count: number;
    created_at: string;
    archived_at?: string;
}

export interface AdminOrganisationDetailResponse extends AdminOrganisationResponse {
    teams: AdminOrganisationTeamResponse[];
    members: OrganisationMemberResponse[];
}
