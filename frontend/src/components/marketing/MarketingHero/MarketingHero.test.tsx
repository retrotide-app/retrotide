import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { MarketingHero } from "./MarketingHero";

describe("MarketingHero", () => {
    it("names its region after the page heading and shows the lede", () => {
        // given
        const ui = <MarketingHero title="Pricing" lede="Free for one team." eyebrow="Plans" />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("region", { name: "Pricing" })).toContainElement(
            screen.getByRole("heading", { level: 1, name: "Pricing" }),
        );
        expect(screen.getByText("Free for one team.")).toBeInTheDocument();
        expect(screen.getByText("Plans")).toBeInTheDocument();
    });

    it("draws its calls to action under the lede", () => {
        // given
        const ui = (
            <MarketingHero title="Templates" lede="Pick one.">
                <a href="/register">Start for free</a>
            </MarketingHero>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("region", { name: "Templates" })).toContainElement(
            screen.getByRole("link", { name: "Start for free" }),
        );
    });
});
