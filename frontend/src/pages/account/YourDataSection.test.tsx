import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { YourDataSection } from "./YourDataSection";

describe("YourDataSection", () => {
    it("offers the signed-in user a download of their own data", () => {
        // when
        renderWithProviders(<YourDataSection />);

        // then
        const section = within(screen.getByRole("region", { name: "Your data" }));
        const link = section.getByRole("link", { name: "Download your data" });
        expect(link).toHaveAttribute("href", "/api/v1/auth/export");
        expect(link).toHaveAttribute("download");
    });
});
