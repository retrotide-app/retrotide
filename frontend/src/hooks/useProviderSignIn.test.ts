import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeIdentityProvider, makeMutationState } from "../test-utils/fixtures";
import { providerWrapper } from "../test-utils/render";
import { useProviderSignIn } from "./useProviderSignIn";

const { useStartProviderSignIn, leaveForExternalPage } = vi.hoisted(() => ({
    useStartProviderSignIn: vi.fn(),
    leaveForExternalPage: vi.fn(),
}));

vi.mock("./mutations/identity", () => ({ useStartProviderSignIn }));
vi.mock("../platform/location", () => ({ leaveForExternalPage }));

const PROVIDER_URL = "https://accounts.example.test/authorise?state=s1";

const google = makeIdentityProvider();
const example = makeIdentityProvider({ id: "example", name: "Example ID" });

function answeringWith(url: string) {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: (data: { url: string }) => void }) => {
            options?.onSuccess?.({ url });
        }),
    });
}

beforeEach(() => {
    leaveForExternalPage.mockReset();
    useStartProviderSignIn.mockReturnValue(answeringWith(PROVIDER_URL));
});

describe("useProviderSignIn", () => {
    it("starts the flow with the provider and the request it is given", () => {
        // given
        const mutation = answeringWith(PROVIDER_URL);
        useStartProviderSignIn.mockReturnValue(mutation);
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // when
        act(() => {
            result.current.start(google, { purpose: "sign_in", return_to: "/account" });
        });

        // then
        expect(mutation.mutate).toHaveBeenCalledWith(
            { provider: google, request: { purpose: "sign_in", return_to: "/account" } },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
    });

    it("starts each flow at the provider chosen for it when the site offers two", () => {
        // given
        const mutation = answeringWith(PROVIDER_URL);
        useStartProviderSignIn.mockReturnValue(mutation);
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // when
        act(() => {
            result.current.start(example, { purpose: "sign_in" });
        });
        act(() => {
            result.current.start(google, { purpose: "sign_in" });
        });

        // then
        expect(mutation.mutate).toHaveBeenNthCalledWith(
            1,
            { provider: example, request: { purpose: "sign_in" } },
            expect.anything(),
        );
        expect(mutation.mutate).toHaveBeenNthCalledWith(
            2,
            { provider: google, request: { purpose: "sign_in" } },
            expect.anything(),
        );
    });

    it("sends the whole page to the provider once the flow has started", () => {
        // given
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // when
        act(() => {
            result.current.start(google, { purpose: "link", current_password: "harbour-lights" });
        });

        // then
        expect(leaveForExternalPage).toHaveBeenCalledWith(PROVIDER_URL);
    });

    it("stays pending while the page is leaving, so the button cannot be pressed twice", () => {
        // given
        useStartProviderSignIn.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // then
        expect(result.current.pending).toBe(true);
    });

    it("is pending while the server is starting the flow", () => {
        // given
        useStartProviderSignIn.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // then
        expect(result.current.pending).toBe(true);
    });

    it("names no provider before a flow has started", () => {
        // given
        useStartProviderSignIn.mockReturnValue(makeMutationState());

        // when
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // then
        expect(result.current.provider).toBeNull();
    });

    it("hands back the server's refusal with the provider it was for, and stays on the page", () => {
        // given
        const failure = makeApiError({ status: 403, message: "wrong password", code: "incorrect_password" });
        useStartProviderSignIn.mockReturnValue(
            makeMutationState({
                isError: true,
                error: failure,
                variables: { provider: example, request: { purpose: "link" } },
            }),
        );

        // when
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // then
        expect(result.current.error).toBe(failure);
        expect(result.current.provider).toEqual(example);
        expect(result.current.pending).toBe(false);
        expect(leaveForExternalPage).not.toHaveBeenCalled();
    });

    it("hands back the browser's refusal to leave for the address and frees the button", () => {
        // given
        const refusal = new Error("refusing to leave for a javascript: address");
        leaveForExternalPage.mockImplementation(() => {
            throw refusal;
        });
        useStartProviderSignIn.mockReturnValue({
            ...answeringWith("javascript:alert(1)"),
            isSuccess: true,
            variables: { provider: google, request: { purpose: "sign_in" } },
        });
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // when
        act(() => {
            result.current.start(google, { purpose: "sign_in" });
        });

        // then
        expect(leaveForExternalPage).toHaveBeenCalledWith("javascript:alert(1)");
        expect(result.current.error).toBe(refusal);
        expect(result.current.provider).toEqual(google);
        expect(result.current.pending).toBe(false);
    });

    it("forgets the browser's refusal through reset", () => {
        // given
        leaveForExternalPage.mockImplementation(() => {
            throw new Error("refusing to leave for a javascript: address");
        });
        const mutation = answeringWith("javascript:alert(1)");
        useStartProviderSignIn.mockReturnValue(mutation);
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });
        act(() => {
            result.current.start(google, { purpose: "sign_in" });
        });

        // when
        act(() => {
            result.current.reset();
        });

        // then
        expect(result.current.error).toBeNull();
        expect(mutation.reset).toHaveBeenCalledOnce();
    });

    it("clears the refusal through reset", () => {
        // given
        const mutation = makeMutationState();
        useStartProviderSignIn.mockReturnValue(mutation);
        const { result } = renderHook(useProviderSignIn, { wrapper: providerWrapper() });

        // when
        act(() => {
            result.current.reset();
        });

        // then
        expect(mutation.reset).toHaveBeenCalledOnce();
    });
});
