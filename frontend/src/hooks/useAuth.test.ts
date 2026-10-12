import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeApiError, makeSessionUser } from "../test-utils/fixtures";
import { providerWrapper } from "../test-utils/render";
import { useAuth } from "./useAuth";

describe("useAuth", () => {
    it("returns the signed in user supplied by the provider", () => {
        // given
        const user = makeSessionUser({ display_name: "Robin" });

        // when
        const { result } = renderHook(useAuth, { wrapper: providerWrapper({ user }) });

        // then
        expect(result.current.user).toEqual(user);
        expect(result.current.loading).toBe(false);
        expect(result.current.error).toBeNull();
    });

    it("returns the staff permissions the session granted", () => {
        // given
        const wrapper = providerWrapper({ user: makeSessionUser(), permissions: ["view_admin_panel"] });

        // when
        const { result } = renderHook(useAuth, { wrapper });

        // then
        expect(result.current.permissions).toEqual(["view_admin_panel"]);
    });

    it("returns a null user and no permissions when nobody is signed in", () => {
        // given
        const wrapper = providerWrapper({ user: null });

        // when
        const { result } = renderHook(useAuth, { wrapper });

        // then
        expect(result.current.user).toBeNull();
        expect(result.current.permissions).toEqual([]);
    });

    it("surfaces the loading flag while the session is being resolved", () => {
        // given
        const wrapper = providerWrapper({ auth: { loading: true } });

        // when
        const { result } = renderHook(useAuth, { wrapper });

        // then
        expect(result.current.loading).toBe(true);
    });

    it("surfaces the error when the session could not be checked", () => {
        // given
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        const wrapper = providerWrapper({ auth: { error: failure } });

        // when
        const { result } = renderHook(useAuth, { wrapper });

        // then
        expect(result.current.error).toBe(failure);
    });

    it("throws when it is used outside an AuthProvider", () => {
        // given
        const consoleError = vi.spyOn(console, "error").mockReturnValue(undefined);

        // when
        const attempt = () => renderHook(useAuth);

        // then
        expect(attempt).toThrow("useAuth must be used within an AuthProvider");
        consoleError.mockRestore();
    });
});
