import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { MarketingLayout } from "./MarketingLayout";

describe("MarketingLayout", () => {
    it("draws its children inside the main landmark", () => {
        // given
        const ui = (
            <MarketingLayout>
                <h1>Page body</h1>
            </MarketingLayout>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: "Page body" }));
        expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
        expect(screen.getByRole("main")).toHaveAttribute("tabindex", "-1");
    });

    it("links the brand back to the root", () => {
        // given
        const ui = <MarketingLayout />;

        // when
        renderWithProviders(ui);

        // then
        expect(within(screen.getByRole("banner")).getByRole("link", { name: "RetroTide" })).toHaveAttribute(
            "href",
            "/",
        );
    });

    it("links the templates, comparison and self-hosting pages from the header, but not pricing while it is hidden", () => {
        // given
        const ui = <MarketingLayout />;

        // when
        renderWithProviders(ui);

        // then
        const site = within(screen.getByRole("navigation", { name: "Site" }));
        expect(site.getAllByRole("link").map(link => link.textContent)).toEqual(["Templates", "Compare", "Self-host"]);
        expect(site.getByRole("link", { name: "Templates" })).toHaveAttribute("href", "/templates");
        expect(site.getByRole("link", { name: "Compare" })).toHaveAttribute("href", "/compare/easyretro");
        expect(site.getByRole("link", { name: "Self-host" })).toHaveAttribute("href", "/self-host");
    });

    it("offers sign in and sign up from the header", () => {
        // given
        const ui = <MarketingLayout />;

        // when
        renderWithProviders(ui);

        // then
        const account = within(screen.getByRole("navigation", { name: "Account" }));
        expect(account.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
        expect(account.getByRole("link", { name: "Start for free" })).toHaveAttribute("href", "/register");
    });

    it("links every legal page from the footer", () => {
        // given
        const ui = <MarketingLayout />;

        // when
        renderWithProviders(ui);

        // then
        const footer = within(screen.getByRole("navigation", { name: "Footer" }));
        expect(footer.getByRole("link", { name: "Privacy" })).toHaveAttribute("href", "/legal/privacy");
        expect(footer.getByRole("link", { name: "Terms" })).toHaveAttribute("href", "/legal/terms");
        expect(footer.getByRole("link", { name: "Acceptable use" })).toHaveAttribute("href", "/legal/acceptable-use");
        expect(footer.getByRole("link", { name: "Cookies" })).toHaveAttribute("href", "/legal/cookies");
        expect(footer.getByRole("link", { name: "Sub-processors" })).toHaveAttribute("href", "/legal/sub-processors");
        expect(footer.getByRole("link", { name: "Security" })).toHaveAttribute("href", "/legal/security");
    });

    it("links the comparison and self-hosting pages from the footer", () => {
        // given
        const ui = <MarketingLayout />;

        // when
        renderWithProviders(ui);

        // then
        const footer = within(screen.getByRole("navigation", { name: "Footer" }));
        expect(footer.getByRole("link", { name: "Compare with EasyRetro" })).toHaveAttribute(
            "href",
            "/compare/easyretro",
        );
        expect(footer.getByRole("link", { name: "Self-host" })).toHaveAttribute("href", "/self-host");
    });

    it("lets the visitor pick a theme from the footer", () => {
        // given
        const ui = <MarketingLayout />;

        // when
        renderWithProviders(ui);

        // then
        expect(within(screen.getByRole("contentinfo")).getByLabelText("Theme")).toBeInTheDocument();
    });

    it("puts the skip link ahead of everything else a keyboard reaches", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<MarketingLayout />);

        // when
        await user.tab();

        // then
        expect(screen.getByRole("link", { name: "Skip to content" })).toHaveFocus();
        expect(screen.getByRole("link", { name: "Skip to content" })).toHaveAttribute("href", "#main-content");
    });
});
