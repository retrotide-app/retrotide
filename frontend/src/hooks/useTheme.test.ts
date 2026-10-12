import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { providerWrapper } from "../test-utils/render";
import { useTheme } from "./useTheme";

describe("useTheme", () => {
    it("returns the theme preference the provider holds", () => {
        // given
        const wrapper = providerWrapper({ theme: { preference: "dark" } });

        // when
        const { result } = renderHook(useTheme, { wrapper });

        // then
        expect(result.current.preference).toBe("dark");
    });

    it("defaults to following the system theme", () => {
        // given
        const wrapper = providerWrapper();

        // when
        const { result } = renderHook(useTheme, { wrapper });

        // then
        expect(result.current.preference).toBe("system");
    });

    it("passes setter calls straight through to the provider", () => {
        // given
        const setPreference = vi.fn();
        const { result } = renderHook(useTheme, { wrapper: providerWrapper({ theme: { setPreference } }) });

        // when
        result.current.setPreference("light");

        // then
        expect(setPreference).toHaveBeenCalledWith("light");
    });

    it("throws when it is used outside a ThemeProvider", () => {
        // given
        const consoleError = vi.spyOn(console, "error").mockReturnValue(undefined);

        // when
        const attempt = () => renderHook(useTheme);

        // then
        expect(attempt).toThrow("useTheme must be used within a ThemeProvider");
        consoleError.mockRestore();
    });
});
