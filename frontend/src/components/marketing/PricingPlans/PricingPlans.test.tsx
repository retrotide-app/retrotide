import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { PricingPlans } from "./PricingPlans";

function plan(name: string): HTMLElement {
    const item = screen.getByRole("heading", { level: 3, name }).closest("li");

    if (item === null) {
        throw new Error(`no plan card for ${name}`);
    }

    return item;
}

describe("PricingPlans", () => {
    it("lists the four plans from free upwards", () => {
        // given
        const ui = <PricingPlans />;

        // when
        renderWithProviders(ui);

        // then
        const names = screen.getAllByRole("heading", { level: 3 }).map(heading => heading.textContent);
        expect(names).toEqual(["Free", "Team", "Organisation", "Enterprise"]);
    });

    it("shows each paid plan's price excluding VAT with the UK price including VAT beside it", () => {
        // given
        const ui = <PricingPlans />;

        // when
        renderWithProviders(ui);

        // then
        expect(within(plan("Free")).getByText("£0")).toBeInTheDocument();
        expect(within(plan("Team")).getByText("£10")).toBeInTheDocument();
        expect(within(plan("Team")).getByText("£12 inc. VAT")).toBeInTheDocument();
        expect(within(plan("Organisation")).getByText("£149")).toBeInTheDocument();
        expect(within(plan("Organisation")).getByText("£178.80 inc. VAT")).toBeInTheDocument();
        expect(within(plan("Enterprise")).getByText("£3,000")).toBeInTheDocument();
        expect(within(plan("Enterprise")).getByText("£3,600 inc. VAT")).toBeInTheDocument();
    });

    it("shows the monthly prices beside the yearly ones", () => {
        // given
        const ui = <PricingPlans />;

        // when
        renderWithProviders(ui);

        // then
        expect(
            within(plan("Team")).getByText("Or £12 (£14.40 inc. VAT) per team a month, billed monthly"),
        ).toBeInTheDocument();
        expect(
            within(plan("Organisation")).getByText("Or £179 (£214.80 inc. VAT) a month, billed monthly"),
        ).toBeInTheDocument();
    });

    it("compares Organisation openly with the same teams on Team", () => {
        // given
        const ui = <PricingPlans />;

        // when
        renderWithProviders(ui);

        // then
        expect(
            within(plan("Organisation")).getByText(
                "The same 10 teams on Team cost £100 (£120 inc. VAT) a month, billed yearly",
            ),
        ).toBeInTheDocument();
        expect(
            within(plan("Organisation")).getByText("Then £12 (£14.40 inc. VAT) a month for each extra team"),
        ).toBeInTheDocument();
    });

    it("offers only the free sign-up and says the paid plans are coming", () => {
        // given
        const ui = <PricingPlans />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getAllByRole("link")).toHaveLength(1);
        expect(screen.getByRole("link", { name: "Start for free" })).toHaveAttribute("href", "/register");
        expect(screen.getAllByText("Coming soon")).toHaveLength(3);
        expect(screen.getByText(/paid plans are coming/i)).toBeInTheDocument();
    });

    it("says prices exclude VAT for businesses and guests don't count as members", () => {
        // given
        const ui = <PricingPlans />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText(/prices exclude vat for business customers/i)).toBeInTheDocument();
        expect(screen.getByText(/don't count as members/i)).toBeInTheDocument();
    });
});
