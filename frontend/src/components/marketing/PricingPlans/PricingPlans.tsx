import {
    ENTERPRISE_YEARLY_FROM,
    EXTRA_TEAM_PRICE,
    FREE_DAILY_BOARD_LIMIT,
    FREE_MEMBER_LIMIT,
    formatPrice,
    formatPriceWithVat,
    includingVat,
    ORGANISATION_INCLUDED_TEAMS,
    ORGANISATION_PRICE,
    TEAM_MEMBER_LIMIT,
    TEAM_PRICE,
    teamPlanCost,
} from "../../../domain/pricing";
import { ButtonLink } from "../../Button/ButtonLink";
import styles from "./PricingPlans.module.css";

interface Plan {
    id: string;
    name: string;
    price: number;
    priceTerms: string;
    showVat: boolean;
    details: string[];
    audience: string;
    includes: string[];
    free: boolean;
}

const PLANS: Plan[] = [
    {
        id: "free",
        name: "Free",
        price: 0,
        priceTerms: "No credit card needed",
        showVat: false,
        details: [],
        audience: `1 team, up to ${FREE_MEMBER_LIMIT} organisation members`,
        includes: [
            `Unlimited boards, with a fair-use limit of ${FREE_DAILY_BOARD_LIMIT} new boards a day`,
            "Unlimited guests",
            "Hidden cards, voting and a timer",
            "Unfinished actions carried to your next retro",
            "Export to CSV, Markdown and JSON",
        ],
        free: true,
    },
    {
        id: "team",
        name: "Team",
        price: TEAM_PRICE.yearly,
        priceTerms: "Per team a month, billed yearly",
        showVat: true,
        details: [`Or ${formatPriceWithVat(TEAM_PRICE.monthly)} per team a month, billed monthly`],
        audience: `Up to ${TEAM_MEMBER_LIMIT} members per team, as many teams as you need`,
        includes: [
            "Everything in Free",
            "Post to Slack and Microsoft Teams",
            "Team insights and trends",
            "Higher limits",
        ],
        free: false,
    },
    {
        id: "organisation",
        name: "Organisation",
        price: ORGANISATION_PRICE.yearly,
        priceTerms: `A month for up to ${ORGANISATION_INCLUDED_TEAMS} teams, billed yearly`,
        showVat: true,
        details: [
            `Or ${formatPriceWithVat(ORGANISATION_PRICE.monthly)} a month, billed monthly`,
            `Then ${formatPriceWithVat(EXTRA_TEAM_PRICE)} a month for each extra team`,
            `The same ${ORGANISATION_INCLUDED_TEAMS} teams on Team cost ${formatPriceWithVat(teamPlanCost(ORGANISATION_INCLUDED_TEAMS, "yearly"))} a month, billed yearly`,
        ],
        audience: "For organisations that need SSO, an audit log and policies",
        includes: ["Everything in Team", "Single sign-on (SSO)", "Audit log", "Policies", "Insights across all teams"],
        free: false,
    },
    {
        id: "enterprise",
        name: "Enterprise",
        price: ENTERPRISE_YEARLY_FROM,
        priceTerms: "A year, starting price",
        showVat: true,
        details: [],
        audience: "For larger companies with security and procurement needs",
        includes: [
            "Everything in Organisation",
            "SAML and SCIM",
            "Invoicing, purchase orders and DPA terms",
            "An SLA of 99.5% a month, excluding announced maintenance",
        ],
        free: false,
    },
];

export function PricingPlans() {
    return (
        <div className={styles.pricing}>
            <ul className={styles.plans}>
                {PLANS.map(plan => (
                    <li key={plan.id} className={styles.plan} data-featured={plan.free}>
                        <h3 className={styles.name}>{plan.name}</h3>
                        <p className={styles.price}>{formatPrice(plan.price)}</p>
                        {plan.showVat && (
                            <p className={styles.vat}>{`${formatPrice(includingVat(plan.price))} inc. VAT`}</p>
                        )}
                        <p className={styles.billing}>{plan.priceTerms}</p>
                        {plan.details.map(detail => (
                            <p key={detail} className={styles.detail}>
                                {detail}
                            </p>
                        ))}
                        <p className={styles.audience}>{plan.audience}</p>
                        <ul className={styles.includes}>
                            {plan.includes.map(item => (
                                <li key={item}>{item}</li>
                            ))}
                        </ul>
                        {plan.free ? (
                            <ButtonLink to="/register" variant="primary" className={styles.action}>
                                Start for free
                            </ButtonLink>
                        ) : (
                            <p className={`${styles.action} ${styles.soon}`}>Coming soon</p>
                        )}
                    </li>
                ))}
            </ul>
            <p className={styles.note}>
                Paid plans are coming. Until then, every team is on Free. Prices exclude VAT for business customers,
                with the UK price including 20% VAT beside them. Guests who join a board by link are always free and
                don't count as members.
            </p>
        </div>
    );
}
