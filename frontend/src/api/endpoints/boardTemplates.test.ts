import { beforeEach, describe, vi } from "vitest";
import * as api from "./boardTemplates";
import { fetchMock, resetTransports, runRequestCases, type RequestCase } from "./testHarness";

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

describe("the board templates", () => {
    const cases: RequestCase[] = [
        {
            name: "listBoardTemplates reads the template catalogue",
            call: () => api.listBoardTemplates(),
            transport: fetchMock,
            request: ["/templates"],
            strict: true,
        },
        {
            name: "getBoardTemplate reads one template by its escaped key",
            call: () => api.getBoardTemplate("a/b"),
            transport: fetchMock,
            request: ["/templates/a%2Fb"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
