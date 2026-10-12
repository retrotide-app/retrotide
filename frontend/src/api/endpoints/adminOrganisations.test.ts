import { beforeEach, describe, vi } from "vitest";
import * as api from "./adminOrganisations";
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

const ORG_ID = "00000000-0000-4000-8000-0000000000cc";

describe("organisation records", () => {
    const cases: RequestCase[] = [
        {
            name: "getAdminOrganisations sends the search and the page",
            call: () => api.getAdminOrganisations({ search: "acme", limit: 20, offset: 40 }),
            transport: fetchMock,
            request: ["/admin/organisations?search=acme&limit=20&offset=40"],
            strict: true,
        },
        {
            name: "getAdminOrganisations leaves out a blank search and the first page's offset",
            call: () => api.getAdminOrganisations({ search: "", limit: 20, offset: 0 }),
            transport: fetchMock,
            request: ["/admin/organisations?limit=20"],
            strict: true,
        },
        {
            name: "getAdminOrganisations escapes the search",
            call: () => api.getAdminOrganisations({ search: "a&b" }),
            transport: fetchMock,
            request: ["/admin/organisations?search=a%26b"],
            strict: true,
        },
        {
            name: "getAdminOrganisation reads one organisation by its id",
            call: () => api.getAdminOrganisation(ORG_ID),
            transport: fetchMock,
            request: [`/admin/organisations/${ORG_ID}`],
            strict: true,
        },
        {
            name: "getAdminOrganisation escapes the id",
            call: () => api.getAdminOrganisation("a/b"),
            transport: fetchMock,
            request: ["/admin/organisations/a%2Fb"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
