import { beforeEach, describe, vi } from "vitest";
import * as api from "./adminStats";
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

describe("site stats", () => {
    const cases: RequestCase[] = [
        {
            name: "getAdminStats reads the site stats",
            call: () => api.getAdminStats(),
            transport: fetchMock,
            request: ["/admin/stats"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
