import { beforeEach, describe, vi } from "vitest";
import * as api from "./cardPolish";
import { deleteWithBodyMock, putMock, resetTransports, runRequestCases, type RequestCase } from "./testHarness";

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

describe("card reactions and pins", () => {
    const cases: RequestCase[] = [
        {
            name: "addReaction puts the reaction on the card with the op id",
            call: () => api.addReaction("b/1", "c/1", "heart", { op_id: "op1" }),
            transport: putMock,
            request: ["/boards/b%2F1/cards/c%2F1/reactions/heart", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "removeReaction deletes the reaction from the card with the op id",
            call: () => api.removeReaction("b1", "c1", "thumbs_up", { op_id: "op2" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/cards/c1/reactions/thumbs_up", { op_id: "op2" }],
            strict: true,
        },
        {
            name: "pinCard puts whether the card is pinned",
            call: () => api.pinCard("b1", "c/1", { op_id: "op3", pinned: true }),
            transport: putMock,
            request: ["/boards/b1/cards/c%2F1/pin", { op_id: "op3", pinned: true }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
