import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeIdentity, makeIdentityProvider } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useStartProviderSignIn, useUnlinkIdentity } from "./identity";

const endpoints = vi.hoisted(() => ({
    listIdentities: vi.fn(),
    startProviderSignIn: vi.fn(),
    unlinkIdentity: vi.fn(),
}));

vi.mock("../../api/endpoints/identity", () => endpoints);

const PROVIDER_URL = "https://accounts.example.test/authorise?x=1";

function client() {
    const qc = createTestQueryClient();

    return { qc, invalidate: vi.spyOn(qc, "invalidateQueries") };
}

beforeEach(() => {
    endpoints.startProviderSignIn.mockResolvedValue({ url: PROVIDER_URL });
    endpoints.unlinkIdentity.mockResolvedValue(undefined);
});

describe("useStartProviderSignIn", () => {
    it("starts the flow and hands back the provider's address without touching the cache", async () => {
        // given
        const { qc, invalidate } = client();
        const { result } = renderHook(useStartProviderSignIn, { wrapper: providerWrapper({ queryClient: qc }) });
        const request = { purpose: "sign_in" as const, return_to: "/account" };

        // when
        let started: unknown;
        await act(async () => {
            started = await result.current.mutateAsync({ provider: makeIdentityProvider(), request });
        });

        // then
        expect(endpoints.startProviderSignIn).toHaveBeenCalledWith("google", request);
        expect(started).toEqual({ url: PROVIDER_URL });
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("starts the flow at the provider it is given", async () => {
        // given
        const { qc } = client();
        const { result } = renderHook(useStartProviderSignIn, { wrapper: providerWrapper({ queryClient: qc }) });
        const provider = makeIdentityProvider({ id: "example", name: "Example ID" });

        // when
        await act(async () => {
            await result.current.mutateAsync({ provider, request: { purpose: "sign_in" } });
        });

        // then
        expect(endpoints.startProviderSignIn).toHaveBeenCalledWith("example", { purpose: "sign_in" });
    });
});

describe("useUnlinkIdentity", () => {
    it("removes the provider's identity and reloads the list", async () => {
        // given
        const { qc } = client();
        const { result } = renderHook(useUnlinkIdentity, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync(makeIdentity());
        });

        // then
        expect(endpoints.unlinkIdentity).toHaveBeenCalledWith("google");
        expectInvalidated(qc, queryKeys.auth.identities());
    });

    it("keeps the identity it removed, so the page can name it afterwards", async () => {
        // given
        const { qc } = client();
        const identity = makeIdentity({ provider: "example", provider_name: "Example ID" });
        const { result } = renderHook(useUnlinkIdentity, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync(identity);
        });

        // then
        expect(endpoints.unlinkIdentity).toHaveBeenCalledWith("example");
        await waitFor(() => {
            expect(result.current.variables).toEqual(identity);
        });
    });

    it("keeps the list when the server refuses to remove the last way to sign in", async () => {
        // given
        const { qc, invalidate } = client();
        endpoints.unlinkIdentity.mockRejectedValue(new Error("that is the account's last way to sign in"));
        const { result } = renderHook(useUnlinkIdentity, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync(makeIdentity())).rejects.toThrow("last way to sign in");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});
