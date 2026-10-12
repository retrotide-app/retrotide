import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeSiteInfo } from "../../test-utils/fixtures";
import { getSiteInfo } from "./siteInfo";
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

describe("site info", () => {
    const cases: RequestCase[] = [
        {
            name: "getSiteInfo reads the public site info",
            call: () => getSiteInfo(),
            transport: fetchMock,
            request: ["/site-info"],
            strict: true,
        },
    ];

    runRequestCases(cases);

    it("hands back the site info the server answered with", async () => {
        // given
        const info = makeSiteInfo({ site_name: "Harbour", identity_providers: [{ id: "google", name: "Google" }] });
        fetchMock.mockResolvedValue(info);

        // when
        const result = await getSiteInfo();

        // then
        expect(result).toEqual(info);
    });
});
