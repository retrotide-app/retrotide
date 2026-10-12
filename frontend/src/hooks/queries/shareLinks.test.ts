import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useBoardSharing, useJoinPreview } from "./shareLinks";

const endpoints = vi.hoisted(() => ({
    getJoinPreview: vi.fn(),
    getBoardSharing: vi.fn(),
}));

vi.mock("../../api/endpoints/shareLinks", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getJoinPreview.mockReset();
    endpoints.getBoardSharing.mockReset();
});

describe("useJoinPreview", () => {
    it("reads what the link needs under the link's preview key", async () => {
        // given
        const preview = { requires_password: true, requires_account: false, can_join: true };
        endpoints.getJoinPreview.mockResolvedValue(preview);

        // when
        const { result, queryClient } = setup(() => useJoinPreview("k7Q2"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(preview));
        expect(endpoints.getJoinPreview).toHaveBeenCalledWith("k7Q2");
        expect(queryClient.getQueryData(queryKeys.shareLinks.preview("k7Q2"))).toEqual(preview);
    });

    it("hands back the refusal when the link leads nowhere", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "board not found" });
        endpoints.getJoinPreview.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useJoinPreview("gone"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });

    it("asks nothing without a slug", () => {
        // when
        setup(() => useJoinPreview(""));

        // then
        expect(endpoints.getJoinPreview).not.toHaveBeenCalled();
    });
});

describe("useBoardSharing", () => {
    it("reads the board's sharing under its sharing key", async () => {
        // given
        const sharing = { share_slug: "k7Q2", link_access: "guests", has_password: false };
        endpoints.getBoardSharing.mockResolvedValue(sharing);

        // when
        const { result, queryClient } = setup(() => useBoardSharing("b1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(sharing));
        expect(endpoints.getBoardSharing).toHaveBeenCalledWith("b1");
        expect(queryClient.getQueryData(queryKeys.boardSharing("b1"))).toEqual(sharing);
    });

    it("hands back the refusal for a viewer who may not manage the board", async () => {
        // given
        const failure = makeApiError({ status: 403, message: "your role on this board does not allow that" });
        endpoints.getBoardSharing.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useBoardSharing("b1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });
});
