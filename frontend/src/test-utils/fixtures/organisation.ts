import type {
    CreatedInviteResponse,
    EntitlementsResponse,
    InviteDetailsResponse,
    InviteRegistrationResponse,
    JoinedOrganisationResponse,
    OrganisationInviteResponse,
    OrganisationMemberResponse,
    OrganisationResponse,
    TeamMemberResponse,
    TeamResponse,
    UpgradeRequiredResponse,
    WorkspaceResponse,
} from "../../types/api";

export function makeOrganisation(overrides: Partial<OrganisationResponse> = {}): OrganisationResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000c1",
        slug: "harbour",
        name: "Harbour",
        allow_guests: true,
        role: "owner",
        created_at: "2026-09-01T09:00:00Z",
        ...overrides,
    };
}

export function makeTeam(overrides: Partial<TeamResponse> = {}): TeamResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000d1",
        org_id: "00000000-0000-4000-8000-0000000000c1",
        name: "Crew",
        role: "lead",
        created_at: "2026-09-01T09:00:00Z",
        ...overrides,
    };
}

export function makeWorkspace(overrides: Partial<WorkspaceResponse> = {}): WorkspaceResponse {
    return {
        organisation: makeOrganisation(),
        team: makeTeam(),
        ...overrides,
    };
}

export function makeOrganisationMember(
    overrides: Partial<OrganisationMemberResponse> = {},
): OrganisationMemberResponse {
    return {
        user_id: "00000000-0000-4000-8000-0000000000aa",
        email: "robin@example.test",
        display_name: "Robin",
        role: "member",
        joined_at: "2026-09-02T10:00:00Z",
        ...overrides,
    };
}

export function makeTeamMember(overrides: Partial<TeamMemberResponse> = {}): TeamMemberResponse {
    return {
        user_id: "00000000-0000-4000-8000-0000000000aa",
        email: "robin@example.test",
        display_name: "Robin",
        role: "member",
        org_role: "member",
        joined_at: "2026-09-02T10:00:00Z",
        ...overrides,
    };
}

export function makeOrganisationInvite(
    overrides: Partial<OrganisationInviteResponse> = {},
): OrganisationInviteResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000e1",
        email: "kit@example.test",
        role: "member",
        use_count: 0,
        expired: false,
        expires_at: "2026-10-03T12:00:00Z",
        created_at: "2026-09-26T12:00:00Z",
        ...overrides,
    };
}

export function makeCreatedInvite(overrides: Partial<CreatedInviteResponse> = {}): CreatedInviteResponse {
    return {
        ...makeOrganisationInvite(),
        ...overrides,
    };
}

export function makeInviteDetails(overrides: Partial<InviteDetailsResponse> = {}): InviteDetailsResponse {
    return {
        org_name: "Harbour",
        org_slug: "harbour",
        inviter_name: "Sam",
        role: "member",
        expires_at: "2026-10-03T12:00:00Z",
        ...overrides,
    };
}

export function makeJoinedOrganisation(
    overrides: Partial<JoinedOrganisationResponse> = {},
): JoinedOrganisationResponse {
    return {
        org_id: "00000000-0000-4000-8000-0000000000c1",
        org_slug: "harbour",
        org_name: "Harbour",
        role: "member",
        ...overrides,
    };
}

export function makeInviteRegistration(
    overrides: Partial<InviteRegistrationResponse> = {},
): InviteRegistrationResponse {
    return {
        email: "kit@example.test",
        verification_required: false,
        joined: makeJoinedOrganisation(),
        ...overrides,
    };
}

export function makeEntitlements(overrides: Partial<EntitlementsResponse> = {}): EntitlementsResponse {
    return {
        plan: "free",
        limits: { teams: 1, members: 10 },
        ...overrides,
    };
}

export function makeUpgradeRequiredError(overrides: Partial<UpgradeRequiredResponse> = {}): Error {
    const body: UpgradeRequiredResponse = {
        error: "upgrade required",
        code: "upgrade_required",
        limit: "teams",
        allowed: 1,
        current: 1,
        plan: "free",
        ...overrides,
    };

    return Object.assign(new Error(body.error), { name: "ApiError", status: 402, code: body.code, body });
}
