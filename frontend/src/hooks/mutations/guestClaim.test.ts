import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useClaimGuestBoards, useDeclineGuestClaim } from "./guestClaim";

const endpoints = vi.hoisted(() => ({
    claimGuestBoards: vi.fn(),
    declineGuestClaim: vi.fn(),
    getGuestClaimOffer: vi.fn(),
}));

vi.mock("../../api/endpoints/guestClaim", () => endpoints);

const OFFER_KEY = queryKeys.auth.guestClaim("u1");

const offer = { boards: [{ board_id: "b1", title: "Sprint 12", display_name: "Robin" }] };

function client() {
    const qc = createTestQueryClient();
    const defaults = qc.getDefaultOptions();
    qc.setDefaultOptions({ ...defaults, queries: { ...defaults.queries, gcTime: Infinity } });
    qc.setQueryData(OFFER_KEY, offer);

    return qc;
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockReset();
    }
    endpoints.getGuestClaimOffer.mockResolvedValue({ boards: [] });
});

describe("useClaimGuestBoards", () => {
    it("claims the chosen boards, ends the offer and rereads the boards the cards moved on", async () => {
        // given
        const qc = client();
        vi.spyOn(qc, "invalidateQueries");
        endpoints.claimGuestBoards.mockResolvedValue({ claimed: ["b1"] });
        const { result } = renderHook(useClaimGuestBoards, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let answer: unknown;
        await act(async () => {
            answer = await result.current.mutateAsync(["b1"]);
        });

        // then
        expect(endpoints.claimGuestBoards).toHaveBeenCalledWith({ board_ids: ["b1"] });
        expect(answer).toEqual({ claimed: ["b1"] });
        expect(qc.getQueryData(OFFER_KEY)).toEqual({ boards: [] });
        expectInvalidated(qc, queryKeys.boardLive.all);
        expectInvalidated(qc, queryKeys.boards.all);
    });

    it("rereads the offer when the browser no longer holds a guest identity", async () => {
        // given
        const qc = client();
        vi.spyOn(qc, "invalidateQueries");
        endpoints.claimGuestBoards.mockRejectedValue(
            makeApiError({ status: 409, message: "this browser holds no guest identity to claim", code: "no_guest" }),
        );
        const { result } = renderHook(useClaimGuestBoards, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync(["b1"]).catch(() => undefined);
        });

        // then
        expectInvalidated(qc, queryKeys.auth.guestClaims);
    });
});

describe("useDeclineGuestClaim", () => {
    it("declines and ends the offer", async () => {
        // given
        const qc = client();
        endpoints.declineGuestClaim.mockResolvedValue(undefined);
        const { result } = renderHook(useDeclineGuestClaim, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync();
        });

        // then
        expect(endpoints.declineGuestClaim).toHaveBeenCalled();
        expect(qc.getQueryData(OFFER_KEY)).toEqual({ boards: [] });
    });

    it("keeps the offer when declining failed", async () => {
        // given
        const qc = client();
        endpoints.declineGuestClaim.mockRejectedValue(makeApiError({ status: 503, message: "service unavailable" }));
        const { result } = renderHook(useDeclineGuestClaim, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync().catch(() => undefined);
        });

        // then
        expect(qc.getQueryData(OFFER_KEY)).toEqual(offer);
    });
});
