import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { PricingPage } from "./PricingPage";

beforeEach(() => {
    document.title = "";
});

describe("PricingPage", () => {
    it("heads the page and titles the document", () => {
        // given
        const ui = <PricingPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Simple pricing, per team" })).toBeInTheDocument();
        expect(document.title).toContain("Pricing");
    });

    it("shows every paid price excluding VAT with the UK price including VAT beside it", () => {
        // given
        const ui = <PricingPage />;

        // when
        renderWithProviders(ui);

        // then
        const plans = within(screen.getByRole("region", { name: "Plans" }));
        expect(plans.getByText("£10")).toBeInTheDocument();
        expect(plans.getByText("£12 inc. VAT")).toBeInTheDocument();
        expect(plans.getByText("£149")).toBeInTheDocument();
        expect(plans.getByText("£178.80 inc. VAT")).toBeInTheDocument();
        expect(plans.getByText("£3,000")).toBeInTheDocument();
        expect(plans.getByText("£3,600 inc. VAT")).toBeInTheDocument();
    });

    it("compares Organisation openly with the same ten teams on Team, yearly and monthly", () => {
        // given
        const ui = <PricingPage />;

        // when
        renderWithProviders(ui);

        // then
        const table = within(screen.getByRole("table"));
        const team = within(table.getByRole("row", { name: /^Team, 10 teams/ }));
        const organisation = within(table.getByRole("row", { name: /^Organisation, up to 10 teams/ }));
        expect(team.getByText("£100 (£120 inc. VAT)")).toBeInTheDocument();
        expect(team.getByText("£120 (£144 inc. VAT)")).toBeInTheDocument();
        expect(organisation.getByText("£149 (£178.80 inc. VAT)")).toBeInTheDocument();
        expect(organisation.getByText("£179 (£214.80 inc. VAT)")).toBeInTheDocument();
    });

    it("says paid plans are coming and offers no way to buy one", () => {
        // given
        const ui = <PricingPage />;

        // when
        renderWithProviders(ui);

        // then
        const main = within(screen.getByRole("main"));
        const destinations = main.getAllByRole("link").map(link => link.getAttribute("href"));
        expect(new Set(destinations)).toEqual(new Set(["/register"]));
        expect(main.queryByRole("link", { name: /choose|buy|upgrade|contact/i })).not.toBeInTheDocument();
        expect(main.queryByRole("button")).not.toBeInTheDocument();
        expect(main.getAllByText(/paid plans are coming/i).length).toBeGreaterThan(0);
    });
});
