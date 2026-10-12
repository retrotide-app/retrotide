import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeOrganisation } from "../test-utils/fixtures";
import { providerWrapper } from "../test-utils/render";
import { useCurrentOrg } from "./useCurrentOrg";

describe("useCurrentOrg", () => {
    it("reads the current organisation and the user's role in it from the provider", () => {
        // given
        const harbour = makeOrganisation({ role: "admin" });
        const wrapper = providerWrapper({ org: { organisations: [harbour] } });

        // when
        const { result } = renderHook(useCurrentOrg, { wrapper });

        // then
        expect(result.current.current).toBe(harbour);
        expect(result.current.role).toBe("admin");
    });

    it("throws when it is used outside an OrgProvider", () => {
        // given
        const consoleError = vi.spyOn(console, "error").mockReturnValue(undefined);

        // when
        const attempt = () => renderHook(useCurrentOrg);

        // then
        expect(attempt).toThrow("useCurrentOrg must be used within an OrgProvider");
        consoleError.mockRestore();
    });
});
