import type { OrganisationInviteResponse, OrgRole } from "./organisation";

export interface CreatedInviteResponse extends OrganisationInviteResponse {
    url?: string;
}

export interface InviteRegistrationRequest {
    token: string;
    email: string;
    display_name: string;
    password: string;
}

export interface JoinedOrganisationResponse {
    org_id: string;
    org_slug: string;
    org_name: string;
    role: OrgRole;
    team_id?: string;
}

export interface InviteRegistrationResponse {
    email: string;
    verification_required: boolean;
    joined: JoinedOrganisationResponse;
}
