import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeIdentity } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useIdentities } from "./identity";

const endpoints = vi.hoisted(() => ({ listIdentities: vi.fn() }));

vi.mock("../../api/endpoints/identity", () => endpoints);

function setup(enabled?: boolean) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(() => useIdentities(enabled), { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.listIdentities.mockResolvedValue({ identities: [] });
});

describe("useIdentities", () => {
    it("reads the account's identities under the identities key", async () => {
        // given
        const listing = { identities: [makeIdentity()] };
        endpoints.listIdentities.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup();

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(queryClient.getQueryData(queryKeys.auth.identities())).toEqual(listing);
    });

    it("asks nothing while it is switched off", () => {
        // given
        const enabled = false;

        // when
        const { result } = setup(enabled);

        // then
        expect(endpoints.listIdentities).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });

    it("hands back the error when the list could not be read", async () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to list the sign-in methods" });
        endpoints.listIdentities.mockRejectedValue(failure);

        // when
        const { result } = setup();

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });
});
