import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeAdminStats, makeApiError } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useAdminStats } from "./adminStats";

const endpoints = vi.hoisted(() => ({ getAdminStats: vi.fn() }));

vi.mock("../../api/endpoints/adminStats", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getAdminStats.mockReset();
    endpoints.getAdminStats.mockResolvedValue(makeAdminStats());
});

describe("useAdminStats", () => {
    it("reads the site stats under the stats key", async () => {
        // given
        const stats = makeAdminStats();
        endpoints.getAdminStats.mockResolvedValue(stats);

        // when
        const { result, queryClient } = setup(() => useAdminStats());

        // then
        await waitFor(() => expect(result.current.data).toEqual(stats));
        expect(queryClient.getQueryData(queryKeys.admin.stats())).toEqual(stats);
    });

    it("asks nothing when it is switched off", () => {
        // when
        const { result } = setup(() => useAdminStats(false));

        // then
        expect(endpoints.getAdminStats).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
        expect(result.current.data).toBeUndefined();
    });

    it("hands back the error when the stats could not load", async () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to load the site stats" });
        endpoints.getAdminStats.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useAdminStats());

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});
