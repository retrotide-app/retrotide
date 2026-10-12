import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import type { BoardSharingResponse } from "../../types/api";
import { useJoinBoardByLink, useRotateShareLink, useSetBoardPassword, useSetLinkAccess } from "./shareLinks";

const endpoints = vi.hoisted(() => ({
    getBoardSharing: vi.fn(),
    getJoinPreview: vi.fn(),
    joinBoardByLink: vi.fn(),
    rotateShareLink: vi.fn(),
    setBoardPassword: vi.fn(),
    setLinkAccess: vi.fn(),
}));

vi.mock("../../api/endpoints/shareLinks", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const anyId = expect.stringMatching(V4);

const BOARD = "b1";

const SHARING_KEY = queryKeys.boardSharing(BOARD);

const sharing: BoardSharingResponse = { share_slug: "k7Q2", link_access: "guests", has_password: false };

function client() {
    const qc = createTestQueryClient();
    const defaults = qc.getDefaultOptions();
    qc.setDefaultOptions({ ...defaults, queries: { ...defaults.queries, gcTime: Infinity } });

    return { qc, invalidate: vi.spyOn(qc, "invalidateQueries") };
}

function render<T>(hook: () => T, qc = client().qc) {
    return renderHook(hook, { wrapper: providerWrapper({ queryClient: qc }) });
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockReset();
    }
});

describe("useJoinBoardByLink", () => {
    it("joins through the link and rereads the session, so a new guest's socket opens", async () => {
        // given
        const { qc, invalidate } = client();
        endpoints.joinBoardByLink.mockResolvedValue({ board_id: BOARD });
        const { result } = render(() => useJoinBoardByLink("k7Q2"), qc);

        // when
        let joined: unknown;
        await act(async () => {
            joined = await result.current.mutateAsync({ display_name: "Robin" });
        });

        // then
        expect(endpoints.joinBoardByLink).toHaveBeenCalledWith("k7Q2", { display_name: "Robin" });
        expect(joined).toEqual({ board_id: BOARD });
        expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.auth.session() });
        expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.boardLive.state(BOARD) });
    });

    it("rereads the link's preview when the join is refused, because the board's sharing changed", async () => {
        // given
        const { qc, invalidate } = client();
        endpoints.joinBoardByLink.mockRejectedValue(
            makeApiError({ status: 403, message: "the board needs you to sign in", code: "account_required" }),
        );
        const { result } = render(() => useJoinBoardByLink("k7Q2"), qc);

        // when
        await act(async () => {
            await result.current.mutateAsync({}).catch(() => undefined);
        });

        // then
        expectInvalidated(qc, queryKeys.shareLinks.preview("k7Q2"));
        expect(invalidate).not.toHaveBeenCalledWith({ queryKey: queryKeys.auth.session() });
    });
});

describe("sharing commands", () => {
    it("sets the link access with a fresh op ID and keeps the answer's sharing", async () => {
        // given
        const { qc } = client();
        const changed = { ...sharing, link_access: "accounts" };
        endpoints.setLinkAccess.mockResolvedValue({ version: 4, sharing: changed });
        const { result } = render(() => useSetLinkAccess(BOARD), qc);

        // when
        await act(async () => {
            await result.current.mutateAsync("accounts");
        });

        // then
        expect(endpoints.setLinkAccess).toHaveBeenCalledWith(BOARD, { op_id: anyId, link_access: "accounts" });
        expect(qc.getQueryData(SHARING_KEY)).toEqual(changed);
    });

    it("sets a password", async () => {
        // given
        const { qc } = client();
        endpoints.setBoardPassword.mockResolvedValue({ version: 4, sharing: { ...sharing, has_password: true } });
        const { result } = render(() => useSetBoardPassword(BOARD), qc);

        // when
        await act(async () => {
            await result.current.mutateAsync("tide");
        });

        // then
        expect(endpoints.setBoardPassword).toHaveBeenCalledWith(BOARD, { op_id: anyId, password: "tide" });
        expect(qc.getQueryData<BoardSharingResponse>(SHARING_KEY)?.has_password).toBe(true);
    });

    it("clears the password with null", async () => {
        // given
        endpoints.setBoardPassword.mockResolvedValue({ version: 5, sharing });
        const { result } = render(() => useSetBoardPassword(BOARD));

        // when
        await act(async () => {
            await result.current.mutateAsync(null);
        });

        // then
        expect(endpoints.setBoardPassword).toHaveBeenCalledWith(BOARD, { op_id: anyId, password: null });
    });

    it("rotates the link and keeps the new slug", async () => {
        // given
        const { qc } = client();
        endpoints.rotateShareLink.mockResolvedValue({ version: 6, sharing: { ...sharing, share_slug: "n3W9" } });
        const { result } = render(() => useRotateShareLink(BOARD), qc);

        // when
        await act(async () => {
            await result.current.mutateAsync(undefined);
        });

        // then
        expect(endpoints.rotateShareLink).toHaveBeenCalledWith(BOARD, { op_id: anyId });
        expect(qc.getQueryData<BoardSharingResponse>(SHARING_KEY)?.share_slug).toBe("n3W9");
    });

    it("rereads the sharing after a replayed command, whose answer carries none", async () => {
        // given
        const { qc } = client();
        qc.setQueryData(SHARING_KEY, sharing);
        endpoints.setLinkAccess.mockResolvedValue({ version: 4, replayed: true });
        endpoints.getBoardSharing.mockResolvedValue(sharing);
        const { result } = render(() => useSetLinkAccess(BOARD), qc);

        // when
        await act(async () => {
            await result.current.mutateAsync("off");
        });

        // then
        expectInvalidated(qc, SHARING_KEY);
    });

    it("draws a new op ID for every attempt", async () => {
        // given
        endpoints.rotateShareLink.mockResolvedValue({ version: 6, sharing });
        const { result } = render(() => useRotateShareLink(BOARD));

        // when
        await act(async () => {
            await result.current.mutateAsync(undefined);
            await result.current.mutateAsync(undefined);
        });

        // then
        const opIds = endpoints.rotateShareLink.mock.calls.map(call => (call[1] as { op_id: string }).op_id);
        expect(new Set(opIds).size).toBe(2);
    });

    it.each([403, 404, 409, 500])("rereads the sharing after a %i, because it may have changed", async status => {
        // given
        const { qc } = client();
        endpoints.setLinkAccess.mockRejectedValue(makeApiError({ status, message: "refused" }));
        const { result } = render(() => useSetLinkAccess(BOARD), qc);

        // when
        await act(async () => {
            await result.current.mutateAsync("off").catch(() => undefined);
        });

        // then
        expectInvalidated(qc, SHARING_KEY);
    });

    it("leaves the sharing alone after a 400, which changed nothing", async () => {
        // given
        const { qc, invalidate } = client();
        endpoints.setBoardPassword.mockRejectedValue(
            makeApiError({ status: 400, message: "the password is blank", code: "board_password_blank" }),
        );
        const { result } = render(() => useSetBoardPassword(BOARD), qc);

        // when
        await act(async () => {
            await result.current.mutateAsync(" ").catch(() => undefined);
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});
