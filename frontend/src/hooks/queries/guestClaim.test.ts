import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useGuestClaimOffer } from "./guestClaim";

const endpoints = vi.hoisted(() => ({
    getGuestClaimOffer: vi.fn(),
}));

vi.mock("../../api/endpoints/guestClaim", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getGuestClaimOffer.mockReset();
});

describe("useGuestClaimOffer", () => {
    it("reads the offer for the signed in user under that user's key", async () => {
        // given
        const offer = { boards: [{ board_id: "b1", title: "Sprint 12", display_name: "Robin" }] };
        endpoints.getGuestClaimOffer.mockResolvedValue(offer);

        // when
        const { result, queryClient } = setup(() => useGuestClaimOffer("u1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(offer));
        expect(queryClient.getQueryData(queryKeys.auth.guestClaim("u1"))).toEqual(offer);
    });

    it("asks nothing while nobody is signed in", () => {
        // when
        setup(() => useGuestClaimOffer(null));

        // then
        expect(endpoints.getGuestClaimOffer).not.toHaveBeenCalled();
    });

    it("hands back the error when the offer could not be read", async () => {
        // given
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        endpoints.getGuestClaimOffer.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useGuestClaimOffer("u1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });
});
