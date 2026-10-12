import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeSiteInfo } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useSiteInfoQuery } from "./siteInfo";

const endpoints = vi.hoisted(() => ({ getSiteInfo: vi.fn() }));

vi.mock("../../api/endpoints/siteInfo", () => endpoints);

function setup() {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(useSiteInfoQuery, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getSiteInfo.mockResolvedValue(makeSiteInfo());
});

describe("useSiteInfoQuery", () => {
    it("reads the site info under the site info key", async () => {
        // given
        const info = makeSiteInfo({ site_name: "Harbour", site_mode: "private", registration_open: false });
        endpoints.getSiteInfo.mockResolvedValue(info);

        // when
        const { result, queryClient } = setup();

        // then
        await waitFor(() => expect(result.current.data).toEqual(info));
        expect(queryClient.getQueryData(queryKeys.siteInfo())).toEqual(info);
    });

    it("reports loading until the site info arrives", async () => {
        // given
        const { result } = setup();

        // when
        const first = result.current;

        // then
        expect(first.loading).toBe(true);
        expect(first.data).toBeUndefined();
        await waitFor(() => expect(result.current.loading).toBe(false));
    });

    it("hands back the server's error rather than a made up site", async () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to read the site info" });
        endpoints.getSiteInfo.mockRejectedValue(failure);

        // when
        const { result } = setup();

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
        expect(result.current.loading).toBe(false);
    });

    it("asks again when refetched", async () => {
        // given
        const { result } = setup();
        await waitFor(() => expect(result.current.loading).toBe(false));

        // when
        await result.current.refetch();

        // then
        expect(endpoints.getSiteInfo).toHaveBeenCalledTimes(2);
    });
});
