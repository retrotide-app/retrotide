import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { LandingPage } from "./LandingPage";

beforeEach(() => {
    document.title = "";
});

describe("LandingPage", () => {
    it("leads with what the product is", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(
            screen.getByRole("heading", { level: 1, name: "Better sprint retrospectives for your team" }),
        ).toBeInTheDocument();
    });

    it("titles the document with the site name alone", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(document.title).toBe("RetroTide");
    });

    it("shows the example board in the hero", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("figure", { name: "Example retro board" })).toBeInTheDocument();
    });

    it("sends the hero's calls to action to sign up and to the prices", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        const hero = within(screen.getByRole("region", { name: "Better sprint retrospectives for your team" }));
        expect(hero.getByRole("link", { name: "Start for free" })).toHaveAttribute("href", "/register");
        expect(hero.getByRole("link", { name: "See pricing" })).toHaveAttribute("href", "#pricing");
    });

    it("gives every section the header links to an anchor to land on", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("region", { name: "How it works" })).toHaveAttribute("id", "features");
        expect(screen.getByRole("region", { name: "Pricing" })).toHaveAttribute("id", "pricing");
        expect(screen.getByRole("region", { name: "Host it yourself" })).toHaveAttribute("id", "self-hosting");
    });

    it("walks through a retro in order", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        const features = screen.getByRole("region", { name: "How it works" });
        const steps = within(features)
            .getAllByRole("heading", { level: 3 })
            .map(heading => heading.textContent);
        expect(steps).toEqual(["Create a board", "Add cards", "Vote", "Track actions", "Also on the free plan"]);
    });

    it("lists the plans in the pricing section", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        const pricing = screen.getByRole("region", { name: "Pricing" });
        expect(within(pricing).getByRole("heading", { name: "Free" })).toBeInTheDocument();
        expect(within(pricing).getByRole("heading", { name: "Enterprise" })).toBeInTheDocument();
    });

    it("links the pricing section to the full pricing page", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        const pricing = screen.getByRole("region", { name: "Pricing" });
        expect(within(pricing).getByRole("link", { name: "See the full pricing" })).toHaveAttribute("href", "/pricing");
    });
});
