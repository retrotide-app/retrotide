import { beforeEach, describe, vi } from "vitest";
import * as api from "./shareLinks";
import { fetchMock, postMock, putMock, resetTransports, runRequestCases, type RequestCase } from "./testHarness";

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

describe("a board's share link", () => {
    const cases: RequestCase[] = [
        {
            name: "getJoinPreview reads what the link needs before anyone joins",
            call: () => api.getJoinPreview("k7Q2"),
            transport: fetchMock,
            request: ["/b/k7Q2"],
            strict: true,
        },
        {
            name: "getJoinPreview escapes the slug so it cannot add a segment",
            call: () => api.getJoinPreview("a/b"),
            transport: fetchMock,
            request: ["/b/a%2Fb"],
            strict: true,
        },
        {
            name: "joinBoardByLink posts the name and password to the link's join route",
            call: () => api.joinBoardByLink("k7Q2", { display_name: "Robin", password: "tide" }),
            transport: postMock,
            request: ["/b/k7Q2/join", { display_name: "Robin", password: "tide" }],
            strict: true,
        },
        {
            name: "joinBoardByLink sends an empty body when nothing was typed",
            call: () => api.joinBoardByLink("k7Q2", {}),
            transport: postMock,
            request: ["/b/k7Q2/join", {}],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("a board's sharing settings", () => {
    const cases: RequestCase[] = [
        {
            name: "getBoardSharing reads the board's link, access and password state",
            call: () => api.getBoardSharing("b1"),
            transport: fetchMock,
            request: ["/boards/b1/sharing"],
            strict: true,
        },
        {
            name: "setLinkAccess puts who may join with the link",
            call: () => api.setLinkAccess("b1", { op_id: "op1", link_access: "accounts" }),
            transport: putMock,
            request: ["/boards/b1/sharing/link-access", { op_id: "op1", link_access: "accounts" }],
            strict: true,
        },
        {
            name: "setBoardPassword puts the new password",
            call: () => api.setBoardPassword("b1", { op_id: "op1", password: "tide" }),
            transport: putMock,
            request: ["/boards/b1/sharing/password", { op_id: "op1", password: "tide" }],
            strict: true,
        },
        {
            name: "setBoardPassword puts null to clear the password",
            call: () => api.setBoardPassword("b1", { op_id: "op1", password: null }),
            transport: putMock,
            request: ["/boards/b1/sharing/password", { op_id: "op1", password: null }],
            strict: true,
        },
        {
            name: "rotateShareLink posts to draw a new link",
            call: () => api.rotateShareLink("b1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/sharing/rotate", { op_id: "op1" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
