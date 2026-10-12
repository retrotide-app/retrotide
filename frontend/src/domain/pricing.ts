export type BillingPeriod = "yearly" | "monthly";

const UK_VAT_PERCENT = 20;

const PENCE_PER_POUND = 100;

export const FREE_MEMBER_LIMIT = 10;

export const FREE_DAILY_BOARD_LIMIT = 30;

export const TEAM_MEMBER_LIMIT = 30;

export const TEAM_PRICE: Readonly<Record<BillingPeriod, number>> = { yearly: 1000, monthly: 1200 };

export const ORGANISATION_PRICE: Readonly<Record<BillingPeriod, number>> = { yearly: 14900, monthly: 17900 };

export const ORGANISATION_INCLUDED_TEAMS = 10;

export const EXTRA_TEAM_PRICE = 1200;

export const ENTERPRISE_YEARLY_FROM = 300000;

export function includingVat(pence: number): number {
    return Math.round((pence * (100 + UK_VAT_PERCENT)) / 100);
}

export function formatPrice(pence: number): string {
    const fractionDigits = pence % PENCE_PER_POUND === 0 ? 0 : 2;
    const formatter = new Intl.NumberFormat("en-GB", {
        style: "currency",
        currency: "GBP",
        minimumFractionDigits: fractionDigits,
        maximumFractionDigits: fractionDigits,
    });

    return formatter.format(pence / PENCE_PER_POUND);
}

export function formatPriceWithVat(pence: number): string {
    return `${formatPrice(pence)} (${formatPrice(includingVat(pence))} inc. VAT)`;
}

export function teamPlanCost(teams: number, period: BillingPeriod): number {
    return teams * TEAM_PRICE[period];
}
