import { beforeEach, describe, vi } from "vitest";
import * as api from "./teams";
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

describe("an organisation's teams", () => {
    const cases: RequestCase[] = [
        {
            name: "listTeams reads the organisation's teams",
            call: () => api.listTeams("o1"),
            transport: fetchMock,
            request: ["/orgs/o1/teams"],
            strict: true,
        },
        {
            name: "createTeam posts the team's name",
            call: () => api.createTeam("o1", { name: "Crew" }),
            transport: postMock,
            request: ["/orgs/o1/teams", { name: "Crew" }],
            strict: true,
        },
        {
            name: "archiveTeam posts to the team's archive route with no body",
            call: () => api.archiveTeam("o1", "t1"),
            transport: postMock,
            request: ["/orgs/o1/teams/t1/archive"],
            strict: true,
        },
        {
            name: "deleteTeam deletes the team under its organisation",
            call: () => api.deleteTeam("o1", "t1"),
            transport: deleteMock,
            request: ["/orgs/o1/teams/t1"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("one team", () => {
    const cases: RequestCase[] = [
        {
            name: "getTeam reads the team",
            call: () => api.getTeam("t1"),
            transport: fetchMock,
            request: ["/teams/t1"],
            strict: true,
        },
        {
            name: "renameTeam puts the new name",
            call: () => api.renameTeam("t1", { name: "Deck" }),
            transport: putMock,
            request: ["/teams/t1", { name: "Deck" }],
            strict: true,
        },
        {
            name: "listTeamMembers reads the team's members",
            call: () => api.listTeamMembers("t1"),
            transport: fetchMock,
            request: ["/teams/t1/members"],
            strict: true,
        },
        {
            name: "addTeamMember posts the member's user id and team role",
            call: () => api.addTeamMember("t1", { user_id: "u2", role: "member" }),
            transport: postMock,
            request: ["/teams/t1/members", { user_id: "u2", role: "member" }],
            strict: true,
        },
        {
            name: "changeTeamMemberRole puts the member's new team role",
            call: () => api.changeTeamMemberRole("t1", "u2", { role: "lead" }),
            transport: putMock,
            request: ["/teams/t1/members/u2/role", { role: "lead" }],
            strict: true,
        },
        {
            name: "removeTeamMember deletes the team place",
            call: () => api.removeTeamMember("t1", "u2"),
            transport: deleteMock,
            request: ["/teams/t1/members/u2"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
