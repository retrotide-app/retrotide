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

    it("sends the hero's call to action to sign up, with no prices while pricing is hidden", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        const hero = within(screen.getByRole("region", { name: "Better sprint retrospectives for your team" }));
        expect(hero.getByRole("link", { name: "Start for free" })).toHaveAttribute("href", "/register");
        expect(hero.queryByRole("link", { name: "See pricing" })).not.toBeInTheDocument();
    });

    it("gives every section an anchor to land on, and shows no pricing section while pricing is hidden", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("region", { name: "How it works" })).toHaveAttribute("id", "features");
        expect(screen.getByRole("region", { name: "Host it yourself" })).toHaveAttribute("id", "self-hosting");
        expect(screen.queryByRole("region", { name: "Pricing" })).not.toBeInTheDocument();
    });

    it("links the self-hosting section to the install page", () => {
        // given
        const ui = <LandingPage />;

        // when
        renderWithProviders(ui);

        // then
        const selfHosting = within(screen.getByRole("region", { name: "Host it yourself" }));
        expect(selfHosting.getByRole("link", { name: "How to self-host RetroTide" })).toHaveAttribute(
            "href",
            "/self-host",
        );
        expect(selfHosting.getByText("Now")).toBeInTheDocument();
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
});
