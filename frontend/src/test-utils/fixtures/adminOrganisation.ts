import type {
    AdminOrganisationDetailResponse,
    AdminOrganisationListResponse,
    AdminOrganisationResponse,
    AdminOrganisationTeamResponse,
} from "../../types/api";

export function makeAdminOrganisation(overrides: Partial<AdminOrganisationResponse> = {}): AdminOrganisationResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000c1",
        name: "Harbour",
        slug: "harbour",
        plan: "free",
        owner_id: "00000000-0000-4000-8000-0000000000aa",
        owner_display_name: "Robin",
        owner_email: "robin@example.test",
        member_count: 4,
        team_count: 2,
        board_count: 7,
        created_at: "2026-09-01T09:00:00Z",
        ...overrides,
    };
}

export function makeAdminOrganisationList(
    organisations: AdminOrganisationResponse[],
    overrides: Partial<AdminOrganisationListResponse> = {},
): AdminOrganisationListResponse {
    return { organisations, total: organisations.length, limit: 20, offset: 0, ...overrides };
}

export function makeAdminOrganisationTeam(
    overrides: Partial<AdminOrganisationTeamResponse> = {},
): AdminOrganisationTeamResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000d1",
        name: "Crew",
        member_count: 3,
        board_count: 5,
        created_at: "2026-09-01T09:00:00Z",
        ...overrides,
    };
}

export function makeAdminOrganisationDetail(
    overrides: Partial<AdminOrganisationDetailResponse> = {},
): AdminOrganisationDetailResponse {
    return {
        ...makeAdminOrganisation(),
        teams: [],
        members: [],
        ...overrides,
    };
}
