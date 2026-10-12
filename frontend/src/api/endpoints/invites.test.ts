import { beforeEach, describe, vi } from "vitest";
import * as api from "./invites";
import { deleteMock, fetchMock, postMock, resetTransports, runRequestCases, type RequestCase } from "./testHarness";

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

describe("managing an organisation's invites", () => {
    const cases: RequestCase[] = [
        {
            name: "listInvites reads the pending invites",
            call: () => api.listInvites("o1"),
            transport: fetchMock,
            request: ["/orgs/o1/invites"],
            strict: true,
        },
        {
            name: "createInvite posts an email invite with its role and team",
            call: () => api.createInvite("o1", { email: "kit@example.test", role: "member", team_id: "t1" }),
            transport: postMock,
            request: ["/orgs/o1/invites", { email: "kit@example.test", role: "member", team_id: "t1" }],
            strict: true,
        },
        {
            name: "createInvite posts a link invite with how many times it works",
            call: () => api.createInvite("o1", { role: "admin", max_uses: 5 }),
            transport: postMock,
            request: ["/orgs/o1/invites", { role: "admin", max_uses: 5 }],
            strict: true,
        },
        {
            name: "resendInvite posts to the invite's resend route with no body",
            call: () => api.resendInvite("o1", "i1"),
            transport: postMock,
            request: ["/orgs/o1/invites/i1/resend"],
            strict: true,
        },
        {
            name: "revokeInvite deletes the invite",
            call: () => api.revokeInvite("o1", "i1"),
            transport: deleteMock,
            request: ["/orgs/o1/invites/i1"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("answering an invite", () => {
    const cases: RequestCase[] = [
        {
            name: "lookUpInvite posts the token from the link, so it never lands in a server log",
            call: () => api.lookUpInvite({ token: "invite-1" }),
            transport: postMock,
            request: ["/invites/lookup", { token: "invite-1" }],
            strict: true,
        },
        {
            name: "acceptInvite posts the token",
            call: () => api.acceptInvite({ token: "invite-1" }),
            transport: postMock,
            request: ["/invites/accept", { token: "invite-1" }],
            strict: true,
        },
        {
            name: "registerThroughInvite posts the token, the address, the name and the password",
            call: () =>
                api.registerThroughInvite({
                    token: "invite-1",
                    email: "kit@example.test",
                    display_name: "Kit",
                    password: "harbour-lights",
                }),
            transport: postMock,
            request: [
                "/invites/register",
                { token: "invite-1", email: "kit@example.test", display_name: "Kit", password: "harbour-lights" },
            ],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
