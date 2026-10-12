import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { clearSessionLost, markSessionLost } from "../api/sessionLost";
import { makeSessionUser } from "../test-utils/fixtures";
import { providerWrapper } from "../test-utils/render";
import { useSessionLost } from "./useSessionLost";

beforeEach(() => {
    clearSessionLost();
});

describe("useSessionLost", () => {
    it("stays quiet while the signed in session is intact", () => {
        // given
        const wrapper = providerWrapper({ user: makeSessionUser() });

        // when
        const { result } = renderHook(useSessionLost, { wrapper });

        // then
        expect(result.current).toBe(false);
    });

    it("reports a signed in user whose session the server has rejected", () => {
        // given
        const wrapper = providerWrapper({ user: makeSessionUser() });
        const { result } = renderHook(useSessionLost, { wrapper });

        // when
        act(() => {
            markSessionLost();
        });

        // then
        expect(result.current).toBe(true);
    });

    it("stays quiet for a signed out visitor even when the session is marked lost", () => {
        // given
        markSessionLost();
        const wrapper = providerWrapper({ user: null });

        // when
        const { result } = renderHook(useSessionLost, { wrapper });

        // then
        expect(result.current).toBe(false);
    });

    it("goes quiet again once the lost session is cleared", () => {
        // given
        markSessionLost();
        const { result } = renderHook(useSessionLost, { wrapper: providerWrapper({ user: makeSessionUser() }) });

        // when
        act(() => {
            clearSessionLost();
        });

        // then
        expect(result.current).toBe(false);
    });
});
