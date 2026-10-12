import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "./boardManagement";
import {
    deleteMock,
    deleteWithBodyMock,
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

describe("a board's management commands", () => {
    const cases: RequestCase[] = [
        {
            name: "archiveBoard posts the op id to the board's archive route",
            call: () => api.archiveBoard("b1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/archive", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "unarchiveBoard posts the op id to the board's unarchive route",
            call: () => api.unarchiveBoard("b1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/unarchive", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "deleteBoard deletes the board with the op id in the body",
            call: () => api.deleteBoard("b1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "moveBoard puts the target team",
            call: () => api.moveBoard("b1", { op_id: "op1", team_id: "t2" }),
            transport: putMock,
            request: ["/boards/b1/team", { op_id: "op1", team_id: "t2" }],
            strict: true,
        },
        {
            name: "moveBoard puts a null team for the organisation",
            call: () => api.moveBoard("b1", { op_id: "op1", team_id: null }),
            transport: putMock,
            request: ["/boards/b1/team", { op_id: "op1", team_id: null }],
            strict: true,
        },
        {
            name: "importCsv posts the CSV text",
            call: () => api.importCsv("b1", { op_id: "op1", csv: "Went well\nShipped\n" }),
            transport: postMock,
            request: ["/boards/b1/import", { op_id: "op1", csv: "Went well\nShipped\n" }],
            strict: true,
        },
        {
            name: "pinBoard puts the pin with no body",
            call: () => api.pinBoard("b1"),
            transport: putMock,
            request: ["/boards/b1/pin", undefined],
            strict: true,
        },
        {
            name: "unpinBoard deletes the pin",
            call: () => api.unpinBoard("b1"),
            transport: deleteMock,
            request: ["/boards/b1/pin"],
            strict: true,
        },
        {
            name: "a board ID is escaped so it cannot add a segment",
            call: () => api.archiveBoard("b/1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b%2F1/archive", { op_id: "op1" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("an organisation's board management", () => {
    const cases: RequestCase[] = [
        {
            name: "listDeletedBoards reads the boards waiting to be restored",
            call: () => api.listDeletedBoards("o1"),
            transport: fetchMock,
            request: ["/orgs/o1/boards/deleted"],
            strict: true,
        },
        {
            name: "restoreBoard posts to the board's restore route",
            call: () => api.restoreBoard("o1", "b1"),
            transport: postMock,
            request: ["/orgs/o1/boards/b1/restore"],
            strict: true,
        },
        {
            name: "bulkBoards posts the action and the boards",
            call: () => api.bulkBoards("o1", { action: "archive", board_ids: ["b1", "b2"] }),
            transport: postMock,
            request: ["/orgs/o1/boards/bulk", { action: "archive", board_ids: ["b1", "b2"] }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("bulkExportUrl", () => {
    it("names every chosen board in its own board_id", () => {
        // when
        const url = api.bulkExportUrl("o1", ["b1", "b2"]);

        // then
        expect(url).toBe("/api/v1/orgs/o1/boards/export?board_id=b1&board_id=b2");
    });

    it("escapes the organisation ID and the board IDs", () => {
        // when
        const url = api.bulkExportUrl("o/1", ["b&1"]);

        // then
        expect(url).toBe("/api/v1/orgs/o%2F1/boards/export?board_id=b%261");
    });
});
