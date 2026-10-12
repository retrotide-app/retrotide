import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeApiError } from "../test-utils/fixtures";
import { providerWrapper } from "../test-utils/render";
import { useSiteInfo, useSiteInfoState } from "./useSiteInfo";

describe("useSiteInfo", () => {
    it("returns the site info the provider holds", () => {
        // given
        const providers = [
            { id: "google", name: "Google" },
            { id: "example", name: "Example ID" },
        ];
        const wrapper = providerWrapper({ siteInfo: { site_name: "Harbour", identity_providers: providers } });

        // when
        const { result } = renderHook(useSiteInfo, { wrapper });

        // then
        expect(result.current.site_name).toBe("Harbour");
        expect(result.current.identity_providers).toEqual(providers);
    });

    it("keeps the default site name when the caller did not override it", () => {
        // given
        const wrapper = providerWrapper();

        // when
        const { result } = renderHook(useSiteInfo, { wrapper });

        // then
        expect(result.current.site_name).toBe("RetroTide");
    });

    it("throws when it is called before the site info has loaded", () => {
        // given
        const consoleError = vi.spyOn(console, "error").mockReturnValue(undefined);
        const wrapper = providerWrapper({ siteInfoContext: { siteInfo: null, loading: true } });

        // when
        const attempt = () => renderHook(useSiteInfo, { wrapper });

        // then
        expect(attempt).toThrow("useSiteInfo was called before the site info loaded");
        consoleError.mockRestore();
    });

    it("throws when it is used outside a SiteInfoProvider", () => {
        // given
        const consoleError = vi.spyOn(console, "error").mockReturnValue(undefined);

        // when
        const attempt = () => renderHook(useSiteInfo);

        // then
        expect(attempt).toThrow("useSiteInfo must be used within a SiteInfoProvider");
        consoleError.mockRestore();
    });
});

describe("useSiteInfoState", () => {
    it("reports loading with no site info while the site info is on its way", () => {
        // given
        const wrapper = providerWrapper({ siteInfoContext: { siteInfo: null, loading: true } });

        // when
        const { result } = renderHook(useSiteInfoState, { wrapper });

        // then
        expect(result.current.siteInfo).toBeNull();
        expect(result.current.loading).toBe(true);
    });

    it("reports the error when the site info could not be read", () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to read the site info" });
        const wrapper = providerWrapper({ siteInfoContext: { siteInfo: null, error: failure } });

        // when
        const { result } = renderHook(useSiteInfoState, { wrapper });

        // then
        expect(result.current.error).toBe(failure);
    });

    it("throws when it is used outside a SiteInfoProvider", () => {
        // given
        const consoleError = vi.spyOn(console, "error").mockReturnValue(undefined);

        // when
        const attempt = () => renderHook(useSiteInfoState);

        // then
        expect(attempt).toThrow("useSiteInfo must be used within a SiteInfoProvider");
        consoleError.mockRestore();
    });
});
