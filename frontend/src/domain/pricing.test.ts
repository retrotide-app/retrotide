import { describe, expect, it } from "vitest";
import {
    ENTERPRISE_YEARLY_FROM,
    EXTRA_TEAM_PRICE,
    formatPrice,
    formatPriceWithVat,
    includingVat,
    ORGANISATION_PRICE,
    TEAM_PRICE,
    teamPlanCost,
} from "./pricing";

describe("includingVat", () => {
    it("adds UK VAT at 20% to every published price", () => {
        // when
        const prices = [
            includingVat(TEAM_PRICE.yearly),
            includingVat(TEAM_PRICE.monthly),
            includingVat(ORGANISATION_PRICE.yearly),
            includingVat(ORGANISATION_PRICE.monthly),
            includingVat(EXTRA_TEAM_PRICE),
            includingVat(ENTERPRISE_YEARLY_FROM),
        ];

        // then
        expect(prices).toEqual([1200, 1440, 17880, 21480, 1440, 360000]);
    });
});

describe("formatPrice", () => {
    it("drops the pence on whole pounds and keeps them otherwise", () => {
        // when
        const labels = [formatPrice(0), formatPrice(1000), formatPrice(1440), formatPrice(17880), formatPrice(300000)];

        // then
        expect(labels).toEqual(["£0", "£10", "£14.40", "£178.80", "£3,000"]);
    });
});

describe("formatPriceWithVat", () => {
    it("puts the UK price including VAT beside the price excluding it", () => {
        // when
        const label = formatPriceWithVat(ORGANISATION_PRICE.yearly);

        // then
        expect(label).toBe("£149 (£178.80 inc. VAT)");
    });
});

describe("teamPlanCost", () => {
    it("charges the Team price once per team", () => {
        // when
        const costs = [teamPlanCost(10, "yearly"), teamPlanCost(10, "monthly")];

        // then
        expect(costs).toEqual([10000, 12000]);
    });
});
