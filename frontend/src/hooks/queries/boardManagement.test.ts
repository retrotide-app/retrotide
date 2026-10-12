import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useBulkExportUrl, useDeletedBoards } from "./boardManagement";

const endpoints = vi.hoisted(() => ({
    listDeletedBoards: vi.fn(),
    bulkExportUrl: vi.fn(),
}));

vi.mock("../../api/endpoints/boardManagement", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.listDeletedBoards.mockReset();
    endpoints.bulkExportUrl.mockReset();
});

describe("useDeletedBoards", () => {
    it("reads the organisation's deleted boards under their own key", async () => {
        // given
        const listing = {
            boards: [
                {
                    id: "b1",
                    title: "Sprint 11",
                    deleted_at: "2026-10-09T09:00:00.000Z",
                    restore_until: "2026-11-08T09:00:00.000Z",
                },
            ],
        };
        endpoints.listDeletedBoards.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useDeletedBoards("o1", true));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listDeletedBoards).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.organisations.deletedBoards("o1"))).toEqual(listing);
    });

    it("asks for nothing until the deleted boards are wanted", () => {
        // when
        const { result } = setup(() => useDeletedBoards("o1", false));

        // then
        expect(result.current.data).toBeUndefined();
        expect(endpoints.listDeletedBoards).not.toHaveBeenCalled();
    });
});

describe("useBulkExportUrl", () => {
    it("hands back the download address for the chosen boards", () => {
        // given
        endpoints.bulkExportUrl.mockReturnValue("/api/v1/orgs/o1/boards/export?board_id=b1");
        const chosen = ["b1"];

        // when
        const { result } = setup(() => useBulkExportUrl("o1", chosen));

        // then
        expect(result.current).toBe("/api/v1/orgs/o1/boards/export?board_id=b1");
        expect(endpoints.bulkExportUrl).toHaveBeenCalledWith("o1", chosen);
    });
});
