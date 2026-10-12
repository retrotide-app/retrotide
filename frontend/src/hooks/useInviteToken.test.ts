import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { PENDING_INVITE_STORAGE_KEY } from "../platform/pendingInvite";
import { useHasPendingInvite, useInviteToken } from "./useInviteToken";

afterEach(() => {
    window.history.replaceState(null, "", "/");
});

describe("useInviteToken", () => {
    it("reads the token from the link's fragment and takes it out of the address bar", () => {
        // given
        window.history.replaceState(null, "", "/invite#invite-1");

        // when
        const { result } = renderHook(useInviteToken);

        // then
        expect(result.current.token).toBe("invite-1");
        expect(window.location.hash).toBe("");
        expect(window.location.pathname).toBe("/invite");
    });

    it("keeps the link's token for the tab, so it survives signing in or a provider's round trip", () => {
        // given
        window.history.replaceState(null, "", "/invite#invite-1");

        // when
        renderHook(useInviteToken);

        // then
        expect(sessionStorage.getItem(PENDING_INVITE_STORAGE_KEY)).toBe("invite-1");
    });

    it("comes back to the kept token when the page opens again without a fragment", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");
        window.history.replaceState(null, "", "/invite");

        // when
        const { result } = renderHook(useInviteToken);

        // then
        expect(result.current.token).toBe("invite-1");
    });

    it("prefers a new link's token to the one kept from an earlier link", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-old");
        window.history.replaceState(null, "", "/invite#invite-new");

        // when
        const { result } = renderHook(useInviteToken);

        // then
        expect(result.current.token).toBe("invite-new");
        expect(sessionStorage.getItem(PENDING_INVITE_STORAGE_KEY)).toBe("invite-new");
    });

    it("finds no token when there is neither a fragment nor a kept token", () => {
        // given
        window.history.replaceState(null, "", "/invite");

        // when
        const { result } = renderHook(useInviteToken);

        // then
        expect(result.current.token).toBe("");
    });

    it("forgets the kept token once the invite is dealt with", () => {
        // given
        window.history.replaceState(null, "", "/invite#invite-1");
        const { result } = renderHook(useInviteToken);

        // when
        result.current.forget();

        // then
        expect(sessionStorage.getItem(PENDING_INVITE_STORAGE_KEY)).toBeNull();
    });
});

describe("useHasPendingInvite", () => {
    it("says an invite is pending while its token is kept", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");

        // when
        const { result } = renderHook(useHasPendingInvite);

        // then
        expect(result.current).toBe(true);
    });

    it("says none is pending without a kept token, whatever the fragment holds", () => {
        // given
        window.history.replaceState(null, "", "/#main-content");

        // when
        const { result } = renderHook(useHasPendingInvite);

        // then
        expect(result.current).toBe(false);
    });
});
