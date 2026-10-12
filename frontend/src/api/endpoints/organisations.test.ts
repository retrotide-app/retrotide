import { beforeEach, describe, vi } from "vitest";
import * as api from "./organisations";
import {
    deleteMock,
    fetchMock,
    postMock,
    putMock,
    resetTransports,
    runRequestCases,
    type RequestCase,
} from "./testHarness";

vi.mock("../client", async importOriginal => {
    const actual = await importOriginal<typeof import("../client")>();

    return {
        ...actual,
        apiFetch: vi.fn(),
        apiPost: vi.fn(),
        apiPut: vi.fn(),
        apiPatch: vi.fn(),
        apiDelete: vi.fn(),
        apiDeleteWithBody: vi.fn(),
    };
});

beforeEach(resetTransports);

describe("organisations", () => {
    const cases: RequestCase[] = [
        {
            name: "createWorkspace posts the organisation name and the first team's name",
            call: () => api.createWorkspace({ name: "Harbour", team_name: "Crew" }),
            transport: postMock,
            request: ["/orgs", { name: "Harbour", team_name: "Crew" }],
            strict: true,
        },
        {
            name: "listOrganisations reads the caller's organisations",
            call: () => api.listOrganisations(),
            transport: fetchMock,
            request: ["/orgs"],
            strict: true,
        },
        {
            name: "getOrganisation reads one organisation",
            call: () => api.getOrganisation("o1"),
            transport: fetchMock,
            request: ["/orgs/o1"],
            strict: true,
        },
        {
            name: "updateOrganisation puts the name, the address and the guest policy together",
            call: () => api.updateOrganisation("o1", { name: "Harbour", slug: "harbour", allow_guests: false }),
            transport: putMock,
            request: ["/orgs/o1", { name: "Harbour", slug: "harbour", allow_guests: false }],
            strict: true,
        },
        {
            name: "deleteOrganisation deletes the organisation",
            call: () => api.deleteOrganisation("o1"),
            transport: deleteMock,
            request: ["/orgs/o1"],
            strict: true,
        },
        {
            name: "restoreOrganisation posts to the restore route with no body",
            call: () => api.restoreOrganisation("o1"),
            transport: postMock,
            request: ["/orgs/o1/restore"],
            strict: true,
        },
        {
            name: "transferOwnership posts the new owner's user id",
            call: () => api.transferOwnership("o1", { user_id: "u2" }),
            transport: postMock,
            request: ["/orgs/o1/transfer", { user_id: "u2" }],
            strict: true,
        },
        {
            name: "leaveOrganisation posts to the leave route with no body",
            call: () => api.leaveOrganisation("o1"),
            transport: postMock,
            request: ["/orgs/o1/leave"],
            strict: true,
        },
        {
            name: "getEntitlements reads the plan and its limits",
            call: () => api.getEntitlements("o1"),
            transport: fetchMock,
            request: ["/orgs/o1/entitlements"],
            strict: true,
        },
        {
            name: "escapes an organisation id so it stays one path segment",
            call: () => api.getOrganisation("o1/../admin"),
            transport: fetchMock,
            request: ["/orgs/o1%2F..%2Fadmin"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("organisation members", () => {
    const cases: RequestCase[] = [
        {
            name: "listOrganisationMembers reads the members",
            call: () => api.listOrganisationMembers("o1"),
            transport: fetchMock,
            request: ["/orgs/o1/members"],
            strict: true,
        },
        {
            name: "changeMemberRole puts the member's new role",
            call: () => api.changeMemberRole("o1", "u2", { role: "admin" }),
            transport: putMock,
            request: ["/orgs/o1/members/u2/role", { role: "admin" }],
            strict: true,
        },
        {
            name: "removeMember deletes the membership",
            call: () => api.removeMember("o1", "u2"),
            transport: deleteMock,
            request: ["/orgs/o1/members/u2"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
