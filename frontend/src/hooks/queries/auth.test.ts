import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeSession, makeSessionUser } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useInvitation, useSession } from "./auth";

const endpoints = vi.hoisted(() => ({ getSession: vi.fn(), getInvitation: vi.fn() }));

vi.mock("../../api/endpoints/auth", () => endpoints);

function setup() {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(useSession, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getSession.mockResolvedValue({ authenticated: false });
    endpoints.getInvitation.mockResolvedValue({ email: "robin@example.test", expires_at: "2026-10-03T12:00:00Z" });
});

describe("useSession", () => {
    it("reads the session under the session key", async () => {
        // given
        const session = makeSession({ user: makeSessionUser(), permissions: ["view_admin_panel"] });
        endpoints.getSession.mockResolvedValue(session);

        // when
        const { result, queryClient } = setup();

        // then
        await waitFor(() => expect(result.current.data).toEqual(session));
        expect(queryClient.getQueryData(queryKeys.auth.session())).toEqual(session);
    });

    it("hands back a signed out session as it came", async () => {
        // given
        endpoints.getSession.mockResolvedValue({ authenticated: false });

        // when
        const { result } = setup();

        // then
        await waitFor(() => expect(result.current.data).toEqual({ authenticated: false }));
        expect(result.current.error).toBeNull();
    });

    it("hands back the error when the session could not be checked", async () => {
        // given
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        endpoints.getSession.mockRejectedValue(failure);

        // when
        const { result } = setup();

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});

describe("useInvitation", () => {
    function setupInvitation(token: string) {
        const queryClient = createTestQueryClient();
        const rendered = renderHook(() => useInvitation(token), { wrapper: providerWrapper({ queryClient }) });

        return { ...rendered, queryClient };
    }

    it("reads the invitation with the token from the link, under that token's key", async () => {
        // given
        const invitation = { email: "robin@example.test", expires_at: "2026-10-03T12:00:00Z" };
        endpoints.getInvitation.mockResolvedValue(invitation);

        // when
        const { result, queryClient } = setupInvitation("invite-1");

        // then
        await waitFor(() => expect(result.current.data).toEqual(invitation));
        expect(endpoints.getInvitation).toHaveBeenCalledExactlyOnceWith({ token: "invite-1" });
        expect(queryClient.getQueryData(queryKeys.auth.invitation("invite-1"))).toEqual(invitation);
    });

    it("asks nothing without a token", () => {
        // when
        const { result } = setupInvitation("");

        // then
        expect(endpoints.getInvitation).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });

    it("hands back the server's refusal of a link that no longer works", async () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "the invitation link is invalid, used or has expired",
            code: "invitation_invalid",
        });
        endpoints.getInvitation.mockRejectedValue(failure);

        // when
        const { result } = setupInvitation("invite-1");

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});
