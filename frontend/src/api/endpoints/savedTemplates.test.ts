import { beforeEach, describe, vi } from "vitest";
import * as api from "./savedTemplates";
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

describe("saved templates", () => {
    const cases: RequestCase[] = [
        {
            name: "listSavedTemplates reads the organisation's saved templates",
            call: () => api.listSavedTemplates("o1"),
            transport: fetchMock,
            request: ["/orgs/o1/saved-templates"],
            strict: true,
        },
        {
            name: "saveBoardAsTemplate posts the name, description and scope for the board",
            call: () => api.saveBoardAsTemplate("b1", { name: "Our retro", description: "", scope: "team" }),
            transport: postMock,
            request: ["/boards/b1/saved-templates", { name: "Our retro", description: "", scope: "team" }],
            strict: true,
        },
        {
            name: "deleteSavedTemplate deletes the template under its organisation",
            call: () => api.deleteSavedTemplate("o1", "s1"),
            transport: deleteMock,
            request: ["/orgs/o1/saved-templates/s1"],
            strict: true,
        },
        {
            name: "every path segment is encoded",
            call: () => api.deleteSavedTemplate("o/1", "s 1"),
            transport: deleteMock,
            request: ["/orgs/o%2F1/saved-templates/s%201"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
